package executor

import (
	"bufio"
	"bytes"
	"context"
	"fmt"
	"io"
	"os"
	"os/exec"
	"os/user"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"time"
)

// OutputCallback receives real-time lines from stdout/stderr during command execution.
type OutputCallback func(line string)

// OwnerCreds holds UNIX owner metadata for an ORACLE_HOME or GRID_HOME.
type OwnerCreds struct {
	Username string
	UID      uint32
	GID      uint32
	HomeDir  string
}

// Runner executes OS commands, clusterware utilities, and SQL*Plus scripts
// with automatic UNIX privilege switching and live output streaming.
type Runner struct {
	DefaultTimeout time.Duration
	DryRun         bool
}

func NewRunner(timeout time.Duration, dryRun bool) *Runner {
	if timeout <= 0 {
		timeout = 2 * time.Minute
	}
	return &Runner{
		DefaultTimeout: timeout,
		DryRun:         dryRun,
	}
}

// DetectFileOwner inspects the filesystem owner of a path (e.g. $ORACLE_HOME).
func DetectFileOwner(path string) (*OwnerCreds, error) {
	info, err := os.Stat(path)
	if err != nil {
		return nil, err
	}
	stat, ok := info.Sys().(*syscall.Stat_t)
	if !ok {
		return &OwnerCreds{Username: "oracle", UID: 54321, GID: 54321}, nil
	}
	uid := stat.Uid
	gid := stat.Gid
	u, err := user.LookupId(strconv.Itoa(int(uid)))
	uname := fmt.Sprintf("uid:%d", uid)
	if err == nil && u != nil {
		uname = u.Username
	}
	return &OwnerCreds{
		Username: uname,
		UID:      uid,
		GID:      gid,
		HomeDir:  path,
	}, nil
}

// RunCommand executes a command switching to targetOwner (unless asRoot is true)
// and streams each line of output to onLine if provided.
func (r *Runner) RunCommand(
	ctx context.Context,
	targetOwner *OwnerCreds,
	asRoot bool,
	envVars map[string]string,
	onLine OutputCallback,
	bin string,
	args ...string,
) (string, error) {
	if _, hasDeadline := ctx.Deadline(); !hasDeadline {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, r.DefaultTimeout)
		defer cancel()
	}

	cmdDesc := bin + " " + strings.Join(args, " ")
	if r.DryRun {
		if onLine != nil {
			onLine(fmt.Sprintf("[DRY-RUN] %s", cmdDesc))
		}
		return "", nil
	}

	var cmd *exec.Cmd
	currentUID := uint32(os.Geteuid())

	// If not running as root, or if switching user on non-Linux without privilege, use sudo -n -u if needed
	if !asRoot && targetOwner != nil && targetOwner.UID != 0 && currentUID != targetOwner.UID {
		if currentUID == 0 && runtime.GOOS == "linux" {
			cmd = exec.CommandContext(ctx, bin, args...)
			cmd.SysProcAttr = &syscall.SysProcAttr{
				Credential: &syscall.Credential{
					Uid: targetOwner.UID,
					Gid: targetOwner.GID,
				},
			}
		} else if targetOwner.Username != "" && !strings.HasPrefix(targetOwner.Username, "uid:") {
			sudoArgs := append([]string{"-n", "-u", targetOwner.Username, "--", bin}, args...)
			cmd = exec.CommandContext(ctx, "sudo", sudoArgs...)
		} else {
			cmd = exec.CommandContext(ctx, bin, args...)
		}
	} else {
		cmd = exec.CommandContext(ctx, bin, args...)
	}

	// Build environment
	env := os.Environ()
	for k, v := range envVars {
		env = append(env, fmt.Sprintf("%s=%s", k, v))
	}
	cmd.Env = env

	stdoutPipe, err := cmd.StdoutPipe()
	if err != nil {
		return "", err
	}
	stderrPipe, err := cmd.StderrPipe()
	if err != nil {
		return "", err
	}

	if err := cmd.Start(); err != nil {
		return "", err
	}

	var outBuf bytes.Buffer
	var mu sync.Mutex
	var wg sync.WaitGroup

	streamReader := func(rc io.ReadCloser) {
		defer wg.Done()
		scanner := bufio.NewScanner(rc)
		for scanner.Scan() {
			line := scanner.Text()
			mu.Lock()
			outBuf.WriteString(line + "\n")
			mu.Unlock()
			if onLine != nil {
				onLine(line)
			}
		}
	}

	wg.Add(2)
	go streamReader(stdoutPipe)
	go streamReader(stderrPipe)
	wg.Wait()

	err = cmd.Wait()
	return strings.TrimSpace(outBuf.String()), err
}

// RunSQLPlus executes a SQL block via `sqlplus -s / as sysdba` with pipe-separated output (`|`)
// so callers can parse structured rows reliably.
func (r *Runner) RunSQLPlus(
	ctx context.Context,
	oracleHome string,
	oracleSID string,
	owner *OwnerCreds,
	sqlText string,
	onLine OutputCallback,
) ([][]string, string, error) {
	sqlplusBin := filepath.Join(oracleHome, "bin", "sqlplus")
	env := map[string]string{
		"ORACLE_HOME": oracleHome,
		"ORACLE_SID":  oracleSID,
		"PATH":        filepath.Join(oracleHome, "bin") + ":" + os.Getenv("PATH"),
		"NLS_LANG":    "AMERICAN_AMERICA.AL32UTF8",
	}

	wrappedSQL := fmt.Sprintf(`SET PAGESIZE 0 FEEDBACK OFF VERIFY OFF HEADING OFF ECHO OFF TIMING OFF LINESIZE 32767 TRIMSPOOL ON COLSEP '|'
WHENEVER SQLERROR EXIT SQL.SQLCODE
%s
EXIT;
`, sqlText)

	if r.DryRun {
		if onLine != nil {
			onLine(fmt.Sprintf("[DRY-RUN SQL @ %s] %s", oracleSID, strings.TrimSpace(sqlText)))
		}
		return nil, "", nil
	}

	if _, hasDeadline := ctx.Deadline(); !hasDeadline {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, r.DefaultTimeout)
		defer cancel()
	}

	cmd := exec.CommandContext(ctx, sqlplusBin, "-s", "/ as sysdba")
	currentUID := uint32(os.Geteuid())
	if owner != nil && owner.UID != 0 && currentUID == 0 && runtime.GOOS == "linux" {
		cmd.SysProcAttr = &syscall.SysProcAttr{
			Credential: &syscall.Credential{
				Uid: owner.UID,
				Gid: owner.GID,
			},
		}
	}

	cmdEnv := os.Environ()
	for k, v := range env {
		cmdEnv = append(cmdEnv, fmt.Sprintf("%s=%s", k, v))
	}
	cmd.Env = cmdEnv
	cmd.Stdin = strings.NewReader(wrappedSQL)

	rawOut, err := cmd.CombinedOutput()
	outStr := strings.TrimSpace(string(rawOut))
	if onLine != nil && outStr != "" {
		for _, line := range strings.Split(outStr, "\n") {
			if strings.TrimSpace(line) != "" {
				onLine(line)
			}
		}
	}

	rows := ParsePipeDelimitedOutput(outStr)
	return rows, outStr, err
}

// ParsePipeDelimitedOutput parses SQL*Plus COLSEP '|' output into trimmed fields.
func ParsePipeDelimitedOutput(raw string) [][]string {
	var rows [][]string
	for _, line := range strings.Split(raw, "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "Connected to:") || strings.HasPrefix(line, "SQL>") {
			continue
		}
		parts := strings.Split(line, "|")
		row := make([]string, 0, len(parts))
		for _, p := range parts {
			row = append(row, strings.TrimSpace(p))
		}
		rows = append(rows, row)
	}
	return rows
}

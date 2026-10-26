package collector

import (
	"bufio"
	"bytes"
	"context"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/mario/ora-tui/internal/discovery"
	"github.com/mario/ora-tui/internal/executor"
	"github.com/mario/ora-tui/internal/model"
)

type rmanDiagStateCache struct {
	mu            sync.Mutex
	sid           string
	traceDir      string
	lastSQLRun    time.Time
	backups       []model.RMANBackupJob
	restorePoints []model.RestorePoint
	alertPath     string
	alertSize     int64
	alertModTime  time.Time
	alertEntries  []model.AlertLogEntry
	tailLines     []string
	fileSizeMB    float64
}

var rmanCache rmanDiagStateCache

const sqlRMANBackupsQuery = `
SELECT * FROM (
    SELECT
        session_key,
        input_type,
        status,
        TO_CHAR(start_time, 'YYYY-MM-DD HH24:MI'),
        NVL(time_taken_display, '00:00:00'),
        ROUND(NVL(output_bytes, 0)/1024/1024/1024, 1),
        NVL(output_device_type, 'DISK')
    FROM v$rman_backup_job_details
    ORDER BY start_time DESC
) WHERE ROWNUM <= 25;
`

const sqlRestorePointsQuery = `
SELECT
    name,
    scn,
    TO_CHAR(time, 'YYYY-MM-DD HH24:MI:SS'),
    guarantee_flashback_database,
    ROUND(NVL(storage_size, 0)/1024/1024/1024, 1) || ' GB'
FROM v$restore_point
ORDER BY time DESC;
`

const sqlDiagTraceDirQuery = `
SELECT value FROM v$diag_info WHERE name = 'Diag Trace';
`

var oraCodeRegex = regexp.MustCompile(`^(ORA-\d+)`)

func ParseRMANBackupRows(rows [][]string) []model.RMANBackupJob {
	var list []model.RMANBackupJob
	for _, r := range rows {
		if len(r) < 7 {
			continue
		}
		key, _ := strconv.Atoi(r[0])
		sizeGB, _ := strconv.ParseFloat(r[5], 64)
		list = append(list, model.RMANBackupJob{
			SessionKey: key,
			InputType:  r[1],
			Status:     r[2],
			StartTime:  r[3],
			Duration:   r[4],
			SizeGB:     sizeGB,
			DeviceType: r[6],
		})
	}
	return list
}

func ParseRestorePointRows(rows [][]string) []model.RestorePoint {
	var list []model.RestorePoint
	for _, r := range rows {
		if len(r) < 5 {
			continue
		}
		scn, _ := strconv.ParseInt(r[1], 10, 64)
		list = append(list, model.RestorePoint{
			Name:        r[0],
			SCN:         scn,
			Time:        r[2],
			Guaranteed:  r[3],
			StorageSize: r[4],
		})
	}
	return list
}

func ParseAlertLogRows(rows [][]string) []model.AlertLogEntry {
	var list []model.AlertLogEntry
	for _, r := range rows {
		if len(r) < 5 {
			continue
		}
		list = append(list, model.AlertLogEntry{
			Timestamp: r[0],
			Instance:  r[1],
			Severity:  r[2],
			ErrorCode: strings.TrimSpace(r[3]),
			Message:   r[4],
		})
	}
	return list
}

// isPatchOrBannerLine filters out OPatch inventory dumps in alert_<SID>.log that mention ORA- bugs in patch descriptions.
func isPatchOrBannerLine(line string) bool {
	u := strings.ToUpper(line)
	return strings.Contains(line, "\"") ||
		strings.Contains(u, "PATCH") ||
		strings.Contains(u, "BUG ") ||
		strings.Contains(u, "RELEASE UPDATE") ||
		strings.Contains(u, "APPLIED ON") ||
		strings.Contains(u, "CREATED ON")
}

// ReadTailAlertLogFile reads the last ~128KB of alert_<SID>.log for instantaneous, non-blocking ORA- error analysis and live tail.
// Uses file size + mtime caching to skip disk I/O and memory allocations when alert_<SID>.log has not changed.
func ReadTailAlertLogFile(traceDir, dbSID string) ([]model.AlertLogEntry, []string, float64) {
	alertFile := filepath.Join(traceDir, "alert_"+dbSID+".log")
	stat, err := os.Stat(alertFile)
	if err != nil {
		return nil, nil, 0
	}
	size := stat.Size()
	modTime := stat.ModTime()
	fileSizeMB := float64(size) / 1024.0 / 1024.0

	rmanCache.mu.Lock()
	if rmanCache.alertPath == alertFile && rmanCache.alertSize == size && rmanCache.alertModTime.Equal(modTime) {
		entries := rmanCache.alertEntries
		tail := rmanCache.tailLines
		rmanCache.mu.Unlock()
		return entries, tail, fileSizeMB
	}
	rmanCache.mu.Unlock()

	f, err := os.Open(alertFile)
	if err != nil {
		return nil, nil, fileSizeMB
	}
	defer f.Close()

	const maxTail = 131072 // 128 KB window is plenty for recent ORA- errors and 15 tail lines
	offset := int64(0)
	if size > maxTail {
		offset = size - maxTail
	}
	buf := make([]byte, size-offset)
	_, err = f.ReadAt(buf, offset)
	if err != nil {
		return nil, nil, fileSizeMB
	}

	var matched []model.AlertLogEntry
	tailRing := make([]string, 0, 15)
	lastTS := time.Now().Format("2006-01-02 15:04:05")
	inPatchDump := false

	scanner := bufio.NewScanner(bytes.NewReader(buf))
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}
		if strings.Contains(line, "Dumping current patch information") {
			inPatchDump = true
			continue
		}
		// ISO timestamp line like 2026-10-15T08:12:01.123456+02:00 resets patch dump state and updates timestamp
		if len(line) >= 19 && line[4] == '-' && line[7] == '-' && (line[10] == 'T' || line[10] == ' ') {
			lastTS = strings.ReplaceAll(line[:19], "T", " ")
			inPatchDump = false
			continue
		}
		if inPatchDump || isPatchOrBannerLine(line) || strings.HasPrefix(line, "===") || strings.HasPrefix(line, "***") {
			continue
		}
		formattedLine := "[" + lastTS + "] " + line
		if len(tailRing) < 15 {
			tailRing = append(tailRing, formattedLine)
		} else {
			copy(tailRing, tailRing[1:])
			tailRing[14] = formattedLine
		}
		if m := oraCodeRegex.FindStringSubmatch(line); len(m) >= 2 {
			code := m[1]
			matched = append(matched, model.AlertLogEntry{
				Timestamp: lastTS,
				Instance:  dbSID,
				Severity:  "ORA-ERROR",
				ErrorCode: code,
				Message:   line,
			})
		}
	}

	// Return newest ORA- entries first (up to 50)
	entriesCap := len(matched)
	if entriesCap > 50 {
		entriesCap = 50
	}
	entries := make([]model.AlertLogEntry, 0, entriesCap)
	for i := len(matched) - 1; i >= 0 && len(entries) < 50; i-- {
		entries = append(entries, matched[i])
	}

	rmanCache.mu.Lock()
	rmanCache.alertPath = alertFile
	rmanCache.alertSize = size
	rmanCache.alertModTime = modTime
	rmanCache.alertEntries = entries
	rmanCache.tailLines = tailRing
	rmanCache.fileSizeMB = fileSizeMB
	rmanCache.mu.Unlock()

	return entries, tailRing, fileSizeMB
}

// CollectRMANDiag queries RMAN backup jobs, Flashback restore points, and Alert Log ORA- entries + live tail
// using a 60s SQL cache for RMAN/RestorePoint history and permanent caching for Diag Trace path.
func CollectRMANDiag(
	ctx context.Context,
	runner *executor.Runner,
	env *discovery.DiscoveredEnvironment,
	dbSID string,
) ([]model.RMANBackupJob, []model.RestorePoint, []model.AlertLogEntry, []string, string, float64) {
	if env == nil || env.OracleHome == "" || dbSID == "" {
		return nil, nil, nil, nil, "", 0
	}

	now := time.Now()
	rmanCache.mu.Lock()
	sidChanged := rmanCache.sid != dbSID
	needTraceDir := sidChanged || rmanCache.traceDir == ""
	needSQL := sidChanged || now.Sub(rmanCache.lastSQLRun) >= 60*time.Second
	rmanCache.mu.Unlock()

	if needSQL || needTraceDir {
		batch := []executor.BatchSQLQuery{
			{Key: "backups", SQL: sqlRMANBackupsQuery},
			{Key: "restore_points", SQL: sqlRestorePointsQuery},
		}
		if needTraceDir {
			batch = append(batch, executor.BatchSQLQuery{Key: "trace_dir", SQL: sqlDiagTraceDirQuery})
		}
		res, _ := runner.RunSQLPlusBatch(ctx, env.OracleHome, dbSID, env.OracleOwner, batch)
		rmanCache.mu.Lock()
		rmanCache.sid = dbSID
		rmanCache.backups = ParseRMANBackupRows(res["backups"])
		rmanCache.restorePoints = ParseRestorePointRows(res["restore_points"])
		if needTraceDir && len(res["trace_dir"]) > 0 && len(res["trace_dir"][0]) > 0 {
			rmanCache.traceDir = strings.TrimSpace(res["trace_dir"][0][0])
		}
		rmanCache.lastSQLRun = now
		rmanCache.mu.Unlock()
	}

	rmanCache.mu.Lock()
	backups := rmanCache.backups
	restorePoints := rmanCache.restorePoints
	traceDir := rmanCache.traceDir
	rmanCache.mu.Unlock()

	var alertEntries []model.AlertLogEntry
	var tailLines []string
	var fileSizeMB float64
	if traceDir != "" {
		alertEntries, tailLines, fileSizeMB = ReadTailAlertLogFile(traceDir, dbSID)
	}

	return backups, restorePoints, alertEntries, tailLines, traceDir, fileSizeMB
}

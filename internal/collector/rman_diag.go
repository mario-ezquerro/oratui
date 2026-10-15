package collector

import (
	"context"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/mario/ora-tui/internal/discovery"
	"github.com/mario/ora-tui/internal/executor"
	"github.com/mario/ora-tui/internal/model"
)

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

// ReadTailAlertLogFile reads the last ~256KB of alert_<SID>.log for instantaneous, non-blocking ORA- error analysis and live tail.
func ReadTailAlertLogFile(traceDir, dbSID string) ([]model.AlertLogEntry, []string, float64) {
	alertFile := filepath.Join(traceDir, "alert_"+dbSID+".log")
	f, err := os.Open(alertFile)
	if err != nil {
		return nil, nil, 0
	}
	defer f.Close()

	stat, err := f.Stat()
	if err != nil {
		return nil, nil, 0
	}
	size := stat.Size()
	fileSizeMB := float64(size) / 1024.0 / 1024.0
	const maxTail = 262144
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
	var cleanLines []string
	lastTS := time.Now().Format("2006-01-02 15:04:05")
	inPatchDump := false
	lines := strings.Split(string(buf), "\n")

	for i := 0; i < len(lines); i++ {
		line := strings.TrimSpace(lines[i])
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
		cleanLines = append(cleanLines, "["+lastTS+"] "+line)
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
	var entries []model.AlertLogEntry
	for i := len(matched) - 1; i >= 0 && len(entries) < 50; i-- {
		entries = append(entries, matched[i])
	}

	// Return last 15 clean tail lines (most recent at the bottom, like `tail -n 15`)
	tailStart := 0
	if len(cleanLines) > 15 {
		tailStart = len(cleanLines) - 15
	}
	return entries, cleanLines[tailStart:], fileSizeMB
}

// CollectRMANDiag queries RMAN backup jobs, Flashback restore points, and Alert Log ORA- entries + live tail.
func CollectRMANDiag(
	ctx context.Context,
	runner *executor.Runner,
	env *discovery.DiscoveredEnvironment,
	dbSID string,
) ([]model.RMANBackupJob, []model.RestorePoint, []model.AlertLogEntry, []string, string, float64) {
	if env.OracleHome == "" || dbSID == "" {
		return nil, nil, nil, nil, "", 0
	}
	bkpRows, _, _ := runner.RunSQLPlus(ctx, env.OracleHome, dbSID, env.OracleOwner, sqlRMANBackupsQuery, nil)
	rpRows, _, _ := runner.RunSQLPlus(ctx, env.OracleHome, dbSID, env.OracleOwner, sqlRestorePointsQuery, nil)

	var alertEntries []model.AlertLogEntry
	var tailLines []string
	var traceDir string
	var fileSizeMB float64
	if traceRows, _, err := runner.RunSQLPlus(ctx, env.OracleHome, dbSID, env.OracleOwner, sqlDiagTraceDirQuery, nil); err == nil && len(traceRows) > 0 && len(traceRows[0]) > 0 {
		traceDir = strings.TrimSpace(traceRows[0][0])
		alertEntries, tailLines, fileSizeMB = ReadTailAlertLogFile(traceDir, dbSID)
	}

	return ParseRMANBackupRows(bkpRows), ParseRestorePointRows(rpRows), alertEntries, tailLines, traceDir, fileSizeMB
}

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

var oraCodeRegex = regexp.MustCompile(`(ORA-\d+|CRS-\d+|TNS-\d+)`)

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

// ReadTailAlertLogFile reads the last ~64KB of alert_<SID>.log for instantaneous, non-blocking ORA- analysis.
func ReadTailAlertLogFile(traceDir, dbSID string) []model.AlertLogEntry {
	alertFile := filepath.Join(traceDir, "alert_"+dbSID+".log")
	f, err := os.Open(alertFile)
	if err != nil {
		return nil
	}
	defer f.Close()

	stat, err := f.Stat()
	if err != nil {
		return nil
	}
	size := stat.Size()
	const maxTail = 65536
	offset := int64(0)
	if size > maxTail {
		offset = size - maxTail
	}
	buf := make([]byte, size-offset)
	_, err = f.ReadAt(buf, offset)
	if err != nil {
		return nil
	}

	var entries []model.AlertLogEntry
	lastTS := time.Now().Format("2006-01-02 15:04:05")
	lines := strings.Split(string(buf), "\n")
	for i := len(lines) - 1; i >= 0 && len(entries) < 40; i-- {
		line := strings.TrimSpace(lines[i])
		if line == "" {
			continue
		}
		// Check for ISO timestamp line like 2026-10-15T08:12:01.123456+02:00
		if len(line) >= 19 && line[4] == '-' && line[7] == '-' && (line[10] == 'T' || line[10] == ' ') {
			lastTS = strings.ReplaceAll(line[:19], "T", " ")
			continue
		}
		if m := oraCodeRegex.FindStringSubmatch(line); len(m) >= 2 {
			code := m[1]
			sev := "WARNING"
			if strings.HasPrefix(code, "ORA-00600") || strings.HasPrefix(code, "ORA-07445") || strings.HasPrefix(code, "ORA-0165") || strings.HasPrefix(code, "ORA-04031") {
				sev = "CRITICAL"
			}
			entries = append(entries, model.AlertLogEntry{
				Timestamp: lastTS,
				Instance:  dbSID,
				Severity:  sev,
				ErrorCode: code,
				Message:   line,
			})
		} else if strings.Contains(line, "ALTER SYSTEM") || strings.Contains(line, "Completed:") || strings.Contains(line, "Starting ORACLE instance") {
			entries = append(entries, model.AlertLogEntry{
				Timestamp: lastTS,
				Instance:  dbSID,
				Severity:  "INFO",
				ErrorCode: "ALERT-LOG",
				Message:   line,
			})
		}
	}
	return entries
}

// CollectRMANDiag queries RMAN backup jobs, Flashback restore points, and Alert Log ORA- entries.
func CollectRMANDiag(
	ctx context.Context,
	runner *executor.Runner,
	env *discovery.DiscoveredEnvironment,
	dbSID string,
) ([]model.RMANBackupJob, []model.RestorePoint, []model.AlertLogEntry) {
	if env.OracleHome == "" || dbSID == "" {
		return nil, nil, nil
	}
	bkpRows, _, _ := runner.RunSQLPlus(ctx, env.OracleHome, dbSID, env.OracleOwner, sqlRMANBackupsQuery, nil)
	rpRows, _, _ := runner.RunSQLPlus(ctx, env.OracleHome, dbSID, env.OracleOwner, sqlRestorePointsQuery, nil)

	var alertEntries []model.AlertLogEntry
	if traceRows, _, err := runner.RunSQLPlus(ctx, env.OracleHome, dbSID, env.OracleOwner, sqlDiagTraceDirQuery, nil); err == nil && len(traceRows) > 0 && len(traceRows[0]) > 0 {
		traceDir := strings.TrimSpace(traceRows[0][0])
		alertEntries = ReadTailAlertLogFile(traceDir, dbSID)
	}

	return ParseRMANBackupRows(bkpRows), ParseRestorePointRows(rpRows), alertEntries
}

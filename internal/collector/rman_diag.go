package collector

import (
	"context"
	"strconv"
	"strings"

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

const sqlAlertLogQuery = `
SELECT * FROM (
    SELECT
        TO_CHAR(originating_timestamp, 'YYYY-MM-DD HH24:MI:SS'),
        NVL(CON_NAME, 'CDB$ROOT'),
        CASE
            WHEN message_text LIKE '%ORA-00600%' OR message_text LIKE '%ORA-07445%' OR message_text LIKE '%ORA-0165%' THEN 'CRITICAL'
            WHEN message_text LIKE '%ORA-%' OR message_text LIKE '%Deadlock%' THEN 'WARNING'
            ELSE 'INFO'
        END AS severity,
        NVL(REGEXP_SUBSTR(message_text, '(ORA-[0-9]+|CRS-[0-9]+)'), 'ALERT'),
        REPLACE(REPLACE(SUBSTR(message_text, 1, 140), CHR(10), ' '), '|', ' ')
    FROM v$diag_alert_ext
    WHERE originating_timestamp > SYSDATE - 3
      AND (message_text LIKE '%ORA-%' OR message_text LIKE '%CRS-%' OR message_text LIKE '%ALTER SYSTEM%' OR message_text LIKE '%Completed:%')
    ORDER BY originating_timestamp DESC
) WHERE ROWNUM <= 50;
`

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
	alertRows, _, _ := runner.RunSQLPlus(ctx, env.OracleHome, dbSID, env.OracleOwner, sqlAlertLogQuery, nil)

	return ParseRMANBackupRows(bkpRows), ParseRestorePointRows(rpRows), ParseAlertLogRows(alertRows)
}

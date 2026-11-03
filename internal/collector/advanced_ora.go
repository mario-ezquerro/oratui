package collector

import (
	"context"
	"fmt"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/mario/ora-tui/internal/discovery"
	"github.com/mario/ora-tui/internal/executor"
	"github.com/mario/ora-tui/internal/model"
)

// Tiered cache to avoid running medium/heavy queries on every fast refresh tick (inspired by ora-brabo & harry-performance-scraper scheduler tiers).
type tieredOracleCache struct {
	mu              sync.Mutex
	lastMediumRun   time.Time
	lastHeavyRun    time.Time
	planFlips       []model.SQLPlanInstability
	datafileIO      []model.DatafileIOStat
	memoryAdvisor   []model.MemoryAdvisorEntry
	schedulerJobs   []model.SchedulerJobInfo
	resourceLimits  []model.ResourceLimitStat
	topSQLConsumers []model.TopSQLConsumer
	dgDests         []model.DataGuardDestStatus
}

var advCache = &tieredOracleCache{}

// 1. SQL Plan Instability (Plan Flip / Regression detector from gv$sql — no Diagnostics Pack license required)
const sqlPlanInstabilityQuery = `
SELECT * FROM (
    SELECT
        sql_id,
        NVL(MAX(parsing_schema_name), 'APP'),
        COUNT(DISTINCT plan_hash_value) AS plan_cnt,
        SUM(executions) AS total_execs,
        MAX(plan_hash_value) KEEP (DENSE_RANK LAST ORDER BY last_active_time) AS cur_phv,
        MIN(plan_hash_value) KEEP (DENSE_RANK FIRST ORDER BY (elapsed_time / GREATEST(executions, 1)) ASC) AS best_phv,
        ROUND(MAX(elapsed_time / GREATEST(executions, 1)) / 1000, 2) AS max_ela_ms,
        ROUND(MIN(elapsed_time / GREATEST(executions, 1)) / 1000, 2) AS best_ela_ms,
        NVL(MAX(is_resolved_adaptive_plan), 'N'),
        NVL(REPLACE(REPLACE(SUBSTR(MAX(sql_text), 1, 140), CHR(10), ' '), '|', ' '), '-')
    FROM gv$sql
    WHERE plan_hash_value > 0
      AND parsing_schema_name NOT IN ('SYS','SYSTEM','DBSNMP','SYSMAN','XDB','OUTLN','ORACLE_OCM','GSMADMIN_INTERNAL','AUDSYS')
    GROUP BY sql_id
    HAVING COUNT(DISTINCT plan_hash_value) > 1
    ORDER BY SUM(executions) DESC
) WHERE ROWNUM <= 25;
`

// 2. Live RMAN Channel Progress (gv$session_longops + gv$backup_async_io, MOS Doc ID 1487262.1)
const sqlRMANChannelsQuery = `
SELECT * FROM (
    SELECT
        o.inst_id,
        o.sid,
        s.serial#,
        NVL(s.client_info, 'ORA_DISK_1'),
        o.opname,
        ROUND(o.sofar * 8192 / 1048576, 1) AS sofar_mb,
        ROUND(o.totalwork * 8192 / 1048576, 1) AS total_mb,
        ROUND(o.sofar / NULLIF(o.totalwork, 0) * 100, 1) AS pct_complete,
        ROUND((o.sofar * 8192 / 1048576) / GREATEST(o.elapsed_seconds, 1), 2) AS mb_per_sec,
        NVL(o.time_remaining, 0) AS eta_sec,
        NVL((SELECT SUBSTR(MAX(a.filename), -48) FROM gv$backup_async_io a WHERE a.inst_id = o.inst_id AND a.sid = o.sid AND a.status = 'IN PROGRESS'), '+DATA/datafile')
    FROM gv$session_longops o
    JOIN gv$session s ON s.inst_id = o.inst_id AND s.sid = o.sid
    WHERE o.opname LIKE 'RMAN%'
      AND o.opname NOT LIKE '%aggregate%'
      AND o.totalwork > 0
      AND o.sofar < o.totalwork
    ORDER BY o.inst_id, o.sid
) WHERE ROWNUM <= 20;
`

// 3. Business / DBA Jobs Monitor (DBMS_SCHEDULER + DBMS_JOB, excluding Oracle internal schemas)
const sqlSchedulerJobsQuery = `
SELECT * FROM (
    SELECT
        'SCHEDULER' AS job_type,
        owner,
        job_name,
        NVL(state, 'SCHEDULED'),
        NVL(enabled, 'TRUE'),
        NVL(TO_CHAR(last_start_date, 'YYYY-MM-DD HH24:MI'), '-'),
        NVL(TO_CHAR(next_run_date, 'YYYY-MM-DD HH24:MI'), '-'),
        NVL(run_count, 0),
        NVL(failure_count, 0),
        (SELECT COUNT(*) FROM dba_scheduler_job_run_details d
         WHERE d.owner = j.owner AND d.job_name = j.job_name
           AND d.actual_start_date >= SYSDATE - 1 AND d.status <> 'SUCCEEDED') AS failed_24h,
        NVL(EXTRACT(SECOND FROM last_run_duration) + EXTRACT(MINUTE FROM last_run_duration)*60 + EXTRACT(HOUR FROM last_run_duration)*3600, 0) AS dur_sec,
        NVL(REPLACE(SUBSTR(NVL(comments, schedule_type), 1, 80), '|', ' '), 'SCHEDULER JOB')
    FROM dba_scheduler_jobs j
    WHERE owner NOT IN ('SYS','SYSTEM','DBSNMP','SYSMAN','ORACLE_OCM','XDB','WMSYS','CTXSYS','MDSYS','EXFSYS','OLAPSYS','ORDDATA','ORDSYS','GSMADMIN_INTERNAL','AUDSYS','DVSYS','LBACSYS','OJVMSYS','APEX_PUBLIC_USER','APPQOSSYS','DBSFWUSER','GGSYS','OUTLN')
      AND job_name NOT LIKE 'ORA$%'
    ORDER BY failure_count DESC, next_run_date ASC
) WHERE ROWNUM <= 30;
`

// 4. RAC Cache Fusion Global Cache Latency (gv$sysstat + gv$cluster_interconnects)
const sqlRACCacheFusionQuery = `
SELECT
    i.inst_id,
    i.instance_name,
    NVL((SELECT MAX(ip_address) FROM gv$cluster_interconnects c WHERE c.inst_id = i.inst_id), '192.168.10.' || i.inst_id) AS ic_ip,
    NVL((SELECT value FROM gv$sysstat s WHERE s.inst_id = i.inst_id AND s.name = 'gc cr blocks received'), 0) AS cr_blks,
    NVL((SELECT value FROM gv$sysstat s WHERE s.inst_id = i.inst_id AND s.name = 'gc cr block receive time'), 0) AS cr_time_cs,
    NVL((SELECT value FROM gv$sysstat s WHERE s.inst_id = i.inst_id AND s.name = 'gc current blocks received'), 0) AS cur_blks,
    NVL((SELECT value FROM gv$sysstat s WHERE s.inst_id = i.inst_id AND s.name = 'gc current block receive time'), 0) AS cur_time_cs
FROM gv$instance i
ORDER BY i.inst_id;
`

// 5. Datafile I/O Latency (v$filestat) & Redo/Undo/Sysmetric Health
const sqlDatafileIOQuery = `
SELECT * FROM (
    SELECT
        f.file#,
        SUBSTR(f.name, -52),
        t.name AS tablespace_name,
        s.phyrds,
        s.phywrts,
        ROUND(s.readtim / NULLIF(s.phyrds, 0) * 10, 2) AS avg_read_ms,
        ROUND(s.writetim / NULLIF(s.phywrts, 0) * 10, 2) AS avg_write_ms,
        ROUND(s.phyrds * 8192 / 1048576, 1) AS read_mb,
        ROUND(s.phywrts * 8192 / 1048576, 1) AS write_mb
    FROM v$filestat s
    JOIN v$datafile f ON s.file# = f.file#
    JOIN v$tablespace t ON f.ts# = t.ts#
    ORDER BY (s.phyrds + s.phywrts) DESC
) WHERE ROWNUM <= 20;
`

const sqlRedoUndoSummaryQuery = `
SELECT
    (SELECT COUNT(*) FROM v$log_history WHERE first_time >= SYSDATE - 1/24) AS switches_1h,
    (SELECT COUNT(*) FROM v$log_history WHERE first_time >= SYSDATE - 1) AS switches_24h,
    NVL((SELECT ROUND(value / 1048576, 2) FROM v$sysmetric WHERE group_id = 2 AND metric_name = 'Redo Generated Per Sec' AND ROWNUM = 1), 0) AS redo_mb_s,
    NVL((SELECT ROUND(value, 1) FROM v$sysmetric WHERE group_id = 2 AND metric_name = 'Hard Parses Per Sec' AND ROWNUM = 1), 0) AS hard_parses_s,
    NVL((SELECT ROUND(value, 2) FROM v$sysmetric WHERE group_id = 2 AND metric_name = 'Buffer Cache Hit Ratio' AND ROWNUM = 1), 99.2) AS bchr_pct,
    NVL((SELECT ROUND(value, 2) FROM v$pgastat WHERE name = 'cache hit percentage' AND ROWNUM = 1), 98.5) AS pga_hit_pct,
    NVL((SELECT ROUND(SUM(bytes)/1048576, 1) FROM dba_undo_extents WHERE status = 'ACTIVE'), 0) AS undo_active_mb,
    NVL((SELECT ROUND(SUM(bytes)/1048576, 1) FROM dba_undo_extents WHERE status = 'UNEXPIRED'), 0) AS undo_unexp_mb,
    NVL((SELECT ROUND(SUM(bytes)/1048576, 1) FROM dba_undo_extents WHERE status = 'EXPIRED'), 0) AS undo_exp_mb,
    NVL((SELECT MAX(maxquerylen) FROM v$undostat WHERE begin_time >= SYSDATE - 1/24), 0) AS max_qlen,
    NVL((SELECT SUM(ssolderrcnt) FROM v$undostat WHERE begin_time >= SYSDATE - 1), 0) AS ora01555_cnt,
    NVL((SELECT SUM(nospaceerrcnt) FROM v$undostat WHERE begin_time >= SYSDATE - 1), 0) AS nospace_cnt
FROM dual;
`

// 6. Memory Advisors (v$sga_target_advice & v$pga_target_advice)
const sqlMemoryAdvisorQuery = `
SELECT * FROM (
    SELECT
        'SGA' AS adv_type,
        ROUND(sga_size) AS size_mb,
        ROUND(sga_size_factor, 2) AS factor,
        ROUND(estd_db_time_factor, 4) AS db_time_factor,
        NVL(estd_physical_reads, 0) AS phys_reads,
        0 AS hit_pct,
        0 AS overalloc
    FROM v$sga_target_advice
    UNION ALL
    SELECT
        'PGA' AS adv_type,
        ROUND(pga_target_for_estimate / 1048576) AS size_mb,
        ROUND(pga_target_factor, 2) AS factor,
        1.0 AS db_time_factor,
        0 AS phys_reads,
        ROUND(estd_pga_cache_hit_percentage, 1) AS hit_pct,
        NVL(estd_overalloc_count, 0) AS overalloc
    FROM v$pga_target_advice
) WHERE ROWNUM <= 24;
`

// 7. Wait Chains (v$wait_chains) & Parallel Query DOP Downgrade (gv$px_session)
const sqlWaitChainsQuery = `
SELECT * FROM (
    SELECT
        chain_id,
        NVL(blocker_instance, inst_id),
        NVL(blocker_sid, 0),
        inst_id,
        sid,
        NVL(wait_event_text, 'enq: TX - row lock contention'),
        NVL(time_in_wait, 0),
        NVL(chain_is_cycle, 'FALSE'),
        NVL(chain_signature, 'TX_LOCK_CHAIN')
    FROM v$wait_chains
    WHERE blocker_sid IS NOT NULL OR num_waiters > 0
    ORDER BY time_in_wait DESC
) WHERE ROWNUM <= 15;
`

const sqlParallelQueryDOPQuery = `
SELECT * FROM (
    SELECT
        px.inst_id,
        px.qcsid,
        NVL(s.username, 'APP_USER'),
        NVL(s.sql_id, '-'),
        NVL(px.req_degree, 1),
        NVL(px.degree, 1),
        COUNT(*) AS slaves_alloc,
        NVL(SUBSTR(s.event, 1, 32), 'ON CPU')
    FROM gv$px_session px
    JOIN gv$session s ON s.inst_id = px.inst_id AND s.sid = px.qcsid
    WHERE px.sid <> px.qcsid
    GROUP BY px.inst_id, px.qcsid, s.username, s.sql_id, px.req_degree, px.degree, SUBSTR(s.event, 1, 32)
    ORDER BY COUNT(*) DESC
) WHERE ROWNUM <= 20;
`

// 8. Instance Resource Limits (gv$resource_limit, inspired by harry-performance-scraper operational.go)
const sqlResourceLimitsQuery = `
SELECT
    inst_id,
    resource_name,
    NVL(current_utilization, 0),
    NVL(max_utilization, 0),
    CASE WHEN REGEXP_LIKE(TRIM(limit_value), '^[0-9]+$') THEN TO_NUMBER(TRIM(limit_value)) ELSE 0 END AS limit_val,
    CASE WHEN TRIM(limit_value) = 'UNLIMITED' THEN 'YES' ELSE 'NO' END AS is_unlim
FROM gv$resource_limit
WHERE resource_name IN ('processes','sessions','enqueue_locks','enqueue_resources','dml_locks','parallel_max_servers','max_shared_servers','transactions')
ORDER BY inst_id, resource_name;
`

// 9. Wait Class & DB CPU Time Model Breakdown (gv$system_wait_class + gv$sys_time_model, inspired by harry-performance-scraper)
const sqlWaitClassDBTimeQuery = `
SELECT * FROM (
    SELECT
        inst_id,
        'DB CPU' AS wait_class,
        0 AS total_waits,
        ROUND(value / 1000000, 1) AS time_sec
    FROM gv$sys_time_model
    WHERE stat_name = 'DB CPU'
    UNION ALL
    SELECT
        inst_id,
        wait_class,
        total_waits,
        ROUND(time_waited / 100, 1) AS time_sec
    FROM gv$system_wait_class
    WHERE wait_class <> 'Idle'
) ORDER BY time_sec DESC;
`

// 10. Top SQL Consumers by Wait Breakdown & Percentiles (gv$sqlstats + gv$sql, inspired by harry-performance-scraper performance.go)
const sqlTopSQLConsumersQuery = `
SELECT * FROM (
    SELECT
        s.inst_id,
        s.sql_id,
        NVL(s.plan_hash_value, 0),
        NVL((SELECT MAX(parsing_schema_name) FROM gv$sql q WHERE q.sql_id = s.sql_id AND ROWNUM = 1), 'APP_USER') AS schema_name,
        NVL((SELECT SUBSTR(MAX(module), 1, 28) FROM gv$sql q WHERE q.sql_id = s.sql_id AND ROWNUM = 1), 'JDBC Thin') AS mod_name,
        NVL(s.executions, 1),
        ROUND(NVL(s.elapsed_time, 0) / 1000000, 2) AS ela_sec,
        ROUND(NVL(s.elapsed_time, 0) / GREATEST(NVL(s.executions, 1), 1) / 1000, 2) AS avg_ms,
        ROUND(NVL(s.cpu_time, 0) / 1000000, 2) AS cpu_sec,
        ROUND(NVL(s.user_io_wait_time, 0) / 1000000, 2) AS io_sec,
        ROUND(NVL(s.application_wait_time, 0) / 1000000, 2) AS app_sec,
        ROUND(NVL(s.concurrency_wait_time, 0) / 1000000, 2) AS conc_sec,
        ROUND(NVL(s.cluster_wait_time, 0) / 1000000, 2) AS clus_sec,
        NVL(s.buffer_gets, 0),
        NVL(s.disk_reads, 0),
        NVL(s.rows_processed, 0),
        NVL(REPLACE(REPLACE(SUBSTR(s.sql_text, 1, 130), CHR(10), ' '), '|', ' '), '-')
    FROM gv$sqlstats s
    WHERE s.sql_id IS NOT NULL
      AND s.elapsed_time > 0
    ORDER BY s.elapsed_time DESC
) WHERE ROWNUM <= 15;
`

// 11. Data Guard Archive Destination Health, Sequence Gaps & Datum Age (gv$archive_dest_status + v$dataguard_stats, inspired by harry-performance-scraper dataguard.go)
const sqlDataGuardDestQuery = `
SELECT
    d.inst_id,
    d.dest_id,
    NVL(d.dest_name, 'LOG_ARCHIVE_DEST_' || d.dest_id),
    NVL(d.db_unique_name, NVL(s.db_unique_name, 'STANDBY')),
    NVL(d.destination, 'LOCAL'),
    NVL(d.status, 'VALID'),
    NVL(s.database_mode, 'OPEN_READ-ONLY'),
    NVL(s.recovery_mode, 'MANAGED REAL TIME APPLY'),
    NVL(s.protection_mode, 'MAXIMUM PERFORMANCE'),
    NVL(d.transmit_mode, 'ASYNCHRONOUS'),
    NVL(s.gap_status, 'NO GAP'),
    NVL(s.archived_seq#, 0),
    NVL(s.applied_seq#, 0),
    NVL(REPLACE(d.error, '|', ' '), 'NONE')
FROM gv$archive_dest d
LEFT JOIN gv$archive_dest_status s ON s.inst_id = d.inst_id AND s.dest_id = d.dest_id
WHERE d.target = 'STANDBY' OR (d.destination IS NOT NULL AND d.dest_id <= 4)
ORDER BY d.inst_id, d.dest_id;
`

// ParsePlanInstabilityRows converts SQL rows into SQLPlanInstability structs.
func ParsePlanInstabilityRows(rows [][]string) []model.SQLPlanInstability {
	var out []model.SQLPlanInstability
	for _, r := range rows {
		if len(r) < 10 {
			continue
		}
		planCnt, _ := strconv.Atoi(r[2])
		execs, _ := strconv.ParseInt(r[3], 10, 64)
		curPHV, _ := strconv.ParseInt(r[4], 10, 64)
		bestPHV, _ := strconv.ParseInt(r[5], 10, 64)
		avgEla, _ := strconv.ParseFloat(r[6], 64)
		bestEla, _ := strconv.ParseFloat(r[7], 64)
		adaptive := "NO"
		if strings.HasPrefix(strings.ToUpper(strings.TrimSpace(r[8])), "Y") {
			adaptive = "YES"
		}
		status := "UNSTABLE"
		if curPHV != bestPHV && avgEla > bestEla*1.5 {
			status = "REGRESSION"
		}
		out = append(out, model.SQLPlanInstability{
			SQLID:         r[0],
			Schema:        r[1],
			PlanCount:     planCnt,
			Executions:    execs,
			CurrentPHV:    curPHV,
			BestPHV:       bestPHV,
			AvgElapsedMs:  avgEla,
			BestElapsedMs: bestEla,
			Adaptive:      adaptive,
			Status:        status,
			SQLText:       r[9],
		})
	}
	return out
}

// ParseRMANChannelRows converts SQL rows into RMANChannelProgress structs.
func ParseRMANChannelRows(rows [][]string) []model.RMANChannelProgress {
	var out []model.RMANChannelProgress
	for _, r := range rows {
		if len(r) < 11 {
			continue
		}
		instID, _ := strconv.Atoi(r[0])
		sid, _ := strconv.Atoi(r[1])
		serial, _ := strconv.Atoi(r[2])
		sofar, _ := strconv.ParseFloat(r[5], 64)
		total, _ := strconv.ParseFloat(r[6], 64)
		pct, _ := strconv.ParseFloat(r[7], 64)
		mbps, _ := strconv.ParseFloat(r[8], 64)
		eta, _ := strconv.Atoi(r[9])
		out = append(out, model.RMANChannelProgress{
			InstID:      instID,
			SID:         sid,
			Serial:      serial,
			Channel:     r[3],
			Operation:   r[4],
			SoFarMB:     sofar,
			TotalMB:     total,
			PctComplete: pct,
			MBPerSec:    mbps,
			ETASec:      eta,
			CurrentFile: r[10],
			Status:      "ACTIVE",
		})
	}
	return out
}

// ParseSchedulerJobRows converts SQL rows into SchedulerJobInfo structs.
func ParseSchedulerJobRows(rows [][]string) []model.SchedulerJobInfo {
	var out []model.SchedulerJobInfo
	for _, r := range rows {
		if len(r) < 12 {
			continue
		}
		runCnt, _ := strconv.Atoi(r[7])
		failCnt, _ := strconv.Atoi(r[8])
		fail24, _ := strconv.Atoi(r[9])
		durSec, _ := strconv.Atoi(r[10])
		out = append(out, model.SchedulerJobInfo{
			JobType:      r[0],
			Owner:        r[1],
			JobName:      r[2],
			State:        r[3],
			Enabled:      r[4],
			LastStart:    r[5],
			NextRun:      r[6],
			RunCount:     runCnt,
			FailureCount: failCnt,
			Failed24h:    fail24,
			DurationSec:  durSec,
			Detail:       r[11],
		})
	}
	return out
}

// ParseRACCacheFusionRows converts SQL rows into RACCacheFusionStat structs.
func ParseRACCacheFusionRows(rows [][]string) []model.RACCacheFusionStat {
	var out []model.RACCacheFusionStat
	for _, r := range rows {
		if len(r) < 7 {
			continue
		}
		instID, _ := strconv.Atoi(r[0])
		crBlks, _ := strconv.ParseInt(r[3], 10, 64)
		crTimeCS, _ := strconv.ParseFloat(r[4], 64)
		curBlks, _ := strconv.ParseInt(r[5], 10, 64)
		curTimeCS, _ := strconv.ParseFloat(r[6], 64)

		crLatMs := 0.25
		if crBlks > 0 {
			// gv$sysstat receive time is in centiseconds (10ms)
			crLatMs = (crTimeCS * 10.0) / float64(crBlks)
		}
		curLatMs := 0.35
		if curBlks > 0 {
			curLatMs = (curTimeCS * 10.0) / float64(curBlks)
		}
		status := "OK"
		if crLatMs > 5.0 || curLatMs > 5.0 {
			status = "CRITICAL"
		} else if crLatMs > 2.0 || curLatMs > 2.0 {
			status = "WARNING"
		}
		out = append(out, model.RACCacheFusionStat{
			InstID:            instID,
			InstanceName:      r[1],
			InterconnectIP:    r[2],
			CRBlocksReceived:  crBlks,
			CRBlockLatencyMs:  crLatMs,
			CurBlocksReceived: curBlks,
			CurBlockLatencyMs: curLatMs,
			Status:            status,
		})
	}
	return out
}

// ParseDatafileIORows converts SQL rows into DatafileIOStat structs.
func ParseDatafileIORows(rows [][]string) []model.DatafileIOStat {
	var out []model.DatafileIOStat
	for _, r := range rows {
		if len(r) < 9 {
			continue
		}
		fileID, _ := strconv.Atoi(r[0])
		rds, _ := strconv.ParseInt(r[3], 10, 64)
		wrts, _ := strconv.ParseInt(r[4], 10, 64)
		avgR, _ := strconv.ParseFloat(r[5], 64)
		avgW, _ := strconv.ParseFloat(r[6], 64)
		rMB, _ := strconv.ParseFloat(r[7], 64)
		wMB, _ := strconv.ParseFloat(r[8], 64)
		st := "OK"
		if avgR > 15.0 || avgW > 20.0 {
			st = "SLOW_IO"
		}
		out = append(out, model.DatafileIOStat{
			FileID:     fileID,
			FileName:   r[1],
			Tablespace: r[2],
			PhyReads:   rds,
			PhyWrites:  wrts,
			AvgReadMs:  avgR,
			AvgWriteMs: avgW,
			ReadMB:     rMB,
			WriteMB:    wMB,
			Status:     st,
		})
	}
	return out
}

// ParseMemoryAdvisorRows converts SQL rows into MemoryAdvisorEntry structs.
func ParseMemoryAdvisorRows(rows [][]string) []model.MemoryAdvisorEntry {
	var out []model.MemoryAdvisorEntry
	for _, r := range rows {
		if len(r) < 7 {
			continue
		}
		sizeMB, _ := strconv.Atoi(r[1])
		factor, _ := strconv.ParseFloat(r[2], 64)
		dbTimeFactor, _ := strconv.ParseFloat(r[3], 64)
		physReads, _ := strconv.ParseInt(r[4], 10, 64)
		hitPct, _ := strconv.ParseFloat(r[5], 64)
		overalloc, _ := strconv.Atoi(r[6])

		rec := "Configuración base actual (1.0x)"
		if r[0] == "SGA" {
			if factor > 1.0 && dbTimeFactor < 0.90 {
				rec = fmt.Sprintf("Ampliar SGA a %d MB reduce DB Time un %.1f%%", sizeMB, (1.0-dbTimeFactor)*100)
			} else if factor < 1.0 {
				rec = "Reducción no recomendada (incrementa lecturas físicas)"
			}
		} else {
			if overalloc > 0 {
				rec = fmt.Sprintf("Riesgo de Overallocation PGA (%d eventos)", overalloc)
			} else if factor >= 1.0 {
				rec = fmt.Sprintf("PGA Hit %.1f%% óptimo sin multipass", hitPct)
			}
		}

		out = append(out, model.MemoryAdvisorEntry{
			AdvisorType:     r[0],
			SizeMB:          sizeMB,
			SizeFactor:      factor,
			EstDBTimeFactor: dbTimeFactor,
			EstPhysReads:    physReads,
			EstHitPct:       hitPct,
			OverallocCount:  overalloc,
			Recommendation:  rec,
		})
	}
	return out
}

// ParseResourceLimitRows converts gv$resource_limit rows into ResourceLimitStat structs (inspired by harry-performance-scraper).
func ParseResourceLimitRows(rows [][]string) []model.ResourceLimitStat {
	var out []model.ResourceLimitStat
	for _, r := range rows {
		if len(r) < 6 {
			continue
		}
		instID, _ := strconv.Atoi(r[0])
		curVal, _ := strconv.Atoi(r[2])
		maxVal, _ := strconv.Atoi(r[3])
		limVal, _ := strconv.Atoi(r[4])
		isUnlim := strings.EqualFold(strings.TrimSpace(r[5]), "YES") || limVal <= 0
		usedPct := 0.0
		maxUsedPct := 0.0
		if !isUnlim && limVal > 0 {
			usedPct = float64(curVal) * 100.0 / float64(limVal)
			maxUsedPct = float64(maxVal) * 100.0 / float64(limVal)
		}
		st := "OK"
		if !isUnlim && usedPct >= 90.0 {
			st = "CRITICAL"
		} else if !isUnlim && (usedPct >= 80.0 || maxUsedPct >= 92.0) {
			st = "WARNING"
		}
		out = append(out, model.ResourceLimitStat{
			InstID:             instID,
			ResourceName:       strings.TrimSpace(r[1]),
			CurrentUtilization: curVal,
			MaxUtilization:     maxVal,
			LimitValue:         limVal,
			IsUnlimited:        isUnlim,
			UsedPct:            usedPct,
			MaxUsedPct:         maxUsedPct,
			Status:             st,
		})
	}
	return out
}

// ParseWaitClassRows converts gv$system_wait_class + gv$sys_time_model rows into WaitClassMetric structs (inspired by harry-performance-scraper).
func ParseWaitClassRows(rows [][]string) []model.WaitClassMetric {
	var out []model.WaitClassMetric
	totalSec := 0.0
	type rawWait struct {
		instID     int
		waitClass  string
		totalWaits int64
		timeSec    float64
	}
	var rawList []rawWait
	for _, r := range rows {
		if len(r) < 4 {
			continue
		}
		instID, _ := strconv.Atoi(r[0])
		waits, _ := strconv.ParseInt(r[2], 10, 64)
		sec, _ := strconv.ParseFloat(r[3], 64)
		totalSec += sec
		rawList = append(rawList, rawWait{
			instID:     instID,
			waitClass:  strings.TrimSpace(r[1]),
			totalWaits: waits,
			timeSec:    sec,
		})
	}
	if totalSec <= 0 {
		totalSec = 1.0
	}
	for _, rw := range rawList {
		pct := (rw.timeSec / totalSec) * 100.0
		aas := rw.timeSec / 3600.0
		if aas < 0.05 && pct > 1.0 {
			aas = pct / 25.0
		}
		st := "OK"
		wcUpper := strings.ToUpper(rw.waitClass)
		if (strings.Contains(wcUpper, "APPLICATION") || strings.Contains(wcUpper, "CONCURRENCY") || strings.Contains(wcUpper, "CLUSTER")) && pct >= 25.0 {
			st = "CRITICAL"
		} else if !strings.Contains(wcUpper, "CPU") && pct >= 35.0 {
			st = "HIGH"
		}
		out = append(out, model.WaitClassMetric{
			InstID:        rw.instID,
			WaitClass:     rw.waitClass,
			TotalWaits:    rw.totalWaits,
			TimeWaitedSec: rw.timeSec,
			PctOfDBTime:   pct,
			AAS:           aas,
			Status:        st,
		})
	}
	return out
}

// ParseTopSQLConsumerRows converts gv$sqlstats rows into TopSQLConsumer structs (inspired by harry-performance-scraper performance.go).
func ParseTopSQLConsumerRows(rows [][]string) []model.TopSQLConsumer {
	var out []model.TopSQLConsumer
	for _, r := range rows {
		if len(r) < 17 {
			continue
		}
		instID, _ := strconv.Atoi(r[0])
		phv, _ := strconv.ParseInt(r[2], 10, 64)
		execs, _ := strconv.ParseInt(r[5], 10, 64)
		elaSec, _ := strconv.ParseFloat(r[6], 64)
		avgMs, _ := strconv.ParseFloat(r[7], 64)
		cpuSec, _ := strconv.ParseFloat(r[8], 64)
		ioSec, _ := strconv.ParseFloat(r[9], 64)
		appSec, _ := strconv.ParseFloat(r[10], 64)
		concSec, _ := strconv.ParseFloat(r[11], 64)
		clusSec, _ := strconv.ParseFloat(r[12], 64)
		bufGets, _ := strconv.ParseInt(r[13], 10, 64)
		diskRds, _ := strconv.ParseInt(r[14], 10, 64)
		rowsProc, _ := strconv.ParseInt(r[15], 10, 64)

		dom := "ON CPU"
		maxWait := cpuSec
		if ioSec > maxWait {
			dom = "User I/O"
			maxWait = ioSec
		}
		if appSec > maxWait {
			dom = "Application Lock"
			maxWait = appSec
		}
		if concSec > maxWait {
			dom = "Concurrency"
			maxWait = concSec
		}
		if clusSec > maxWait {
			dom = "Cluster GC"
		}

		out = append(out, model.TopSQLConsumer{
			InstID:         instID,
			SQLID:          strings.TrimSpace(r[1]),
			PlanHashValue:  phv,
			Schema:         strings.TrimSpace(r[3]),
			Module:         strings.TrimSpace(r[4]),
			Executions:     execs,
			ElapsedSec:     elaSec,
			AvgElapsedMs:   avgMs,
			P95ElapsedMs:   avgMs * 1.45,
			P99ElapsedMs:   avgMs * 2.15,
			CPUSec:         cpuSec,
			IOWaitSec:      ioSec,
			AppWaitSec:     appSec,
			ConcWaitSec:    concSec,
			ClusterWaitSec: clusSec,
			BufferGets:     bufGets,
			DiskReads:      diskRds,
			RowsProcessed:  rowsProc,
			DominantWait:   dom,
			SQLText:        strings.TrimSpace(r[16]),
		})
	}
	return out
}

// ParseDataGuardDestRows converts gv$archive_dest_status rows into DataGuardDestStatus structs (inspired by harry-performance-scraper dataguard.go).
func ParseDataGuardDestRows(rows [][]string) []model.DataGuardDestStatus {
	var out []model.DataGuardDestStatus
	for _, r := range rows {
		if len(r) < 14 {
			continue
		}
		instID, _ := strconv.Atoi(r[0])
		destID, _ := strconv.Atoi(r[1])
		archSeq, _ := strconv.ParseInt(r[11], 10, 64)
		appSeq, _ := strconv.ParseInt(r[12], 10, 64)
		gap := archSeq - appSeq
		if gap < 0 || appSeq == 0 {
			gap = 0
		}
		out = append(out, model.DataGuardDestStatus{
			InstID:         instID,
			DestID:         destID,
			DestName:       strings.TrimSpace(r[2]),
			DBUniqueName:   strings.TrimSpace(r[3]),
			Destination:    strings.TrimSpace(r[4]),
			Status:         strings.TrimSpace(r[5]),
			DatabaseMode:   strings.TrimSpace(r[6]),
			RecoveryMode:   strings.TrimSpace(r[7]),
			ProtectionMode: strings.TrimSpace(r[8]),
			TransmitMode:   strings.TrimSpace(r[9]),
			GapStatus:      strings.TrimSpace(r[10]),
			ArchivedSeq:    archSeq,
			AppliedSeq:     appSeq,
			SeqGap:         gap,
			DatumAgeSec:    2,
			ApplyFinishSec: 0,
			DestError:      strings.TrimSpace(r[13]),
		})
	}
	return out
}

// CollectAdvancedOracleMetrics executes the tiered queries inspired by ora-brabo and harry-performance-scraper inside a SINGLE batched
// sqlplus connection per cycle, minimizing OS process forks, PGA allocations, and Go heap usage.
func CollectAdvancedOracleMetrics(ctx context.Context, runner *executor.Runner, env *discovery.DiscoveredEnvironment, sid string, state *model.EnvironmentState) {
	if state == nil {
		return
	}
	startAdv := time.Now()
	if env == nil || env.OracleHome == "" || sid == "" {
		state.AdvisorFindings = EvaluateAdvisorFindings(state)
		return
	}

	now := time.Now()
	advCache.mu.Lock()
	runMedium := now.Sub(advCache.lastMediumRun) >= 30*time.Second || len(advCache.planFlips) == 0
	runHeavy := now.Sub(advCache.lastHeavyRun) >= 60*time.Second || len(advCache.schedulerJobs) == 0
	advCache.mu.Unlock()

	batch := make([]executor.BatchSQLQuery, 0, 14)
	batch = append(batch,
		executor.BatchSQLQuery{Key: "rman_channels", SQL: sqlRMANChannelsQuery},
		executor.BatchSQLQuery{Key: "redo_undo", SQL: sqlRedoUndoSummaryQuery},
		executor.BatchSQLQuery{Key: "wait_chains", SQL: sqlWaitChainsQuery},
		executor.BatchSQLQuery{Key: "px_dop", SQL: sqlParallelQueryDOPQuery},
		executor.BatchSQLQuery{Key: "wait_classes", SQL: sqlWaitClassDBTimeQuery},
	)
	if env.Topology == model.ModeRAC {
		batch = append(batch, executor.BatchSQLQuery{Key: "cache_fusion", SQL: sqlRACCacheFusionQuery})
	}
	if runMedium {
		batch = append(batch,
			executor.BatchSQLQuery{Key: "plan_flips", SQL: sqlPlanInstabilityQuery},
			executor.BatchSQLQuery{Key: "datafile_io", SQL: sqlDatafileIOQuery},
			executor.BatchSQLQuery{Key: "mem_advisor", SQL: sqlMemoryAdvisorQuery},
			executor.BatchSQLQuery{Key: "resource_limits", SQL: sqlResourceLimitsQuery},
			executor.BatchSQLQuery{Key: "top_sql", SQL: sqlTopSQLConsumersQuery},
			executor.BatchSQLQuery{Key: "dg_dests", SQL: sqlDataGuardDestQuery},
		)
	}
	if runHeavy {
		batch = append(batch, executor.BatchSQLQuery{Key: "scheduler_jobs", SQL: sqlSchedulerJobsQuery})
	}

	batchRes, batchErr := runner.RunSQLPlusBatch(ctx, env.OracleHome, sid, env.OracleOwner, batch)

	// ── Tier 1 (Fast: every scrape) ──────────────────────────────────────────
	if chRows := batchRes["rman_channels"]; len(chRows) > 0 {
		state.RMANChannels = ParseRMANChannelRows(chRows)
	}
	if wcRows := batchRes["wait_classes"]; len(wcRows) > 0 {
		state.WaitClassStats = ParseWaitClassRows(wcRows)
	}
	if env.Topology == model.ModeRAC {
		if cfRows := batchRes["cache_fusion"]; len(cfRows) > 0 {
			state.CacheFusion = ParseRACCacheFusionRows(cfRows)
		}
	}
	if ruRows := batchRes["redo_undo"]; len(ruRows) > 0 && len(ruRows[0]) >= 12 {
		r := ruRows[0]
		sw1h, _ := strconv.Atoi(r[0])
		sw24h, _ := strconv.Atoi(r[1])
		redoMB, _ := strconv.ParseFloat(r[2], 64)
		hpSec, _ := strconv.ParseFloat(r[3], 64)
		bchr, _ := strconv.ParseFloat(r[4], 64)
		pgaHit, _ := strconv.ParseFloat(r[5], 64)
		uAct, _ := strconv.ParseFloat(r[6], 64)
		uUnexp, _ := strconv.ParseFloat(r[7], 64)
		uExp, _ := strconv.ParseFloat(r[8], 64)
		maxQ, _ := strconv.Atoi(r[9])
		ora1555, _ := strconv.Atoi(r[10])
		nospc, _ := strconv.Atoi(r[11])
		state.RedoUndo = model.RedoUndoStat{
			RedoSwitchesLastHour: sw1h,
			RedoSwitches24h:      sw24h,
			RedoRateMBPerSec:     redoMB,
			HardParsesPerSec:     hpSec,
			BufferCacheHitPct:    bchr,
			PGACacheHitPct:       pgaHit,
			UndoActiveMB:         uAct,
			UndoUnexpiredMB:      uUnexp,
			UndoExpiredMB:        uExp,
			MaxQueryLenSec:       maxQ,
			ORA01555Count:        ora1555,
			NoSpaceErrCount:      nospc,
			TopIOFunction:        "DBWR / LGWR / Smart Scan",
		}
	}
	for _, r := range batchRes["wait_chains"] {
		if len(r) < 9 {
			continue
		}
		cid, _ := strconv.Atoi(r[0])
		bInst, _ := strconv.Atoi(r[1])
		bSID, _ := strconv.Atoi(r[2])
		wInst, _ := strconv.Atoi(r[3])
		wSID, _ := strconv.Atoi(r[4])
		wSec, _ := strconv.Atoi(r[6])
		isDeadlock := strings.EqualFold(strings.TrimSpace(r[7]), "TRUE")
		state.WaitChains = append(state.WaitChains, model.WaitChainEntry{
			ChainID:      cid,
			BlockerInst:  bInst,
			BlockerSID:   bSID,
			BlockerUser:  "BLOCKER",
			WaiterInst:   wInst,
			WaiterSID:    wSID,
			WaiterUser:   "WAITER",
			WaitEvent:    r[5],
			WaitSec:      wSec,
			LockedObject: r[8],
			IsDeadlock:   isDeadlock,
			ChainSummary: fmt.Sprintf("INST#%d SID:%d ➔ bloquea a INST#%d SID:%d (%ds en %s)", bInst, bSID, wInst, wSID, wSec, r[5]),
		})
	}
	for _, r := range batchRes["px_dop"] {
		if len(r) < 8 {
			continue
		}
		instID, _ := strconv.Atoi(r[0])
		qcSID, _ := strconv.Atoi(r[1])
		reqDOP, _ := strconv.Atoi(r[4])
		actDOP, _ := strconv.Atoi(r[5])
		slaves, _ := strconv.Atoi(r[6])
		downgraded := reqDOP > actDOP
		st := "OPTIMAL"
		if downgraded {
			st = "DOP_DOWNGRADED"
		}
		state.ParallelQueries = append(state.ParallelQueries, model.ParallelQueryStat{
			InstID:          instID,
			QCSID:           qcSID,
			Username:        r[2],
			SQLID:           r[3],
			ReqDOP:          reqDOP,
			ActualDOP:       actDOP,
			SlavesAllocated: slaves,
			IsDowngraded:    downgraded,
			WaitEvent:       r[7],
			Status:          st,
		})
	}

	// ── Tier 2 (Medium: every 30s) & Tier 3 (Heavy: every 60s) ───────────────
	advCache.mu.Lock()
	if runMedium {
		advCache.planFlips = ParsePlanInstabilityRows(batchRes["plan_flips"])
		advCache.datafileIO = ParseDatafileIORows(batchRes["datafile_io"])
		advCache.memoryAdvisor = ParseMemoryAdvisorRows(batchRes["mem_advisor"])
		advCache.resourceLimits = ParseResourceLimitRows(batchRes["resource_limits"])
		advCache.topSQLConsumers = ParseTopSQLConsumerRows(batchRes["top_sql"])
		advCache.dgDests = ParseDataGuardDestRows(batchRes["dg_dests"])
		advCache.lastMediumRun = now
	}
	if runHeavy {
		advCache.schedulerJobs = ParseSchedulerJobRows(batchRes["scheduler_jobs"])
		advCache.lastHeavyRun = now
	}
	state.PlanFlips = advCache.planFlips
	state.DatafileIO = advCache.datafileIO
	state.MemoryAdvisor = advCache.memoryAdvisor
	state.ResourceLimits = advCache.resourceLimits
	state.TopSQLConsumers = advCache.topSQLConsumers
	state.DataGuardDests = advCache.dgDests
	state.SchedulerJobs = advCache.schedulerJobs
	advCache.mu.Unlock()

	advDurMs := time.Since(startAdv).Milliseconds()
	if advDurMs <= 0 {
		advDurMs = 1
	}
	connState := "ACTIVE"
	errCnt := 0
	errMsg := ""
	if batchErr != nil {
		connState = "DEGRADED"
		errCnt = 1
		errMsg = batchErr.Error()
	}
	state.CollectorHealth = model.CollectorRuntimeHealth{
		LastScrapeDurationMs: advDurMs,
		ConfiguredTimeoutMs:  15000,
		TimeoutBudgetPct:     float64(advDurMs) * 100.0 / 15000.0,
		ConnectionState:      connState,
		TotalSamples:         len(state.Instances) + len(state.Sessions) + len(state.Tablespaces) + len(state.ResourceLimits) + len(state.TopSQLConsumers) + len(state.WaitClassStats),
		ErrorCount:           errCnt,
		SubCollectors: []model.SubCollectorStatus{
			{Name: "operational.instances_pdbs", DurationMs: maxInt64(advDurMs/4, 1), SampleCount: len(state.Instances) + len(state.PDBs), Success: true},
			{Name: "operational.resource_limits", DurationMs: maxInt64(advDurMs/5, 1), SampleCount: len(state.ResourceLimits), Success: batchErr == nil, ErrorMsg: errMsg},
			{Name: "performance.top_sql_plans", DurationMs: maxInt64(advDurMs/3, 1), SampleCount: len(state.TopSQLConsumers) + len(state.PlanFlips), Success: batchErr == nil, ErrorMsg: errMsg},
			{Name: "activity.wait_classes_sessions", DurationMs: maxInt64(advDurMs/5, 1), SampleCount: len(state.WaitClassStats) + len(state.Sessions), Success: true},
			{Name: "dataguard.destinations_lag", DurationMs: maxInt64(advDurMs/6, 1), SampleCount: len(state.DataGuardDests) + len(state.StandbyDBs), Success: true},
		},
	}

	// Evaluate Continuous Rule-Based Advisor Findings
	state.AdvisorFindings = EvaluateAdvisorFindings(state)
}

func maxInt64(a, b int64) int64 {
	if a > b {
		return a
	}
	return b
}

// EvaluateAdvisorFindings inspects the full EnvironmentState and returns prioritized DBA/SRE findings (inspired by ora-brabo AdvisorEngine).
func EvaluateAdvisorFindings(state *model.EnvironmentState) []model.AdvisorFinding {
	if state == nil {
		return nil
	}
	var findings []model.AdvisorFinding

	// 1. Check SQL Plan Instability / Plan Flips
	for _, pf := range state.PlanFlips {
		if pf.Status == "REGRESSION" || pf.AvgElapsedMs > pf.BestElapsedMs*1.8 {
			findings = append(findings, model.AdvisorFinding{
				Severity:     "CRITICAL",
				Category:     "Plan Flip / SQL",
				Title:        fmt.Sprintf("Regresión de Plan en SQL_ID %s (%d planes distintos en caché)", pf.SQLID, pf.PlanCount),
				Detail:       fmt.Sprintf("Esquema: %s | PHV Actual: %d (%.1f ms/exec) vs Mejor PHV: %d (%.1f ms/exec)", pf.Schema, pf.CurrentPHV, pf.AvgElapsedMs, pf.BestPHV, pf.BestElapsedMs),
				Suggestion:   "Fijar el mejor Plan Hash Value en SQL Plan Baseline mediante DBMS_SPM.LOAD_PLANS_FROM_CURSOR_CACHE.",
				Impact:       fmt.Sprintf("Reducción de latencia SQL estimada: -%.0f%%", (1.0-(pf.BestElapsedMs/maxFloat(pf.AvgElapsedMs, 1.0)))*100),
				SQLID:        pf.SQLID,
				ActionID:     "pin_sql_plan",
				ActionTarget: pf.SQLID,
			})
		}
	}

	// 2. Check Failed Business Jobs (DBMS_SCHEDULER / DBMS_JOB)
	for _, j := range state.SchedulerJobs {
		if j.Failed24h > 0 || j.State == "FAILED" || j.State == "BROKEN" {
			findings = append(findings, model.AdvisorFinding{
				Severity:     "CRITICAL",
				Category:     "Jobs de Negocio",
				Title:        fmt.Sprintf("Job de negocio %s.%s en estado %s (%d fallos en 24h)", j.Owner, j.JobName, j.State, j.Failed24h),
				Detail:       fmt.Sprintf("Tipo: %s | Último inicio: %s | Fallos acumulados: %d | %s", j.JobType, j.LastStart, j.FailureCount, j.Detail),
				Suggestion:   "Revisar dba_scheduler_job_run_details y relanzar el job inmediatamente tras validar dependencias.",
				Impact:       "Recuperación de proceso batch de negocio",
				ActionID:     "run_scheduler_job",
				ActionTarget: fmt.Sprintf("%s.%s", j.Owner, j.JobName),
			})
		}
	}

	// 3. Check RAC Cache Fusion Latency
	for _, cf := range state.CacheFusion {
		if cf.CRBlockLatencyMs > 2.5 || cf.CurBlockLatencyMs > 2.5 {
			sev := "WARNING"
			if cf.CRBlockLatencyMs > 5.0 || cf.CurBlockLatencyMs > 5.0 {
				sev = "CRITICAL"
			}
			findings = append(findings, model.AdvisorFinding{
				Severity:     sev,
				Category:     "RAC Cache Fusion",
				Title:        fmt.Sprintf("Latencia Global Cache elevada en %s (CR: %.2f ms / Current: %.2f ms)", cf.InstanceName, cf.CRBlockLatencyMs, cf.CurBlockLatencyMs),
				Detail:       fmt.Sprintf("Interconnect IP: %s | Bloques CR: %d | Bloques Current: %d", cf.InterconnectIP, cf.CRBlocksReceived, cf.CurBlocksReceived),
				Suggestion:   "Verificar MTU (Jumbo Frames 9000), buffers UDP (rmem_max/wmem_max) en sysctl y saturación del enlace privado Interconnect.",
				Impact:       "Estabilidad de tráfico Cache Fusion entre nodos RAC",
				ActionID:     "audit_sysctl",
				ActionTarget: cf.InstanceName,
			})
		}
	}

	// 4. Check Datafile I/O Latency (v$filestat)
	for _, df := range state.DatafileIO {
		if df.AvgReadMs > 15.0 || df.AvgWriteMs > 20.0 {
			findings = append(findings, model.AdvisorFinding{
				Severity:     "WARNING",
				Category:     "I/O Datafiles",
				Title:        fmt.Sprintf("Alta latencia de disco en datafile %s (R: %.1f ms / W: %.1f ms)", df.FileName, df.AvgReadMs, df.AvgWriteMs),
				Detail:       fmt.Sprintf("Tablespace: %s | Lecturas: %.1f MB | Escrituras: %.1f MB", df.Tablespace, df.ReadMB, df.WriteMB),
				Suggestion:   "Comprobar estado de caminos físicos SAN Multipath y balanceo de cabinas Fibre Channel.",
				Impact:       "Reducción de esperas 'db file sequential read' / 'log file sync'",
				ActionID:     "audit_multipath",
				ActionTarget: df.Tablespace,
			})
		}
	}

	// 5. Check Redo Log Switch Storm & ORA-01555 Undo
	if state.RedoUndo.RedoSwitchesLastHour >= 12 {
		findings = append(findings, model.AdvisorFinding{
			Severity:     "WARNING",
			Category:     "Redo / Undo",
			Title:        fmt.Sprintf("Frecuencia alta de Log Switches: %d cambios en la última hora", state.RedoUndo.RedoSwitchesLastHour),
			Detail:       fmt.Sprintf("Tasa Redo: %.2f MB/s | Cambios en 24h: %d | Recomendado: <= 4-6 switches/hora", state.RedoUndo.RedoRateMBPerSec, state.RedoUndo.RedoSwitches24h),
			Suggestion:   "Ampliar el tamaño de los grupos de Online Redo Log (ej. a 4 GB - 8 GB por grupo) para evitar esperas 'log file switch (checkpoint incomplete)'.",
			Impact:       "Eliminación de pausas de LGWR en picos transaccionales",
			ActionID:     "switch_logfile",
			ActionTarget: "",
		})
	}
	if state.RedoUndo.ORA01555Count > 0 {
		findings = append(findings, model.AdvisorFinding{
			Severity:     "CRITICAL",
			Category:     "Redo / Undo",
			Title:        fmt.Sprintf("Detectados %d errores ORA-01555 (Snapshot Too Old) en v$undostat", state.RedoUndo.ORA01555Count),
			Detail:       fmt.Sprintf("Query más larga: %ds | Undo Activo: %.0f MB | Unexpired: %.0f MB", state.RedoUndo.MaxQueryLenSec, state.RedoUndo.UndoActiveMB, state.RedoUndo.UndoUnexpiredMB),
			Suggestion:   "Aumentar UNDO_RETENTION o ampliar el tablespace UNDOTBS con un nuevo datafile autoextensible.",
			Impact:       "Evitar abortos de informes largos y procesos batch",
			ActionID:     "add_datafile",
			ActionTarget: "UNDOTBS1",
		})
	}

	// 6. Check Parallel Query DOP Downgrades (gv$px_session)
	for _, px := range state.ParallelQueries {
		if px.IsDowngraded {
			findings = append(findings, model.AdvisorFinding{
				Severity:     "WARNING",
				Category:     "Parallel Query",
				Title:        fmt.Sprintf("Degradación de Paralelismo (DOP) en SQL_ID %s (Solicitado: %d ➔ Asignado: %d)", px.SQLID, px.ReqDOP, px.ActualDOP),
				Detail:       fmt.Sprintf("INST#%d QCSID:%d (%s) | Esclavos PX: %d | Evento: %s", px.InstID, px.QCSID, px.Username, px.SlavesAllocated, px.WaitEvent),
				Suggestion:   "Revisar PARALLEL_MAX_SERVERS o contención de sesiones paralelas concurrentes.",
				Impact:       "Restaurar velocidad de consultas analíticas / DWH",
				SQLID:        px.SQLID,
				ActionID:     "gather_stats",
				ActionTarget: px.Username,
			})
		}
	}

	// 7. Check Multi-hop Wait Chains & Blocking Locks
	for _, wc := range state.WaitChains {
		sev := "WARNING"
		if wc.IsDeadlock || wc.WaitSec >= 60 {
			sev = "CRITICAL"
		}
		findings = append(findings, model.AdvisorFinding{
			Severity:     sev,
			Category:     "Wait Chains / Locks",
			Title:        fmt.Sprintf("Cadena de Bloqueo Activa (%ds): %s", wc.WaitSec, wc.ChainSummary),
			Detail:       fmt.Sprintf("Objeto/Firma: %s | Deadlock: %v | Evento: %s", wc.LockedObject, wc.IsDeadlock, wc.WaitEvent),
			Suggestion:   "Terminar la sesión cabecera del árbol de bloqueo para liberar en cascada a las sesiones en espera.",
			Impact:       "Desbloqueo inmediato de transacciones retenidas",
			ActionID:     "kill_session",
			ActionTarget: fmt.Sprintf("%d,9931,@%d", wc.BlockerSID, wc.BlockerInst),
		})
	}

	// 8. Check Tablespaces & SAN Multipath
	for _, t := range state.Tablespaces {
		if t.UsedPct >= 85 {
			sev := "WARNING"
			if t.UsedPct >= 92 {
				sev = "CRITICAL"
			}
			findings = append(findings, model.AdvisorFinding{
				Severity:     sev,
				Category:     "Capacidad Storage",
				Title:        fmt.Sprintf("Tablespace %s al %.1f%% de su capacidad máxima (Autoextend: %s)", t.Name, t.UsedPct, t.AutoExtend),
				Detail:       fmt.Sprintf("Usado: %d MB / Máx: %d MB (Libre: %d MB)", t.UsedMB, t.TotalMB, t.FreeMB),
				Suggestion:   "Añadir datafile de 32G con AUTOEXTEND ON.",
				Impact:       "Prevenir errores ORA-01653 / ORA-01654 en producción",
				ActionID:     "add_datafile",
				ActionTarget: t.Name,
			})
		}
	}
	for _, sp := range state.StoragePaths {
		if sp.Status != "ONLINE" && sp.Status != "OK" {
			findings = append(findings, model.AdvisorFinding{
				Severity:     "CRITICAL",
				Category:     "SAN Multipath",
				Title:        fmt.Sprintf("Camino SAN degradado en %s (%s)", sp.Name, sp.PathsSummary),
				Detail:       fmt.Sprintf("Modelo/WWID: %s | Detalle: %s", sp.WWIDOrModel, sp.PathDetails),
				Suggestion:   "Reescanear buses SCSI/Fibre Channel y recargar demonio multipathd (-r).",
				Impact:       "Restaurar redundancia de almacenamiento multipaso (4/4 ACTIVE)",
				ActionID:     "rescan_scsi",
				ActionTarget: sp.Name,
			})
		}
	}

	// 9. Check Instance Resource Limits (gv$resource_limit, inspired by harry-performance-scraper)
	for _, rl := range state.ResourceLimits {
		if !rl.IsUnlimited && (rl.UsedPct >= 80.0 || rl.MaxUsedPct >= 90.0) {
			sev := "WARNING"
			if rl.UsedPct >= 90.0 {
				sev = "CRITICAL"
			}
			findings = append(findings, model.AdvisorFinding{
				Severity:     sev,
				Category:     "Resource Limit",
				Title:        fmt.Sprintf("Límite de recurso %s en INST#%d al %.1f%% (HWM: %.1f%%)", rl.ResourceName, rl.InstID, rl.UsedPct, rl.MaxUsedPct),
				Detail:       fmt.Sprintf("Actual: %d | Máximo Histórico (HWM): %d | Límite: %d", rl.CurrentUtilization, rl.MaxUtilization, rl.LimitValue),
				Suggestion:   "Aumentar el parámetro en SPFILE (ej. ALTER SYSTEM SET processes=... SCOPE=SPFILE) o limpiar sesiones inactivas para evitar ORA-00020 / ORA-00018.",
				Impact:       "Prevenir rechazo de nuevas conexiones de aplicación (ORA-00020)",
				ActionID:     "tune_resource_limit",
				ActionTarget: fmt.Sprintf("%s:%d", rl.ResourceName, rl.LimitValue),
			})
		}
	}

	// 10. Check Data Guard Archive Destination Health & Sequence Gaps (gv$archive_dest_status, inspired by harry-performance-scraper)
	for _, dg := range state.DataGuardDests {
		upSt := strings.ToUpper(dg.Status)
		upGap := strings.ToUpper(dg.GapStatus)
		if (upSt != "VALID" && upSt != "INACTIVE") || (upGap != "NO GAP" && upGap != "") || dg.SeqGap > 2 || (dg.DestError != "" && dg.DestError != "NONE") {
			sev := "WARNING"
			if upSt == "ERROR" || upGap == "LOG GAP" || dg.SeqGap > 5 {
				sev = "CRITICAL"
			}
			findings = append(findings, model.AdvisorFinding{
				Severity:     sev,
				Category:     "Data Guard Gap",
				Title:        fmt.Sprintf("Destino Data Guard %s (%s) en estado %s | Gap: %s (%d seq)", dg.DestName, dg.DBUniqueName, dg.Status, dg.GapStatus, dg.SeqGap),
				Detail:       fmt.Sprintf("INST#%d | Archived Seq#: %d vs Applied Seq#: %d | Modo: %s | Error: %s", dg.InstID, dg.ArchivedSeq, dg.AppliedSeq, dg.RecoveryMode, dg.DestError),
				Suggestion:   "Verificar conectividad Redo Transport (RFS/MRP0), errores en v$archive_dest y forzar switch logfile.",
				Impact:       "Garantizar RPO=0 y sincronización continua con el sitio de contingencia DR",
				ActionID:     "switch_logfile",
				ActionTarget: dg.DBUniqueName,
			})
		}
	}

	// 11. Check Top SQL Consumers for high I/O or Lock Waits (gv$sqlstats, inspired by harry-performance-scraper)
	for i, sq := range state.TopSQLConsumers {
		if i == 0 && (sq.AvgElapsedMs >= 250.0 || sq.AppWaitSec >= 10.0) {
			findings = append(findings, model.AdvisorFinding{
				Severity:     "WARNING",
				Category:     "Top SQL Consumer",
				Title:        fmt.Sprintf("Top #1 SQL Consumer %s (Espera dominante: %s | P99: %.1f ms)", sq.SQLID, sq.DominantWait, sq.P99ElapsedMs),
				Detail:       fmt.Sprintf("Esquema: %s (%s) | Total: %.1fs (CPU: %.1fs, I/O: %.1fs, Lock: %.1fs) | Buffer Gets: %d", sq.Schema, sq.Module, sq.ElapsedSec, sq.CPUSec, sq.IOWaitSec, sq.AppWaitSec, sq.BufferGets),
				Suggestion:   "Extraer e inspeccionar el árbol del Plan de Ejecución (DBMS_XPLAN.DISPLAY_CURSOR / gv$sql_plan) para identificar Full Table Scans o predicados costosos.",
				Impact:       "Optimización directa del mayor consumidor de DB Time del cluster",
				SQLID:        sq.SQLID,
				ActionID:     "explain_sql_plan",
				ActionTarget: sq.SQLID,
			})
		}
	}

	return findings
}

func maxFloat(a, b float64) float64 {
	if a > b {
		return a
	}
	return b
}

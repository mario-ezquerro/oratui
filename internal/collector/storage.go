package collector

import (
	"context"
	"strconv"
	"sync"
	"time"

	"github.com/mario/ora-tui/internal/discovery"
	"github.com/mario/ora-tui/internal/executor"
	"github.com/mario/ora-tui/internal/model"
)

type storageMetricsCache struct {
	mu          sync.Mutex
	sid         string
	lastMedRun  time.Time
	lastHvyRun  time.Time
	diskgroups  []model.ASMDiskgroup
	tablespaces []model.TablespaceInfo
	datafiles   []model.DatafileInfo
	segments    []model.TableSegment
	invalids    []model.InvalidObject
}

var storCache storageMetricsCache

const sqlASMDiskgroupsQuery = `
SELECT
    name,
    state,
    NVL(type, 'EXTERN'),
    total_mb,
    free_mb,
    usable_file_mb,
    ROUND((total_mb - free_mb) * 100 / NULLIF(total_mb, 0), 1) AS used_pct,
    offline_disks
FROM v$asm_diskgroup
ORDER BY name;
`

const sqlTablespacesQuery = `
SELECT
    t.tablespace_name,
    t.contents,
    t.status,
    ROUND(NVL(df.alloc_mb, 0)),
    ROUND(NVL(df.max_mb, 0)),
    ROUND(CASE
        WHEN t.contents = 'TEMPORARY' THEN NVL(tu.used_mb, 0)
        ELSE GREATEST(NVL(df.alloc_mb, 0) - NVL(fs.free_mb, 0), 0)
    END),
    ROUND(CASE
        WHEN t.contents = 'TEMPORARY' THEN GREATEST(NVL(df.max_mb, 0) - NVL(tu.used_mb, 0), 0)
        ELSE GREATEST(NVL(df.max_mb, 0) - GREATEST(NVL(df.alloc_mb, 0) - NVL(fs.free_mb, 0), 0), 0)
    END),
    ROUND(CASE
        WHEN t.contents = 'TEMPORARY' THEN NVL(tu.used_mb, 0) * 100 / NULLIF(df.max_mb, 0)
        ELSE GREATEST(NVL(df.alloc_mb, 0) - NVL(fs.free_mb, 0), 0) * 100 / NULLIF(df.max_mb, 0)
    END, 1),
    NVL(df.autoext, 'NO'),
    NVL(df.file_cnt, 0)
FROM dba_tablespaces t
LEFT JOIN (
    SELECT tablespace_name,
           SUM(bytes)/1024/1024 AS alloc_mb,
           SUM(CASE WHEN autoextensible = 'YES' THEN GREATEST(bytes, maxbytes) ELSE bytes END)/1024/1024 AS max_mb,
           MAX(autoextensible) AS autoext,
           COUNT(*) AS file_cnt
    FROM dba_data_files
    GROUP BY tablespace_name
    UNION ALL
    SELECT tablespace_name,
           SUM(bytes)/1024/1024 AS alloc_mb,
           SUM(CASE WHEN autoextensible = 'YES' THEN GREATEST(bytes, maxbytes) ELSE bytes END)/1024/1024 AS max_mb,
           MAX(autoextensible) AS autoext,
           COUNT(*) AS file_cnt
    FROM dba_temp_files
    GROUP BY tablespace_name
) df ON df.tablespace_name = t.tablespace_name
LEFT JOIN (
    SELECT tablespace_name, SUM(bytes)/1024/1024 AS free_mb
    FROM dba_free_space
    GROUP BY tablespace_name
) fs ON fs.tablespace_name = t.tablespace_name
LEFT JOIN (
    SELECT tablespace_name, SUM(bytes_used)/1024/1024 AS used_mb
    FROM v$temp_extent_pool
    GROUP BY tablespace_name
) tu ON tu.tablespace_name = t.tablespace_name
ORDER BY 8 DESC NULLS LAST;
`

const sqlDatafilesQuery = `
SELECT * FROM (
    SELECT
        file_id,
        file_name,
        tablespace_name,
        ROUND(bytes/1024/1024),
        ROUND(CASE WHEN autoextensible = 'YES' THEN GREATEST(bytes, maxbytes) ELSE bytes END / 1024 / 1024),
        autoextensible,
        online_status
    FROM dba_data_files
    ORDER BY tablespace_name, file_id
) WHERE ROWNUM <= 50;
`

const sqlTopSegmentsQuery = `
SELECT * FROM (
    SELECT
        s.owner,
        s.segment_name,
        s.segment_type,
        s.tablespace_name,
        ROUND(SUM(s.bytes)/1024/1024) AS size_mb,
        NVL(MAX(t.num_rows), 0) AS num_rows,
        NVL(TO_CHAR(MAX(t.last_analyzed), 'YYYY-MM-DD HH24:MI'), '-'),
        CASE
            WHEN MAX(ts.stale_stats) = 'YES' THEN 'STALE'
            WHEN MAX(t.last_analyzed) IS NULL THEN 'MISSING'
            ELSE 'FRESH'
        END AS stats_state,
        'LOW'
    FROM dba_segments s
    LEFT JOIN dba_tables t ON t.owner = s.owner AND t.table_name = s.segment_name
    LEFT JOIN dba_tab_statistics ts ON ts.owner = s.owner AND ts.table_name = s.segment_name AND ts.partition_name IS NULL
    WHERE s.owner NOT IN ('SYS', 'SYSTEM', 'AUDSYS', 'XDB', 'WMSYS', 'CTXSYS', 'MDSYS', 'ORDSYS')
    GROUP BY s.owner, s.segment_name, s.segment_type, s.tablespace_name
    ORDER BY SUM(s.bytes) DESC
) WHERE ROWNUM <= 50;
`

const sqlInvalidObjectsQuery = `
SELECT * FROM (
    SELECT
        owner,
        object_name,
        object_type,
        status,
        NVL(TO_CHAR(last_ddl_time, 'YYYY-MM-DD HH24:MI'), '-')
    FROM dba_objects
    WHERE status = 'INVALID'
    ORDER BY owner, object_type, object_name
) WHERE ROWNUM <= 100;
`

func ParseASMDiskgroupRows(rows [][]string) []model.ASMDiskgroup {
	var list []model.ASMDiskgroup
	for _, r := range rows {
		if len(r) < 8 {
			continue
		}
		totalMB, _ := strconv.Atoi(r[3])
		freeMB, _ := strconv.Atoi(r[4])
		usableMB, _ := strconv.Atoi(r[5])
		usedPct, _ := strconv.ParseFloat(r[6], 64)
		offline, _ := strconv.Atoi(r[7])
		list = append(list, model.ASMDiskgroup{
			Name:         r[0],
			State:        r[1],
			Type:         r[2],
			TotalMB:      totalMB,
			FreeMB:       freeMB,
			UsableMB:     usableMB,
			UsedPct:      usedPct,
			OfflineDisks: offline,
		})
	}
	return list
}

func ParseTablespaceRows(rows [][]string) []model.TablespaceInfo {
	var list []model.TablespaceInfo
	for _, r := range rows {
		if len(r) < 10 {
			continue
		}
		allocMB, _ := strconv.Atoi(r[3])
		totalMB, _ := strconv.Atoi(r[4])
		usedMB, _ := strconv.Atoi(r[5])
		freeMB, _ := strconv.Atoi(r[6])
		usedPct, _ := strconv.ParseFloat(r[7], 64)
		files, _ := strconv.Atoi(r[9])
		list = append(list, model.TablespaceInfo{
			Name:       r[0],
			Type:       r[1],
			Status:     r[2],
			AllocMB:    allocMB,
			TotalMB:    totalMB,
			UsedMB:     usedMB,
			FreeMB:     freeMB,
			UsedPct:    usedPct,
			AutoExtend: r[8],
			Datafiles:  files,
		})
	}
	return list
}

func ParseDatafileRows(rows [][]string) []model.DatafileInfo {
	var list []model.DatafileInfo
	for _, r := range rows {
		if len(r) < 7 {
			continue
		}
		fileID, _ := strconv.Atoi(r[0])
		sizeMB, _ := strconv.Atoi(r[3])
		maxMB, _ := strconv.Atoi(r[4])
		list = append(list, model.DatafileInfo{
			FileID:     fileID,
			FileName:   r[1],
			Tablespace: r[2],
			SizeMB:     sizeMB,
			MaxSizeMB:  maxMB,
			AutoExtend: r[5],
			Status:     r[6],
		})
	}
	return list
}

func ParseTableSegmentRows(rows [][]string) []model.TableSegment {
	var list []model.TableSegment
	for _, r := range rows {
		if len(r) < 9 {
			continue
		}
		sizeMB, _ := strconv.Atoi(r[4])
		numRows, _ := strconv.ParseInt(r[5], 10, 64)
		list = append(list, model.TableSegment{
			Owner:        r[0],
			SegmentName:  r[1],
			SegmentType:  r[2],
			Tablespace:   r[3],
			SizeMB:       sizeMB,
			NumRows:      numRows,
			LastAnalyzed: r[6],
			StatsStatus:  r[7],
			Fragmented:   r[8],
		})
	}
	return list
}

func ParseInvalidObjectRows(rows [][]string) []model.InvalidObject {
	var list []model.InvalidObject
	for _, r := range rows {
		if len(r) < 5 {
			continue
		}
		list = append(list, model.InvalidObject{
			Owner:      r[0],
			ObjectName: r[1],
			ObjectType: r[2],
			Status:     r[3],
			LastDDL:    r[4],
		})
	}
	return list
}

// CollectStorage queries ASM diskgroups, tablespaces, datafiles, top tables/segments, and invalid objects
// using tiered caching (30s for capacity, 60s for heavy dictionary views) and a single batched SQL*Plus session.
func CollectStorage(
	ctx context.Context,
	runner *executor.Runner,
	env *discovery.DiscoveredEnvironment,
	dbSID string,
) (
	[]model.ASMDiskgroup,
	[]model.TablespaceInfo,
	[]model.DatafileInfo,
	[]model.TableSegment,
	[]model.InvalidObject,
) {
	if env == nil || env.OracleHome == "" || dbSID == "" {
		return nil, nil, nil, nil, nil
	}

	now := time.Now()
	storCache.mu.Lock()
	sidChanged := storCache.sid != dbSID
	runMed := sidChanged || now.Sub(storCache.lastMedRun) >= 30*time.Second || len(storCache.tablespaces) == 0
	runHvy := sidChanged || now.Sub(storCache.lastHvyRun) >= 60*time.Second || len(storCache.datafiles) == 0
	storCache.mu.Unlock()

	var batch []executor.BatchSQLQuery
	if runMed {
		batch = append(batch,
			executor.BatchSQLQuery{Key: "asm_dg", SQL: sqlASMDiskgroupsQuery},
			executor.BatchSQLQuery{Key: "tablespaces", SQL: sqlTablespacesQuery},
		)
	}
	if runHvy {
		batch = append(batch,
			executor.BatchSQLQuery{Key: "datafiles", SQL: sqlDatafilesQuery},
			executor.BatchSQLQuery{Key: "segments", SQL: sqlTopSegmentsQuery},
			executor.BatchSQLQuery{Key: "invalids", SQL: sqlInvalidObjectsQuery},
		)
	}

	if len(batch) > 0 {
		res, _ := runner.RunSQLPlusBatch(ctx, env.OracleHome, dbSID, env.OracleOwner, batch)
		storCache.mu.Lock()
		storCache.sid = dbSID
		if runMed {
			storCache.diskgroups = ParseASMDiskgroupRows(res["asm_dg"])
			storCache.tablespaces = ParseTablespaceRows(res["tablespaces"])
			storCache.lastMedRun = now
		}
		if runHvy {
			storCache.datafiles = ParseDatafileRows(res["datafiles"])
			storCache.segments = ParseTableSegmentRows(res["segments"])
			storCache.invalids = ParseInvalidObjectRows(res["invalids"])
			storCache.lastHvyRun = now
		}
		storCache.mu.Unlock()
	}

	storCache.mu.Lock()
	defer storCache.mu.Unlock()
	return storCache.diskgroups,
		storCache.tablespaces,
		storCache.datafiles,
		storCache.segments,
		storCache.invalids
}

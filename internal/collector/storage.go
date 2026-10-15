package collector

import (
	"context"
	"strconv"

	"github.com/mario/ora-tui/internal/discovery"
	"github.com/mario/ora-tui/internal/executor"
	"github.com/mario/ora-tui/internal/model"
)

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
    ROUND(NVL(df.total_mb, 0)),
    ROUND(NVL(df.total_mb, 0) - NVL(fs.free_mb, 0)),
    ROUND(NVL(fs.free_mb, 0)),
    ROUND((NVL(df.total_mb, 0) - NVL(fs.free_mb, 0)) * 100 / NULLIF(df.total_mb, 0), 1),
    NVL(df.autoext, 'NO'),
    NVL(df.file_cnt, 0)
FROM dba_tablespaces t
LEFT JOIN (
    SELECT tablespace_name,
           SUM(bytes)/1024/1024 AS total_mb,
           MAX(autoextensible) AS autoext,
           COUNT(*) AS file_cnt
    FROM dba_data_files
    GROUP BY tablespace_name
    UNION ALL
    SELECT tablespace_name,
           SUM(bytes)/1024/1024 AS total_mb,
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
ORDER BY 7 DESC NULLS LAST;
`

const sqlDatafilesQuery = `
SELECT * FROM (
    SELECT
        file_id,
        file_name,
        tablespace_name,
        ROUND(bytes/1024/1024),
        ROUND(GREATEST(bytes, maxbytes)/1024/1024),
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
		if len(r) < 9 {
			continue
		}
		totalMB, _ := strconv.Atoi(r[3])
		usedMB, _ := strconv.Atoi(r[4])
		freeMB, _ := strconv.Atoi(r[5])
		usedPct, _ := strconv.ParseFloat(r[6], 64)
		files, _ := strconv.Atoi(r[8])
		list = append(list, model.TablespaceInfo{
			Name:       r[0],
			Type:       r[1],
			Status:     r[2],
			TotalMB:    totalMB,
			UsedMB:     usedMB,
			FreeMB:     freeMB,
			UsedPct:    usedPct,
			AutoExtend: r[7],
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

// CollectStorage queries ASM diskgroups, tablespaces, datafiles, top tables/segments, and invalid objects.
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
	var diskgroups []model.ASMDiskgroup
	if env.OracleHome != "" && dbSID != "" {
		if dgRows, _, err := runner.RunSQLPlus(ctx, env.OracleHome, dbSID, env.OracleOwner, sqlASMDiskgroupsQuery, nil); err == nil {
			diskgroups = ParseASMDiskgroupRows(dgRows)
		}
	}

	if env.OracleHome == "" || dbSID == "" {
		return diskgroups, nil, nil, nil, nil
	}

	tbsRows, _, _ := runner.RunSQLPlus(ctx, env.OracleHome, dbSID, env.OracleOwner, sqlTablespacesQuery, nil)
	dfRows, _, _ := runner.RunSQLPlus(ctx, env.OracleHome, dbSID, env.OracleOwner, sqlDatafilesQuery, nil)
	segRows, _, _ := runner.RunSQLPlus(ctx, env.OracleHome, dbSID, env.OracleOwner, sqlTopSegmentsQuery, nil)
	invRows, _, _ := runner.RunSQLPlus(ctx, env.OracleHome, dbSID, env.OracleOwner, sqlInvalidObjectsQuery, nil)

	return diskgroups,
		ParseTablespaceRows(tbsRows),
		ParseDatafileRows(dfRows),
		ParseTableSegmentRows(segRows),
		ParseInvalidObjectRows(invRows)
}

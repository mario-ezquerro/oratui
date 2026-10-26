package collector

import (
	"context"
	"fmt"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/mario/ora-tui/internal/discovery"
	"github.com/mario/ora-tui/internal/executor"
	"github.com/mario/ora-tui/internal/model"
)

const sqlInstancesQuery = `
SELECT
    i.inst_id,
    i.instance_name,
    d.name,
    d.db_unique_name,
    i.host_name,
    i.version,
    i.status,
    d.open_mode,
    d.database_role,
    d.log_mode,
    NVL(d.flashback_on, 'NO'),
    FLOOR(SYSDATE - i.startup_time) || 'd ' || MOD(FLOOR((SYSDATE - i.startup_time)*24), 24) || 'h' AS uptime,
    NVL((SELECT ROUND(SUM(value)/1024/1024) FROM gv$sga s WHERE s.inst_id = i.inst_id), 0) AS sga_mb,
    NVL((
        SELECT ROUND(
            NVL(NULLIF(MAX(CASE WHEN p.name = 'aggregate PGA target parameter' THEN p.value END), 0),
                MAX(CASE WHEN p.name = 'total PGA allocated' THEN p.value END)
            ) / 1024 / 1024
        )
        FROM gv$pgastat p
        WHERE p.inst_id = i.inst_id
    ), 0) AS pga_mb,
    NVL((SELECT ROUND(space_used*100/NULLIF(space_limit,0)) FROM v$recovery_file_dest WHERE ROWNUM = 1), 0) AS fra_pct
FROM gv$instance i
CROSS JOIN v$database d
ORDER BY i.inst_id;
`

const sqlPDBsQuery = `
SELECT
    con_id,
    name,
    open_mode,
    NVL(restricted, 'NO'),
    ROUND(total_size/1024/1024/1024, 1) AS size_gb,
    LISTAGG(inst_id, ',') WITHIN GROUP (ORDER BY inst_id) AS instances
FROM gv$pdbs
GROUP BY con_id, name, open_mode, restricted, ROUND(total_size/1024/1024/1024, 1)
ORDER BY con_id;
`

const sqlDataGuardStatsQuery = `
SELECT
    name,
    NVL(value, '0 seconds')
FROM v$dataguard_stats
WHERE name IN ('transport lag', 'apply lag');
`

// ParseInstanceRows converts pipe-delimited SQL rows into InstanceInfo structs.
func ParseInstanceRows(rows [][]string) []model.InstanceInfo {
	var list []model.InstanceInfo
	for _, r := range rows {
		if len(r) < 15 {
			continue
		}
		instID, _ := strconv.Atoi(r[0])
		sgaMB, _ := strconv.Atoi(r[12])
		pgaMB, _ := strconv.Atoi(r[13])
		fraPct, _ := strconv.Atoi(r[14])
		list = append(list, model.InstanceInfo{
			InstID:       instID,
			InstName:     r[1],
			DBName:       r[2],
			DBUniqueName: r[3],
			NodeName:     r[4],
			Version:      r[5],
			Status:       r[6],
			OpenMode:     r[7],
			Role:         r[8],
			LogMode:      r[9],
			FlashbackOn:  r[10],
			Uptime:       r[11],
			SGAMB:        sgaMB,
			PGAMB:        pgaMB,
			FRAUsedPct:   fraPct,
			DGLag:        "0s",
		})
	}
	return list
}

// ParsePDBRows converts pipe-delimited SQL rows into PDBInfo structs.
func ParsePDBRows(rows [][]string) []model.PDBInfo {
	var pdbs []model.PDBInfo
	for _, r := range rows {
		if len(r) < 6 {
			continue
		}
		conID, _ := strconv.Atoi(r[0])
		sizeGB, _ := strconv.ParseFloat(r[4], 64)
		pdbs = append(pdbs, model.PDBInfo{
			ConID:      conID,
			Name:       r[1],
			OpenMode:   r[2],
			Restricted: r[3],
			SizeGB:     sizeGB,
			InstName:   r[5],
		})
	}
	return pdbs
}

// ParseSrvctlServiceStatus parses output of `srvctl status service -d <db>`.
// Example:
// Service svc_erp is running on instance(s) PRODCDB1,PRODCDB2
// Service svc_bi is not running.
func ParseSrvctlServiceStatus(dbName, raw string) []model.RACService {
	var svcs []model.RACService
	for _, line := range strings.Split(raw, "\n") {
		line = strings.TrimSpace(line)
		if !strings.HasPrefix(line, "Service ") {
			continue
		}
		fields := strings.Fields(line)
		if len(fields) < 4 {
			continue
		}
		svcName := fields[1]
		if strings.Contains(line, "is running on") {
			idx := strings.LastIndex(line, " ")
			activeInst := ""
			if idx > 0 {
				activeInst = strings.TrimSpace(line[idx+1:])
			}
			svcs = append(svcs, model.RACService{
				Name:       svcName,
				DBName:     dbName,
				State:      "ONLINE",
				Preferred:  activeInst,
				Available:  "-",
				ActiveInst: activeInst,
			})
		} else {
			svcs = append(svcs, model.RACService{
				Name:       svcName,
				DBName:     dbName,
				State:      "OFFLINE",
				Preferred:  "-",
				Available:  "-",
				ActiveInst: "NONE",
			})
		}
	}
	return svcs
}

// CollectDatabase queries SQL*Plus and srvctl for database, PDB, service, and Data Guard metrics
// using a single batched SQL*Plus connection to minimize OS process forks and PGA memory.
func CollectDatabase(ctx context.Context, runner *executor.Runner, env *discovery.DiscoveredEnvironment, sid string) ([]model.InstanceInfo, []model.PDBInfo, []model.RACService, []model.DataGuardStandby) {
	if env.OracleHome == "" || sid == "" {
		return nil, nil, nil, nil
	}

	batchRes, err := runner.RunSQLPlusBatch(ctx, env.OracleHome, sid, env.OracleOwner, []executor.BatchSQLQuery{
		{Key: "instances", SQL: sqlInstancesQuery},
		{Key: "pdbs", SQL: sqlPDBsQuery},
		{Key: "dg_stats", SQL: sqlDataGuardStatsQuery},
	})
	if err != nil && len(batchRes) == 0 {
		return nil, nil, nil, nil
	}

	instances := ParseInstanceRows(batchRes["instances"])
	if len(instances) == 0 {
		return nil, nil, nil, nil
	}
	pdbs := ParsePDBRows(batchRes["pdbs"])
	dgRows := batchRes["dg_stats"]
	transportLag := "0 seconds"
	applyLag := "0 seconds"
	for _, r := range dgRows {
		if len(r) >= 2 {
			if strings.EqualFold(r[0], "transport lag") {
				transportLag = r[1]
			} else if strings.EqualFold(r[0], "apply lag") {
				applyLag = r[1]
			}
		}
	}
	for i := range instances {
		instances[i].DGLag = fmt.Sprintf("T:%s / A:%s", transportLag, applyLag)
	}

	var services []model.RACService
	if len(instances) > 0 && env.Topology != model.ModeSingle {
		dbName := instances[0].DBUniqueName
		if dbName == "" {
			dbName = instances[0].DBName
		}
		srvctlBin := filepath.Join(env.OracleHome, "bin", "srvctl")
		if out, err := runner.RunCommand(ctx, env.OracleOwner, false, map[string]string{
			"ORACLE_HOME": env.OracleHome,
		}, nil, srvctlBin, "status", "service", "-d", dbName); err == nil && out != "" {
			services = ParseSrvctlServiceStatus(dbName, out)
		}
	}

	var standbys []model.DataGuardStandby
	if len(instances) > 0 {
		standbys = append(standbys, model.DataGuardStandby{
			DBUniqueName: instances[0].DBUniqueName,
			Role:         instances[0].Role,
			Host:         instances[0].NodeName,
			OpenMode:     instances[0].OpenMode,
			TransportLag: transportLag,
			ApplyLag:     applyLag,
			MRPStatus:    "ACTIVE",
			Switchover:   "READY",
		})
	}

	return instances, pdbs, services, standbys
}

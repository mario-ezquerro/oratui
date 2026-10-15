package collector

import (
	"time"

	"github.com/mario/ora-tui/internal/model"
)

func NewMockState(topology model.TopologyMode) *model.EnvironmentState {
	if topology == "" {
		topology = model.ModeRAC
	}

	state := &model.EnvironmentState{
		Topology:       topology,
		BorderMode:     model.BorderMixedUnicode,
		HostName:       "ora-rac01.corp.local",
		ClusterName:    "crs-prod-cluster",
		OracleHome:     "/u01/app/oracle/product/19.0.0/dbhome_1",
		OracleOwner:    "oracle (uid=54321)",
		GridHome:       "/u01/app/19.0.0/grid",
		GridOwner:      "grid (uid=54322)",
		ReleaseVersion: "19.24.0.0.0 (RU Oct 2024)",
		IsMock:         true,
		LastUpdated:    time.Now(),
		Nodes: []model.ClusterNode{
			{
				ID:           1,
				Name:         "ora-rac01",
				State:        "ONLINE",
				Role:         "HUB",
				VIP:          "10.20.30.111 (ONLINE)",
				VIPState:     "ONLINE",
				Interconnect: "bond1: 192.168.10.1 (OK)",
				CRS:          "ONLINE",
				CSSD:         "ONLINE",
				EVMD:         "ONLINE",
				ASM:          "ONLINE (+ASM1)",
			},
			{
				ID:           2,
				Name:         "ora-rac02",
				State:        "ONLINE",
				Role:         "HUB",
				VIP:          "10.20.30.112 (ONLINE)",
				VIPState:     "ONLINE",
				Interconnect: "bond1: 192.168.10.2 (OK)",
				CRS:          "ONLINE",
				CSSD:         "ONLINE",
				EVMD:         "ONLINE",
				ASM:          "ONLINE (+ASM2)",
			},
		},
		Instances: []model.InstanceInfo{
			{
				InstID:       1,
				InstName:     "PRODCDB1",
				DBName:       "PRODCDB",
				DBUniqueName: "PRODCDB_SITE1",
				NodeName:     "ora-rac01",
				Version:      "19.24.0.0.0",
				Status:       "OPEN",
				OpenMode:     "READ WRITE",
				Role:         "PRIMARY",
				LogMode:      "ARCHIVELOG",
				FlashbackOn:  "YES",
				Uptime:       "42d 14h 22m",
				SGAMB:        32768,
				PGAMB:        8192,
				FRAUsedPct:   48,
				DGLag:        "0s (SYNC)",
			},
			{
				InstID:       2,
				InstName:     "PRODCDB2",
				DBName:       "PRODCDB",
				DBUniqueName: "PRODCDB_SITE1",
				NodeName:     "ora-rac02",
				Version:      "19.24.0.0.0",
				Status:       "OPEN",
				OpenMode:     "READ WRITE",
				Role:         "PRIMARY",
				LogMode:      "ARCHIVELOG",
				FlashbackOn:  "YES",
				Uptime:       "42d 14h 19m",
				SGAMB:        32768,
				PGAMB:        8192,
				FRAUsedPct:   48,
				DGLag:        "0s (SYNC)",
			},
		},
		PDBs: []model.PDBInfo{
			{ConID: 2, Name: "PDB$SEED", OpenMode: "READ ONLY", Restricted: "NO", SizeGB: 1.2, InstName: "ALL"},
			{ConID: 3, Name: "ERP_PROD", OpenMode: "READ WRITE", Restricted: "NO", SizeGB: 412.5, InstName: "PRODCDB1,2"},
			{ConID: 4, Name: "CRM_PROD", OpenMode: "READ WRITE", Restricted: "NO", SizeGB: 185.0, InstName: "PRODCDB1,2"},
			{ConID: 5, Name: "BI_STAGE", OpenMode: "MOUNTED", Restricted: "NO", SizeGB: 94.8, InstName: "PRODCDB1"},
		},
		Services: []model.RACService{
			{Name: "svc_erp_oltp", DBName: "PRODCDB", State: "ONLINE", Preferred: "PRODCDB1,PRODCDB2", Available: "-", ActiveInst: "PRODCDB1,PRODCDB2", PDB: "ERP_PROD"},
			{Name: "svc_crm_app", DBName: "PRODCDB", State: "ONLINE", Preferred: "PRODCDB1", Available: "PRODCDB2", ActiveInst: "PRODCDB1", PDB: "CRM_PROD"},
			{Name: "svc_bi_batch", DBName: "PRODCDB", State: "OFFLINE", Preferred: "PRODCDB2", Available: "PRODCDB1", ActiveInst: "NONE", PDB: "BI_STAGE"},
		},
		Sessions: []model.UserSession{
			{
				InstID:          1,
				SID:             142,
				Serial:          48219,
				Username:        "ERP_APP",
				Status:          "ACTIVE",
				Schema:          "ERP_CORE",
				Machine:         "app-srv-01.corp",
				Program:         "JDBC Thin Client",
				SQLID:           "8f4k2m91x7p0q",
				WaitClass:       "User I/O",
				WaitEvent:       "db file sequential read",
				ElapsedSec:      14,
				BlockingSession: "-",
				LogonTime:       "06:12:04",
				PGAMB:           64.5,
				SQLText:         "SELECT /*+ INDEX(o ORD_PK) */ o.order_id, o.customer_id, l.amount FROM erp_orders o JOIN erp_order_lines l ON o.order_id = l.order_id WHERE o.status = :1",
			},
			{
				InstID:          1,
				SID:             289,
				Serial:          11042,
				Username:        "CRM_BATCH",
				Status:          "ACTIVE",
				Schema:          "CRM_DATA",
				Machine:         "batch-02.corp",
				Program:         "python3@batch-02",
				SQLID:           "3c9a11b04z88w",
				WaitClass:       "Application",
				WaitEvent:       "enq: TX - row lock contention",
				ElapsedSec:      342,
				BlockingSession: "INST:2 SID:512",
				LogonTime:       "05:50:19",
				PGAMB:           128.0,
				SQLText:         "UPDATE crm_customers SET credit_status = 'HOLD', updated_at = SYSDATE WHERE customer_id = :b1",
			},
			{
				InstID:          2,
				SID:             512,
				Serial:          9931,
				Username:        "CRM_ADMIN",
				Status:          "INACTIVE",
				Schema:          "CRM_DATA",
				Machine:         "dba-wks-04.corp",
				Program:         "sqlplus@dba-wks-04",
				SQLID:           "0000000000000",
				WaitClass:       "Idle",
				WaitEvent:       "SQL*Net message from client",
				ElapsedSec:      610,
				BlockingSession: "BLOCKER (1)",
				LogonTime:       "05:45:00",
				PGAMB:           18.2,
				SQLText:         "UPDATE crm_customers SET credit_limit = credit_limit * 1.1 WHERE region_id = 10 /* UNCOMMITTED TX */",
			},
			{
				InstID:          2,
				SID:             604,
				Serial:          33105,
				Username:        "BI_REPORT",
				Status:          "ACTIVE",
				Schema:          "ERP_CORE",
				Machine:         "bi-node-01.corp",
				Program:         "obi-server.bin",
				SQLID:           "7p2v90n1k4m2a",
				WaitClass:       "CPU",
				WaitEvent:       "ON CPU",
				ElapsedSec:      89,
				BlockingSession: "-",
				LogonTime:       "07:10:33",
				PGAMB:           512.4,
				SQLText:         "SELECT region, TRUNC(order_date,'MM'), SUM(total_amount) FROM erp_orders GROUP BY CUBE(region, TRUNC(order_date,'MM'))",
			},
			{
				InstID:          1,
				SID:             77,
				Serial:          5012,
				Username:        "SYS",
				Status:          "INACTIVE",
				Schema:          "SYS",
				Machine:         "ora-rac01.corp",
				Program:         "rman@ora-rac01",
				SQLID:           "-",
				WaitClass:       "Idle",
				WaitEvent:       "SQL*Net message from client",
				ElapsedSec:      1820,
				BlockingSession: "-",
				LogonTime:       "04:00:01",
				PGAMB:           32.0,
				SQLText:         "-- RMAN maintenance channel idle",
			},
		},
		Tablespaces: []model.TablespaceInfo{
			{Name: "SYSTEM", Type: "PERMANENT", Status: "ONLINE", TotalMB: 4096, UsedMB: 3120, FreeMB: 976, UsedPct: 76.2, AutoExtend: "YES", Datafiles: 1},
			{Name: "SYSAUX", Type: "PERMANENT", Status: "ONLINE", TotalMB: 8192, UsedMB: 6910, FreeMB: 1282, UsedPct: 84.4, AutoExtend: "YES", Datafiles: 2},
			{Name: "UNDOTBS1", Type: "UNDO", Status: "ONLINE", TotalMB: 16384, UsedMB: 4210, FreeMB: 12174, UsedPct: 25.7, AutoExtend: "YES", Datafiles: 1},
			{Name: "UNDOTBS2", Type: "UNDO", Status: "ONLINE", TotalMB: 16384, UsedMB: 3890, FreeMB: 12494, UsedPct: 23.7, AutoExtend: "YES", Datafiles: 1},
			{Name: "ERP_DATA_TBS", Type: "PERMANENT", Status: "ONLINE", TotalMB: 327680, UsedMB: 304742, FreeMB: 22938, UsedPct: 93.0, AutoExtend: "NO", Datafiles: 8},
			{Name: "ERP_IDX_TBS", Type: "PERMANENT", Status: "ONLINE", TotalMB: 131072, UsedMB: 108789, FreeMB: 22283, UsedPct: 83.0, AutoExtend: "YES", Datafiles: 4},
			{Name: "CRM_DATA_TBS", Type: "PERMANENT", Status: "ONLINE", TotalMB: 163840, UsedMB: 99942, FreeMB: 63898, UsedPct: 61.0, AutoExtend: "YES", Datafiles: 4},
			{Name: "TEMP", Type: "TEMPORARY", Status: "ONLINE", TotalMB: 32768, UsedMB: 11468, FreeMB: 21300, UsedPct: 35.0, AutoExtend: "YES", Datafiles: 2},
		},
		Datafiles: []model.DatafileInfo{
			{FileID: 1, FileName: "+DATA/PRODCDB/DATAFILE/system.256.1120", Tablespace: "SYSTEM", SizeMB: 4096, MaxSizeMB: 32768, AutoExtend: "YES", Status: "ONLINE"},
			{FileID: 2, FileName: "+DATA/PRODCDB/DATAFILE/sysaux.257.1120", Tablespace: "SYSAUX", SizeMB: 8192, MaxSizeMB: 32768, AutoExtend: "YES", Status: "ONLINE"},
			{FileID: 10, FileName: "+DATA/PRODCDB/ERP_PROD/erp_data01.dbf", Tablespace: "ERP_DATA_TBS", SizeMB: 32768, MaxSizeMB: 32768, AutoExtend: "NO", Status: "ONLINE"},
			{FileID: 11, FileName: "+DATA/PRODCDB/ERP_PROD/erp_data02.dbf", Tablespace: "ERP_DATA_TBS", SizeMB: 32768, MaxSizeMB: 32768, AutoExtend: "NO", Status: "ONLINE"},
			{FileID: 18, FileName: "+DATA/PRODCDB/CRM_PROD/crm_data01.dbf", Tablespace: "CRM_DATA_TBS", SizeMB: 32768, MaxSizeMB: 65536, AutoExtend: "YES", Status: "ONLINE"},
		},
		TableSegments: []model.TableSegment{
			{Owner: "ERP_CORE", SegmentName: "ERP_ORDER_LINES", SegmentType: "TABLE PARTITION", Tablespace: "ERP_DATA_TBS", SizeMB: 118420, NumRows: 482910400, LastAnalyzed: "2026-10-14 22:00", StatsStatus: "FRESH", Fragmented: "LOW (4%)"},
			{Owner: "ERP_CORE", SegmentName: "ERP_ORDERS", SegmentType: "TABLE", Tablespace: "ERP_DATA_TBS", SizeMB: 64210, NumRows: 95120300, LastAnalyzed: "2026-09-28 02:15", StatsStatus: "STALE", Fragmented: "HIGH (29%)"},
			{Owner: "ERP_CORE", SegmentName: "ERP_AUDIT_LOG", SegmentType: "TABLE", Tablespace: "ERP_DATA_TBS", SizeMB: 52800, NumRows: 210450000, LastAnalyzed: "2026-08-10 01:00", StatsStatus: "STALE", Fragmented: "HIGH (34%)"},
			{Owner: "CRM_DATA", SegmentName: "CRM_CUSTOMERS", SegmentType: "TABLE", Tablespace: "CRM_DATA_TBS", SizeMB: 31240, NumRows: 18400210, LastAnalyzed: "2026-10-15 01:30", StatsStatus: "FRESH", Fragmented: "LOW (6%)"},
			{Owner: "CRM_DATA", SegmentName: "CRM_INTERACTIONS", SegmentType: "TABLE", Tablespace: "CRM_DATA_TBS", SizeMB: 44120, NumRows: 74910000, LastAnalyzed: "2026-10-01 03:00", StatsStatus: "STALE", Fragmented: "MED (16%)"},
			{Owner: "ERP_CORE", SegmentName: "IDX_ORDERS_CUST_DT", SegmentType: "INDEX", Tablespace: "ERP_IDX_TBS", SizeMB: 28900, NumRows: 95120300, LastAnalyzed: "2026-09-28 02:20", StatsStatus: "STALE", Fragmented: "HIGH (31%)"},
		},
		InvalidObjects: []model.InvalidObject{
			{Owner: "ERP_CORE", ObjectName: "PKG_INVOICE_BATCH", ObjectType: "PACKAGE BODY", Status: "INVALID", LastDDL: "2026-10-14 19:42"},
			{Owner: "ERP_CORE", ObjectName: "VW_MONTHLY_REVENUE", ObjectType: "VIEW", Status: "INVALID", LastDDL: "2026-10-14 19:42"},
			{Owner: "CRM_DATA", ObjectName: "TRG_CUST_AUDIT", ObjectType: "TRIGGER", Status: "INVALID", LastDDL: "2026-10-13 11:15"},
		},
		Diskgroups: []model.ASMDiskgroup{
			{Name: "+CRS_VOTE", State: "MOUNTED", Type: "HIGH", TotalMB: 61440, FreeMB: 52100, UsableMB: 17366, UsedPct: 15.2, OfflineDisks: 0},
			{Name: "+DATA", State: "MOUNTED", Type: "NORMAL", TotalMB: 2097152, FreeMB: 335544, UsableMB: 167772, UsedPct: 84.0, OfflineDisks: 0},
			{Name: "+FRA", State: "MOUNTED", Type: "EXTERN", TotalMB: 1048576, FreeMB: 545259, UsableMB: 545259, UsedPct: 48.0, OfflineDisks: 0},
			{Name: "+REDO", State: "MOUNTED", Type: "HIGH", TotalMB: 131072, FreeMB: 98304, UsableMB: 32768, UsedPct: 25.0, OfflineDisks: 0},
		},
		Listeners: []model.ListenerInfo{
			{Name: "LISTENER", Type: "LOCAL", Node: "ora-rac01", State: "ONLINE", Endpoint: "TCP:1521", Services: 6},
			{Name: "LISTENER", Type: "LOCAL", Node: "ora-rac02", State: "ONLINE", Endpoint: "TCP:1521", Services: 5},
			{Name: "LISTENER_SCAN1", Type: "SCAN", Node: "ora-rac01", State: "ONLINE", Endpoint: "TCP:1521 (10.20.30.121)", Services: 6},
			{Name: "LISTENER_SCAN2", Type: "SCAN", Node: "ora-rac02", State: "ONLINE", Endpoint: "TCP:1521 (10.20.30.122)", Services: 6},
			{Name: "LISTENER_SCAN3", Type: "SCAN", Node: "ora-rac02", State: "ONLINE", Endpoint: "TCP:1521 (10.20.30.123)", Services: 6},
		},
		RMANBackups: []model.RMANBackupJob{
			{SessionKey: 1409, InputType: "DB INCR L0 (FULL)", Status: "COMPLETED", StartTime: "2026-10-14 23:00", Duration: "01h 12m", SizeGB: 498.4, DeviceType: "SBT_TAPE"},
			{SessionKey: 1412, InputType: "DB INCR L1", Status: "COMPLETED", StartTime: "2026-10-15 03:00", Duration: "00h 18m", SizeGB: 42.1, DeviceType: "DISK (+FRA)"},
			{SessionKey: 1415, InputType: "ARCHIVELOG", Status: "COMPLETED", StartTime: "2026-10-15 07:00", Duration: "00h 04m", SizeGB: 14.8, DeviceType: "DISK (+FRA)"},
			{SessionKey: 1402, InputType: "ARCHIVELOG", Status: "FAILED", StartTime: "2026-10-13 18:00", Duration: "00h 02m", SizeGB: 0.0, DeviceType: "SBT_TAPE"},
		},
		RestorePoints: []model.RestorePoint{
			{Name: "GRP_PRE_RELEASE_2026_10", SCN: 94810294812, Time: "2026-10-14 18:30:00", Guaranteed: "YES", StorageSize: "18.4 GB (+FRA)"},
			{Name: "RP_BEFORE_BATCH_ERP", SCN: 94819910200, Time: "2026-10-15 05:00:00", Guaranteed: "NO", StorageSize: "0.0 GB"},
		},
		StandbyDBs: []model.DataGuardStandby{
			{
				DBUniqueName: "PRODCDB_DR",
				Role:         "PHYSICAL STANDBY",
				Host:         "ora-dr01.corp.local",
				OpenMode:     "READ ONLY WITH APPLY",
				TransportLag: "0 seconds",
				ApplyLag:     "0 seconds",
				MRPStatus:    "APPLYING_LOG (ACTIVE)",
				Switchover:   "TO STANDBY (READY)",
			},
		},
		AlertEntries: []model.AlertLogEntry{
			{Timestamp: "2026-10-15 07:41:12", Instance: "PRODCDB1", Severity: "CRITICAL", ErrorCode: "ORA-01654", Message: "unable to extend index ERP_CORE.IDX_ORDERS_CUST_DT by 1024 in tablespace ERP_DATA_TBS"},
			{Timestamp: "2026-10-15 07:38:05", Instance: "PRODCDB1", Severity: "WARNING", ErrorCode: "ORA-00060", Message: "deadlock detected while waiting for resource (SID 289 / SID 512)"},
			{Timestamp: "2026-10-15 07:15:00", Instance: "PRODCDB2", Severity: "INFO", ErrorCode: "ARC-LOG", Message: "Thread 2 advanced to log sequence 84291 (LGWR switch)"},
			{Timestamp: "2026-10-15 06:30:44", Instance: "PRODCDB1", Severity: "WARNING", ErrorCode: "ORA-3136", Message: "inbound connection timed out (ORA-3136) from client 10.20.88.14"},
			{Timestamp: "2026-10-15 03:18:22", Instance: "PRODCDB1", Severity: "INFO", ErrorCode: "RMAN-BKP", Message: "Starting backup of datafile 1..18 (Incremental Level 1)"},
		},
	}

	if topology == model.ModeSingle {
		state.HostName = "ora-single01.corp.local"
		state.ClusterName = "N/A (Single Instance)"
		state.GridHome = "N/A (No Grid Infrastructure)"
		state.GridOwner = "N/A"
		state.Nodes = []model.ClusterNode{
			{
				ID:           1,
				Name:         "ora-single01",
				State:        "ONLINE",
				Role:         "STANDALONE",
				VIP:          "N/A",
				VIPState:     "N/A",
				Interconnect: "N/A",
				CRS:          "N/A",
				CSSD:         "N/A",
				EVMD:         "N/A",
				ASM:          "N/A (Filesystem)",
			},
		}
		state.Instances = state.Instances[:1]
		state.Instances[0].InstName = "ORCL"
		state.Instances[0].DBName = "ORCL"
		state.Instances[0].NodeName = "ora-single01"
		state.Listeners = state.Listeners[:1]
		state.Listeners[0].Node = "ora-single01"
	}

	return state
}

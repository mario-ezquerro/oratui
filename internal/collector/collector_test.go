package collector

import (
	"testing"

	"github.com/mario/ora-tui/internal/executor"
)

func TestParseOlsnodesAndCRS(t *testing.T) {
	olsnodesOut := `ora-rac01	1	Active	Unpinned	Hub
ora-rac02	2	Active	Unpinned	Hub`
	nodes := ParseOlsnodes(olsnodesOut)
	if len(nodes) != 2 {
		t.Fatalf("expected 2 nodes, got %d", len(nodes))
	}
	if nodes[0].Name != "ora-rac01" || nodes[1].ID != 2 {
		t.Fatalf("unexpected nodes: %+v", nodes)
	}

	crsCheckOut := `
**************************************************************
ora-rac01:
CRS-4537: Cluster Ready Services is online
CRS-4529: Cluster Synchronization Services is online
CRS-4533: Event Manager is online
**************************************************************
ora-rac02:
CRS-4537: Cluster Ready Services is offline
CRS-4529: Cluster Synchronization Services is online
CRS-4533: Event Manager is online
`
	nodes = ParseCRSCheckAll(crsCheckOut, nodes)
	if nodes[0].CRS != "ONLINE" || nodes[1].CRS != "OFFLINE" {
		t.Fatalf("unexpected CRS status: %+v", nodes)
	}
}

func TestParsePipeDelimitedSQLPlus(t *testing.T) {
	raw := `
1|PRODCDB1|PRODCDB|PRODCDB_SITE1|ora-rac01|19.24.0.0.0|OPEN|READ WRITE|PRIMARY|ARCHIVELOG|YES|42d 14h|32768|8192|48
`
	rows := executor.ParsePipeDelimitedOutput(raw)
	insts := ParseInstanceRows(rows)
	if len(insts) != 1 || insts[0].InstName != "PRODCDB1" || insts[0].FRAUsedPct != 48 {
		t.Fatalf("unexpected parsed instance: %+v", insts)
	}
}

func TestParseTablespaceRowsAutoextend(t *testing.T) {
	raw := `
USERS|PERMANENT|ONLINE|19456|32768|18350|14418|56.0|YES|1
`
	rows := executor.ParsePipeDelimitedOutput(raw)
	tbs := ParseTablespaceRows(rows)
	if len(tbs) != 1 {
		t.Fatalf("expected 1 tablespace, got %d", len(tbs))
	}
	if tbs[0].AllocMB != 19456 || tbs[0].TotalMB != 32768 || tbs[0].UsedMB != 18350 || tbs[0].FreeMB != 14418 || tbs[0].UsedPct != 56.0 {
		t.Fatalf("unexpected tablespace calculation: %+v", tbs[0])
	}
}

func TestParseLsnrctlStatus(t *testing.T) {
	raw := `
STATUS of the LISTENER
Listening Endpoints Summary...
  (DESCRIPTION=(ADDRESS=(PROTOCOL=tcp)(HOST=servidor-local)(PORT=1521)))
Services Summary...
Service "servidor-local" has 2 instance(s).
  Instance "servidor-local", status UNKNOWN, has 1 handler(s) for this service...
  Instance "servidor-local", status READY, has 1 handler(s) for this service...
Service "servidor-localXDB" has 1 instance(s).
  Instance "servidor-local", status READY, has 1 handler(s) for this service...
The command completed successfully
`
	info := ParseLsnrctlStatus("LISTENER", "servidor-local", raw)
	if info.State != "ONLINE" || info.ReadyCount != 2 || info.UnknownCount != 1 || len(info.ServiceList) != 3 {
		t.Fatalf("unexpected listener info: %+v", info)
	}
}

func TestParseMultipathAndDF(t *testing.T) {
	mpathRaw := "ORA_DATA_01 (3624a9370a1b2c3d400010002) dm-3 PURE,FlashArray\n" +
		"size=500G features='0' hwhandler='1 alua' wp=rw\n" +
		"|-+- policy='service-time 0' prio=50 status=active\n" +
		"| |- 1:0:0:1 sdb 8:16 active ready running\n" +
		"| `- 2:0:0:1 sdc 8:32 active ready running\n" +
		"`-+- policy='service-time 0' prio=10 status=enabled\n" +
		"  |- 1:0:1:1 sdd 8:48 active ready running\n" +
		"  `- 2:0:1:1 sde 8:64 failed faulty offline\n"
	paths := ParseMultipathOutput(mpathRaw)
	if len(paths) != 1 {
		t.Fatalf("expected 1 multipath LUN, got %d", len(paths))
	}
	if paths[0].Status != "WARNING" || paths[0].PathsSummary != "3/4 DEGRADED (1 DOWN)" || paths[0].Size != "500G" {
		t.Fatalf("unexpected multipath parse result: %+v", paths[0])
	}

	dfBlocks := `Filesystem                Type  1048576-blocks   Used Available Capacity Mounted on
/dev/mapper/vg_ora-lv_u01 xfs           204800 147456     57344      72% /u01
tmpfs                     tmpfs          65536   4096     61440       7% /dev/shm
`
	dfInodes := `Filesystem                 Inodes  IUsed   IFree IUse% Mounted on
/dev/mapper/vg_ora-lv_u01 1048576 192938  855638   19% /u01
tmpfs                     8388608    120 8388488    1% /dev/shm
`
	mounts := ParseDFOutput(dfBlocks, dfInodes)
	if len(mounts) != 2 || mounts[0].MountPoint != "/u01" || mounts[0].InodeUsedPct != 19 {
		t.Fatalf("unexpected df parse result: %+v", mounts)
	}
}



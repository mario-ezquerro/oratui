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
  (DESCRIPTION=(ADDRESS=(PROTOCOL=tcp)(HOST=srv-dbgriar-des)(PORT=1521)))
Services Summary...
Service "griar" has 2 instance(s).
  Instance "griar", status UNKNOWN, has 1 handler(s) for this service...
  Instance "griar", status READY, has 1 handler(s) for this service...
Service "griarXDB" has 1 instance(s).
  Instance "griar", status READY, has 1 handler(s) for this service...
The command completed successfully
`
	info := ParseLsnrctlStatus("LISTENER", "srv-dbgriar-des", raw)
	if info.State != "ONLINE" || info.ReadyCount != 2 || info.UnknownCount != 1 || len(info.ServiceList) != 3 {
		t.Fatalf("unexpected listener info: %+v", info)
	}
}



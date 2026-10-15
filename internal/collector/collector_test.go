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

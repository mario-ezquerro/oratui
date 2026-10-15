package discovery

import (
	"testing"

	"github.com/mario/ora-tui/internal/model"
)

func TestParseOCRLocContent(t *testing.T) {
	racContent := `
#Device/file getting replaced by device +CRS_VOTE
ocrconfig_loc=+CRS_VOTE
local_only=FALSE
`
	if got := ParseOCRLocContent(racContent); got != model.ModeRAC {
		t.Fatalf("expected ModeRAC, got %v", got)
	}

	restartContent := `
ocrconfig_loc=+DATA
local_only=TRUE
`
	if got := ParseOCRLocContent(restartContent); got != model.ModeRestart {
		t.Fatalf("expected ModeRestart, got %v", got)
	}
}

func TestParseOratabContent(t *testing.T) {
	raw := `
# Multiple entries in /etc/oratab
+ASM1:/u01/app/19.0.0/grid:N
PRODCDB:/u01/app/oracle/product/19.0.0/dbhome_1:Y
`
	entries := ParseOratabContent(raw)
	if len(entries) != 2 {
		t.Fatalf("expected 2 entries, got %d", len(entries))
	}
	if entries[0].SID != "+ASM1" || entries[1].SID != "PRODCDB" || entries[1].AutoStart != "Y" {
		t.Fatalf("unexpected parsed entries: %+v", entries)
	}
}

func TestExtractSIDFromPMON(t *testing.T) {
	sid, isASM := ExtractSIDFromPMON("ora_pmon_PRODCDB1")
	if sid != "PRODCDB1" || isASM {
		t.Fatalf("expected PRODCDB1/false, got %s/%v", sid, isASM)
	}
	sid, isASM = ExtractSIDFromPMON("db_pmon_FREE")
	if sid != "FREE" || isASM {
		t.Fatalf("expected FREE/false, got %s/%v", sid, isASM)
	}
	sid, isASM = ExtractSIDFromPMON("asm_pmon_+ASM1")
	if sid != "+ASM1" || !isASM {
		t.Fatalf("expected +ASM1/true, got %s/%v", sid, isASM)
	}
}

func TestParseVersionString(t *testing.T) {
	raw := "SQL*Plus: Release 19.0.0.0.0 - Production\nVersion 19.24.0.0.0\n"
	if got := ParseVersionString(raw); got != "19.24.0.0.0" {
		t.Fatalf("expected 19.24.0.0.0, got %s", got)
	}
}

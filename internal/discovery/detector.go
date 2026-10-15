package discovery

import (
	"bufio"
	"encoding/xml"
	"os"
	"path/filepath"
	"strings"

	"github.com/mario/ora-tui/internal/executor"
	"github.com/mario/ora-tui/internal/model"
)

// OratabEntry represents a parsed line from /etc/oratab.
type OratabEntry struct {
	SID        string
	OracleHome string
	AutoStart  string
}

// DiscoveredEnvironment holds auto-detected paths, owners, and running PMON SIDs.
type DiscoveredEnvironment struct {
	Topology    model.TopologyMode
	HostName    string
	Oratab      []OratabEntry
	RunningSIDs []string
	ASMSID      string
	OracleHome  string
	OracleOwner *executor.OwnerCreds
	GridHome    string
	GridOwner   *executor.OwnerCreds
}

// DetectTopology inspects ocr.loc and olr.loc to determine RAC, Restart, or Single Instance mode.
func DetectTopology(ocrLocPath, olrLocPath string) model.TopologyMode {
	if ocrLocPath == "" {
		ocrLocPath = "/etc/oracle/ocr.loc"
	}
	if olrLocPath == "" {
		olrLocPath = "/etc/oracle/olr.loc"
	}

	if data, err := os.ReadFile(ocrLocPath); err == nil {
		return ParseOCRLocContent(string(data))
	}
	if _, err := os.Stat(olrLocPath); err == nil {
		return model.ModeRestart
	}
	return model.ModeSingle
}

// ParseOCRLocContent parses the contents of /etc/oracle/ocr.loc.
func ParseOCRLocContent(content string) model.TopologyMode {
	for _, line := range strings.Split(content, "\n") {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "#") || line == "" {
			continue
		}
		kv := strings.SplitN(line, "=", 2)
		if len(kv) == 2 && strings.TrimSpace(strings.ToLower(kv[0])) == "local_only" {
			val := strings.TrimSpace(strings.ToLower(kv[1]))
			if val == "true" {
				return model.ModeRestart
			}
			if val == "false" {
				return model.ModeRAC
			}
		}
	}
	return model.ModeRAC
}

// ParseOratabContent parses /etc/oratab content ignoring comments, wildcard '*', and empty lines.
func ParseOratabContent(content string) []OratabEntry {
	var entries []OratabEntry
	scanner := bufio.NewScanner(strings.NewReader(content))
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		parts := strings.Split(line, ":")
		if len(parts) >= 2 {
			sid := strings.TrimSpace(parts[0])
			home := strings.TrimSpace(parts[1])
			auto := "N"
			if len(parts) >= 3 {
				auto = strings.TrimSpace(parts[2])
			}
			if sid != "" && sid != "*" && home != "" {
				entries = append(entries, OratabEntry{
					SID:        sid,
					OracleHome: home,
					AutoStart:  auto,
				})
			}
		}
	}
	return entries
}

// ScanProcPMON scans /proc/[0-9]*/cmdline for ora_pmon_<SID>, db_pmon_<SID>, and asm_pmon_+ASM*.
// It also attempts to resolve ORACLE_HOME from /proc/[pid]/exe if not found in /etc/oratab.
func ScanProcPMON(procRoot string) (dbSIDs []string, asmSID string, detectedDBHome string, detectedGridHome string) {
	if procRoot == "" {
		procRoot = "/proc"
	}
	entries, err := os.ReadDir(procRoot)
	if err != nil {
		return nil, "", "", ""
	}
	seen := make(map[string]bool)
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		name := entry.Name()
		if name[0] < '0' || name[0] > '9' {
			continue
		}
		cmdlineBytes, err := os.ReadFile(filepath.Join(procRoot, name, "cmdline"))
		if err != nil || len(cmdlineBytes) == 0 {
			continue
		}
		cmdline := strings.ReplaceAll(string(cmdlineBytes), "\x00", " ")
		cmdline = strings.TrimSpace(cmdline)
		sid, isASM := ExtractSIDFromPMON(cmdline)
		if sid != "" {
			exePath, err := os.Readlink(filepath.Join(procRoot, name, "exe"))
			homeFromExe := ""
			if err == nil && strings.HasSuffix(exePath, "/bin/oracle") {
				homeFromExe = strings.TrimSuffix(exePath, "/bin/oracle")
			}
			if isASM {
				asmSID = sid
				if homeFromExe != "" && detectedGridHome == "" {
					detectedGridHome = homeFromExe
				}
			} else {
				if !seen[sid] {
					seen[sid] = true
					dbSIDs = append(dbSIDs, sid)
				}
				if homeFromExe != "" && detectedDBHome == "" {
					detectedDBHome = homeFromExe
				}
			}
		}
	}
	return dbSIDs, asmSID, detectedDBHome, detectedGridHome
}

// ExtractSIDFromPMON extracts the SID from a process command line like ora_pmon_ORCL1, db_pmon_FREE, or asm_pmon_+ASM1.
func ExtractSIDFromPMON(cmdline string) (sid string, isASM bool) {
	fields := strings.Fields(cmdline)
	if len(fields) == 0 {
		return "", false
	}
	proc := filepath.Base(fields[0])
	switch {
	case strings.HasPrefix(proc, "ora_pmon_"):
		return strings.TrimPrefix(proc, "ora_pmon_"), false
	case strings.HasPrefix(proc, "db_pmon_"):
		return strings.TrimPrefix(proc, "db_pmon_"), false
	case strings.HasPrefix(proc, "asm_pmon_"):
		return strings.TrimPrefix(proc, "asm_pmon_"), true
	}
	return "", false
}

type inventoryXML struct {
	HomeList struct {
		Homes []struct {
			Name    string `xml:"NAME,attr"`
			Loc     string `xml:"LOC,attr"`
			CRS     string `xml:"CRS,attr"`
			Removed string `xml:"REMOVED,attr"`
		} `xml:"HOME"`
	} `xml:"HOME_LIST"`
}

// ParseInventoryXML extracts GridHome and OracleHome paths from oraInventory/ContentsXML/inventory.xml.
func ParseInventoryXML(xmlContent []byte) (gridHome string, dbHomes []string) {
	var inv inventoryXML
	if err := xml.Unmarshal(xmlContent, &inv); err != nil {
		return "", nil
	}
	for _, h := range inv.HomeList.Homes {
		if strings.EqualFold(h.Removed, "T") {
			continue
		}
		if strings.EqualFold(h.CRS, "true") || strings.Contains(strings.ToLower(h.Name), "grid") || strings.Contains(strings.ToLower(h.Loc), "grid") {
			if gridHome == "" {
				gridHome = h.Loc
			}
		} else if h.Loc != "" {
			dbHomes = append(dbHomes, h.Loc)
		}
	}
	return gridHome, dbHomes
}

// Discover inspects the host OS to build a DiscoveredEnvironment.
func Discover() (*DiscoveredEnvironment, error) {
	host, _ := os.Hostname()
	topo := DetectTopology("/etc/oracle/ocr.loc", "/etc/oracle/olr.loc")

	env := &DiscoveredEnvironment{
		Topology: topo,
		HostName: host,
	}

	if data, err := os.ReadFile("/etc/oratab"); err == nil {
		env.Oratab = ParseOratabContent(string(data))
	}

	invPaths := []string{
		"/u01/app/oraInventory/ContentsXML/inventory.xml",
		"/u01/app/oracle/oraInventory/ContentsXML/inventory.xml",
	}
	for _, p := range invPaths {
		if xmlBytes, err := os.ReadFile(p); err == nil {
			gHome, dbHomes := ParseInventoryXML(xmlBytes)
			if gHome != "" && env.GridHome == "" {
				env.GridHome = gHome
			}
			if len(dbHomes) > 0 && env.OracleHome == "" {
				env.OracleHome = dbHomes[0]
			}
			break
		}
	}

	for _, entry := range env.Oratab {
		if strings.HasPrefix(entry.SID, "+ASM") || strings.HasPrefix(entry.SID, "-MGMTDB") {
			if env.GridHome == "" && strings.HasPrefix(entry.SID, "+ASM") {
				env.GridHome = entry.OracleHome
			}
			if strings.HasPrefix(entry.SID, "+ASM") {
				env.ASMSID = entry.SID
			}
		} else if env.OracleHome == "" {
			env.OracleHome = entry.OracleHome
		}
	}

	dbSIDs, asmSID, procDBHome, procGridHome := ScanProcPMON("/proc")
	env.RunningSIDs = dbSIDs
	if asmSID != "" {
		env.ASMSID = asmSID
	}
	if env.OracleHome == "" && procDBHome != "" {
		env.OracleHome = procDBHome
	}
	if env.GridHome == "" && procGridHome != "" {
		env.GridHome = procGridHome
	}

	if env.OracleHome != "" {
		if creds, err := executor.DetectFileOwner(env.OracleHome); err == nil {
			env.OracleOwner = creds
		}
	}
	if env.GridHome != "" {
		if creds, err := executor.DetectFileOwner(env.GridHome); err == nil {
			env.GridOwner = creds
		}
	}

	return env, nil
}

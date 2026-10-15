package collector

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"

	"github.com/mario/ora-tui/internal/discovery"
	"github.com/mario/ora-tui/internal/executor"
	"github.com/mario/ora-tui/internal/model"
)

var (
	portRegex     = regexp.MustCompile(`(?i)PORT\s*=\s*(\d+)`)
	serviceRegex  = regexp.MustCompile(`(?i)(?:Service|servicio)\s+"([^"]+)"`)
	instanceRegex = regexp.MustCompile(`(?i)(?:Instance|instancia)\s+"([^"]+)",\s*(?:status|con estado)\s+([A-Z_]+),\s*(?:has|tiene)\s+(\d+)`)
	hexGuidRegex  = regexp.MustCompile(`^[0-9a-f]{32}$`)
)

// ParseLsnrctlStatus parses `lsnrctl status` output into a ListenerInfo entry with full service & READY/UNKNOWN/BLOCKED details.
func ParseLsnrctlStatus(listenerName, nodeName, raw string) model.ListenerInfo {
	info := model.ListenerInfo{
		Name:        listenerName,
		Type:        "LOCAL",
		Node:        nodeName,
		State:       "OFFLINE",
		ReadyStatus: "OFFLINE",
		Endpoint:    "TCP:1521",
		Services:    0,
	}
	if strings.Contains(strings.ToUpper(listenerName), "SCAN") {
		info.Type = "SCAN"
	}

	if strings.Contains(raw, "The command completed successfully") ||
		strings.Contains(raw, "Status of the LISTENER") ||
		strings.Contains(raw, "STATUS of the LISTENER") ||
		strings.Contains(raw, "ESTADO del LISTENER") ||
		strings.Contains(raw, "El comando ha terminado correctamente") {
		info.State = "ONLINE"
	}
	if m := portRegex.FindStringSubmatch(raw); len(m) >= 2 {
		info.Endpoint = "TCP:" + m[1]
	}

	var currentService string
	svcNames := make(map[string]bool)
	svcStates := make(map[string][]string)
	var svcOrder []string

	for _, line := range strings.Split(raw, "\n") {
		trimmed := strings.TrimSpace(line)
		if sm := serviceRegex.FindStringSubmatch(trimmed); len(sm) >= 2 {
			currentService = sm[1]
			if !svcNames[currentService] {
				svcNames[currentService] = true
				svcOrder = append(svcOrder, currentService)
			}
			continue
		}
		if currentService != "" {
			if im := instanceRegex.FindStringSubmatch(trimmed); len(im) >= 4 {
				instName := im[1]
				st := strings.ToUpper(im[2])
				handlers, _ := strconv.Atoi(im[3])

				displaySvc := currentService
				if hexGuidRegex.MatchString(strings.ToLower(currentService)) {
					displaySvc = currentService[:8] + "..(GUID)"
				}

				info.ServiceList = append(info.ServiceList, model.ListenerServiceHandler{
					ListenerName: listenerName,
					ServiceName:  displaySvc,
					InstanceName: instName,
					Status:       st,
					Handlers:     handlers,
				})
				svcStates[currentService] = append(svcStates[currentService], st)

				switch st {
				case "READY":
					info.ReadyCount++
				case "UNKNOWN":
					info.UnknownCount++
				case "BLOCKED", "RESTRICTED":
					info.BlockedCount++
				}
			}
		}
	}

	info.Services = len(svcOrder)

	// Build concise human-readable ServiceSummary (skipping raw 32-hex internal GUIDs unless nothing else exists)
	var summaryParts []string
	for _, sName := range svcOrder {
		if hexGuidRegex.MatchString(strings.ToLower(sName)) && len(svcOrder) > 1 {
			continue
		}
		states := svcStates[sName]
		if len(states) > 0 {
			summaryParts = append(summaryParts, fmt.Sprintf("%s[%s]", sName, strings.Join(states, "+")))
		} else {
			summaryParts = append(summaryParts, sName)
		}
	}
	if len(summaryParts) > 0 {
		info.ServiceSummary = strings.Join(summaryParts, ", ")
	} else if info.State == "ONLINE" {
		info.ServiceSummary = "Sin servicios registrados"
	} else {
		info.ServiceSummary = "-"
	}

	// Compute overall ReadyStatus
	switch {
	case info.State != "ONLINE":
		info.ReadyStatus = "OFFLINE"
	case info.ReadyCount > 0 && info.BlockedCount == 0:
		if info.UnknownCount > 0 {
			info.ReadyStatus = "READY (+Est. UNKNOWN)"
		} else {
			info.ReadyStatus = "READY"
		}
	case info.ReadyCount > 0 && info.BlockedCount > 0:
		info.ReadyStatus = "PARCIAL (READY+BLOCKED)"
	case info.BlockedCount > 0:
		info.ReadyStatus = "BLOCKED (No Acepta)"
	case info.UnknownCount > 0:
		info.ReadyStatus = "NO READY (Solo UNKNOWN)"
	default:
		info.ReadyStatus = "SIN SERVICIOS"
	}

	return info
}

type discoveredLsnrProc struct {
	Name string
	Home string
}

// scanProcListeners finds running tnslsnr processes in /proc to detect custom listener names and homes.
func scanProcListeners(procRoot string) []discoveredLsnrProc {
	entries, err := os.ReadDir(procRoot)
	if err != nil {
		return nil
	}
	seen := make(map[string]bool)
	var result []discoveredLsnrProc
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		pid := entry.Name()
		if pid[0] < '0' || pid[0] > '9' {
			continue
		}
		raw, err := os.ReadFile(filepath.Join(procRoot, pid, "cmdline"))
		if err != nil || len(raw) == 0 {
			continue
		}
		clean := strings.ReplaceAll(string(raw), "\x00", " ")
		fields := strings.Fields(clean)
		if len(fields) == 0 || !strings.HasSuffix(fields[0], "tnslsnr") {
			continue
		}
		binPath := fields[0]
		home := filepath.Dir(filepath.Dir(binPath))
		lsnrName := "LISTENER"
		if len(fields) > 1 && !strings.HasPrefix(fields[1], "-") {
			lsnrName = fields[1]
		}
		key := strings.ToUpper(lsnrName)
		if !seen[key] {
			seen[key] = true
			result = append(result, discoveredLsnrProc{Name: lsnrName, Home: home})
		}
	}
	return result
}

// CollectNetwork inspects local and SCAN listeners via crsctl resources, /proc tnslsnr scan, and `lsnrctl status`.
func CollectNetwork(
	ctx context.Context,
	runner *executor.Runner,
	env *discovery.DiscoveredEnvironment,
	crsResources []CRSResource,
) []model.ListenerInfo {
	var listeners []model.ListenerInfo
	seenLocal := make(map[string]bool)

	runLsnrStatus := func(lsnrName, preferredHome string) (model.ListenerInfo, bool) {
		home := preferredHome
		owner := env.OracleOwner
		if home == "" {
			if env.GridHome != "" {
				home = env.GridHome
				if env.GridOwner != nil {
					owner = env.GridOwner
				}
			} else {
				home = env.OracleHome
			}
		} else if env.GridHome != "" && strings.HasPrefix(home, env.GridHome) && env.GridOwner != nil {
			owner = env.GridOwner
		}
		if home == "" {
			return model.ListenerInfo{}, false
		}
		lsnrBin := filepath.Join(home, "bin", "lsnrctl")
		out, _ := runner.RunCommand(ctx, owner, false, map[string]string{
			"ORACLE_HOME": home,
			"LC_ALL":      "C",
			"NLS_LANG":    "AMERICAN_AMERICA.AL32UTF8",
		}, nil, lsnrBin, "status", lsnrName)
		if out == "" {
			return model.ListenerInfo{}, false
		}
		return ParseLsnrctlStatus(lsnrName, env.HostName, out), true
	}

	for _, res := range crsResources {
		if strings.HasSuffix(res.Name, ".lsnr") {
			lType := "LOCAL"
			if strings.Contains(strings.ToUpper(res.Name), "SCAN") {
				lType = "SCAN"
			}
			shortName := strings.TrimPrefix(res.Name, "ora.")
			shortName = strings.TrimSuffix(shortName, ".lsnr")

			if strings.EqualFold(res.Node, env.HostName) || res.Node == "" {
				seenLocal[strings.ToUpper(shortName)] = true
				if parsed, ok := runLsnrStatus(shortName, env.GridHome); ok {
					parsed.Type = lType
					listeners = append(listeners, parsed)
					continue
				}
			}

			readySt := "OFFLINE"
			if res.State == "ONLINE" {
				readySt = "ONLINE (Remoto)"
			}
			listeners = append(listeners, model.ListenerInfo{
				Name:           shortName,
				Type:           lType,
				Node:           res.Node,
				State:          res.State,
				ReadyStatus:    readySt,
				Endpoint:       "TCP:1521",
				Services:       1,
				ServiceSummary: "(Nodo remoto " + res.Node + ")",
			})
		}
	}

	// Also check running tnslsnr processes on the local host (handles Single Instance and custom listener names)
	for _, proc := range scanProcListeners("/proc") {
		if seenLocal[strings.ToUpper(proc.Name)] {
			continue
		}
		seenLocal[strings.ToUpper(proc.Name)] = true
		if parsed, ok := runLsnrStatus(proc.Name, proc.Home); ok {
			listeners = append(listeners, parsed)
		}
	}

	if len(listeners) == 0 {
		if env.OracleHome != "" {
			if parsed, ok := runLsnrStatus("LISTENER", env.OracleHome); ok {
				listeners = append(listeners, parsed)
			}
		}
		if len(listeners) == 0 {
			listeners = append(listeners, model.ListenerInfo{
				Name:           "LISTENER",
				Type:           "LOCAL",
				Node:           env.HostName,
				State:          "OFFLINE",
				ReadyStatus:    "OFFLINE",
				Endpoint:       "TCP:1521",
				Services:       0,
				ServiceSummary: "Listener detenido (Pulse Enter para Arrancar)",
			})
		}
	}

	return listeners
}

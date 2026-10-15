package collector

import (
	"context"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/mario/ora-tui/internal/discovery"
	"github.com/mario/ora-tui/internal/executor"
	"github.com/mario/ora-tui/internal/model"
)

var portRegex = regexp.MustCompile(`PORT\s*=\s*(\d+)`)

// ParseLsnrctlStatus parses `lsnrctl status` output into a ListenerInfo entry.
func ParseLsnrctlStatus(listenerName, nodeName, raw string) model.ListenerInfo {
	info := model.ListenerInfo{
		Name:     listenerName,
		Type:     "LOCAL",
		Node:     nodeName,
		State:    "OFFLINE",
		Endpoint: "TCP:1521",
		Services: 0,
	}
	if strings.Contains(strings.ToUpper(listenerName), "SCAN") {
		info.Type = "SCAN"
	}

	if strings.Contains(raw, "The command completed successfully") || strings.Contains(raw, "Status of the LISTENER") {
		info.State = "ONLINE"
	}
	if m := portRegex.FindStringSubmatch(raw); len(m) >= 2 {
		info.Endpoint = "TCP:" + m[1]
	}

	svcCount := 0
	for _, line := range strings.Split(raw, "\n") {
		if strings.HasPrefix(strings.TrimSpace(line), "Service \"") {
			svcCount++
		}
	}
	info.Services = svcCount
	return info
}

// CollectNetwork inspects local and SCAN listeners via crsctl resources or lsnrctl status.
func CollectNetwork(
	ctx context.Context,
	runner *executor.Runner,
	env *discovery.DiscoveredEnvironment,
	crsResources []CRSResource,
) []model.ListenerInfo {
	var listeners []model.ListenerInfo

	for _, res := range crsResources {
		if strings.HasSuffix(res.Name, ".lsnr") {
			lType := "LOCAL"
			if strings.Contains(strings.ToUpper(res.Name), "SCAN") {
				lType = "SCAN"
			}
			shortName := strings.TrimPrefix(res.Name, "ora.")
			shortName = strings.TrimSuffix(shortName, ".lsnr")
			listeners = append(listeners, model.ListenerInfo{
				Name:     shortName,
				Type:     lType,
				Node:     res.Node,
				State:    res.State,
				Endpoint: "TCP:1521",
				Services: 1,
			})
		}
	}

	if len(listeners) == 0 && env.OracleHome != "" {
		lsnrBin := filepath.Join(env.OracleHome, "bin", "lsnrctl")
		out, _ := runner.RunCommand(ctx, env.OracleOwner, false, map[string]string{
			"ORACLE_HOME": env.OracleHome,
		}, nil, lsnrBin, "status")
		listeners = append(listeners, ParseLsnrctlStatus("LISTENER", env.HostName, out))
	}

	return listeners
}

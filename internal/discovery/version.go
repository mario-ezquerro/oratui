package discovery

import (
	"context"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"github.com/mario/ora-tui/internal/executor"
)

var versionRegex = regexp.MustCompile(`(\d+\.\d+\.\d+\.\d+\.\d+)`)

// ParseVersionString extracts the exact 5-part Oracle RU version string (e.g., 19.24.0.0.0)
// from command output (prioritizing the "Version X.Y.Z.W.V" line over the base "Release 19.0.0.0.0" line).
func ParseVersionString(raw string) string {
	allMatches := versionRegex.FindAllStringSubmatch(raw, -1)
	if len(allMatches) > 0 {
		return allMatches[len(allMatches)-1][1]
	}
	for _, line := range strings.Split(raw, "\n") {
		line = strings.TrimSpace(line)
		if strings.Contains(line, "Version") || strings.Contains(line, "Release") {
			return line
		}
	}
	return strings.TrimSpace(raw)
}

// DetectVersion executes sqlplus -V and crsctl query crs activeversion to determine the exact release.
func DetectVersion(ctx context.Context, runner *executor.Runner, env *DiscoveredEnvironment) string {
	ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()

	if env.OracleHome != "" {
		sqlplusBin := filepath.Join(env.OracleHome, "bin", "sqlplus")
		out, err := runner.RunCommand(ctx, env.OracleOwner, false, map[string]string{
			"ORACLE_HOME": env.OracleHome,
		}, nil, sqlplusBin, "-V")
		if err == nil && out != "" {
			if v := ParseVersionString(out); v != "" {
				return v
			}
		}
	}

	if env.GridHome != "" {
		crsctlBin := filepath.Join(env.GridHome, "bin", "crsctl")
		out, err := runner.RunCommand(ctx, env.GridOwner, false, map[string]string{
			"ORACLE_HOME": env.GridHome,
		}, nil, crsctlBin, "query", "crs", "activeversion", "-f")
		if err == nil && out != "" {
			if v := ParseVersionString(out); v != "" {
				return v
			}
		}
	}

	return "Unknown"
}

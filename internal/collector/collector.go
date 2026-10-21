package collector

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/mario/ora-tui/internal/discovery"
	"github.com/mario/ora-tui/internal/executor"
	"github.com/mario/ora-tui/internal/model"
)

// CollectLiveEnvironment discovers and collects the full live Oracle environment state.
func CollectLiveEnvironment(ctx context.Context, runner *executor.Runner) (*model.EnvironmentState, *discovery.DiscoveredEnvironment, error) {
	env, err := discovery.Discover()
	if err != nil {
		return nil, nil, err
	}

	version := discovery.DetectVersion(ctx, runner, env)

	oracleOwnerStr := "oracle"
	if env.OracleOwner != nil {
		oracleOwnerStr = fmt.Sprintf("%s (uid=%d)", env.OracleOwner.Username, env.OracleOwner.UID)
	}
	gridOwnerStr := "grid"
	if env.GridOwner != nil {
		gridOwnerStr = fmt.Sprintf("%s (uid=%d)", env.GridOwner.Username, env.GridOwner.UID)
	}

	nodes, crsResources := CollectCluster(ctx, runner, env)

	// Pick active DB SID (skipping +ASM and -MGMTDB entries)
	activeSID := ""
	if len(env.RunningSIDs) > 0 {
		activeSID = env.RunningSIDs[0]
	} else {
		for _, entry := range env.Oratab {
			if !strings.HasPrefix(entry.SID, "+ASM") && !strings.HasPrefix(entry.SID, "-MGMT") {
				activeSID = entry.SID
				break
			}
		}
	}

	instances, pdbs, services, standbys := CollectDatabase(ctx, runner, env, activeSID)
	sessions := CollectSessions(ctx, runner, env, activeSID)
	diskgroups, tablespaces, datafiles, segments, invalidObjs := CollectStorage(ctx, runner, env, activeSID)
	rmanBackups, restorePoints, alertEntries, alertTail, traceDir, alertSizeMB := CollectRMANDiag(ctx, runner, env, activeSID)
	listeners := CollectNetwork(ctx, runner, env, crsResources)
	osMetrics, filesystems, storagePaths := CollectOSHealth(ctx, runner, env)

	clusterName := "N/A (Single Instance)"
	if env.Topology == model.ModeRAC {
		clusterName = "RAC-Cluster"
	} else if env.Topology == model.ModeRestart {
		clusterName = "Oracle-Restart"
	}

	state := &model.EnvironmentState{
		Topology:        env.Topology,
		BorderMode:      model.BorderMixedUnicode,
		HostName:        env.HostName,
		ClusterName:     clusterName,
		OracleHome:      env.OracleHome,
		OracleOwner:     oracleOwnerStr,
		GridHome:        env.GridHome,
		GridOwner:       gridOwnerStr,
		ReleaseVersion:  version,
		IsMock:          false,
		LastUpdated:     time.Now(),
		Nodes:           nodes,
		Instances:       instances,
		PDBs:            pdbs,
		Services:        services,
		Sessions:        sessions,
		Tablespaces:     tablespaces,
		Datafiles:       datafiles,
		TableSegments:   segments,
		InvalidObjects:  invalidObjs,
		Diskgroups:      diskgroups,
		Listeners:       listeners,
		RMANBackups:     rmanBackups,
		RestorePoints:   restorePoints,
		StandbyDBs:      standbys,
		AlertEntries:    alertEntries,
		AlertTailLines:  alertTail,
		DiagTraceDir:    traceDir,
		AlertFileSizeMB: alertSizeMB,
		OSMetrics:       osMetrics,
		Filesystems:     filesystems,
		StoragePaths:    storagePaths,
	}

	CollectAdvancedOracleMetrics(ctx, runner, env, activeSID, state)

	return state, env, nil
}

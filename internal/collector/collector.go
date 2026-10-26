package collector

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/mario/ora-tui/internal/discovery"
	"github.com/mario/ora-tui/internal/executor"
	"github.com/mario/ora-tui/internal/model"
)

type environmentMetaCache struct {
	mu              sync.Mutex
	lastDiscoverRun time.Time
	env             *discovery.DiscoveredEnvironment
	lastVersionRun  time.Time
	version         string
	lastNetworkRun  time.Time
	listeners       []model.ListenerInfo
}

var metaCache environmentMetaCache

// CollectLiveEnvironment discovers and collects the full live Oracle environment state
// using tiered caching and batched SQL*Plus sessions to keep daemon RSS and CPU footprint minimal.
func CollectLiveEnvironment(ctx context.Context, runner *executor.Runner) (*model.EnvironmentState, *discovery.DiscoveredEnvironment, error) {
	now := time.Now()

	metaCache.mu.Lock()
	cachedEnv := metaCache.env
	needDiscover := cachedEnv == nil || now.Sub(metaCache.lastDiscoverRun) >= 60*time.Second
	cachedVer := metaCache.version
	needVersion := cachedVer == "" || cachedVer == "Unknown" || now.Sub(metaCache.lastVersionRun) >= 10*time.Minute
	needNetwork := len(metaCache.listeners) == 0 || now.Sub(metaCache.lastNetworkRun) >= 30*time.Second
	metaCache.mu.Unlock()

	var env *discovery.DiscoveredEnvironment
	if needDiscover {
		discovered, err := discovery.Discover()
		if err != nil {
			if cachedEnv == nil {
				return nil, nil, err
			}
			env = cachedEnv
		} else {
			env = discovered
			metaCache.mu.Lock()
			metaCache.env = env
			metaCache.lastDiscoverRun = now
			metaCache.mu.Unlock()
		}
	} else {
		env = cachedEnv
	}

	version := cachedVer
	if needVersion {
		version = discovery.DetectVersion(ctx, runner, env)
		metaCache.mu.Lock()
		metaCache.version = version
		metaCache.lastVersionRun = now
		metaCache.mu.Unlock()
	}

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

	var listeners []model.ListenerInfo
	if needNetwork {
		listeners = CollectNetwork(ctx, runner, env, crsResources)
		metaCache.mu.Lock()
		metaCache.listeners = listeners
		metaCache.lastNetworkRun = now
		metaCache.mu.Unlock()
	} else {
		metaCache.mu.Lock()
		listeners = metaCache.listeners
		metaCache.mu.Unlock()
	}

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

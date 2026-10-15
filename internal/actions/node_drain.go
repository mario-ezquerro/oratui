package actions

import (
	"context"
	"fmt"
	"path/filepath"
	"strings"

	"github.com/mario/ora-tui/internal/discovery"
	"github.com/mario/ora-tui/internal/executor"
	"github.com/mario/ora-tui/internal/model"
)

// DrainClusterNode executes the 4-step graceful node eviction/drain workflow:
// Step 1: Pre-check online failover nodes & active services
// Step 2: Relocate services away from target node
// Step 3: Stop local database instances on target node
// Step 4: Stop Clusterware stack (crsctl stop crs as root)
func DrainClusterNode(
	ctx context.Context,
	runner *executor.Runner,
	env *discovery.DiscoveredEnvironment,
	state *model.EnvironmentState,
	targetNode string,
	stopCRS bool,
	forceCRS bool,
	onLine executor.OutputCallback,
) error {
	// Step 1: Pre-check failover node
	if onLine != nil {
		onLine(fmt.Sprintf("[STEP 1/4] Pre-checking failover nodes for %s...", targetNode))
	}
	var failoverNode string
	for _, n := range state.Nodes {
		if !strings.EqualFold(n.Name, targetNode) && strings.EqualFold(n.State, "ONLINE") {
			failoverNode = n.Name
			break
		}
	}
	if failoverNode == "" {
		return fmt.Errorf("pre-check failed: no ONLINE failover node available in cluster")
	}

	// Map instances on targetNode vs failoverNode
	var localInst, targetInst, dbName string
	for _, inst := range state.Instances {
		if dbName == "" {
			dbName = inst.DBUniqueName
			if dbName == "" {
				dbName = inst.DBName
			}
		}
		if strings.EqualFold(inst.NodeName, targetNode) {
			localInst = inst.InstName
		} else if strings.EqualFold(inst.NodeName, failoverNode) {
			targetInst = inst.InstName
		}
	}

	// Step 2: Relocate active services
	if onLine != nil {
		onLine(fmt.Sprintf("[STEP 2/4] Relocating active services from %s (%s) to %s (%s)...", targetNode, localInst, failoverNode, targetInst))
	}
	if localInst != "" && targetInst != "" && dbName != "" {
		for _, svc := range state.Services {
			if strings.Contains(svc.ActiveInst, localInst) {
				_ = RelocateRACService(ctx, runner, env, dbName, svc.Name, localInst, targetInst, onLine)
			}
		}
	}

	// Step 3: Stop local DB instances on target node
	if onLine != nil {
		onLine(fmt.Sprintf("[STEP 3/4] Stopping database instances on node %s...", targetNode))
	}
	if dbName != "" && env.OracleHome != "" {
		srvctl := filepath.Join(env.OracleHome, "bin", "srvctl")
		_, err := runner.RunCommand(ctx, env.OracleOwner, false, map[string]string{
			"ORACLE_HOME": env.OracleHome,
		}, onLine, srvctl, "stop", "instance", "-d", dbName, "-n", targetNode, "-stopoption", "IMMEDIATE")
		if err != nil {
			return fmt.Errorf("failed stopping instance on %s: %w", targetNode, err)
		}
	}

	// Step 4: Optionally stop CRS stack as root
	if stopCRS && env.GridHome != "" {
		if onLine != nil {
			onLine(fmt.Sprintf("[STEP 4/4] Stopping Oracle Clusterware stack (crsctl stop crs) as root on %s...", targetNode))
		}
		crsctl := filepath.Join(env.GridHome, "bin", "crsctl")
		args := []string{"stop", "crs"}
		if forceCRS {
			args = append(args, "-f")
		}
		_, err := runner.RunCommand(ctx, nil, true, map[string]string{
			"ORACLE_HOME": env.GridHome,
		}, onLine, crsctl, args...)
		if err != nil {
			return fmt.Errorf("crsctl stop crs failed: %w", err)
		}
	}

	return nil
}

// RejoinClusterNode starts CRS as root and starts local instances on the node.
func RejoinClusterNode(
	ctx context.Context,
	runner *executor.Runner,
	env *discovery.DiscoveredEnvironment,
	dbName, targetNode string,
	onLine executor.OutputCallback,
) error {
	if env.GridHome != "" {
		if onLine != nil {
			onLine(fmt.Sprintf("[STEP 1/2] Starting Oracle Clusterware stack (crsctl start crs) as root on %s...", targetNode))
		}
		crsctl := filepath.Join(env.GridHome, "bin", "crsctl")
		if _, err := runner.RunCommand(ctx, nil, true, map[string]string{
			"ORACLE_HOME": env.GridHome,
		}, onLine, crsctl, "start", "crs"); err != nil {
			return err
		}
	}

	if dbName != "" && env.OracleHome != "" {
		if onLine != nil {
			onLine(fmt.Sprintf("[STEP 2/2] Starting database instance on node %s...", targetNode))
		}
		srvctl := filepath.Join(env.OracleHome, "bin", "srvctl")
		_, err := runner.RunCommand(ctx, env.OracleOwner, false, map[string]string{
			"ORACLE_HOME": env.OracleHome,
		}, onLine, srvctl, "start", "instance", "-d", dbName, "-n", targetNode)
		return err
	}
	return nil
}

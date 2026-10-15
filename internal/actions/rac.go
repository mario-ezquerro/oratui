package actions

import (
	"context"
	"path/filepath"

	"github.com/mario/ora-tui/internal/discovery"
	"github.com/mario/ora-tui/internal/executor"
)

// StopRACInstance stops a specific RAC instance using srvctl.
func StopRACInstance(
	ctx context.Context,
	runner *executor.Runner,
	env *discovery.DiscoveredEnvironment,
	dbName, instName string,
	onLine executor.OutputCallback,
) error {
	srvctl := filepath.Join(env.OracleHome, "bin", "srvctl")
	_, err := runner.RunCommand(ctx, env.OracleOwner, false, map[string]string{
		"ORACLE_HOME": env.OracleHome,
	}, onLine, srvctl, "stop", "instance", "-d", dbName, "-i", instName, "-o", "immediate")
	return err
}

// StartRACInstance starts a specific RAC instance using srvctl.
func StartRACInstance(
	ctx context.Context,
	runner *executor.Runner,
	env *discovery.DiscoveredEnvironment,
	dbName, instName string,
	onLine executor.OutputCallback,
) error {
	srvctl := filepath.Join(env.OracleHome, "bin", "srvctl")
	_, err := runner.RunCommand(ctx, env.OracleOwner, false, map[string]string{
		"ORACLE_HOME": env.OracleHome,
	}, onLine, srvctl, "start", "instance", "-d", dbName, "-i", instName)
	return err
}

// StopRACDatabase stops the entire RAC database across all nodes.
func StopRACDatabase(
	ctx context.Context,
	runner *executor.Runner,
	env *discovery.DiscoveredEnvironment,
	dbName string,
	onLine executor.OutputCallback,
) error {
	srvctl := filepath.Join(env.OracleHome, "bin", "srvctl")
	_, err := runner.RunCommand(ctx, env.OracleOwner, false, map[string]string{
		"ORACLE_HOME": env.OracleHome,
	}, onLine, srvctl, "stop", "database", "-d", dbName, "-o", "immediate")
	return err
}

// StartRACDatabase starts the entire RAC database across all nodes.
func StartRACDatabase(
	ctx context.Context,
	runner *executor.Runner,
	env *discovery.DiscoveredEnvironment,
	dbName string,
	onLine executor.OutputCallback,
) error {
	srvctl := filepath.Join(env.OracleHome, "bin", "srvctl")
	_, err := runner.RunCommand(ctx, env.OracleOwner, false, map[string]string{
		"ORACLE_HOME": env.OracleHome,
	}, onLine, srvctl, "start", "database", "-d", dbName)
	return err
}

// RelocateRACService relocates a service from oldInst to newInst.
func RelocateRACService(
	ctx context.Context,
	runner *executor.Runner,
	env *discovery.DiscoveredEnvironment,
	dbName, serviceName, oldInst, newInst string,
	onLine executor.OutputCallback,
) error {
	srvctl := filepath.Join(env.OracleHome, "bin", "srvctl")
	_, err := runner.RunCommand(ctx, env.OracleOwner, false, map[string]string{
		"ORACLE_HOME": env.OracleHome,
	}, onLine, srvctl, "relocate", "service", "-d", dbName, "-s", serviceName, "-oldinst", oldInst, "-newinst", newInst)
	return err
}

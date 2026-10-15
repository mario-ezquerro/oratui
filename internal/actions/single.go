package actions

import (
	"context"
	"fmt"
	"path/filepath"

	"github.com/mario/ora-tui/internal/discovery"
	"github.com/mario/ora-tui/internal/executor"
)

// StopSingleInstance stops the listener and executes SHUTDOWN IMMEDIATE via sqlplus.
func StopSingleInstance(
	ctx context.Context,
	runner *executor.Runner,
	env *discovery.DiscoveredEnvironment,
	sid string,
	onLine executor.OutputCallback,
) error {
	if onLine != nil {
		onLine(fmt.Sprintf("[STEP 1/2] Stopping local listener for %s...", sid))
	}
	lsnrBin := filepath.Join(env.OracleHome, "bin", "lsnrctl")
	_, _ = runner.RunCommand(ctx, env.OracleOwner, false, map[string]string{
		"ORACLE_HOME": env.OracleHome,
	}, onLine, lsnrBin, "stop")

	if onLine != nil {
		onLine(fmt.Sprintf("[STEP 2/2] Executing SHUTDOWN IMMEDIATE on %s...", sid))
	}
	_, _, err := runner.RunSQLPlus(ctx, env.OracleHome, sid, env.OracleOwner, "SHUTDOWN IMMEDIATE;", onLine)
	return err
}

// StartSingleInstance starts the listener, mounts/opens the CDB, and opens all PDBs.
func StartSingleInstance(
	ctx context.Context,
	runner *executor.Runner,
	env *discovery.DiscoveredEnvironment,
	sid string,
	onLine executor.OutputCallback,
) error {
	if onLine != nil {
		onLine(fmt.Sprintf("[STEP 1/2] Starting local listener for %s...", sid))
	}
	lsnrBin := filepath.Join(env.OracleHome, "bin", "lsnrctl")
	_, _ = runner.RunCommand(ctx, env.OracleOwner, false, map[string]string{
		"ORACLE_HOME": env.OracleHome,
	}, onLine, lsnrBin, "start")

	if onLine != nil {
		onLine(fmt.Sprintf("[STEP 2/2] Executing STARTUP and opening all PDBs on %s...", sid))
	}
	sql := "STARTUP;\nALTER PLUGGABLE DATABASE ALL OPEN;"
	_, _, err := runner.RunSQLPlus(ctx, env.OracleHome, sid, env.OracleOwner, sql, onLine)
	return err
}

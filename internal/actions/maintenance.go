package actions

import (
	"context"
	"fmt"
	"path/filepath"

	"github.com/mario/ora-tui/internal/discovery"
	"github.com/mario/ora-tui/internal/executor"
)

// KillUserSession executes ALTER SYSTEM KILL SESSION 'sid,serial#,@inst_id' IMMEDIATE.
func KillUserSession(
	ctx context.Context,
	runner *executor.Runner,
	env *discovery.DiscoveredEnvironment,
	dbSID string,
	instID, sid, serial int,
	onLine executor.OutputCallback,
) error {
	sql := fmt.Sprintf("ALTER SYSTEM KILL SESSION '%d,%d,@%d' IMMEDIATE;", sid, serial, instID)
	_, _, err := runner.RunSQLPlus(ctx, env.OracleHome, dbSID, env.OracleOwner, sql, onLine)
	return err
}

// AddTablespaceDatafile adds a new datafile to the specified tablespace.
func AddTablespaceDatafile(
	ctx context.Context,
	runner *executor.Runner,
	env *discovery.DiscoveredEnvironment,
	dbSID, tablespaceName, datafileSpec string,
	sizeGB int,
	onLine executor.OutputCallback,
) error {
	if datafileSpec == "" {
		datafileSpec = "'+DATA'"
	}
	if sizeGB <= 0 {
		sizeGB = 32
	}
	sql := fmt.Sprintf("ALTER TABLESPACE %s ADD DATAFILE %s SIZE %dG AUTOEXTEND ON NEXT 1G MAXSIZE UNLIMITED;", tablespaceName, datafileSpec, sizeGB)
	_, _, err := runner.RunSQLPlus(ctx, env.OracleHome, dbSID, env.OracleOwner, sql, onLine)
	return err
}

// CoalesceTablespace consolidates contiguous free extents in a tablespace.
func CoalesceTablespace(
	ctx context.Context,
	runner *executor.Runner,
	env *discovery.DiscoveredEnvironment,
	dbSID, tablespaceName string,
	onLine executor.OutputCallback,
) error {
	sql := fmt.Sprintf("ALTER TABLESPACE %s COALESCE;", tablespaceName)
	_, _, err := runner.RunSQLPlus(ctx, env.OracleHome, dbSID, env.OracleOwner, sql, onLine)
	return err
}

// GatherTableStats collects optimizer statistics for a table and its indexes.
func GatherTableStats(
	ctx context.Context,
	runner *executor.Runner,
	env *discovery.DiscoveredEnvironment,
	dbSID, owner, tableName string,
	onLine executor.OutputCallback,
) error {
	sql := fmt.Sprintf(
		"BEGIN DBMS_STATS.GATHER_TABLE_STATS(ownname => '%s', tabname => '%s', estimate_percent => DBMS_STATS.AUTO_SAMPLE_SIZE, cascade => TRUE); END;\n/",
		owner, tableName,
	)
	_, _, err := runner.RunSQLPlus(ctx, env.OracleHome, dbSID, env.OracleOwner, sql, onLine)
	return err
}

// ShrinkTableOnline enables row movement and shrinks space on a table and its dependent segments.
func ShrinkTableOnline(
	ctx context.Context,
	runner *executor.Runner,
	env *discovery.DiscoveredEnvironment,
	dbSID, owner, tableName string,
	onLine executor.OutputCallback,
) error {
	sql := fmt.Sprintf(
		"ALTER TABLE %s.%s ENABLE ROW MOVEMENT;\nALTER TABLE %s.%s SHRINK SPACE CASCADE;",
		owner, tableName, owner, tableName,
	)
	_, _, err := runner.RunSQLPlus(ctx, env.OracleHome, dbSID, env.OracleOwner, sql, onLine)
	return err
}

// CompileSchemaInvalidObjects recompiles all invalid objects in a schema.
func CompileSchemaInvalidObjects(
	ctx context.Context,
	runner *executor.Runner,
	env *discovery.DiscoveredEnvironment,
	dbSID, schema string,
	onLine executor.OutputCallback,
) error {
	sql := fmt.Sprintf("BEGIN DBMS_UTILITY.COMPILE_SCHEMA(schema => '%s', compile_all => FALSE); END;\n/", schema)
	_, _, err := runner.RunSQLPlus(ctx, env.OracleHome, dbSID, env.OracleOwner, sql, onLine)
	return err
}

// CreateGuaranteedRestorePoint creates a Flashback Guaranteed Restore Point.
func CreateGuaranteedRestorePoint(
	ctx context.Context,
	runner *executor.Runner,
	env *discovery.DiscoveredEnvironment,
	dbSID, rpName string,
	onLine executor.OutputCallback,
) error {
	sql := fmt.Sprintf("CREATE RESTORE POINT %s GUARANTEE FLASHBACK DATABASE;", rpName)
	_, _, err := runner.RunSQLPlus(ctx, env.OracleHome, dbSID, env.OracleOwner, sql, onLine)
	return err
}

// PurgeADRCI purges diagnostic traces older than ageMinutes using adrci.
func PurgeADRCI(
	ctx context.Context,
	runner *executor.Runner,
	env *discovery.DiscoveredEnvironment,
	ageMinutes int,
	onLine executor.OutputCallback,
) error {
	if ageMinutes <= 0 {
		ageMinutes = 10080 // 7 days
	}
	adrciBin := filepath.Join(env.OracleHome, "bin", "adrci")
	execArg := fmt.Sprintf("exec=purge -age %d -type TRACE", ageMinutes)
	_, err := runner.RunCommand(ctx, env.OracleOwner, false, map[string]string{
		"ORACLE_HOME": env.OracleHome,
	}, onLine, adrciBin, execArg)
	return err
}

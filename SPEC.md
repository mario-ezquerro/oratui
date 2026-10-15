# Technical Specification: `ora-tui` (Oracle Cluster & Single Instance TUI Manager)

## 1. Project Overview
Develop an interactive Terminal User Interface (TUI) application in **Go (Golang)** targeting Linux servers (RHEL / Oracle Linux 7, 8, and 9), packaged as a zero-dependency RPM installable via `dnf` (and runnable in `--mock` mode on macOS/Linux for simulation and design testing).

The tool automatically discovers Oracle environments (**Single Instance**, **Oracle Restart**, or **Oracle RAC**), provides a real-time comprehensive health and analysis dashboard of the entire stack (Cluster, Databases/PDBs, Connected Users/Sessions, Tablespaces, Tables/Segments, ASM Storage, Listeners, RMAN Backups, Flashback/Restore Points, Data Guard, and Alert Log `ORA-` Errors), and executes safe operational and maintenance workflows navigated entirely via **arrow keys** and **Unicode Box-Drawing Windows**:
* **Borde Simple (`┌`, `┐`, `└`, `┘`, `─`, `│`, `├`, `┤`, `┬`, `┴`, `┼`)** for standard containers, sub-panels, and data tables.
* **Borde Doble (`╔`, `╗`, `╚`, `╝`, `═`, `║`, `╠`, `╣`, `╦`, `╩`, `╬`)** for the main header, focused active windows, interactive popup action menus, and security confirmation modals (with live toggle via key `b` between Mixed `┌─┐/╔═╗`, Full Double `╔═╗`, Full Single `┌─┐`, and Classic ASCII `+-|`).

## 2. Technical Constraints & Environment
* **Language:** Go 1.22+ compiled as a static binary (`CGO_ENABLED=0 GOOS=linux GOARCH=amd64`).
* **TUI Framework:** `github.com/rivo/tview` + `github.com/gdamore/tcell/v2` with custom Unicode Box-Drawing themes (`┌─┐│└┘` and `╔═╗║╚╝`).
* **Packaging:** RPM package built via `nfpm` or `rpmbuild`, installable via `dnf install ora-tui`.
* **OS Privilege Management:** The binary runs with `root` privileges (via `sudo`) and dynamically switches execution context (`syscall.Credential` / `sudo -u <owner>`) based on the file owner of `$ORACLE_HOME` (typically `oracle`) or `$GRID_HOME` (typically `grid`), reserving `root` execution strictly for clusterware stack commands (`crsctl start/stop crs`).
* **Simulation / Mock Mode (`--mock`):** Built-in interactive simulator supporting `--mock` (and `--topology=rac|restart|single`) to preview all screens, simulated metrics, arrow-key menus, and maintenance workflows without requiring a live Oracle installation.

## 3. Repository Structure
```text
ora-tui/
├── .agent/
│   └── skills/
│       └── oracle-tui/
│           └── SKILL.md         # Antigravity agent skill instructions
├── cmd/
│   └── ora-tui/
│       └── main.go              # Entrypoint, CLI flags (--mock, --topology, --preview), TUI bootstrap
├── internal/
│   ├── model/
│   │   └── state.go             # Domain models: Cluster, Instance, PDB, Session, Tablespace, TableSegment, ASM, Listener, RMAN, AlertLog
│   ├── discovery/
│   │   ├── detector.go          # Parses /etc/oratab, ocr.loc, olr.loc, and /proc PMON processes
│   │   └── version.go           # Extracts exact version (sqlplus -V, crsctl query crs activeversion)
│   ├── collector/
│   │   ├── cluster.go           # crsctl status resource -t, olsnodes, crsctl check cluster -all
│   │   ├── database.go          # srvctl status, sqlplus queries (gv$instance, gv$pdbs, Data Guard, FRA)
│   │   ├── sessions.go          # Connected users & sessions (gv$session, blocking locks, wait events, SQL_ID)
│   │   ├── storage.go           # ASM diskgroups (v$asm_diskgroup), Tablespaces (dba_tablespaces), Datafiles, Top Tables/Segments
│   │   ├── rman_diag.go         # RMAN backup jobs (v$rman_backup_job_details), Restore Points (v$restore_point), Alert Log (v$diag_alert_ext)
│   │   ├── network.go           # Local Listeners and SCAN Listeners status
│   │   └── mock.go              # Realistic mock data generator for RAC / Restart / Single Instance
│   ├── actions/
│   │   ├── single.go            # Start/Stop via sqlplus (SHUTDOWN IMMEDIATE / STARTUP) and lsnrctl
│   │   ├── rac.go               # Start/Stop via srvctl (instances, databases, services)
│   │   ├── node_drain.go        # Graceful node eviction/drain and re-joining workflow
│   │   └── maintenance.go       # Tablespace resize/add datafile, table stats/shrink/index rebuild, compile invalid objects, kill session, RMAN/GRP, ADRCI
│   ├── executor/
│   │   └── runner.go            # OS command & SQL*Plus runner with UID switching, timeouts, and live output streaming
│   └── ui/
│       ├── ascii.go             # Unicode Simple (┌─┐) & Double (╔═╗) border themes and block progress bars [████████░░]
│       ├── app.go               # Main tview layout, 8 views, arrow-key menus, and double-bordered modal popups
│       └── preview.go           # Static UTF-8 box-drawing screen renderer for CLI preview
├── packaging/
│   ├── nfpm.yaml                # Automated RPM generator config
│   ├── ora-tui.spec             # Standard RPM .spec file for rpmbuild
│   └── sudoers.ora-tui          # Sudoers rules for dba/oinstall groups
└── SPEC.md
```

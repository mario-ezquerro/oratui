# Technical Specification: `ora-tui` v1.6.0 (Oracle Cluster, DB, Server/SAN & OpenTelemetry Manager)

## 1. Project Overview
`ora-tui` is an interactive Terminal User Interface (TUI) and embedded **OpenTelemetry / Prometheus Exporter** written in **Go (Golang)** targeting Linux database servers (RHEL / Oracle Linux 7, 8, and 9, as well as Debian/Ubuntu), packaged as a zero-dependency `.rpm` and `.deb` (`CGO_ENABLED=0`) and runnable in `--mock` mode on macOS/Linux.

The tool automatically discovers Oracle environments (**Single Instance**, **Oracle Restart**, or **Oracle RAC**), provides a real-time 9-module health and analysis dashboard (Cluster, Databases/PDBs, Connected Users/Sessions, Tablespaces, Tables/Segments, ASM Storage, Listeners, RMAN Backups, Flashback/Restore Points, Data Guard, Alert Log `ORA-` Errors, and **Linux Server/Memory/Filesystem Inodes/SAN Multipath & FC HBAs**), and executes operational and maintenance workflows with a **scrollable popup Operation Output Window** showing complete command results.

## 2. Key Capabilities
1. **Zero-Config Discovery (`internal/discovery`)**:
   - Inspects `/etc/oracle/ocr.loc`, `/etc/oracle/olr.loc`, `/etc/oratab`, `inventory.xml`, and `/proc/[0-9]*/cmdline` (`ora_pmon_*`, `db_pmon_*`, `asm_pmon_*`) to detect topology, active instances, `$ORACLE_HOME`, `$GRID_HOME`, and exact RU patch level.
2. **Dynamic UNIX Privilege Switching (`internal/executor`)**:
   - Detects UID/GID ownership of `$ORACLE_HOME` (`oracle`) and `$GRID_HOME` (`grid`) via `os.Stat` and runs child processes under the appropriate OS account, reserving `root` strictly for `crsctl start/stop crs` and SCSI/Multipath bus rescans.
3. **9 Interactive Analysis & Maintenance Modules (`internal/collector`, `internal/actions`, `internal/ui`)**:
   - Module 1: Clusterware (`olsnodes`, `crsctl`), Database Instances (`gv$instance`), PDBs (`gv$pdbs`), and Services + Graceful Node Drain.
   - Module 2: Active/Inactive Sessions (`gv$session`), Wait Events, PGA Memory, and TX/TM Blocking Lock Chains + `KILL SESSION` & `DBMS_XPLAN`.
   - Module 3: Tablespaces & Datafiles (`dba_tablespaces`, `dba_data_files`) with real `AUTOEXTEND MAXBYTES` capacity calculation.
   - Module 4: Top Table/Index Segments (`dba_segments`) & `INVALID` Objects (`dba_objects`) + `DBMS_STATS`, Online Shrink, Index Rebuild, and `UTL_RECOMP`.
   - Module 5: ASM Diskgroups (`v$asm_diskgroup`) & Local/SCAN Listeners (`lsnrctl status` `READY` vs `UNKNOWN` handlers).
   - Module 6: RMAN Backups (`v$rman_backup_job_details`), Flashback Restore Points (`v$restore_point`), and Data Guard Standby (`MRP0`, Transport/Apply Lag).
   - Module 7: Strict `^ORA-` Alert Log Collector & Live Tail Viewer + ADRCI purge and log rotation.
   - Module 8: Linux Server Health (`Load Average` vs CPU Cores, Physical RAM, Swap, Static `HugePages`, `Transparent HugePages [never]`, Kernel Async I/O `fs.aio-nr`, File Descriptors `fs.file-nr`), Filesystem Space & Inode Exhaustion (`df -PTm` & `df -Pi` to prevent `ORA-09925`), and SAN Multipath (`multipath -ll` active vs faulty paths, Fibre Channel HBA ports `/sys/class/fc_host/host*`, and Block Disks).
   - Module 9: Centralized Guided Maintenance Wizard Catalog.
4. **Popup Operation Output Modal (`showOperationOutputModal`)**:
   - Displays full multi-line command outputs in a centered, scrollable modal window whenever any diagnostic or maintenance task is executed.
5. **4 Authentic Retro MS-DOS Color Themes (`c` key) & Unicode Box-Drawing Modes (`b` key)**:
   - *Norton Commander 5.0*, *Borland Turbo C++ 3.0*, *IBM 5151 Green Phosphor CRT* (with 100% full-line inverse video selection bar), and *Modern Dark*.
6. **Embedded OpenTelemetry (`OTLP JSON`) & Prometheus (`/metrics`) Exporter (`internal/telemetry`)**:
   - Activated via `--opentelemetry` (or `--opentlemetry` / `--otel`), listening on port `9161` by default (`--otel-port`), in hybrid TUI+Exporter mode or headless `--daemon` mode (`ora-tui-otel.service`).
   - Exposes 52 Prometheus metrics on `/metrics`, OTLP/HTTP JSON on `/otel/v1/metrics`, full state JSON on `/api/v1/state`, and healthcheck on `/healthz`.
7. **Ready-to-Run Docker `POC/` Stack (`POC/configure-poc.sh`)**:
   - Self-contained `POC/` folder with an interactive setup script (`./configure-poc.sh`) that prompts for the target Oracle hostname/IP every time it runs, configures Prometheus with 15 alert rules, and auto-provisions Grafana with a 26-panel full observability home dashboard.

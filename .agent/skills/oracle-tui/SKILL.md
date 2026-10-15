---
name: oracle-tui
description: Guide for extending and maintaining ora-tui (Oracle Cluster & Single Instance TUI Manager in Go). Use when adding new Oracle SQL collectors, clusterware workflows, TUI panels, or RPM packaging adjustments.
---

# `ora-tui` Agent Skill

## Architecture Rules
1. **Zero C Dependencies:** Always build with `CGO_ENABLED=0` and avoid any CGO-based Oracle driver; interact with Oracle strictly via `$ORACLE_HOME/bin/sqlplus -s / as sysdba`, `srvctl`, `crsctl`, `lsnrctl`, `rman`, and `adrci` through `internal/executor/runner.go`.
2. **Privilege Isolation:**
   - Database commands (`sqlplus`, `srvctl`, `lsnrctl`, `rman`, `adrci`) MUST execute under the UNIX owner of `$ORACLE_HOME` (typically `oracle`).
   - Grid queries (`olsnodes`, `crsctl status`) execute under the UNIX owner of `$GRID_HOME` (typically `grid`).
   - Root execution (`asRoot=true`) is reserved strictly for `crsctl start crs` and `crsctl stop crs`.
3. **Unicode & ASCII Window Frames:**
   - All TUI views in `internal/ui/` use `ApplyBorderTheme()` supporting Simple Unicode (`┌ ─ ┐ │ └ ┘`) for panels and Double Unicode (`╔ ═ ╗ ║ ╚ ╝`) for focused views, popups, and confirmation modals, with `b` key toggling.
4. **Safety First:** Every state-changing or destructive action must be wrapped in a `showConfirmModal` confirmation dialog and stream real-time output to the bottom Log View.

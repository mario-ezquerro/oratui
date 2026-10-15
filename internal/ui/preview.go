package ui

import (
	"fmt"
	"strings"

	"github.com/mario/ora-tui/internal/model"
)

// RenderStaticASCIIMock prints a UTF-8 Box-Drawing snapshot of the TUI screens and popup menu windows
// combining Simple Borders (┌ ─ ┐ │ └ ┘) and Double Borders (╔ ═ ╗ ║ ╚ ╝).
func RenderStaticASCIIMock(state *model.EnvironmentState) string {
	var b strings.Builder

	b.WriteString("╔══════════════════════════════════════════════════════════════════════════════════════════════════════════════════════╗\n")
	b.WriteString("║                              ══ ORA-TUI :: ORACLE CLUSTER & DATABASE MANAGER ══                                      ║\n")
	b.WriteString("╠══════════════════════════════════════════════════════════════════════════════════════════════════════════════════════╣\n")
	b.WriteString(fmt.Sprintf("║ Modo: [%s] [MOCK DESIGN] │ Host: %-20s │ Cluster: %-18s │ Ver: %-22s ║\n",
		state.Topology, state.HostName, state.ClusterName, state.ReleaseVersion))
	b.WriteString(fmt.Sprintf("║ ORACLE_HOME: %-40s │ GRID_HOME: %-49s ║\n", state.OracleHome, state.GridHome))
	b.WriteString("║ Estado: 2 Inst OPEN │ 4 PDBs │ 3 Sesiones (1 Lock TX) │ 1 Tablespace >90% │ 3 Inválidos │ 1 Alerta Crítica ORA-01654 ║\n")
	b.WriteString("╠═════════════════════════════╦════════════════════════════════════════════════════════════════════════════════════════╣\n")
	b.WriteString("║ MENU [Flechas ↑↓ + Enter]   ║ PANTALLA ACTIVA: VISTA INTEGRAL + VENTANA EMERGENTE (Borde Simple ┌─┐ y Doble ╔═╗)     ║\n")
	b.WriteString("╠═════════════════════════════╬════════════════════════════════════════════════════════════════════════════════════════╣\n")
	b.WriteString("║ ▸ 1. Cluster & DB Estado    ║ ┌─ NODOS DEL CLUSTER & INSTANCIAS (ORACLE RAC / SINGLE) ─────────────────────────────┐ ║\n")
	b.WriteString("║   2. Usuarios Conectados    ║ │ NODO       ESTADO   CRS     CSSD    ASM            INST      MODO        FRA_USO   │ ║\n")
	b.WriteString("║   3. Tablespaces & Files    ║ │ ora-rac01  ONLINE   ONLINE  ONLINE  ONLINE(+ASM1)  PRODCDB1  READ WRITE  [████░░░░]│ ║\n")
	b.WriteString("║   4. Tablas & Objetos       ║ │ ora-rac02  ONLINE   ONLINE  ONLINE  ONLINE(+ASM2)  PRODCDB2  READ WRITE  [████░░░░]│ ║\n")
	b.WriteString("║   5. ASM & Red Listeners    ║ └────────────────────────────────────────────────────────────────────────────────────┘ ║\n")
	b.WriteString("║   6. RMAN, Flashback & DG   ║ ┌─ USUARIOS CONECTADOS (gv$session) ───────────┬─ TABLESPACES & BACKUPS RMAN ────────┐ ║\n")
	b.WriteString("║   7. Alert Log & Errores    ║ │ SID,SER#  USUARIO    ESTADO   EVENTO / LOCK  │ RECURSO       ESTADO / USO %        │ ║\n")
	b.WriteString("║   8. Centro Mantenimiento   ║ │ 142,48219 ERP_APP    ACTIVE   db file seq rd │ ERP_DATA_TBS  [█████████░] 93.0% !! │ ║\n")
	b.WriteString("║   b. Cambiar Marcos ┌─┐/╔═╗ ║ │ 289,11042 CRM_BATCH  ACTIVE   enq: TX lock   │ SYSAUX        [████████░░] 84.4%    │ ║\n")
	b.WriteString("║   t. Cambiar Topologia      ║ │ 512,9931  CRM_ADMIN  INACT    BLOCKER (289)  │ RMAN INCR L0  COMPLETED (498.4 GB)  │ ║\n")
	b.WriteString("║   q. Salir (Quit)           ║ │ 604,33105 BI_REPORT  ACTIVE   ON CPU (512MB) │ GRP_RELEASE   GUARANTEED (18.4 GB)  │ ║\n")
	b.WriteString("║                             ║ └──────────────────────────────────────────────┴─────────────────────────────────────┘ ║\n")
	b.WriteString("║                             ║                                                                                        ║\n")
	b.WriteString("║                             ║      ╔══════════════════════════════════════════════════════════════════════════╗      ║\n")
	b.WriteString("║                             ║      ║      ══ VENTANA DE MANTENIMIENTO [Seleccione con Flechas ↑↓ + Enter] ══  ║      ║\n")
	b.WriteString("║                             ║      ╠══════════════════════════════════════════════════════════════════════════╣      ║\n")
	b.WriteString("║                             ║      ║ ▸ [1] Drenar / Aislar Nodo Cluster (Relocate Services + Stop CRS)        ║      ║\n")
	b.WriteString("║                             ║      ║   [2] Parar / Arrancar Instancia o Base de Datos (srvctl / sqlplus)      ║      ║\n")
	b.WriteString("║                             ║      ║   [3] Ampliar Tablespace Crítico (+Datafile 32G a ERP_DATA_TBS 93%)      ║      ║\n")
	b.WriteString("║                             ║      ║   [4] Matar Sesión Bloqueante (KILL SESSION '512,9931,@2' IMMEDIATE)     ║      ║\n")
	b.WriteString("║                             ║      ║   [5] Actualizar Estadísticas (DBMS_STATS) & Shrink Tablas Online        ║      ║\n")
	b.WriteString("║                             ║      ║   [6] Recompilar Objetos Inválidos (UTL_RECOMP / COMPILE_SCHEMA)         ║      ║\n")
	b.WriteString("║                             ║      ║   [7] Backup RMAN Archivelog + Crear Punto Restauración Garantizado      ║      ║\n")
	b.WriteString("║                             ║      ║   [8] Analizar Errores ORA- en Alert Log & Purgar Trazas ADRCI           ║      ║\n")
	b.WriteString("║                             ║      ║   [Esc] Cancelar / Cerrar Ventana                                        ║      ║\n")
	b.WriteString("║                             ║      ╚══════════════════════════════════════════════════════════════════════════╝      ║\n")
	b.WriteString("╠═════════════════════════════╩════════════════════════════════════════════════════════════════════════════════════════╣\n")
	b.WriteString("║ ┌─ CONSOLA DE EJECUCION & AUDITORIA EN VIVO ───────────────────────────────────────────────────────────────────────┐ ║\n")
	b.WriteString("║ │ [08:05:01] Auto-Discovery: Modo=ORACLE RAC │ Cluster=crs-prod-cluster │ Release=19.24.0.0.0                      │ ║\n")
	b.WriteString("║ │ [08:05:02] ALERTA ORA-01654: Tablespace ERP_DATA_TBS al 93.0% │ Bloqueo TX detectado en SID 512 -> SID 289       │ ║\n")
	b.WriteString("║ └──────────────────────────────────────────────────────────────────────────────────────────────────────────────────┘ ║\n")
	b.WriteString("╚══════════════════════════════════════════════════════════════════════════════════════════════════════════════════════╝\n")
	b.WriteString(" [Flechas ↑↓←→] Navegar │ [Enter] Ventana Acciones │ [Tab] Cambiar Panel │ [m] Mantenimiento │ [b] Marcos ┌─┐/╔═╗ │ [q] Salir\n")

	return b.String()
}

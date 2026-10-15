# `ora-tui` — Oracle Cluster (RAC / Restart) & Single Instance TUI Manager

![Go Version](https://img.shields.io/badge/Go-1.22%2B-00ADD8?logo=go)
![Platform](https://img.shields.io/badge/Platform-RHEL%20%7C%20Oracle%20Linux%207%2F8%2F9%20%7C%20macOS%20(Mock)-red)
![Dependencies](https://img.shields.io/badge/CGO_ENABLED-0%20(Zero%20Deps)-success)
![License](https://img.shields.io/badge/License-Apache%202.0-blue)

**`ora-tui`** es una herramienta interactiva de consola (**Terminal User Interface - TUI**) escrita en **Go** y compilada como un único binario estático sin dependencias externas (`CGO_ENABLED=0`). Permite analizar en tiempo real entornos **Oracle RAC**, **Oracle Restart** y **Oracle Single Instance**, inspeccionar usuarios conectados, bloqueos, tablespaces, tablas, almacenamiento ASM, backups RMAN y errores del Alert Log, y ejecutar tareas de mantenimiento críticas mediante **menús navegables con las flechas del teclado** y **ventanas con marcos Unicode simples (`┌─┐`) y dobles (`╔═╗`)**.

---

## Vista Previa de la Interfaz (`ora-tui --preview`)

```text
╔══════════════════════════════════════════════════════════════════════════════════════════════════════════════════════╗
║                              ══ ORA-TUI :: ORACLE CLUSTER & DATABASE MANAGER ══                                      ║
╠══════════════════════════════════════════════════════════════════════════════════════════════════════════════════════╣
║ Modo: [ORACLE RAC] [MOCK DESIGN] │ Host: ora-rac01.corp.local │ Cluster: crs-prod-cluster   │ Ver: 19.24.0.0.0       ║
║ ORACLE_HOME: /u01/app/oracle/product/19.0.0/dbhome_1  │ GRID_HOME: /u01/app/19.0.0/grid                              ║
║ Estado: 2 Inst OPEN │ 4 PDBs │ 3 Sesiones (1 Lock TX) │ 1 Tablespace >90% │ 3 Inválidos │ 1 Alerta Crítica ORA-01654 ║
╠═════════════════════════════╦════════════════════════════════════════════════════════════════════════════════════════╣
║ MENU [Flechas ↑↓ + Enter]   ║ PANTALLA ACTIVA: VISTA INTEGRAL + VENTANA EMERGENTE (Borde Simple ┌─┐ y Doble ╔═╗)     ║
╠═════════════════════════════╬════════════════════════════════════════════════════════════════════════════════════════╣
║ ▸ 1. Cluster & DB Estado    ║ ┌─ NODOS DEL CLUSTER & INSTANCIAS (ORACLE RAC / SINGLE) ─────────────────────────────┐ ║
║   2. Usuarios Conectados    ║ │ NODO       ESTADO   CRS     CSSD    ASM            INST      MODO        FRA_USO   │ ║
║   3. Tablespaces & Files    ║ │ ora-rac01  ONLINE   ONLINE  ONLINE  ONLINE(+ASM1)  PRODCDB1  READ WRITE  [████░░░░]│ ║
║   4. Tablas & Objetos       ║ │ ora-rac02  ONLINE   ONLINE  ONLINE  ONLINE(+ASM2)  PRODCDB2  READ WRITE  [████░░░░]│ ║
║   5. ASM & Red Listeners    ║ └────────────────────────────────────────────────────────────────────────────────────┘ ║
║   6. RMAN, Flashback & DG   ║ ┌─ USUARIOS CONECTADOS (gv$session) ───────────┬─ TABLESPACES & BACKUPS RMAN ────────┐ ║
║   7. Alert Log & Errores    ║ │ SID,SER#  USUARIO    ESTADO   EVENTO / LOCK  │ RECURSO       ESTADO / USO %        │ ║
║   8. Centro Mantenimiento   ║ │ 142,48219 ERP_APP    ACTIVE   db file seq rd │ ERP_DATA_TBS  [█████████░] 93.0% !! │ ║
║   b. Cambiar Marcos ┌─┐/╔═╗ ║ │ 289,11042 CRM_BATCH  ACTIVE   enq: TX lock   │ SYSAUX        [████████░░] 84.4%    │ ║
║   t. Cambiar Topologia      ║ │ 512,9931  CRM_ADMIN  INACT    BLOCKER (289)  │ RMAN INCR L0  COMPLETED (498.4 GB)  │ ║
║   q. Salir (Quit)           ║ │ 604,33105 BI_REPORT  ACTIVE   ON CPU (512MB) │ GRP_RELEASE   GUARANTEED (18.4 GB)  │ ║
║                             ║ └──────────────────────────────────────────────┴─────────────────────────────────────┘ ║
║                             ║                                                                                        ║
║                             ║      ╔══════════════════════════════════════════════════════════════════════════╗      ║
║                             ║      ║      ══ VENTANA DE MANTENIMIENTO [Seleccione con Flechas ↑↓ + Enter] ══  ║      ║
║                             ║      ╠══════════════════════════════════════════════════════════════════════════╣      ║
║                             ║      ║ ▸ [1] Drenar / Aislar Nodo Cluster (Relocate Services + Stop CRS)        ║      ║
║                             ║      ║   [2] Parar / Arrancar Instancia o Base de Datos (srvctl / sqlplus)      ║      ║
║                             ║      ║   [3] Ampliar Tablespace Crítico (+Datafile 32G a ERP_DATA_TBS 93%)      ║      ║
║                             ║      ║   [4] Matar Sesión Bloqueante (KILL SESSION '512,9931,@2' IMMEDIATE)     ║      ║
║                             ║      ║   [5] Actualizar Estadísticas (DBMS_STATS) & Shrink Tablas Online        ║      ║
║                             ║      ║   [6] Recompilar Objetos Inválidos (UTL_RECOMP / COMPILE_SCHEMA)         ║      ║
║                             ║      ║   [7] Backup RMAN Archivelog + Crear Punto Restauración Garantizado      ║      ║
║                             ║      ║   [8] Analizar Errores ORA- en Alert Log & Purgar Trazas ADRCI           ║      ║
║                             ║      ║   [Esc] Cancelar / Cerrar Ventana                                        ║      ║
║                             ║      ╚══════════════════════════════════════════════════════════════════════════╝      ║
╠═════════════════════════════╩════════════════════════════════════════════════════════════════════════════════════════╣
║ ┌─ CONSOLA DE EJECUCION & AUDITORIA EN VIVO ───────────────────────────────────────────────────────────────────────┐ ║
║ │ [08:05:01] Auto-Discovery: Modo=ORACLE RAC │ Cluster=crs-prod-cluster │ Release=19.24.0.0.0                      │ ║
║ │ [08:05:02] ALERTA ORA-01654: Tablespace ERP_DATA_TBS al 93.0% │ Bloqueo TX detectado en SID 512 -> SID 289       │ ║
║ └──────────────────────────────────────────────────────────────────────────────────────────────────────────────────┘ ║
╚══════════════════════════════════════════════════════════════════════════════════════════════════════════════════════╝
```

---

## ¿Cómo funciona `ora-tui`?

### 1. Auto-Descubrimiento sin Archivos de Configuración (`internal/discovery`)
Al arrancar en cualquier servidor Linux (Oracle 11gR2, 12c, 19c, 21c o 23ai), `ora-tui` descubre automáticamente la topología sin pedir rutas ni credenciales:
1. **Detección de Topología (`detector.go`):**
   * Inspecciona `/etc/oracle/ocr.loc` y `/etc/oracle/olr.loc`.
   * Si `ocr.loc` contiene `local_only=false` $\rightarrow$ activa modo **`ORACLE RAC`**.
   * Si `ocr.loc` contiene `local_only=true` (o existe `olr.loc`) $\rightarrow$ activa modo **`ORACLE RESTART`**.
   * Si no existen $\rightarrow$ activa modo **`SINGLE INSTANCE`**.
2. **Descubrimiento de Homes y Procesos Activos:**
   * Parsea `/etc/oratab` e `inventory.xml` (`/u01/app/oraInventory/ContentsXML/inventory.xml`) para ubicar `$ORACLE_HOME` y `$GRID_HOME`.
   * Escanea `/proc/[0-9]*/cmdline` buscando procesos `ora_pmon_<SID>`, `db_pmon_<SID>` (21c/23ai) y `asm_pmon_+ASM*` para detectar instancias en ejecución incluso si no figuran en `/etc/oratab`.
3. **Fingerprinting de Versión Exacta (`version.go`):**
   * Ejecuta `$ORACLE_HOME/bin/sqlplus -V` y `$GRID_HOME/bin/crsctl query crs activeversion -f` para obtener el nivel exacto de Release Update (por ejemplo `19.24.0.0.0`).

### 2. Gestión Dinámica de Privilegios UNIX (`internal/executor`)
En servidores Oracle de producción, las herramientas de Grid Infrastructure pertenecen a `grid`, los binarios de RDBMS pertenecen a `oracle`, y el control de la pila CRS requiere `root`.
* `ora-tui` inspecciona mediante `os.Stat` el propietario UNIX (`uid`/`gid`) de cada `$ORACLE_HOME` y `$GRID_HOME`.
* Cuando ejecuta consultas SQL (`sqlplus -s / as sysdba`), `srvctl`, `lsnrctl`, `rman` o `adrci`, cambia dinámicamente el contexto del proceso hijo (`syscall.Credential` / `sudo -n -u`) al usuario propietario (`oracle` o `grid`).
* Reserva la ejecución como `root` **exclusivamente** para el arranque o parada de la pila de clusterware (`crsctl start/stop crs`).

### 3. Colectores Asíncronos No Bloqueantes (`internal/collector`)
Un coordinador en segundo plano actualiza las métricas cada 10 segundos (configurable con `--interval`) usando consultas delimitadas por tuberías (`COLSEP '|'`) sin bloquear la interfaz ni requerir librerías C de Oracle Instant Client (`libclntsh.so`).

### 4. Sistema de Ventanas con Marcos Unicode y Menús por Flechas (`internal/ui`)
La interfaz utiliza `tview` + `tcell` con una jerarquía visual de marcos personalizable en vivo con la tecla **`b`**:
* **Borde Simple (`┌ ─ ┐ │ └ ┘`)**: Para paneles de datos secundarios y tablas.
* **Borde Doble (`╔ ═ ╗ ║ ╚ ╝`)**: Para el marco principal, el panel que tiene el foco activo, las **ventanas emergentes de acciones** y los **modales de confirmación de seguridad**.
* **ASCII Clásico (`+ - = |`)**: Modo alternativo de 7 bits para terminales serie o sesiones legacy sin UTF-8.

---

## Los 8 Módulos de Análisis y Mantenimiento

Al navegar con las **Flechas (`↑`/`↓`)** y pulsar **`Enter`** sobre cualquier fila de una tabla, se abre una ventana emergente (`╔═══╗`) con las operaciones de mantenimiento contextuales para ese recurso:

1. **Cluster & DB Estado:**
   * Estado de nodos (`olsnodes`), demonios (`CRS`, `CSSD`, `EVMD`, `ASM`), VIPs, Interconnect, instancias (`gv$instance`), PDBs (`gv$pdbs`) y servicios RAC.
   * **Mantenimiento:** Parada/arranque de instancia o BD (`srvctl` / `SHUTDOWN IMMEDIATE`), apertura/cierre de PDBs, **Drenado seguro de nodo RAC (Graceful Node Drain)** en 4 pasos y **Reincorporación de nodo (Rejoin)**.
2. **Usuarios Conectados (`gv$session`):**
   * Sesiones activas/inactivas, `SID,SERIAL#`, máquina, programa, `SQL_ID`, evento de espera, memoria `PGA`, cadena de bloqueos (`BLOCKER`) y texto SQL en vivo.
   * **Mantenimiento:** `ALTER SYSTEM KILL SESSION 'sid,ser#,@inst' IMMEDIATE`, `DISCONNECT SESSION POST_TRANSACTION` y visualización de plan de ejecución (`DBMS_XPLAN`).
3. **Tablespaces & Datafiles (`dba_tablespaces`, `dba_data_files`):**
   * Capacidad total, espacio usado/libre, barras de progreso visuales (`[████████░░]`), alertas por umbral (>80% amarillo, >90% rojo) y estado de `AUTOEXTEND`.
   * **Mantenimiento:** Añadir nuevo datafile (`+DATA` o filesystem), activar `AUTOEXTEND ON` y consolidar espacio (`COALESCE`).
4. **Tablas, Segmentos & Objetos Inválidos (`dba_segments`, `dba_objects`):**
   * Top tablas e índices por tamaño, filas, fecha de análisis, estado de estadísticas (`FRESH`/`STALE`), fragmentación y listado de objetos `INVALID`.
   * **Mantenimiento:** Recolectar estadísticas (`DBMS_STATS.GATHER_TABLE_STATS`), compactar tabla online (`ENABLE ROW MOVEMENT` + `SHRINK SPACE CASCADE`), reconstruir índices (`REBUILD ONLINE`) y recompilar objetos inválidos (`COMPILE_SCHEMA` / `UTL_RECOMP`).
5. **ASM & Red Listeners (`v$asm_diskgroup`, `lsnrctl`):**
   * Estado y redundancia de diskgroups ASM (`USABLE_FILE_MB`, `OFFLINE_DISKS`) y Listeners locales/SCAN.
   * **Mantenimiento:** `lsnrctl start`, `stop` y `reload`.
6. **RMAN, Flashback & Data Guard:**
   * Historial de backups (`v$rman_backup_job_details`), puntos de restauración garantizados (`v$restore_point`) y estado de sincronización de Data Guard Standby (`MRP0`, `Transport/Apply Lag`).
   * **Mantenimiento:** Lanzar backup de Archivelogs o Incremental Nivel 1, `CROSSCHECK`, y crear *Guaranteed Restore Points (GRP)*.
7. **Alert Log & Errores `ORA-` (`v$diag_alert_ext`):**
   * Visor filtrado de errores críticos (`ORA-01654`, `ORA-00060`, `ORA-00600`) con salto directo al módulo de resolución y limpieza de trazas antiguas mediante `adrci exec="purge -age 10080 -type TRACE"`.
8. **Centro de Mantenimiento:**
   * Catálogo centralizado con todos los asistentes guiados paso a paso.

---

## Controles de Teclado

| Tecla | Acción |
| :--- | :--- |
| **`↑` / `↓` / `←` / `→`** | Moverse por el menú lateral, filas de tablas y opciones de ventanas emergentes |
| **`Enter`** | Abrir ventana emergente de mantenimiento para el elemento seleccionado / Confirmar |
| **`Tab`** | Alternar el foco entre el menú lateral izquierdo y la tabla de datos activa |
| **`1` .. `8`** | Saltar directamente a cualquiera de los 8 módulos |
| **`m`** | Abrir la ventana rápida global de tareas de mantenimiento desde cualquier vista |
| **`b`** | Alternar en vivo el estilo de marcos: *Mixto (`┌─┐`/`╔═╗`)* $\rightarrow$ *Doble (`╔═╗`)* $\rightarrow$ *Simple (`┌─┐`)* $\rightarrow$ *ASCII (`+-|`)* |
| **`t`** | Alternar topología en modo simulación (`ORACLE RAC` $\leftrightarrow$ `SINGLE INSTANCE`) |
| **`Esc`** | Cerrar ventana modal emergente o devolver el foco al menú principal |
| **`q`** | Salir de `ora-tui` |

---

## Instalación y Uso

### 1. Instalar directamente en RHEL / Oracle Linux 7, 8 o 9 (vía `dnf`)

#### Opción A — Instalar directamente desde GitHub por URL (1 comando en el servidor)
Si el servidor tiene salida a GitHub, puedes instalar el paquete `.rpm` precompilado directamente con `dnf`:
```bash
sudo dnf install -y https://github.com/mario-ezquerro/oratui/raw/main/dist/ora-tui-1.0.0-1.x86_64.rpm
sudo ora-tui
```

#### Opción B — Copiar el `.rpm` por `scp` a un servidor sin salida a Internet
Desde tu equipo local (donde has clonado el repositorio):
```bash
scp dist/ora-tui-1.0.0-1.x86_64.rpm root@srv-dbgriar-des:/root/
```
Y dentro del servidor Linux (`root@srv-dbgriar-des`):
```bash
sudo dnf install -y /root/ora-tui-1.0.0-1.x86_64.rpm
sudo ora-tui
```

---

### 2. Ejecutar en Modo Simulación / Mock (macOS / Linux sin Oracle instalado)
Si ejecutas `ora-tui` en un equipo de desarrollo sin Oracle instalado (o pasas `--mock`), se inicia automáticamente el simulador interactivo completo:

```bash
# Compilar y lanzar el TUI interactivo en modo Mock RAC:
make mock

# O ejecutar directamente el binario:
./bin/ora-tui --mock --topology=rac
./bin/ora-tui --mock --topology=single

# Imprimir vista previa estática en el terminal:
./bin/ora-tui --preview
```

---

### 3. Compilar Binario Estático y Generar el Paquete RPM desde el Código Fuente
```bash
# Ejecutar tests unitarios y compilar binarios (nativo + Linux amd64 estático CGO_ENABLED=0):
make all

# Generar automáticamente el paquete dist/ora-tui-1.0.0-1.x86_64.rpm (usa nfpm vía Go):
make rpm
```

---

## Estructura del Proyecto

```text
ora-tui/
├── cmd/ora-tui/main.go            # Punto de entrada, flags CLI y ciclo de refresco
├── internal/
│   ├── model/state.go             # Modelos de dominio (Cluster, Instancias, Sesiones, Tablespaces, RMAN, etc.)
│   ├── discovery/                 # Auto-detección de ocr.loc, oratab, inventory.xml, /proc PMON y versión
│   ├── collector/                 # Colectores SQL*Plus, crsctl, olsnodes, srvctl, lsnrctl y modo Mock
│   ├── actions/                   # Flujos de parada/arranque, Node Drain, mantenimiento DBA, RMAN y ADRCI
│   ├── executor/runner.go         # Ejecutor de comandos con cambio de UID/GID y streaming en vivo
│   └── ui/                        # Interfaz tview/tcell con marcos Unicode (┌─┐ / ╔═╗) y modales
├── packaging/                     # Configuración RPM (nfpm.yaml, ora-tui.spec, sudoers.ora-tui)
├── Makefile                       # Automatización de compilación, tests y empaquetado
└── SPEC.md                        # Especificación técnica detallada
```

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

## Los 9 Módulos de Análisis y Mantenimiento

Al hacer **clic con el ratón** o navegar con las **Flechas (`↑`/`↓`) + `Enter`** sobre cualquier fila de una tabla, se abre una ventana emergente con las operaciones de mantenimiento contextuales para ese recurso:

1. **Cluster & DB Estado:**
   * Estado de nodos (`olsnodes`), demonios (`CRS`, `CSSD`, `EVMD`, `ASM`), VIPs, Interconnect, instancias (`gv$instance`), PDBs (`gv$pdbs`) y servicios RAC.
   * **Mantenimiento:** Parada/arranque de instancia o BD (`srvctl` / `SHUTDOWN IMMEDIATE`), apertura/cierre de PDBs, **Drenado seguro de nodo RAC (Graceful Node Drain)** en 4 pasos y **Reincorporación de nodo (Rejoin)**.
2. **Usuarios Conectados (`gv$session`):**
   * Sesiones activas/inactivas, `SID,SERIAL#`, máquina, programa, `SQL_ID`, evento de espera, memoria `PGA`, cadena de bloqueos (`BLOCKER`) y texto SQL en vivo.
   * **Mantenimiento:** `ALTER SYSTEM KILL SESSION 'sid,ser#,@inst' IMMEDIATE`, `DISCONNECT SESSION POST_TRANSACTION` y visualización de plan de ejecución (`DBMS_XPLAN`).
3. **Tablespaces & Datafiles (`dba_tablespaces`, `dba_data_files`):**
   * Capacidad total (considerando `AUTOEXTEND` y `MAXBYTES`), espacio usado/libre real, barras de progreso visuales (`[████████░░]`) y alertas por umbral (>80% amarillo, >90% rojo).
   * **Mantenimiento:** Añadir nuevo datafile (`+DATA` o filesystem), activar `AUTOEXTEND ON` y consolidar espacio (`COALESCE`).
4. **Tablas, Segmentos & Objetos Inválidos (`dba_segments`, `dba_objects`):**
   * Top tablas e índices por tamaño, filas, fecha de análisis, estado de estadísticas (`FRESH`/`STALE`), fragmentación y listado de objetos `INVALID`.
   * **Mantenimiento:** Recolectar estadísticas (`DBMS_STATS.GATHER_TABLE_STATS`), compactar tabla online (`ENABLE ROW MOVEMENT` + `SHRINK SPACE CASCADE`), reconstruir índices (`REBUILD ONLINE`) y recompilar objetos inválidos (`COMPILE_SCHEMA` / `UTL_RECOMP`).
5. **ASM & Red Listeners (`v$asm_diskgroup`, `lsnrctl`):**
   * Estado y redundancia de diskgroups ASM (`USABLE_FILE_MB`, `OFFLINE_DISKS`) y Listeners locales/SCAN con distinción entre servicios **`READY`** (LREG/PMON dinámico) y **`UNKNOWN`** (`listener.ora` estático).
   * **Mantenimiento:** `lsnrctl start`, `stop`, `reload`, reinicio completo y `ALTER SYSTEM REGISTER`.
6. **RMAN, Flashback & Data Guard:**
   * Historial de backups (`v$rman_backup_job_details`), puntos de restauración garantizados (`v$restore_point`) y estado de sincronización de Data Guard Standby (`MRP0`, `Transport/Apply Lag`).
   * **Mantenimiento:** Lanzar backup de Archivelogs o Incremental Nivel 1, `CROSSCHECK`, y crear *Guaranteed Restore Points (GRP)*.
7. **Alert Log & Errores `ORA-` (`alert_<SID>.log`):**
   * Tabla estricta de errores `^ORA-` (ignorando falsos positivos de parches OPatch), herramientas de diagnóstico ADR (`oerr`, `adrci purge`, `SWITCH LOGFILE`, `CHECKPOINT` y rotación del `alert.log`) y visor *tail* en vivo.
8. **Servidor, Discos & SAN (`RAM, Load, HugePages, Inodos, HBA FC & Multipath`):**
   * **Panel `[1/3]` (CPU, Load, Memoria & Kernel Oracle):** Carga del sistema (`Load Average 1m/5m/15m` frente al número de cores), Memoria RAM física/disponible/caché, Swap y `vm.swappiness`, **HugePages estáticas (`2MB`)**, verificación de **Transparent HugePages (`THP = [never]`)**, slots de I/O asíncrono del kernel (`fs.aio-nr` / `fs.aio-max-nr` para DBWR/LGWR) y descriptores de fichero (`fs.file-nr`).
   * **Panel `[2/3]` (Filesystems & Inodos — `df -PTm` / `df -Pi`):** Ocupación en GB y porcentaje de todos los puntos de montaje (`/u01`, `/u02`, `/`, `/dev/shm`, `/var`, `/tmp`) junto con el **% de uso de Inodos** (crítico para evitar `ORA-09925` por millones de ficheros `.aud`/`.trc`).
   * **Panel `[3/3]` (Almacenamiento SAN, Multipath `multipath -ll`, HBA Fibre Channel & Discos):** Comprobación de que **todos los caminos físicos (`paths`) de cada LUN SAN están activos (`4/4 ACTIVE`)** o detección inmediata de caminos caídos (`3/4 DEGRADED - FAULTY`), estado y velocidad (`16G`/`32G`) de tarjetas HBA Fibre Channel (`/sys/class/fc_host/host*`), discos `ORACLEASM` y dispositivos de bloque con su *I/O Scheduler*.
   * **Mantenimiento de Servidor:** Auditoría `sysctl`, Top procesos por CPU/RAM, liberación segura de `PageCache` (`vm.drop_caches=3`), análisis `du -shx` por punto de montaje, y **reescaneo en caliente de buses SCSI/FC y Multipath (`scsi_host scan` + `multipath -r`)**.
9. **Centro de Mantenimiento:**
   * Catálogo centralizado con todos los asistentes guiados paso a paso.

---

## Controles de Teclado y Ratón (SSH)

| Tecla / Ratón | Acción |
| :--- | :--- |
| **Ratón (Hover / Clic / Clic Derecho)** | **Mover el ratón** resalta en tiempo real la opción del menú o fila de tabla bajo el puntero. **Clic izquierdo** selecciona o abre el submenú de acciones. **Clic derecho** cierra cualquier ventana emergente |
| **`↑` / `↓` / `←` / `→`** | Moverse por el menú lateral, paneles de ventanas (`←`/`→`), filas de tablas (`↑`/`↓`) y opciones de submenús |
| **`Enter`** | Abrir ventana emergente de mantenimiento para el elemento seleccionado / Confirmar |
| **`Tab` / `Shift+Tab`** | Alternar el foco entre el menú lateral izquierdo y las sub-ventanas del módulo activo |
| **`1` .. `9`** | Saltar directamente a cualquiera de los 9 módulos (`8` = Servidor, Discos & SAN) |
| **`F1` / `h`** | **Activar / Desactivar la ayuda en los menús** (compacta el menú lateral y los submenús emergentes) |
| **`c`** | **Cambiar tema de color retro en vivo**: *Norton Commander* $\rightarrow$ *Borland Turbo C++ 3.0* $\rightarrow$ *Fósforo Verde (IBM 5151 CRT)* $\rightarrow$ *Terminal Oscuro* |
| **`b`** | Alternar en vivo el estilo de marcos: *Mixto (`┌─┐`/`╔═╗`)* $\rightarrow$ *Doble (`╔═╗`)* $\rightarrow$ *Simple (`┌─┐`)* $\rightarrow$ *ASCII (`+-|`)* |
| **`m`** | Abrir la ventana rápida global de tareas de mantenimiento desde cualquier vista |
| **`t`** | Alternar topología en modo simulación (`ORACLE RAC` $\leftrightarrow$ `SINGLE INSTANCE`) |
| **`Esc` / `←`** | Cerrar ventana modal emergente o devolver el foco al menú principal |
| **`Ctrl+C` / `q`** | Salir inmediatamente de `ora-tui` |

---

## Instalación y Reinstalación (`dnf`, `rpm` y `apt-get`)

### 1. Usando `dnf` (RHEL / Oracle Linux / Rocky / AlmaLinux 8 y 9)

#### Instalar por primera vez con `dnf`
* **Directamente desde GitHub:**
  ```bash
  sudo dnf install -y https://github.com/mario-ezquerro/oratui/raw/main/dist/ora-tui-1.0.0-1.x86_64.rpm
  ```
* **Desde un archivo `.rpm` descargado localmente (o copiado por `scp`):**
  ```bash
  sudo dnf install -y ./ora-tui-1.0.0-1.x86_64.rpm
  ```

#### Reinstalar / Actualizar a la última compilación con `dnf`
Cuando se actualiza el paquete en GitHub manteniendo la misma versión (`1.0.0-1`), usa `dnf reinstall` (y limpia la caché si es necesario):
```bash
sudo dnf clean packages
sudo dnf reinstall -y https://github.com/mario-ezquerro/oratui/raw/main/dist/ora-tui-1.0.0-1.x86_64.rpm
```
*(O si tienes el archivo en local: `sudo dnf reinstall -y ./ora-tui-1.0.0-1.x86_64.rpm`)*

---

### 2. Usando `rpm` directamente (RHEL / Oracle Linux / CentOS / SUSE)

#### Instalar por primera vez con `rpm`
* **Directamente desde GitHub:**
  ```bash
  sudo rpm -ivh https://github.com/mario-ezquerro/oratui/raw/main/dist/ora-tui-1.0.0-1.x86_64.rpm
  ```
* **Desde un archivo local:**
  ```bash
  sudo rpm -ivh ./ora-tui-1.0.0-1.x86_64.rpm
  ```

#### Reinstalar / Forzar actualización con `rpm`
Para reinstalar encima de una versión ya instalada (forzando la sustitución de los ficheros con `--replacepkgs` / `--force`):
* **Directamente desde GitHub:**
  ```bash
  sudo rpm -Uvh --replacepkgs --force https://github.com/mario-ezquerro/oratui/raw/main/dist/ora-tui-1.0.0-1.x86_64.rpm
  ```
* **Desde un archivo local:**
  ```bash
  sudo rpm -Uvh --replacepkgs --force ./ora-tui-1.0.0-1.x86_64.rpm
  ```

---

### 3. Usando `apt-get` / `apt` (Debian / Ubuntu)

El repositorio incluye también el paquete Debian/Ubuntu precompilado en `dist/ora-tui_1.0.0-1_amd64.deb`.

#### Instalar por primera vez con `apt-get`
```bash
curl -fL -o /tmp/ora-tui_1.0.0-1_amd64.deb https://github.com/mario-ezquerro/oratui/raw/main/dist/ora-tui_1.0.0-1_amd64.deb
sudo apt-get install -y /tmp/ora-tui_1.0.0-1_amd64.deb
```
*(También puedes usar `sudo dpkg -i /tmp/ora-tui_1.0.0-1_amd64.deb`)*

#### Reinstalar / Actualizar a la última compilación con `apt-get`
Para volver a descargar el paquete `.deb` actualizado desde GitHub y forzar su reinstalación con `apt-get`:
```bash
curl -fL -o /tmp/ora-tui_1.0.0-1_amd64.deb https://github.com/mario-ezquerro/oratui/raw/main/dist/ora-tui_1.0.0-1_amd64.deb
sudo apt-get install --reinstall -y /tmp/ora-tui_1.0.0-1_amd64.deb
```
*(O bien con `dpkg`: `sudo dpkg -i --force-overwrite /tmp/ora-tui_1.0.0-1_amd64.deb`)*

---

### 4. Ejecutar `ora-tui` una vez instalado
```bash
sudo ora-tui
```

---

### 5. Ejecutar en Modo Simulación / Mock (macOS / Linux sin Oracle instalado)
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

### 6. Compilar Binario Estático y Generar los Paquetes `.rpm` y `.deb` desde el Código Fuente
```bash
# Ejecutar tests unitarios y compilar binarios (nativo + Linux amd64 estático CGO_ENABLED=0):
make all

# Generar automáticamente ambos paquetes en dist/ (.rpm y .deb):
make packages
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
│   └── ui/                        # Interfaz tview/tcell con ratón SSH y temas Norton Commander / Turbo C / Fósforo Verde
├── packaging/                     # Configuración nfpm.yaml, ora-tui.spec, sudoers.ora-tui
├── dist/                          # Paquetes precompilados (.rpm y .deb) listos para instalar desde GitHub
├── Makefile                       # Automatización de compilación, tests y empaquetado (.rpm / .deb)
└── SPEC.md                        # Especificación técnica detallada
```

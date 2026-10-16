# `ora-tui` — Oracle Cluster (RAC / Restart), Single Instance, Server/SAN & OpenTelemetry Manager

![Go Version](https://img.shields.io/badge/Go-1.22%2B-00ADD8?logo=go)
![Version](https://img.shields.io/badge/Version-1.6.0-brightgreen)
![Platform](https://img.shields.io/badge/Platform-RHEL%20%7C%20Oracle%20Linux%207%2F8%2F9%20%7C%20Debian%2FUbuntu%20%7C%20macOS%20(Mock)-red)
![Dependencies](https://img.shields.io/badge/CGO_ENABLED-0%20(Zero%20Deps)-success)
![Telemetry](https://img.shields.io/badge/OpenTelemetry%20%26%20Prometheus-Port%209161-orange)
![License](https://img.shields.io/badge/License-Apache%202.0-blue)

**`ora-tui`** es una herramienta interactiva de consola (**Terminal User Interface - TUI**) y **Exportador OpenTelemetry / Prometheus embebido** escrita en **Go** y compilada como un único binario estático sin dependencias externas (`CGO_ENABLED=0`).

Permite analizar y operar en tiempo real entornos **Oracle RAC**, **Oracle Restart**, **Oracle Single Instance** y la **salud del servidor Linux y almacenamiento SAN/Multipath**:
* **9 Módulos Interactivos de Análisis y Mantenimiento**: Clusterware/Instancias/PDBs, Sesiones/Bloqueos TX-TM, Tablespaces/Datafiles (`AUTOEXTEND`), Segmentos/Objetos Inválidos, ASM/Listeners, Backups RMAN/Flashback/Data Guard, Alert Log (`ORA-`), **Servidor Linux/Discos/SAN Multipath (`multipath -ll`, HBA FC, Inodos `df -i`, HugePages/THP)** y Centro de Mantenimiento.
* **Ventana Emergente de Resultados de Operaciones**: Toda operación, diagnóstico o comando ejecutado muestra una ventana modal emergente desplazable (`↑`/`↓`/`PgUp`/`PgDn`) con la salida completa en tiempo real (`multipath -ll`, `lsblk`, `sysctl`, `df -Phi`, `du -shx`, `sqlplus`, `rman`, `lsnrctl`, `crsctl`, `srvctl`, `adrci`).
* **4 Temas de Color Retro MS-DOS conmutables en vivo (`c`)**: *Norton Commander 5.0*, *Borland Turbo C++ 3.0*, *Monitor CRT de Fósforo Verde (IBM 5151 P1)* y *Terminal Oscuro*.
* **Exportador OpenTelemetry (`OTLP JSON`) y Prometheus (`/metrics`) integrado (`--opentelemetry` / `--daemon`)**: Expone las **52 métricas** del servidor y base de datos en el puerto `9161` (configurable con `--otel-port`), e incluye una carpeta **[`POC/`](file:///Users/mario/repositorios/oratui/POC)** lista para levantar en cualquier servidor con Docker un stack completo de **Prometheus + Alertas + Grafana (26 paneles autoprovisionados)** ejecutando un único script interactivo (`./configure-poc.sh`).

---

## Vista Previa de la Interfaz (`ora-tui --preview`)

```text
╔══════════════════════════════════════════════════════════════════════════════════════════════════════════════════════╗
║                    ══ NORTON COMMANDER ORA-TUI v1.6.0 :: ORACLE CLUSTER, DB & SERVER MANAGER ══                      ║
╠══════════════════════════════════════════════════════════════════════════════════════════════════════════════════════╣
║ Modo: [ORACLE RAC] │ Host: ora-rac01.corp.local │ Cluster: crs-prod-cluster │ Ver: 19.24.0.0.0 │ Tema: NORTON        ║
║ HOME: /u01/app/oracle/product/19.0.0/dbhome_1 (oracle) │ 2 Inst OPEN │ 4 PDBs │ 4 Sesiones (1 Locks) │ 1 Errores ORA-║
╠═════════════════════════════╦════════════════════════════════════════════════════════════════════════════════════════╣
║ MENU [Flechas ↑↓ + Enter]   ║ PANTALLA ACTIVA: VISTA INTEGRAL + VENTANA EMERGENTE (Borde Simple ┌─┐ y Doble ╔═╗)     ║
╠═════════════════════════════╬════════════════════════════════════════════════════════════════════════════════════════╣
║ ▸ 1. Cluster & DB Estado    ║ ┌─ NODOS DEL CLUSTER & INSTANCIAS (ORACLE RAC / SINGLE) ─────────────────────────────┐ ║
║   2. Usuarios Conectados    ║ │ NODO       ESTADO   CRS     CSSD    ASM            INST      MODO        FRA_USO   │ ║
║   3. Tablespaces & Files    ║ │ ora-rac01  ONLINE   ONLINE  ONLINE  ONLINE(+ASM1)  PRODCDB1  READ WRITE  [████░░░░]│ ║
║   4. Tablas & Objetos       ║ │ ora-rac02  ONLINE   ONLINE  ONLINE  ONLINE(+ASM2)  PRODCDB2  READ WRITE  [████░░░░]│ ║
║   5. ASM & Red Listeners    ║ └────────────────────────────────────────────────────────────────────────────────────┘ ║
║   6. RMAN, Flashback & DG   ║ ┌─ USUARIOS CONECTADOS (gv$session) ───────────┬─ TABLESPACES & SAN MULTIPATH ───────┐ ║
║   7. Alert Log & Errores    ║ │ SID,SER#  USUARIO    ESTADO   EVENTO / LOCK  │ RECURSO       ESTADO / USO %        │ ║
║   8. Servidor, Discos & SAN ║ │ 142,48219 ERP_APP    ACTIVE   db file seq rd │ ERP_DATA_TBS  [█████████░] 93.0% !! │ ║
║   9. Centro Mantenimiento   ║ │ 289,11042 CRM_BATCH  ACTIVE   enq: TX lock   │ mpatha (ASM)  4/4 PATHS ACTIVE (OK) │ ║
║   c. Cambiar Tema Color     ║ │ 512,9931  CRM_ADMIN  INACT    BLOCKER (289)  │ mpathc (FRA)  3/4 DEGRADED (FAULTY) │ ║
║   b. Cambiar Marcos ┌─┐/╔═╗ ║ │ 604,33105 BI_REPORT  ACTIVE   ON CPU (512MB) │ /u01 Inodos   14.2% (Libre ORA-9925)│ ║
║   F1/h. Mostrar/Ocultar Ayu ║ └──────────────────────────────────────────────┴─────────────────────────────────────┘ ║
║   t. Cambiar Topologia      ║                                                                                        ║
║   q. Salir (Quit)           ║      ╔══════════════════════════════════════════════════════════════════════════╗      ║
║                             ║      ║      ══ VENTANA DE MANTENIMIENTO [Seleccione con Flechas ↑↓ + Enter] ══  ║      ║
║                             ║      ╠══════════════════════════════════════════════════════════════════════════╣      ║
║                             ║      ║ ▸ [1] Drenar / Aislar Nodo Cluster (Relocate Services + Stop CRS)        ║      ║
║                             ║      ║   [2] Parar / Arrancar Instancia o Base de Datos (srvctl / sqlplus)      ║      ║
║                             ║      ║   [3] Ampliar Tablespace Crítico (+Datafile 32G a ERP_DATA_TBS 93%)      ║      ║
║                             ║      ║   [4] Matar Sesión Bloqueante (KILL SESSION '512,9931,@2' IMMEDIATE)     ║      ║
║                             ║      ║   [5] Auditoría Completa SAN Multipath (multipath -ll) & HBA FC          ║      ║
║                             ║      ║   [Esc] Cancelar / Cerrar Ventana                                        ║      ║
║                             ║      ╚══════════════════════════════════════════════════════════════════════════╝      ║
╠═════════════════════════════╩════════════════════════════════════════════════════════════════════════════════════════╣
║ ┌─ C:\ORACLE> CONSOLA MS-DOS & AUDITORIA EN VIVO ──────────────────────────────────────────────────────────────────┐ ║
║ │ C:\> [08:05:01] Auto-Discovery: Modo=ORACLE RAC │ Cluster=crs-prod-cluster │ Release=19.24.0.0.0                 │ ║
║ │ C:\> [08:05:02] ALERTA SAN: mpathc presenta 1 camino caído (3/4 DEGRADED) │ Tablespace ERP_DATA_TBS al 93.0%     │ ║
║ └──────────────────────────────────────────────────────────────────────────────────────────────────────────────────┘ ║
╚══════════════════════════════════════════════════════════════════════════════════════════════════════════════════════╝
```

---

## Arquitectura y Funcionamiento Interno

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
* Reserva la ejecución como `root` **exclusivamente** para el arranque o parada de la pila de clusterware (`crsctl start/stop crs`) o reescaneo de buses SCSI/Multipath.

### 3. Colectores Asíncronos No Bloqueantes (`internal/collector`)
Un coordinador en segundo plano actualiza las métricas cada 10 segundos (configurable con `--interval`) usando consultas delimitadas por tuberías (`COLSEP '|'`) e inspecciones nativas de `/proc` y `/sys` sin bloquear la interfaz ni requerir librerías C de Oracle Instant Client (`libclntsh.so`).

### 4. Ventana Emergente de Resultados de Operaciones (`Operation Output Modal`)
Cada vez que ejecutas una acción desde cualquier submenú (como auditar `multipath -ll`, inspeccionar `lsblk`, `sysctl`, `df -Phi`, `du -shx`, `lsnrctl status`, planes `DBMS_XPLAN`, backups RMAN o comandos `srvctl`/`crsctl`), `ora-tui`:
1. Abre automáticamente una **ventana modal centrada de gran tamaño (`╔══ RESULTADO DE OPERACION: ... ══╗`)** que muestra en tiempo real **todas las líneas de salida (`stdout`/`stderr`)** del comando ejecutado.
2. Permite desplazarse libremente por salidas largas con **`↑` / `↓` / `PgUp` / `PgDn`** y cerrarla con **`Enter`**, **`Esc`**, **`q`** o **clic derecho del ratón**.

### 5. Los 4 Temas de Color Retro MS-DOS (`c`) y Marcos Unicode (`b`)
Pulsando la tecla **`c`** en cualquier momento puedes alternar en vivo entre 4 paletas clásicas:
1. **`NORTON COMMANDER` (Por defecto)**: Fondo Azul Real (`#0000AF`), bordes y datos en Cian Brillante (`#5FFFFF`), cabeceras de columna en Amarillo Brillante (`#FFFF5F`), barra selectora en Turquesa (`#00AFAF`) con texto negro y consola inferior MS-DOS sobre fondo negro (`C:\ORACLE>`).
2. **`TURBO C++ 3.0` (Borland IDE)**: Barra superior gris plata (`#A8A8A8`) con icono rojo `[≡]`, ventana principal de edición en Azul Real (`#0000AF`) con marco doble blanco (`#FFFFFF`), barra activa en Verde (`#00AF00`) y ventana inferior de proyecto (`Project:`) en Turquesa (`#00AFAF`) con marco y texto Azul Oscuro (`#0000AF`).
3. **`FOSFORO VERDE (CRT)` (Monitor monocromo IBM 5151 P1)**: Fondo negro verdoso profundo (`#001000`), texto en verde fósforo brillante (`#00FF5F`) y barra selectora en **vídeo inverso real al 100% del ancho de fila** (fondo `#00FF5F` con texto `#001000`).
4. **`TERMINAL OSCURO` (Modern Dark)**: Tema oscuro de alto contraste sobre fondo negro.

Además, pulsando la tecla **`b`** puedes alternar los marcos de las ventanas entre:
* **Mixto (`┌─┐` / `╔═╗`)**, **Doble Completo (`╔═╗`)**, **Simple Completo (`┌─┐`)** y **ASCII Clásico de 7 bits (`+ - |`)**.

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
   * Capacidad total real (considerando `AUTOEXTEND` y `MAXBYTES`), espacio asignado en disco, espacio usado/libre real, barras de progreso visuales (`[████████░░]`) y alertas por umbral (>80% amarillo, >90% rojo).
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
   * **Mantenimiento de Servidor:** Auditoría `sysctl`, Top procesos por CPU/RAM, liberación segura de `PageCache` (`vm.drop_caches=3`), análisis `du -shx` por punto de montaje, auditoría completa `multipath -ll` + `lsblk`, y **reescaneo en caliente de buses SCSI/FC y Multipath (`scsi_host scan` + `multipath -r`)**.
9. **Centro de Mantenimiento:**
   * Catálogo centralizado con todos los asistentes guiados paso a paso.

---

## Controles de Teclado y Ratón (SSH)

| Tecla / Ratón | Acción |
| :--- | :--- |
| **Ratón (Hover / Clic / Clic Derecho)** | **Mover el ratón** resalta en tiempo real la opción del menú o fila de tabla bajo el puntero. **Clic izquierdo** selecciona o abre el submenú de acciones. **Clic derecho** cierra cualquier ventana emergente |
| **`↑` / `↓` / `←` / `→`** | Moverse por el menú lateral, paneles de ventanas (`←`/`→`), filas de tablas (`↑`/`↓`), opciones de submenús y desplazamiento en ventanas de resultados |
| **`Enter`** | Abrir ventana emergente de mantenimiento para el elemento seleccionado / Confirmar / Cerrar ventana de resultado |
| **`Tab` / `Shift+Tab`** | Alternar el foco entre el menú lateral izquierdo y las sub-ventanas del módulo activo |
| **`1` .. `9`** | Saltar directamente a cualquiera de los 9 módulos (`8` = Servidor, Discos & SAN, `9` = Centro Mantenimiento) |
| **`F1` / `h`** | **Activar / Desactivar la ayuda en los menús** (compacta el menú lateral y los submenús emergentes) |
| **`c`** | **Cambiar tema de color retro en vivo**: *Norton Commander* $\rightarrow$ *Borland Turbo C++ 3.0* $\rightarrow$ *Fósforo Verde (IBM 5151 CRT)* $\rightarrow$ *Terminal Oscuro* |
| **`b`** | Alternar en vivo el estilo de marcos: *Mixto (`┌─┐`/`╔═╗`)* $\rightarrow$ *Doble (`╔═╗`)* $\rightarrow$ *Simple (`┌─┐`)* $\rightarrow$ *ASCII (`+-|`)* |
| **`m`** | Abrir la ventana rápida global de tareas de mantenimiento desde cualquier vista |
| **`t`** | Alternar topología en modo simulación (`ORACLE RAC` $\leftrightarrow$ `SINGLE INSTANCE`) |
| **`Esc` / `←`** | Cerrar ventana modal emergente o devolver el foco al menú principal |
| **`Ctrl+C` / `q`** | Salir inmediatamente de `ora-tui` |

---

## Parámetros de Línea de Comandos (CLI Flags)

| Parámetro / Flag | Descripción | Valor por defecto |
| :--- | :--- | :--- |
| **`--opentelemetry`** *(alias: `--opentlemetry`, `--otel`)* | Activa el servidor HTTP embebido con métricas **Prometheus (`/metrics`)**, **OpenTelemetry OTLP JSON (`/otel/v1/metrics`)** y snapshot JSON (`/api/v1/state`) | `false` |
| **`--otel-port <puerto>`** *(alias: `--port`)* | Puerto TCP donde escucha el exportador OpenTelemetry/Prometheus (libre de colisión con puertos de Oracle) | `9161` |
| **`--otel-addr <ip:puerto>`** | Dirección IP y puerto específicos de escucha (ej. `0.0.0.0:9161` o `127.0.0.1:9161`) | `:9161` |
| **`--daemon`** | Ejecuta `ora-tui` en modo demonio sin interfaz gráfica (headless), ideal para `systemd`, contenedores o agentes de monitorización | `false` |
| **`--interval <duración>`** | Intervalo de refresco automático de los colectores de Oracle y sistema operativo (ej. `10s`, `15s`, `30s`) | `10s` |
| **`--mock`** | Fuerza el modo de simulación interactivo con datos de ejemplo completos (sin requerir Oracle instalado) | `false` (auto si no hay Oracle) |
| **`--topology <rac\|restart\|single>`** | Topología simulada cuando se ejecuta con `--mock` | `rac` |
| **`--preview`** | Imprime por salida estándar (`stdout`) una captura estática Unicode de la interfaz y termina | `false` |
| **`--version`** | Muestra la versión compilada de `ora-tui` y termina | `false` |

---

## Instalación y Reinstalación (`dnf`, `rpm` y `apt-get`)

### 1. Usando `dnf` (RHEL / Oracle Linux / Rocky / AlmaLinux 8 y 9)

#### Instalar por primera vez con `dnf`
* **Directamente desde GitHub:**
  ```bash
  sudo dnf install -y https://github.com/mario-ezquerro/oratui/raw/main/dist/ora-tui-1.6.0-1.x86_64.rpm
  ```
* **Desde un archivo `.rpm` descargado localmente (o copiado por `scp`):**
  ```bash
  sudo dnf install -y ./ora-tui-1.6.0-1.x86_64.rpm
  ```

#### Reinstalar / Actualizar a la última compilación con `dnf`
Para actualizar o reinstalar saltándose tanto la caché local de `dnf` como la caché CDN de 5 minutos de GitHub (`raw.githubusercontent.com`):
```bash
sudo dnf clean all
curl -fL -H "Cache-Control: no-cache" -o /tmp/ora-tui.rpm "https://github.com/mario-ezquerro/oratui/raw/main/dist/ora-tui-1.6.0-1.x86_64.rpm?t=$(date +%s)"
sudo dnf install -y /tmp/ora-tui.rpm || sudo dnf reinstall -y /tmp/ora-tui.rpm
```
*(O si ya tienes el archivo en local: `sudo dnf reinstall -y ./ora-tui-1.6.0-1.x86_64.rpm`)*

---

### 2. Usando `rpm` directamente (RHEL / Oracle Linux / CentOS / SUSE)

#### Instalar por primera vez con `rpm`
* **Directamente desde GitHub:**
  ```bash
  sudo rpm -ivh https://github.com/mario-ezquerro/oratui/raw/main/dist/ora-tui-1.6.0-1.x86_64.rpm
  ```
* **Desde un archivo local:**
  ```bash
  sudo rpm -ivh ./ora-tui-1.6.0-1.x86_64.rpm
  ```

#### Reinstalar / Forzar actualización con `rpm`
Para reinstalar o actualizar encima de cualquier versión previa forzando la sustitución del binario (`/usr/bin/ora-tui`) y evitando la caché CDN de GitHub:
* **Directamente desde GitHub (sin caché):**
  ```bash
  curl -fL -H "Cache-Control: no-cache" -o /tmp/ora-tui.rpm "https://github.com/mario-ezquerro/oratui/raw/main/dist/ora-tui-1.6.0-1.x86_64.rpm?t=$(date +%s)"
  sudo rpm -Uvh --replacepkgs --force /tmp/ora-tui.rpm
  ```
* **Desde un archivo local:**
  ```bash
  sudo rpm -Uvh --replacepkgs --force ./ora-tui-1.6.0-1.x86_64.rpm
  ```

---

### 3. Usando `apt-get` / `apt` (Debian / Ubuntu)

El repositorio incluye también el paquete Debian/Ubuntu precompilado en `dist/ora-tui_1.6.0-1_amd64.deb`.

#### Instalar por primera vez con `apt-get`
```bash
curl -fL -o /tmp/ora-tui.deb https://github.com/mario-ezquerro/oratui/raw/main/dist/ora-tui_1.6.0-1_amd64.deb
sudo apt-get install -y /tmp/ora-tui.deb
```
*(También puedes usar `sudo dpkg -i /tmp/ora-tui.deb`)*

#### Reinstalar / Actualizar a la última compilación con `apt-get`
Para volver a descargar el paquete `.deb` actualizado desde GitHub sin caché CDN y forzar su reinstalación con `apt-get`:
```bash
curl -fL -H "Cache-Control: no-cache" -o /tmp/ora-tui.deb "https://github.com/mario-ezquerro/oratui/raw/main/dist/ora-tui_1.6.0-1_amd64.deb?t=$(date +%s)"
sudo apt-get install --reinstall -y /tmp/ora-tui.deb
```
*(O bien con `dpkg`: `sudo dpkg -i --force-overwrite /tmp/ora-tui.deb`)*

---

### 4. Ejecutar `ora-tui` una vez instalado
```bash
sudo ora-tui
```

---

## Exportador OpenTelemetry & Prometheus (`--opentelemetry` / `--daemon`) + Grafana & Alertas

`ora-tui` incorpora un servidor de telemetría embebido (**OpenTelemetry OTLP JSON** + **Prometheus/OpenMetrics**) que expone **todos los datos recolectados** (Clusterware, Instancias, PDBs, Tablespaces con `AUTOEXTEND`, Sesiones, Bloqueos TX/TM, Objetos Inválidos, ASM Diskgroups, Listeners, Backups RMAN, Data Guard Lag, Errores `ORA-` del Alert Log, CPU/Load, RAM, HugePages, Inodos de Filesystems y Caminos SAN Multipath).

Por defecto utiliza el puerto **`9161`** (puerto estándar de monitorización Oracle en Prometheus, completamente libre de colisiones con puertos de Oracle como `1521-1529`, `5500` EM Express, `8080` APEX o `4903` OEM Agent), y puedes cambiarlo libremente con **`--otel-port`**.

### 1. Cómo lanzarlo por línea de comandos
* **Modo Híbrido (Interfaz TUI interactiva + Exportador OpenTelemetry en segundo plano):**
  ```bash
  sudo ora-tui --opentelemetry
  # O cambiando el puerto (por ejemplo al 9464):
  sudo ora-tui --opentelemetry --otel-port=9464
  ```
* **Modo Daemon / Headless (Sin interfaz gráfica, ideal para `systemd` o contenedores):**
  ```bash
  sudo ora-tui --opentelemetry --daemon --otel-port=9161 --interval=15s
  ```
  *(Nota: también acepta los alias `--opentlemetry` y `--otel`)*

### 2. Endpoints HTTP expuestos por `ora-tui`
Una vez iniciado, puedes consultar:
* **`http://<IP-SERVIDOR>:9161/metrics`** $\rightarrow$ Formato **Prometheus / OpenMetrics** (52 métricas listas para `prometheus.yml` o `otel-collector`).
* **`http://<IP-SERVIDOR>:9161/otel/v1/metrics`** $\rightarrow$ Formato **OpenTelemetry OTLP/HTTP JSON (`resourceMetrics`)**.
* **`http://<IP-SERVIDOR>:9161/api/v1/state`** $\rightarrow$ Snapshot completo del estado en **JSON**.
* **`http://<IP-SERVIDOR>:9161/healthz`** $\rightarrow$ Healthcheck ligero (`HTTP 200 OK`).
* **`http://<IP-SERVIDOR>:9161/`** $\rightarrow$ Página web de resumen con enlaces directos a los endpoints.

### 3. Cómo iniciarlo como Servicio Daemon (`systemd`)
El paquete `.rpm` y `.deb` instala automáticamente el servicio **`/usr/lib/systemd/system/ora-tui-otel.service`** (y una copia en `/usr/share/ora-tui/ora-tui-otel.service`). Para dejarlo activo de forma permanente al arrancar el servidor Linux:

```bash
# Recargar demonios, habilitar en el arranque e iniciar ahora mismo:
sudo systemctl daemon-reload
sudo systemctl enable --now ora-tui-otel

# Verificar estado y logs en vivo del daemon:
sudo systemctl status ora-tui-otel
sudo journalctl -u ora-tui-otel -f

# Comprobar que responde métricas Prometheus:
curl -s http://localhost:9161/metrics | head -n 40
```

Si deseas cambiar el puerto (por ejemplo a `9464`) o el intervalo de muestreo en el servicio `systemd`:
```bash
sudo systemctl edit ora-tui-otel
# Añade:
# [Service]
# Environment="ORA_TUI_PORT=9464"
# Environment="ORA_TUI_INTERVAL=10s"
sudo systemctl restart ora-tui-otel
```

### 4. Catálogo Completo de Métricas Expuestas (`/metrics`)

| Familia de Métricas Prometheus | Etiquetas (`Labels`) | Descripción |
| :--- | :--- | :--- |
| `oratui_info` | `host`, `cluster`, `topology`, `oracle_version`, `oracle_home`, `grid_home`, `version` | Metadatos del servidor Oracle, versión exacta y topología (`1`) |
| `oratui_last_scrape_timestamp_seconds` | `host`, `cluster` | Timestamp Unix del último muestreo completado |
| `oratui_cluster_node_up` | `host`, `cluster`, `node`, `role`, `vip`, `interconnect` | Estado de cada nodo del cluster (`1` = ONLINE, `0` = OFFLINE) |
| `oratui_cluster_daemon_up` | `host`, `cluster`, `node`, `daemon`, `raw_state` | Estado de los demonios `crs`, `cssd`, `evmd`, `asm` (`1` = ONLINE, `0` = OFFLINE) |
| `oratui_instance_up` | `host`, `cluster`, `instance`, `db_name`, `node`, `status`, `open_mode`, `role` | Estado de la instancia (`1` = OPEN, `0.5` = MOUNTED, `0` = DOWN) |
| `oratui_instance_fra_used_percent` | `host`, `cluster`, `instance`, `db_name`, `node` | Porcentaje de ocupación de la Fast Recovery Area (`FRA %`) |
| `oratui_pdb_up` / `oratui_pdb_size_gb` | `host`, `cluster`, `pdb`, `con_id`, `open_mode`, `restricted` | Estado (`1` = READ WRITE/ONLY, `0` = MOUNTED) y tamaño en GB de cada PDB |
| `oratui_service_up` | `host`, `cluster`, `service`, `pdb`, `preferred`, `available`, `active_inst` | Estado de cada servicio de base de datos/cluster (`1` = ONLINE) |
| `oratui_sessions_total` / `active_total` / `inactive_total` | `host`, `cluster` | Número de sesiones totales, activas e inactivas en `gv$session` |
| `oratui_sessions_blocking_locks_total` | `host`, `cluster` | Número de sesiones involucradas en bloqueos `TX`/`TM` |
| `oratui_session_pga_mb` | `host`, `cluster`, `inst_id`, `sid`, `serial`, `username`, `status`, `sql_id`, `event`, `blocking` | Consumo de memoria PGA (MB) y evento de espera por sesión |
| `oratui_tablespace_used_percent` | `host`, `cluster`, `tablespace`, `type`, `status`, `autoextend` | % real de ocupación del tablespace considerando `AUTOEXTEND MAXBYTES` |
| `oratui_tablespace_alloc_mb` / `used_mb` / `free_mb` / `max_mb` | `host`, `cluster`, `tablespace`, `type`, `status`, `autoextend` | Espacio asignado en disco, usado, libre real y máximo autoextensible en MB |
| `oratui_datafile_size_mb` / `oratui_datafile_max_mb` | `host`, `cluster`, `file_id`, `tablespace`, `file_name`, `autoextend`, `status` | Tamaño actual y máximo en MB de cada datafile/tempfile |
| `oratui_segment_size_mb` / `rows` / `stale_stats` | `host`, `cluster`, `owner`, `segment`, `type`, `tablespace`, `stats_status`, `fragmented` | Tamaño en MB, filas estimadas y estado de estadísticas (`1` = STALE) de los mayores segmentos |
| `oratui_invalid_objects_total` / `oratui_invalid_object_info` | `host`, `cluster`, `owner`, `object_name`, `object_type`, `last_ddl` | Total y detalle de cada objeto en estado `INVALID` (`dba_objects`) |
| `oratui_asm_diskgroup_used_percent` / `total_mb` / `free_mb` / `usable_mb` / `offline_disks` | `host`, `cluster`, `diskgroup`, `state`, `redundancy` | Capacidad, espacio usable, % de ocupación y discos `OFFLINE` de cada Diskgroup ASM |
| `oratui_listener_up` / `services_ready` / `service_status` | `host`, `cluster`, `listener`, `type`, `node`, `endpoint`, `ready_status` | Estado de cada Listener TNS/SCAN y estado `READY`/`UNKNOWN` de sus servicios |
| `oratui_rman_backup_status` / `oratui_rman_backup_size_gb` | `host`, `cluster`, `session_key`, `input_type`, `device_type`, `start_time`, `status` | Estado (`1` = COMPLETED, `0.5` = RUNNING/WARN, `0` = FAILED) y tamaño en GB de jobs RMAN |
| `oratui_restore_points_total` | `host`, `cluster` | Número de puntos de restauración Flashback activos |
| `oratui_dataguard_mrp_up` / `transport_lag_seconds` / `apply_lag_seconds` | `host`, `cluster`, `standby_db`, `role`, `standby_host`, `open_mode`, `switchover` | Estado del proceso `MRP0` y retraso de transporte/aplicación en segundos |
| `oratui_alert_log_size_mb` / `ora_errors_total` / `ora_error_count` | `host`, `cluster`, `error_code`, `instance`, `severity` | Tamaño del `alert.log` en MB, total de errores `ORA-` y contador por código exacto (`ORA-xxxxx`) |
| `oratui_os_metric_used_percent` / `oratui_os_metric_healthy` | `host`, `cluster`, `category`, `component`, `status`, `current_value` | Salud (`1` = OK, `0` = WARN/CRIT) y % de uso de CPU Load, RAM, Swap, HugePages/THP y Kernel AIO/FD |
| `oratui_filesystem_used_percent` / `inodes_used_percent` / `total_mb` / `used_mb` / `avail_mb` | `host`, `cluster`, `mountpoint`, `device`, `fstype`, `status` | Uso de espacio e inodos (`df -Pi`, prevención de `ORA-09925`) de todos los filesystems montados |
| `oratui_storage_path_healthy` | `host`, `cluster`, `name`, `type`, `size`, `wwid_model`, `paths_summary`, `status` | Salud de cada LUN SAN Multipath (`4/4 ACTIVE` = `1`, `DEGRADED/FAULTY` = `0`), puertos HBA FC y discos |

---

## Carpeta `POC/` Lista para Montar en un Servidor con Docker (`configure-poc.sh`)

En la carpeta **[`POC/`](file:///Users/mario/repositorios/oratui/POC)** del repositorio (y también instalada automáticamente en **`/usr/share/ora-tui/POC/`** al instalar el paquete `.rpm` o `.deb`) tienes todo preparado para copiarla a cualquier máquina con Docker y levantar **Prometheus + 15 Reglas de Alerta + Grafana con 26 gráficas autoprovisionadas**:

### 1. Copiar la carpeta `POC` al servidor con Docker
```bash
# Desde el servidor Oracle donde instalaste ora-tui:
scp -r /usr/share/ora-tui/POC usuario@servidor-docker:~/POC

# O clonando directamente el repositorio en el servidor Docker:
git clone https://github.com/mario-ezquerro/oratui.git
cd oratui/POC
```

### 2. Ejecutar el script interactivo `./configure-poc.sh` (o `./start-poc.sh`)
Cada vez que ejecutes el script, **te pedirá interactivamente el nombre o IP del host Oracle a monitorizar** (y el puerto, por defecto `9161`), generará [`POC/prometheus/prometheus.yml`](file:///Users/mario/repositorios/oratui/POC/prometheus/prometheus.yml), verificará la conexión HTTP contra `ora-tui` y levantará/recargará los contenedores de Prometheus y Grafana:

```bash
cd ~/POC
chmod +x configure-poc.sh start-poc.sh
./configure-poc.sh
```

Salida interactiva:
```text
==========================================================================
  CONFIGURACIÓN DE LA POC ORA-TUI (PROMETHEUS + GRAFANA)
==========================================================================

👉 Introduce el nombre o IP del host Oracle a monitorizar [servidor-oracle.local]: 192.168.1.50
👉 Introduce el puerto OpenTelemetry de ora-tui [9161]: 9161

>>> Generando configuración de Prometheus para monitorizar: 192.168.1.50:9161 ...
>>> Verificando conexión con http://192.168.1.50:9161/healthz ...
    [OK] Conexión exitosa con ora-tui en 192.168.1.50:9161.
>>> Levantando / recargando contenedores Docker (Prometheus + Grafana)...
```
*(Si más adelante deseas apuntar la POC a otro servidor Oracle distinto, basta con volver a ejecutar `./configure-poc.sh` e introducir el nuevo host).*

### 3. Acceder a Grafana y Prometheus
* **Grafana (`http://<IP_SERVIDOR_DOCKER>:3000`, usuario `admin` / `admin`)**: Abre directamente en portada el dashboard **`POC - Oracle Cluster, DB, ASM & Linux/SAN Full Observability (ora-tui)`** con 26 paneles.
* **Prometheus (`http://<IP_SERVIDOR_DOCKER>:9090/alerts`)**: Muestra en vivo el estado de las **15 reglas de alarma** incluidas en [`POC/prometheus/oracle_alerts.yml`](file:///Users/mario/repositorios/oratui/POC/prometheus/oracle_alerts.yml):
  0. `OraTuiExporterDown` (`up{job=~"oracle-oratui.*"} == 0`)
  1. `OracleInstanceDown` (`oratui_instance_up < 1`)
  2. `OracleClusterDaemonDown` (`oratui_cluster_node_up == 0` o `oratui_cluster_daemon_up == 0`)
  3. `OraclePDBNotOpen` (`oratui_pdb_up == 0`)
  4. `OracleTablespaceWarning` (`>= 85%`) y `OracleTablespaceCritical` (`>= 92%` real con `AUTOEXTEND`)
  5. `OracleFRASpaceCritical` (`oratui_instance_fra_used_percent >= 85`)
  6. `OracleBlockingSessionsDetected` (`oratui_sessions_blocking_locks_total > 0` durante `2m`)
  7. `OracleASMDiskgroupCritical` (`oratui_asm_diskgroup_used_percent >= 88` o `oratui_asm_diskgroup_offline_disks > 0`)
  8. `OracleListenerDown` (`oratui_listener_up == 0`)
  9. `OracleAlertLogORAError` (`oratui_alert_ora_errors_total > 0`)
  10. `OracleRMANBackupFailed` (`oratui_rman_backup_status == 0`)
  11. `OracleDataGuardSyncIssue` (`oratui_dataguard_mrp_up == 0` o `oratui_dataguard_apply_lag_seconds > 300`)
  12. `LinuxServerHealthDegraded` (`oratui_os_metric_healthy == 0` — Load, RAM, Swap, THP o Kernel AIO)
  13. `LinuxFilesystemOrInodesCritical` (`oratui_filesystem_used_percent >= 90` o `oratui_filesystem_inodes_used_percent >= 85` — previene `ORA-09925`)
  14. `LinuxSANMultipathPathDown` (`oratui_storage_path_healthy == 0` — alerta inmediata si cae un camino de fibra SAN/Multipath)

---

## Ejecutar en Modo Simulación / Mock (macOS / Linux sin Oracle instalado)

Si ejecutas `ora-tui` en un equipo de desarrollo sin Oracle instalado (o pasas `--mock`), se inicia automáticamente el simulador interactivo completo:

```bash
# Compilar y lanzar el TUI interactivo en modo Mock RAC:
make mock

# Probar el exportador OpenTelemetry/Prometheus en modo Mock daemon:
./bin/ora-tui --mock --opentelemetry --daemon --otel-port=9161
```

## Compilar Binario Estático y Generar los Paquetes `.rpm` y `.deb` desde el Código Fuente

```bash
make all && make packages
```

---

## Estructura del Proyecto

```text
ora-tui/
├── cmd/ora-tui/main.go            # Punto de entrada y flags CLI (--opentelemetry, --daemon, --otel-port, --mock)
├── internal/
│   ├── model/state.go             # Modelos de dominio (Cluster, Instancias, Sesiones, Tablespaces, RMAN, OS, SAN)
│   ├── discovery/                 # Auto-detección de ocr.loc, oratab, inventory.xml, /proc PMON y versión
│   ├── collector/                 # Colectores SQL*Plus, crsctl, olsnodes, srvctl, lsnrctl, OS Health y SAN
│   ├── telemetry/                 # Exportador OpenTelemetry (OTLP JSON) y Prometheus (/metrics)
│   ├── actions/                   # Flujos de parada/arranque, Node Drain, mantenimiento DBA, RMAN, ADRCI y SAN
│   ├── executor/runner.go         # Ejecutor de comandos con cambio de UID/GID y streaming en vivo
│   └── ui/                        # Interfaz tview/tcell con ratón SSH, ventana de resultados y 4 temas retro
├── POC/                           # Carpeta lista para copiar a un servidor Docker (Prometheus + Grafana POC)
│   ├── configure-poc.sh           # Script interactivo que pide el host Oracle a monitorizar y levanta Docker
│   ├── start-poc.sh               # Script de configuración interactiva y arranque de contenedores
│   ├── docker-compose.yml         # Stack Docker Compose con autoprovisionamiento de Grafana y Prometheus
│   ├── README.md                  # Guía detallada paso a paso de la POC
│   ├── prometheus/                # prometheus.yml y oracle_alerts.yml (15 reglas de alarma)
│   └── grafana/                   # Datasource automático y Dashboard completo de 26 paneles (Home Dashboard)
├── monitoring/                    # Plantillas independientes de Prometheus, Alertas y Dashboard de Grafana
├── packaging/                     # nfpm.yaml, ora-tui.spec, sudoers.ora-tui y ora-tui-otel.service (systemd)
├── dist/                          # Paquetes precompilados (.rpm y .deb) listos para instalar desde GitHub
├── Makefile                       # Automatización de compilación, tests y empaquetado (.rpm / .deb)
└── SPEC.md                        # Especificación técnica detallada de arquitectura y módulos
```

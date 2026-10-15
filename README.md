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
  sudo dnf install -y https://github.com/mario-ezquerro/oratui/raw/main/dist/ora-tui-1.5.0-1.x86_64.rpm
  ```
* **Desde un archivo `.rpm` descargado localmente (o copiado por `scp`):**
  ```bash
  sudo dnf install -y ./ora-tui-1.5.0-1.x86_64.rpm
  ```

#### Reinstalar / Actualizar a la última compilación con `dnf`
Para actualizar o reinstalar saltándose tanto la caché local de `dnf` como la caché CDN de 5 minutos de GitHub (`raw.githubusercontent.com`):
```bash
sudo dnf clean all
curl -fL -H "Cache-Control: no-cache" -o /tmp/ora-tui.rpm "https://github.com/mario-ezquerro/oratui/raw/main/dist/ora-tui-1.5.0-1.x86_64.rpm?t=$(date +%s)"
sudo dnf install -y /tmp/ora-tui.rpm || sudo dnf reinstall -y /tmp/ora-tui.rpm
```
*(O si ya tienes el archivo en local: `sudo dnf reinstall -y ./ora-tui-1.5.0-1.x86_64.rpm`)*

---

### 2. Usando `rpm` directamente (RHEL / Oracle Linux / CentOS / SUSE)

#### Instalar por primera vez con `rpm`
* **Directamente desde GitHub:**
  ```bash
  sudo rpm -ivh https://github.com/mario-ezquerro/oratui/raw/main/dist/ora-tui-1.5.0-1.x86_64.rpm
  ```
* **Desde un archivo local:**
  ```bash
  sudo rpm -ivh ./ora-tui-1.5.0-1.x86_64.rpm
  ```

#### Reinstalar / Forzar actualización con `rpm`
Para reinstalar o actualizar encima de cualquier versión previa forzando la sustitución del binario (`/usr/bin/ora-tui`) y evitando la caché CDN de GitHub:
* **Directamente desde GitHub (sin caché):**
  ```bash
  curl -fL -H "Cache-Control: no-cache" -o /tmp/ora-tui.rpm "https://github.com/mario-ezquerro/oratui/raw/main/dist/ora-tui-1.5.0-1.x86_64.rpm?t=$(date +%s)"
  sudo rpm -Uvh --replacepkgs --force /tmp/ora-tui.rpm
  ```
* **Desde un archivo local:**
  ```bash
  sudo rpm -Uvh --replacepkgs --force ./ora-tui-1.5.0-1.x86_64.rpm
  ```

---

### 3. Usando `apt-get` / `apt` (Debian / Ubuntu)

El repositorio incluye también el paquete Debian/Ubuntu precompilado en `dist/ora-tui_1.5.0-1_amd64.deb`.

#### Instalar por primera vez con `apt-get`
```bash
curl -fL -o /tmp/ora-tui.deb https://github.com/mario-ezquerro/oratui/raw/main/dist/ora-tui_1.5.0-1_amd64.deb
sudo apt-get install -y /tmp/ora-tui.deb
```
*(También puedes usar `sudo dpkg -i /tmp/ora-tui.deb`)*

#### Reinstalar / Actualizar a la última compilación con `apt-get`
Para volver a descargar el paquete `.deb` actualizado desde GitHub sin caché CDN y forzar su reinstalación con `apt-get`:
```bash
curl -fL -H "Cache-Control: no-cache" -o /tmp/ora-tui.deb "https://github.com/mario-ezquerro/oratui/raw/main/dist/ora-tui_1.5.0-1_amd64.deb?t=$(date +%s)"
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

### 2. Endpoints expuestos por `ora-tui`
Una vez iniciado, puedes consultar:
* **`http://<IP-SERVIDOR>:9161/metrics`** $\rightarrow$ Formato **Prometheus / OpenMetrics** (listo para `prometheus.yml` o `otel-collector`).
* **`http://<IP-SERVIDOR>:9161/otel/v1/metrics`** $\rightarrow$ Formato **OpenTelemetry OTLP/HTTP JSON (`resourceMetrics`)**.
* **`http://<IP-SERVIDOR>:9161/api/v1/state`** $\rightarrow$ Snapshot completo en **JSON**.
* **`http://<IP-SERVIDOR>:9161/`** $\rightarrow$ Página web de estado del exportador.

### 3. Cómo iniciarlo como Servicio Daemon (`systemd`)
El paquete `.rpm` y `.deb` instala automáticamente el servicio **`/usr/lib/systemd/system/ora-tui-otel.service`**. Para dejarlo activo de forma permanente al arrancar el servidor Linux:

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

### 4. Configuración de Prometheus (`prometheus.yml`) y Reglas de Alarma (`oracle_alerts.yml`)
Al instalar el paquete, encontrarás las plantillas listas para producción en **`/usr/share/ora-tui/monitoring/`** (y en la carpeta [`monitoring/`](file:///Users/mario/repositorios/oratui/monitoring) del repositorio):

* **[`monitoring/prometheus/prometheus.yml`](file:///Users/mario/repositorios/oratui/monitoring/prometheus/prometheus.yml)**:
  ```yaml
  global:
    scrape_interval: 15s
    evaluation_interval: 15s

  rule_files:
    - "oracle_alerts.yml"

  scrape_configs:
    - job_name: "oratui-oracle-exporter"
      scrape_interval: 15s
      metrics_path: "/metrics"
      static_configs:
        - targets:
            - "srv-dbgriar-des:9161"
  ```
* **[`monitoring/prometheus/oracle_alerts.yml`](file:///Users/mario/repositorios/oratui/monitoring/prometheus/oracle_alerts.yml)** — Incluye **14 reglas de alarma preconfiguradas**:
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

### 5. Dashboard de Grafana Listo para Importar (`oratui_dashboard.json`)
El archivo **[`monitoring/grafana/oratui_dashboard.json`](file:///Users/mario/repositorios/oratui/monitoring/grafana/oratui_dashboard.json)** (instalado también en `/usr/share/ora-tui/monitoring/grafana/oratui_dashboard.json`) contiene un **Dashboard completo de 11 paneles** listo para importar en Grafana (**Dashboards $\rightarrow$ New $\rightarrow$ Import $\rightarrow$ Upload JSON file**):
1. Estado en vivo de Instancias Oracle (`OPEN` / `MOUNTED` / `DOWN`).
2. Estado de Nodos y Demonios Clusterware (`CRS`, `CSSD`, `EVMD`, `ASM`).
3. Sesiones Activas vs Bloqueos TX/TM en curso.
4. Contador de Errores `ORA-*` en el Alert Log y Objetos `INVALID`.
5. Salud de todos los Caminos SAN Multipath (`4/4 ACTIVE` vs `DEGRADED`) y Puertos HBA Fibre Channel.
6. Barras de Ocupación Real de Tablespaces (`%` considerando `AUTOEXTEND MAXBYTES`).
7. Uso de ASM Diskgroups (`%`) y Fast Recovery Area (`FRA %`).
8. Salud del Servidor Linux (`CPU Load %`, `RAM Física %`, `Swap %`, `HugePages %`, `Kernel AIO %`).
9. Ocupación de Filesystems (`% Espacio`) y Uso de Inodos (`% Inodos` para prevenir `ORA-09925`).
10. Gráfica temporal de Sesiones, Bloqueos TX/TM y Retraso de Data Guard (`Apply Lag`).
11. Gráfica temporal de Errores `ORA-` por código, Estado de Listeners y Backups RMAN.

*(Si quieres levantar Prometheus + Grafana rápidamente con Docker, tienes listo [`monitoring/docker-compose.yml`](file:///Users/mario/repositorios/oratui/monitoring/docker-compose.yml) en `/usr/share/ora-tui/monitoring/docker-compose.yml`).*

---

### 6. Ejecutar en Modo Simulación / Mock (macOS / Linux sin Oracle instalado)
Si ejecutas `ora-tui` en un equipo de desarrollo sin Oracle instalado (o pasas `--mock`), se inicia automáticamente el simulador interactivo completo:

```bash
# Compilar y lanzar el TUI interactivo en modo Mock RAC:
make mock

# Probar el exportador OpenTelemetry/Prometheus en modo Mock daemon:
./bin/ora-tui --mock --opentelemetry --daemon --otel-port=9161
```

---

### 7. Compilar Binario Estático y Generar los Paquetes `.rpm` y `.deb` desde el Código Fuente
```bash
make all && make packages
```

---

## Estructura del Proyecto

```text
ora-tui/
├── cmd/ora-tui/main.go            # Punto de entrada, flags CLI (--opentelemetry, --daemon, --otel-port)
├── internal/
│   ├── model/state.go             # Modelos de dominio (Cluster, Instancias, Sesiones, Tablespaces, RMAN, SAN)
│   ├── discovery/                 # Auto-detección de ocr.loc, oratab, inventory.xml, /proc PMON y versión
│   ├── collector/                 # Colectores SQL*Plus, crsctl, olsnodes, srvctl, lsnrctl, OS Health y SAN
│   ├── telemetry/                 # Exportador OpenTelemetry (OTLP JSON) y Prometheus (/metrics)
│   ├── actions/                   # Flujos de parada/arranque, Node Drain, mantenimiento DBA, RMAN y ADRCI
│   ├── executor/runner.go         # Ejecutor de comandos con cambio de UID/GID y streaming en vivo
│   └── ui/                        # Interfaz tview/tcell con ratón SSH, ventana de resultados y temas retro
├── monitoring/                    # Configuración lista para Prometheus, Alertas y Dashboard de Grafana
│   ├── prometheus/prometheus.yml  # Scrape config en puerto 9161
│   ├── prometheus/oracle_alerts.yml # 14 reglas de alarma para Oracle RAC/BD y Servidor/SAN
│   ├── grafana/oratui_dashboard.json # Dashboard completo importable en Grafana
│   └── docker-compose.yml         # Stack rápido de Prometheus + Grafana
├── packaging/                     # nfpm.yaml, ora-tui.spec, sudoers.ora-tui y ora-tui-otel.service (systemd)
├── dist/                          # Paquetes precompilados (.rpm y .deb) listos para instalar desde GitHub
├── Makefile                       # Automatización de compilación, tests y empaquetado (.rpm / .deb)
└── SPEC.md                        # Especificación técnica detallada
```

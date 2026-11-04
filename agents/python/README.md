# 🐍 Guía de Despliegue Híbrido: Linux (Go) + IBM AIX con Oracle RAC (Python) + IBM i / AS400 (Python)

Esta carpeta (**[`agents/python/`](file:///Users/mario/repositorios/oratui/agents/python)**) contiene el agente oficial en **Python 3 (100% Librería Estándar, cero dependencias externas `pip` ni compiladores C)** diseñado para sistemas **IBM Power (AIX con Oracle Database RAC)** e **IBM i (AS/400 PASE)** donde no se pueden desplegar binarios estáticos de Go (`x86_64`) o existen políticas corporativas que exigen exclusivamente Python.

Tanto el binario en **Go** (para Linux) como el agente en **Python** (para AIX y AS/400) comparten **exactamente el mismo contrato en el puerto `:9161`**:
* **`GET /metrics`**: Métricas Prometheus (`oratui_info`, `oratui_cluster_node_up`, `oratui_instance_up`, `oratui_sessions_*`, `oratui_tablespace_*`, `oratui_asm_*`, `oratui_resource_limit_*`, `oratui_wait_class_*`, `oratui_top_sql_*`, `oratui_dataguard_dest_*`, `oratui_collector_*`, `oratui_os_metric_*`, `oratui_filesystem_*`, `oratui_storage_path_healthy`).
* **`GET /api/v1/state`**: Snapshot JSON completo para **Flutter Web Command Center (`:8088`)**.
* **`GET /api/v1/sre`**: Estado de SLOs, Error Budget y Alertas.
* **`GET /api/v1/report`**: Informe ejecutivo HTML/PDF imprimible.
* **`POST /api/v1/action`**: Ejecución remota autenticada (`BasicAuth` / `X-OraTui-Token`).
* **`GET /healthz`**: Healthcheck (`HTTP 200 OK`).

Gracias a esta arquitectura, **Prometheus (`:9090`), Grafana (`:3000`) y Flutter Web (`:8088`) ven y operan de forma idéntica servidores Linux, nodos IBM AIX con Oracle RAC y sistemas IBM i (AS/400)**.

---

## 📋 Resumen Comparativo de los 3 Escenarios

| Sistema Operativo | Motor del Agente | Uso Interactivo (Consola TUI) | Gestor de Daemon Nativo | Persistencia en el Arranque (Boot) |
| :--- | :--- | :--- | :--- | :--- |
| **1. Linux (`RHEL / OL / Debian`)** | Binario estático **Go** (`/usr/bin/ora-tui`) | `sudo ora-tui` | **`systemd`** (`ora-tui-otel.service`) | `systemctl enable ora-tui-otel` |
| **2. IBM AIX (Oracle RAC + SAN)** | **Python 3 Stdlib** (`/opt/ora-tui/ora_tui_agent.py`) | `ora-tui` (interfaz `curses`) | **AIX `SRC`** (`startsrc -s oratui` / `lssrc -s oratui`) | `/etc/inittab` (`mkitab`) |
| **3. IBM i / AS400 (`PASE`)** | **Python 3 Stdlib** (`/QOpenSys/opt/ora-tui/ora_tui_agent.py`) | `/QOpenSys/usr/bin/ora-tui` | **Trabajo Batch `SBMJOB`** (`JOB(ORATUI)` en `QUSRNOMAX`) | Entrada Autostart **`ADDAJE`** en `QSYSWRK` |

---

## Caso 1: Servidores Linux (`RHEL / Oracle Linux / Rocky / Debian / Ubuntu`) — Con Go

En Linux se utiliza el binario compilado en Go (`CGO_ENABLED=0`) empaquetado en `.rpm` o `.deb`.

### 1.1 Instalación del paquete
```bash
# En RHEL / Oracle Linux / Rocky:
curl -fL -H "Cache-Control: no-cache" -o /tmp/ora-tui.rpm "https://github.com/mario-ezquerro/oratui/raw/main/dist/ora-tui-1.10.0-1.x86_64.rpm?t=$(date +%s)"
sudo rpm -Uvh --replacepkgs --force /tmp/ora-tui.rpm

# En Debian / Ubuntu:
curl -fL -H "Cache-Control: no-cache" -o /tmp/ora-tui.deb "https://github.com/mario-ezquerro/oratui/raw/main/dist/ora-tui_1.10.0-1_amd64.deb?t=$(date +%s)"
sudo apt-get install --reinstall -y /tmp/ora-tui.deb
```

### 1.2 Cómo usar la aplicación interactiva TUI en Linux
Conéctate por SSH al servidor Linux y ejecuta:
```bash
sudo ora-tui
```
* Navega entre los **10 menús (`1`..`9` y `0`)** con las flechas `↑`/`↓` o pulsando el número directamente (`0` abre **Diagnóstico Avanzado & Advisor** con los 3 paquetes de `harry-performance-scraper`).
* Pulsa `Enter` sobre cualquier fila para abrir la ventana modal de acciones DBA/SAN, `c` para cambiar el tema retro MS-DOS y `q` para salir.

### 1.3 Cómo montarlo como Daemon en Linux (`systemd`)
El paquete `.rpm`/`.deb` ya instala la unidad **`/usr/lib/systemd/system/ora-tui-otel.service`** con límites de memoria (`GOMEMLIMIT=48MiB`, `MemoryHigh=48M`, `MemoryMax=96M`, `CPUQuota=15%`):
```bash
# Habilitar en el arranque e iniciar el daemon en el puerto 9161:
sudo systemctl daemon-reload
sudo systemctl enable --now ora-tui-otel

# Comprobar estado y logs:
sudo systemctl status ora-tui-otel
sudo journalctl -u ora-tui-otel -f

# Verificar que exporta métricas Prometheus:
curl -s http://localhost:9161/metrics | head -n 30
```

---

## Caso 2: IBM AIX con Oracle Database RAC — Con Python (`ora_tui_agent.py`)

En **IBM AIX (7.1 / 7.2 / 7.3 sobre Power8 / Power9 / Power10)** donde corre **Oracle Database RAC (Real Application Clusters)** y Grid Infrastructure (`+ASM`), el agente [`ora_tui_agent.py`](file:///Users/mario/repositorios/oratui/agents/python/ora_tui_agent.py) funciona con **Python 3 estándar** (incluido en AIX Toolbox en `/usr/bin/python3` o `/opt/freeware/bin/python3`) sin instalar librerías adicionales (`cx_Oracle` / `oracledb` no son necesarias).

### ¿Qué descubre y recolecta en vivo en AIX + Oracle RAC?
1. **Autodescubrimiento RAC y Grid Infrastructure**:
   * Lee `/etc/oracle/ocr.loc` (`local_only=false` $\rightarrow$ `ORACLE RAC`), `/etc/oratab` y los procesos activos `ora_pmon_<SID>` y `asm_pmon_+ASM*` en `ps -ef`.
   * Descubre automáticamente `$ORACLE_HOME`, `$GRID_HOME` y el usuario propietario (`oracle` / `grid`).
   * Ejecuta `$GRID_HOME/bin/olsnodes -n -s -t` para listar todos los nodos del cluster RAC y su estado (`ACTIVE` / `INACTIVE`).
2. **Batch Único `sqlplus -s / as sysdba` Multi-Instancia (`gv$*`)**:
   * Ejecuta en **una sola conexión por ciclo** (usando `su - oracle -c "sqlplus -s / as sysdba"`) todas las consultas globales del RAC:
     * Instancias del RAC (`gv$instance`) y PDBs (`v$pdbs`).
     * Sesiones activas, inactivas y bloqueos `TX`/`TM` cross-instance (`gv$session`).
     * Tablespaces con `AUTOEXTEND` (`dba_tablespace_usage_metrics` / `dba_data_files`).
     * Diskgroups ASM (`v$asm_diskgroup`: `+DATA`, `+FRA`, `+OCR_VOTE` y discos `OFFLINE`).
     * **Pack 1 (`harry-performance-scraper`)**: Límites y *High-Water Mark* en `gv$resource_limit` (`processes`, `sessions`, `dml_locks`, `parallel_max_servers`) y clases de espera / **AAS** en `gv$system_wait_class`.
     * **Pack 2 (`harry-performance-scraper`)**: Top SQL Consumers en `gv$sql` con cálculo de percentiles **P95/P99**, espera dominante y visor remoto `DBMS_XPLAN.DISPLAY_CURSOR`.
     * **Pack 3 (`harry-performance-scraper`)**: Destinos Data Guard y brechas de secuencia (`SeqGap`) en `gv$archive_dest_status` + salud del recolector (`CollectorRuntimeHealth`).
3. **Métricas Nativas de IBM AIX y Almacenamiento SAN MPIO**:
   * **CPU / LPAR**: `uptime` y `lparstat` (Logical CPUs y Entitled Capacity).
   * **Memoria RAM y Paging Space**: `svmon -G` (páginas de 4 KB convertidas a GB) y `lsps -s` (% de uso de Paging Space).
   * **Filesystems JFS2**: `df -P -m` (`/u01`, `/`, `/var`, `/tmp`, `/opt`).
   * **SAN Multipath MPIO y HBA Fibre Channel**: Ejecuta `lspath -F "name:parent:connection:status"` sobre cada `hdisk*` para detectar al instante caminos `Enabled` vs `Failed`/`Missing` (`DEGRADED`) y `lsdev -Cc adapter` para las tarjetas Fibre Channel (`fcs0`, `fcs1`, ...).

### 2.1 Instalación Rápida en cada Nodo del Cluster AIX (Interfaz + Daemon)
Copia la carpeta `agents/python/` a cada nodo AIX del RAC (por ejemplo a `/tmp/oratui-python`) y ejecuta como `root`:
```bash
cd /tmp/oratui-python
chmod +x install-aix.sh ora_tui_agent.py
./install-aix.sh --port=9161 --interval=5
```
El script [`install-aix.sh`](file:///Users/mario/repositorios/oratui/agents/python/install-aix.sh) realiza automáticamente:
1. Instala el agente en **`/opt/ora-tui/ora_tui_agent.py`**.
2. Crea el comando interactivo **`/usr/bin/ora-tui`**.
3. Registra y arranca el daemon nativo **`oratui`** en el **System Resource Controller (`SRC`) de AIX** y lo añade a `/etc/inittab`.

### 2.2 Cómo usar la aplicación interactiva TUI en IBM AIX
Desde cualquier sesión SSH en el nodo AIX, ejecuta:
```bash
ora-tui
# O directamente:
python3 /opt/ora-tui/ora_tui_agent.py
```
* Se abrirá en tu terminal la interfaz interactiva **Norton Commander (`curses`)** mostrando en tiempo real los nodos del RAC, las instancias `gv$instance`, tablespaces, diskgroups `+ASM`, los 3 paquetes de rendimiento (`gv$resource_limit` HWM, clases de espera AAS, Top SQL P95/P99, Data Guard Gaps) y el estado de los caminos SAN MPIO (`hdisk*` / `fcs*`).
* **Teclas en la TUI de AIX**:
  * **`1` .. `5` o Flechas `↑`/`↓`**: Cambiar entre las vistas (*1. Cluster RAC & DB*, *2. Límites HWM & Wait Classes*, *3. Top SQL P95/P99*, *4. Data Guard & Recolector*, *5. AIX LPAR, JFS2 & SAN MPIO*).
  * **`r`**: Forzar refresco inmediato de SQL*Plus y MPIO.
  * **`q`**: Salir de la consola TUI (el daemon en segundo plano sigue corriendo sin interrupción).
* También puedes imprimir una vista previa estática sin abrir `curses`:
  ```bash
  ora-tui --preview
  ```

### 2.3 Cómo montar y operar el Daemon en IBM AIX paso a paso (`SRC` + `/etc/inittab`)
A diferencia de Linux (que usa `systemd`), **IBM AIX gestiona sus demonios del sistema mediante el System Resource Controller (`SRC`)**. Si deseas montarlo o administrarlo manualmente paso a paso:

#### Paso 1: Crear el script wrapper del daemon (`/opt/ora-tui/run-src-daemon.ksh`)
```ksh
cat << 'EOF' > /opt/ora-tui/run-src-daemon.ksh
#!/bin/ksh
export PATH=/usr/bin:/etc:/usr/sbin:/usr/ucb:/sbin:/opt/freeware/bin
export LANG=C
exec /usr/bin/python3 /opt/ora-tui/ora_tui_agent.py --opentelemetry --daemon --otel-port=9161 --interval=5 >> /var/log/ora-tui-otel.log 2>&1
EOF
chmod 755 /opt/ora-tui/run-src-daemon.ksh
```

#### Paso 2: Registrar el subsistema `oratui` en AIX SRC con `mkssys`
```ksh
# -s oratui : Nombre del servicio en AIX
# -u 0      : Ejecutar como root (para poder consultar lspath MPIO y hacer su - oracle)
# -S -n 15 -f 9 : Parada limpia con SIGTERM (15) y forzada con SIGKILL (9)
# -R        : Respawn automático si el proceso se detiene inesperadamente
mkssys -s oratui -p /opt/ora-tui/run-src-daemon.ksh -u 0 -S -n 15 -f 9 -R -G tcpip
```

#### Paso 3: Configurar el arranque automático al iniciar el LPAR AIX (`/etc/inittab`)
```ksh
mkitab "oratui:2:once:/usr/bin/startsrc -s oratui > /dev/console 2>&1"
```

#### Paso 4: Comandos de operación diaria del Daemon en AIX
```ksh
# Arrancar el daemon:
startsrc -s oratui

# Consultar estado y PID del daemon:
lssrc -s oratui
# Salida esperada:
# Subsystem         Group            PID          Status
#  oratui           tcpip            14024912     active

# Parar el daemon:
stopsrc -s oratui

# Ver el log en vivo:
tail -f /var/log/ora-tui-otel.log

# Comprobar que expone métricas Prometheus y estado JSON:
curl -s http://localhost:9161/healthz
curl -s http://localhost:9161/metrics | head -n 30
```

---

## Caso 3: IBM i (AS/400 — Entorno PASE) — Con Python (`ora_tui_agent.py`)

En sistemas **IBM i (AS/400 — V7R3, V7R4, V7R5)**, Python 3 se ejecuta de forma nativa dentro del entorno **PASE (`/QOpenSys/pkgs/bin/python3`)**. El agente [`ora_tui_agent.py`](file:///Users/mario/repositorios/oratui/agents/python/ora_tui_agent.py) detecta automáticamente `/QOpenSys` (`os.uname().sysname == 'OS400'`) y adapta sus colectores al subsistema IBM i / DB2 for i.

### ¿Qué descubre y recolecta en vivo en IBM i (AS/400)?
1. **Estadísticas del Sistema y LPAR (`QSYS2.SYSTEM_STATUS_INFO`)**:
   * Consulta vía `qsh -c "db2 ..."` el porcentaje de CPU usada, capacidad de almacenamiento del **System ASP (`SYSTEM_ASP_USED`)**, número de trabajos activos en el sistema (`ACTIVE_JOBS_IN_SYSTEM`) y estado de partición.
2. **Sistemas de Ficheros IFS (`/QOpenSys/usr/bin/df -P -k`)**:
   * Monitoriza la ocupación de `/`, `/QOpenSys`, `/home`, `/tmp` y `/var`.
3. **Subsistema DB2 for i (`QSYSWRK` / `QUSRWRK`) y Almacenamiento Multipath SAN (`ASP1_MPATH`)**:
   * Expone todas las métricas bajo el formato estándar `oratui_*` en el puerto `:9161` para integrarse en los mismos paneles de Grafana y Flutter Web.

### 3.1 Instalación Rápida en IBM i (PASE)
Abre una sesión SSH o `QP2TERM` en el AS/400, copia la carpeta `agents/python/` a `/tmp/oratui-python` y ejecuta:
```sh
cd /tmp/oratui-python
chmod +x install-ibmi-as400.sh ora_tui_agent.py
./install-ibmi-as400.sh --port=9161 --interval=10
```
El script [`install-ibmi-as400.sh`](file:///Users/mario/repositorios/oratui/agents/python/install-ibmi-as400.sh) instala `/QOpenSys/opt/ora-tui/ora_tui_agent.py`, crea el comando `/QOpenSys/usr/bin/ora-tui` y somete el trabajo batch **`ORATUI`** mediante `SBMJOB`.

### 3.2 Cómo usar la aplicación interactiva TUI en IBM i (AS/400)
Conéctate por SSH al entorno PASE del AS/400 (con un cliente VT100/xterm como PuTTY, OpenSSH o iTerm2) y ejecuta:
```sh
/QOpenSys/usr/bin/ora-tui
# O para ver una captura estática en texto:
/QOpenSys/usr/bin/ora-tui --preview
```

### 3.3 Cómo montar y operar el Daemon en IBM i (AS/400) paso a paso (`SBMJOB` + `ADDAJE`)
En IBM i (AS/400), los servicios de larga duración se ejecutan como **trabajos batch multi-hilo (`ALWMLTTHD(*YES)`)** dentro de un subsistema sin límite de tiempo como **`QSYS/QUSRNOMAX`** o **`QSYS/QSYSWRK`**.

#### Paso 1: Someter (arrancar) el Daemon como Trabajo Batch (`SBMJOB`)
Desde SSH (PASE):
```sh
/QOpenSys/usr/bin/system "SBMJOB CMD(QSH CMD('/QOpenSys/opt/ora-tui/start-daemon-pase.sh')) JOB(ORATUI) JOBD(QGPL/QDFTJOBD) JOBQ(QSYS/QUSRNOMAX) ALWMLTTHD(*YES)"
```
O desde una línea de comandos CL tradicional (`5250`):
```cl
SBMJOB CMD(QSH CMD('/QOpenSys/opt/ora-tui/start-daemon-pase.sh'))
       JOB(ORATUI)
       JOBD(QGPL/QDFTJOBD)
       JOBQ(QSYS/QUSRNOMAX)
       ALWMLTTHD(*YES)
```

#### Paso 2: Configurar el Arranque Automático tras cada IPL (Reboot del AS/400)
Para que el trabajo `ORATUI` arranque solo cada vez que se inicie el subsistema `QSYSWRK` tras un IPL:
1. Crea una descripción de trabajo dedicada (`JOBD`) que invoque el daemon:
   ```cl
   CRTJOBD JOBD(QGPL/ORATUIJD)
           JOBQ(QSYS/QUSRNOMAX)
           TEXT('ORA-TUI OpenTelemetry & SRE Daemon')
           USER(QSECOFR)
           RQSDTA('QSH CMD(''/QOpenSys/opt/ora-tui/start-daemon-pase.sh'')')
           ALWMLTTHD(*YES)
   ```
2. Añade la entrada de trabajo de arranque automático (**`ADDAJE` — Add Autostart Job Entry**) al subsistema `QSYSWRK`:
   ```cl
   ADDAJE SBSD(QSYS/QSYSWRK) JOB(ORATUI) JOBD(QGPL/ORATUIJD)
   ```
*(Alternativamente, puedes añadir el comando `SBMJOB` al programa de arranque del sistema definido en el valor del sistema `QSTRUPPGM`).*

#### Paso 3: Comandos de operación diaria del Daemon en IBM i (AS/400)
```sh
# 1. Verificar que el trabajo ORATUI está activo en el subsistema:
/QOpenSys/usr/bin/system "WRKACTJOB JOB(ORATUI)"

# 2. Detener el daemon de forma ordenada (*CNTRLD) o inmediata (*IMMED):
/QOpenSys/usr/bin/system "ENDJOB JOB(ORATUI) OPTION(*CNTRLD) DELAY(10)"

# 3. Ver el log del daemon en el IFS:
tail -f /var/log/ora-tui-otel.log

# 4. Comprobar los endpoints HTTP en el puerto 9161:
curl -s http://localhost:9161/healthz
curl -s http://localhost:9161/metrics | head -n 30
```

---

## 🌐 Cómo dar de alta los servidores AIX y AS/400 en Prometheus, Grafana y Flutter Web

Una vez que el daemon Python está corriendo en el puerto `:9161` de tu servidor **IBM AIX** o **IBM i (AS/400)**, **se añade exactamente igual que un servidor Linux**:

### Opción A: Con el asistente interactivo `./configure-poc.sh`
En el servidor central donde corre Docker (`POC/`):
```bash
cd POC
./configure-poc.sh
# Elige la opción [1] "Añadir un nuevo servidor Oracle a la monitorización"
# Introduce el nombre o IP del nodo AIX o AS/400 y el puerto 9161.
```
El script lo añadirá automáticamente a `prometheus/targets.yml` y `flutter-web/build/web/servers.json` sin reiniciar nada.

### Opción B: Con Ansible (`POC/ansible/inventory.ini`)
En [`POC/ansible/inventory.ini`](file:///Users/mario/repositorios/oratui/POC/ansible/inventory.ini) puedes mezclar nodos Linux (Go), nodos AIX con Oracle RAC (Python) y nodos IBM i / AS400 (Python) dentro del grupo `[oracle_servers]`:
```ini
[oracle_servers]
# Nodos Linux (instalan el binario Go .rpm/.deb + systemd):
ora-linux-rac01.local  ansible_host=192.168.10.11  env=PROD  cluster_label=rac-linux  otel_port=9161

# Nodos IBM AIX con Oracle RAC (instalan el agente Python + daemon AIX SRC):
aix-rac01.local        ansible_host=192.168.10.21  env=PROD  cluster_label=rac-aix    otel_port=9161  agent_runtime=python
aix-rac02.local        ansible_host=192.168.10.22  env=PROD  cluster_label=rac-aix    otel_port=9161  agent_runtime=python

# Sistemas IBM i / AS400 en PASE (instalan el agente Python + daemon SBMJOB):
ibmi-as400-01.local    ansible_host=192.168.10.31  env=PROD  cluster_label=as400-db2  otel_port=9161  agent_runtime=python  ansible_python_interpreter=/QOpenSys/pkgs/bin/python3
```
Al ejecutar `ansible-playbook -i inventory.ini site.yml`, Ansible detectará automáticamente si el servidor es **Linux (`RedHat`/`Debian`)**, **IBM AIX (`AIX`)** o **IBM i (`OS400`)** e instalará el binario Go o el agente Python con su daemon nativo correspondiente.

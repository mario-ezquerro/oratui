# 🚀 POC de Observabilidad Completa para Oracle & Linux (`ora-tui` + Prometheus + Grafana)

Esta carpeta **`POC/`** está lista para copiarse en **cualquier servidor con Docker**. Incluye el script interactivo de configuración (**`./configure-poc.sh`** / **`./start-poc.sh`**) que **te preguntará cada vez que lo ejecutes el nombre o IP del servidor Oracle que quieres monitorizar**, generará automáticamente el fichero `prometheus/prometheus.yml`, comprobará la conexión y levantará:

1. **Prometheus (`:9090`)**: Configurado para leer cada **10 segundos** todas las métricas expuestas por `ora-tui --opentelemetry --daemon` en el host que hayas indicado y evaluar **15 reglas de alerta** críticas (Instancia caída, Tablespaces >85%/92%, FRA llena, ASM Diskgroups o discos Offline, Bloqueos `TX/TM`, Lag de Data Guard / `MRP0`, errores `ORA-` en el Alert Log, Inodos llenos `ORA-09925` y fallos de caminos SAN Multipath).
2. **Grafana (`:3000`)**: Con **autoprovisionamiento 100% automático** (sin tener que configurar nada a mano). Al entrar en Grafana tendrás como **página de inicio por defecto** el dashboard **"POC - Oracle Cluster, DB, ASM & Linux/SAN Full Observability (ora-tui)"** con **26 paneles** cubriendo todas las gráficas posibles del servidor.

---

## 🛠️ Paso 1: En el Servidor Oracle a Monitorizar (ej. `srv-dbgriar-des`)

Asegúrate de tener arrancado el colector `ora-tui` en modo demonio OpenTelemetry/Prometheus (puerto `9161`):

```bash
# Opción A: Activarlo como servicio systemd permanente (recomendado)
sudo systemctl daemon-reload
sudo systemctl enable --now ora-tui-otel
sudo systemctl status ora-tui-otel

# Opción B: Lanzarlo directamente en segundo plano
sudo nohup ora-tui --opentelemetry --daemon --otel-port=9161 > /tmp/ora-tui-otel.log 2>&1 &
```

Si el servidor Oracle tiene `firewalld` activo, abre el puerto `9161/tcp` para que el servidor Docker pueda leer las métricas:

```bash
sudo firewall-cmd --add-port=9161/tcp --permanent
sudo firewall-cmd --reload
```

---

## 🐳 Paso 2: En el Servidor con Docker (Copiar carpeta `POC` y Ejecutar el Script)

1. **Copia la carpeta `POC/`** al servidor donde tengas Docker instalado:
   * Si ya instalaste el paquete `.rpm`/`.deb` de `ora-tui`, la carpeta está disponible en `/usr/share/ora-tui/POC`:
     ```bash
     scp -r /usr/share/ora-tui/POC usuario@servidor-docker:~/POC
     ```
   * O bien clonando el repositorio directamente en el servidor Docker:
     ```bash
     git clone https://github.com/mario-ezquerro/oratui.git
     cd oratui/POC
     ```

2. **Ejecuta el script de configuración (`./configure-poc.sh` o `./start-poc.sh`)**:
   Cada vez que lo ejecutes, te pedirá por pantalla el **nombre o IP del host Oracle a monitorizar** (y el puerto, por defecto `9161`):

   ```bash
   cd POC
   chmod +x configure-poc.sh start-poc.sh
   ./configure-poc.sh
   ```

   Ejemplo de ejecución interactiva:
   ```text
   ==========================================================================
     CONFIGURACIÓN DE LA POC ORA-TUI (PROMETHEUS + GRAFANA)
   ==========================================================================

   👉 Introduce el nombre o IP del host Oracle a monitorizar [srv-dbgriar-des]: srv-dbgriar-des
   👉 Introduce el puerto OpenTelemetry de ora-tui [9161]: 9161

   >>> Generando configuración de Prometheus para monitorizar: srv-dbgriar-des:9161 ...
   >>> Verificando conexión con http://srv-dbgriar-des:9161/healthz ...
       [OK] Conexión exitosa con ora-tui en srv-dbgriar-des:9161.
   >>> Levantando / recargando contenedores Docker (Prometheus + Grafana)...
   ```

   *(Si más adelante quieres apuntar la POC a otro servidor Oracle distinto, simplemente vuelve a ejecutar `./configure-poc.sh`, escribe el nuevo nombre o IP y recargará Prometheus automáticamente).*

---

## 📊 Paso 3: Acceder a Grafana y Prometheus

Una vez finalizado el script:

- **Grafana (Dashboard Completo cargado como página de inicio)**:
  - **URL**: `http://<IP_SERVIDOR_DOCKER>:3000`
  - **Usuario / Contraseña**: `admin` / `admin` (también permite visualización directa sin login).
- **Prometheus (Estado del Target y Reglas de Alerta)**:
  - **Targets**: `http://<IP_SERVIDOR_DOCKER>:9090/targets`
  - **Alertas**: `http://<IP_SERVIDOR_DOCKER>:9090/alerts`

---

## 📈 ¿Qué gráficas incluye el Dashboard de la POC (`oratui_full_poc_dashboard.json`)?

Está organizado en **4 bloques con 26 paneles** que representan el 100% de las métricas recolectadas por `ora-tui`:

1. **Resumen Ejecutivo, Clusterware, Instancias, FRA, PDBs, Servicios & Listeners**:
   - Servidor, topología (`ORACLE RAC`, `ORACLE RESTART`, `SINGLE INSTANCE`) y versión exacta de Oracle.
   - Estado de cada Instancia Oracle (`OPEN`, `MOUNTED`, `DOWN`).
   - Gauge de ocupación de la **Fast Recovery Area (`FRA %`)**.
   - Contador en vivo de **Sesiones Activas**, **Bloqueos `TX/TM`**, **Errores `ORA-`** y **Objetos Inválidos**.
   - Estado de **Listeners Oracle Net**, **Nodos del Cluster**, **Demonios (`crs`, `cssd`, `evmd`, `asm`)**, **PDBs (`READ WRITE` / `MOUNTED`)**, **Tamaño de cada PDB (GB)** y **Servicios Activos**.
2. **Almacenamiento Oracle (Tablespaces con `AUTOEXTEND`, Datafiles, ASM, Top Segmentos & Inválidos)**:
   - Barras de `% de Ocupación Real por Tablespace` (teniendo en cuenta `AUTOEXTEND MAXBYTES`).
   - Gráfica temporal de evolución en MB (**Usado vs Asignado en Disco vs Libre Real vs Max Autoextend**).
   - Barras LCD de `% de Uso de Diskgroups ASM` (`+DATA`, `+FRA`, `+OCR`) y `% FRA`.
   - Ranking de **Datafiles por tamaño actual en disco (MB)**.
   - **Top Tablas y Segmentos por tamaño (MB)** indicando si sus estadísticas están `FRESH` o `STALE`.
   - Detalle de **Objetos Inválidos (`dba_objects`)** por propietario, nombre y tipo.
3. **Rendimiento de Sesiones, Bloqueos TX/TM, Memoria PGA, Backups RMAN, Data Guard & Alert Log**:
   - Gráfica temporal de **Sesiones Totales, Activas, Inactivas y Bloqueos TX/TM**.
   - Ranking de **Consumo de Memoria PGA por Sesión (MB)** mostrando `SID,SERIAL#`, usuario, evento de espera y sesión bloqueante.
   - Gráfica de barras del **Alert Log** desglosando cada código de error `ORA-xxxxx` detectado y su severidad.
   - Estado del **Último Backup RMAN (`COMPLETED` / `RUNNING` / `FAILED`)**, **Tamaño de Backups en GB** y **Puntos de Restauración Flashback**.
   - Gráfica temporal de **Data Guard Standby** (`Apply Lag`, `Transport Lag` en segundos y estado del proceso `MRP0`).
4. **Salud del Servidor Linux, Memoria, Kernel, Filesystems/Inodos & SAN Multipath**:
   - Barras y gráfica temporal de **CPU Load %, Memoria RAM Física %, Swap %, HugePages % y Kernel Async I/O (`aio-nr`) %**.
   - Barras LCD de **% Espacio Usado vs % Inodos (`df -PTm` / `df -Pi`)** en cada punto de montaje (`/u01`, `/`, `/var`, `/tmp`) para prevenir bloqueos `ORA-09925`.
   - Semáforo de salud de **Caminos SAN Multipath (`4/4 ACTIVE` vs `DEGRADED / FAULTY`)**, **Tarjetas FC HBA**, **Discos de Bloque** y **Chequeos de Kernel**.

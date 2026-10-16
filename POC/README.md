# 🚀 POC Multi-Servidor de Observabilidad y Operación Activa Oracle & Linux (`ora-tui` + Flutter Web Command & SRE Center + Prometheus + Grafana + Ansible + Jenkins)

Esta carpeta **`POC/`** proporciona una arquitectura integral de **Observabilidad, Ingeniería de Fiabilidad (SRE/SLO) y Operación Remota Activa** para **1 o 100 servidores Oracle**:

1. **⚡ Contenedor `ORA-TUI Flutter Web Command & SRE Center` (`:8088`, en [`POC/flutter-web/`](file:///Users/mario/repositorios/oratui/POC/flutter-web)) — ¡Sustituto Interactivo de Grafana con Operación Directa!**:
   Mientras que Grafana únicamente muestra gráficas en modo lectura, la aplicación **Flutter Web** empaquetada en Docker (`oratui-flutter-web`, puerto `:8088`):
   * **Muestra todas las gráficas temporales y medidores en tiempo real** (Sesiones vs. Bloqueos TX/TM, CPU vs. RAM, Tablespaces vs. FRA vs. ASM, Errores `ORA-` vs. Caminos SAN Multipath caídos) con estética conmutable *MS-DOS Norton Commander* o *Dark Mode*.
   * **Permite ejecutar acciones correctivas y de mantenimiento directamente sobre los servidores Oracle** (`POST /api/v1/action`) mostrando la salida real de consola (`SQL*Plus`, `RMAN`, `lsnrctl`, `ADRCI`, `multipath`, `sysctl`) en un modal interactivo:
     * **Matar sesiones bloqueantes** (`ALTER SYSTEM KILL SESSION 'sid,serial,@inst' IMMEDIATE`).
     * **Ampliar tablespaces** (`ADD DATAFILE SIZE 32G AUTOEXTEND ON`) y consolidar espacio (`COALESCE`).
     * **Recompilar objetos inválidos** (`UTL_RECOMP.RECOMP_PARALLEL(4)`) y actualizar **estadísticas CBO** (`DBMS_STATS.GATHER_TABLE_STATS`).
     * **Recargar Listeners** (`lsnrctl reload`), forzar `SWITCH LOGFILE` + `CHECKPOINT`, crear **Restore Points Garantizados** (`GRP`), ejecutar **Backup/Crosscheck RMAN de Archivelogs** y **purgar trazas ADRCI** para liberar inodos.
     * **Auditar y recuperar caminos SAN Multipath** (`multipath -ll`, `lsblk`, reescaneo en caliente de buses SCSI/Fibre Channel `/sys/class/scsi_host/host*/scan` + `multipath -r`), auditar `sysctl` y listar el Top 20 de procesos por CPU/RAM.
   * **Incluye un Motor SRE, SLOs (Service Level Objectives) y Avisos Críticos por Telegram y Email SMTP (`GET/POST /api/v1/sre`)**:
     * Calcula en tiempo real **4 SLOs clave** (*Disponibilidad BD & Clusterware 99.95%*, *Operación Libre de Bloqueos TX/TM 99.50%*, *Salud de Capacidad 99.90%* y *Redundancia SAN Multipath 99.99%*), su **SLI actual (%)**, el **Error Budget restante (%)** y la velocidad de consumo (**Burn Rate**).
     * Detecta **Alarmas Delicadas** (bloqueos TX/TM prolongados, caminos SAN degradados, llenado de tablespaces/FRA/ASM/inodos, errores `ORA-00600`/`ORA-07445`/`ORA-01654`, lag en Data Guard, instancias o demonios CRS caídos) y ofrece un botón de **remediación en 1 clic** junto a cada alarma.
     * Permite configurar desde la propia web el envío automático o manual de avisos críticos a **Telegram Bot API (`api.telegram.org`)** y **Correo Electrónico SMTP (`STARTTLS` / `AUTH PLAIN`)**.

2. **Prometheus con Autodescubrimiento en Caliente (`file_sd_configs` en `:9090`)**:
   Lee dinámicamente el fichero [`prometheus/targets.yml`](file:///Users/mario/repositorios/oratui/POC/prometheus/targets.yml) cada 15 segundos sin necesidad de reiniciar el contenedor, y evalúa **15 reglas de alarma** críticas ([`prometheus/oracle_alerts.yml`](file:///Users/mario/repositorios/oratui/POC/prometheus/oracle_alerts.yml)).

3. **Grafana Multi-Servidor en 2 Niveles (`:3000`, usuario `admin` / `admin`)**:
   * **Nivel 1 (Home Dashboard) — [`01. Vista Global Multi-Servidor — Flota Oracle & SAN`](file:///Users/mario/repositorios/oratui/POC/grafana/dashboards/oratui_fleet_overview.json)**: Mapa semáforo de todas las máquinas de la flota y rankings `Top 15` con *Drill-Down* en 1 clic.
   * **Nivel 2 (Detalle Completo por Servidor) — [`02. Detalle Completo de Servidor (26 Paneles)`](file:///Users/mario/repositorios/oratui/POC/grafana/dashboards/oratui_full_poc_dashboard.json)**: 26 paneles detallados con filtros jerárquicos encadenados (`Entorno` $\rightarrow$ `Cluster` $\rightarrow$ `Servidor`).

4. **Automatización con Ansible ([`POC/ansible/`](file:///Users/mario/repositorios/oratui/POC/ansible)) y Pipeline CI/CD en Jenkins ([`Jenkinsfile`](file:///Users/mario/repositorios/oratui/Jenkinsfile))**:
   Un único inventario ([`ansible/inventory.ini`](file:///Users/mario/repositorios/oratui/POC/ansible/inventory.ini)) despliega el daemon `ora-tui-otel` en todas las máquinas `[oracle_servers]` y actualiza automáticamente tanto `prometheus/targets.yml` como el catálogo `flutter-web/build/web/servers.json` en `[monitoring_server]`.

---

## ⚡ Opción A: Configuración Interactiva Rápida con `./configure-poc.sh`

Copia la carpeta `POC/` a tu servidor con Docker y ejecuta:

```bash
cd POC
chmod +x configure-poc.sh start-poc.sh
./configure-poc.sh
```

El script sincroniza automáticamente `prometheus/targets.yml` y `flutter-web/build/web/servers.json`, y levanta los 3 contenedores (`oratui-poc-flutter-web` en `:8088`, `oratui-poc-grafana` en `:3000` y `oratui-poc-prometheus` en `:9090`).

---

## 🤖 Opción B: Despliegue y Mantenimiento Automático con Ansible (`POC/ansible/`)

```bash
cd POC/ansible
# Instala/actualiza el daemon ora-tui-otel en [oracle_servers] y despliega Flutter Web (:8088) + Prometheus (:9090) + Grafana (:3000) en [monitoring_server]:
ansible-playbook -i inventory.ini site.yml
```

---

## 🏗️ Opción C: Gobierno de Flota con Jenkins ([`Jenkinsfile`](file:///Users/mario/repositorios/oratui/Jenkinsfile))

Consulta todos los detalles del pipeline parametrizado en **[`POC/jenkins/README.md`](file:///Users/mario/repositorios/oratui/POC/jenkins/README.md)**.

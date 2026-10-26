# 🚀 POC Multi-Servidor de Observabilidad y Operación Activa Oracle & Linux (`ora-tui v1.10.0` + Flutter Web Command & SRE Center + Prometheus + Grafana + Ansible + Jenkins)

Esta carpeta **`POC/`** proporciona una arquitectura integral de **Observabilidad, Ingeniería de Fiabilidad (SRE/SLO), Seguridad Sincronizada y Operación Remota Activa** para **1 o 100 servidores Oracle**:

1. **⚡ Contenedor `ORA-TUI Flutter Web Command & SRE Center` (`:8088`, en [`POC/flutter-web/`](file:///Users/mario/repositorios/oratui/POC/flutter-web)) — ¡Sustituto Interactivo de Grafana con Operación Directa y Login!**:
   Mientras que Grafana únicamente muestra gráficas en modo lectura, la aplicación **Flutter Web** empaquetada en Docker (`oratui-flutter-web`, puerto `:8088`):
   * **Pantalla de Login blindada por `.env`**: Solicita usuario y contraseña definidos en [`POC/.env`](file:///Users/mario/repositorios/oratui/POC/.env) (`FLUTTER_WEB_USER=admin` / `FLUTTER_WEB_PASSWORD=admin` por defecto para la POC) e inyecta automáticamente la cabecera `Authorization: Basic base64(user:pass)` (y `X-OraTui-Token` si se define `ORATUI_API_TOKEN`) en todas las llamadas hacia los servidores Oracle (`:9161`).
   * **Muestra todas las gráficas temporales y barras LCD de alto contraste en tiempo real** (Sesiones vs. Bloqueos TX/TM, CPU vs. RAM, Tablespaces con cálculo automático de `%` real vs. FRA vs. ASM, Errores `ORA-` vs. Caminos SAN Multipath caídos) con estética conmutable *MS-DOS Norton Commander* o *Dark Mode*.
   * **Permite ejecutar acciones correctivas y de mantenimiento directamente sobre los servidores Oracle** (`POST /api/v1/action`) mostrando la salida real de consola (`SQL*Plus`, `RMAN`, `lsnrctl`, `ADRCI`, `multipath`, `sysctl`) en un modal interactivo:
     * **Matar sesiones bloqueantes** (`ALTER SYSTEM KILL SESSION 'sid,serial,@inst' IMMEDIATE`).
     * **Ampliar tablespaces** (`ADD DATAFILE SIZE 32G AUTOEXTEND ON`) y consolidar espacio (`COALESCE`).
     * **Recompilar objetos inválidos** (`UTL_RECOMP.RECOMP_PARALLEL(4)`) y actualizar **estadísticas CBO** (`DBMS_STATS.GATHER_TABLE_STATS`).
     * **Recargar Listeners** (`lsnrctl reload`), forzar `SWITCH LOGFILE` + `CHECKPOINT`, crear **Restore Points Garantizados** (`GRP`), ejecutar **Backup/Crosscheck RMAN de Archivelogs** y **purgar trazas ADRCI** para liberar inodos.
     * **Auditar y recuperar caminos SAN Multipath** (`multipath -ll`, `lsblk`, reescaneo en caliente de buses SCSI/Fibre Channel `/sys/class/scsi_host/host*/scan` + `multipath -r`), auditar `sysctl` y listar el Top 20 de procesos por CPU/RAM.
   * **Incluye un Motor SRE, SLOs (Service Level Objectives) y Avisos Críticos por Telegram y Email SMTP (`GET/POST /api/v1/sre`)**:
     * Calcula en tiempo real **4 SLOs clave** (*Disponibilidad BD & Clusterware 99.95%*, *Operación Libre de Bloqueos TX/TM 99.50%*, *Salud de Capacidad 99.90%* y *Redundancia SAN Multipath 99.99%*), su **SLI actual (%)**, el **Error Budget restante (%)** y la velocidad de consumo (**Burn Rate**).
     * Detecta **Alarmas Delicadas** y ofrece un botón de **remediación en 1 clic** junto a cada alarma.
     * Permite configurar desde la propia web el envío automático o manual de avisos críticos a **Telegram Bot API (`api.telegram.org`)** y **Correo Electrónico SMTP (`STARTTLS` / `AUTH PLAIN`)**, guardando las credenciales con permisos `0600` en `/etc/ora-tui-sre.json` y devolviéndolas enmascaradas (`********`).

2. **🔐 Arquitectura de Seguridad entre Flutter Web (`:8088`) y el Servidor Oracle (`:9161`)**:
   * **Lista Blanca Cerrada (`Action Whitelist`)**: El navegador nunca envía comandos SQL ni Bash arbitrarios; solo envía un identificador cerrado (`action: "switch_logfile"`) que el binario Go traduce internamente.
   * **Filtro Estricto Anti-Inyección (`validateSafeTarget`)**: El parámetro `target` se valida contra la expresión regular `^[a-zA-Z0-9_.,@#$+-]{0,96}$`, bloqueando con `HTTP 400` cualquier intento de inyección (`;`, `--`, `DROP`, `UNION`, `` ` ``, `$`).
   * **Autenticación Sincronizada (`HTTP Basic Auth` + `X-OraTui-Token`)**:
     * **Modo POC (por defecto)**: Tanto `POC/.env` como el agente `ora-tui` usan `admin` / `admin`. Al hacer login en Flutter Web, esta envía `Authorization: Basic YWRtaW46YWRtaW4=` y el agente Oracle lo valida automáticamente.
     * **Modo Producción (con Ansible o `.env`)**: Al definir `flutter_web_password` y `oratui_api_token` en [`POC/ansible/inventory.ini`](file:///Users/mario/repositorios/oratui/POC/ansible/inventory.ini), el playbook [`POC/ansible/site.yml`](file:///Users/mario/repositorios/oratui/POC/ansible/site.yml) inyecta automáticamente las mismas credenciales tanto en el servicio `systemd` (`/etc/systemd/system/ora-tui-otel.service`, modo `0600`) de **todos los servidores Oracle** como en el fichero `.env` del contenedor **Flutter Web**.

3. **Prometheus con Autodescubrimiento en Caliente (`file_sd_configs` en `:9090`)**:
   Lee dinámicamente el fichero [`prometheus/targets.yml`](file:///Users/mario/repositorios/oratui/POC/prometheus/targets.yml) cada 15 segundos sin necesidad de reiniciar el contenedor, y evalúa **15 reglas de alarma** críticas ([`prometheus/oracle_alerts.yml`](file:///Users/mario/repositorios/oratui/POC/prometheus/oracle_alerts.yml)).

4. **Grafana Multi-Servidor en 2 Niveles (`:3000`, usuario `admin` / `admin`)**:
   * **Nivel 1 (Home Dashboard) — [`01. Vista Global Multi-Servidor — Flota Oracle & SAN`](file:///Users/mario/repositorios/oratui/POC/grafana/dashboards/oratui_fleet_overview.json)**: Mapa semáforo de todas las máquinas de la flota y rankings `Top 15` con *Drill-Down* en 1 clic.
   * **Nivel 2 (Detalle Completo por Servidor) — [`02. Detalle Completo de Servidor (26 Paneles)`](file:///Users/mario/repositorios/oratui/POC/grafana/dashboards/oratui_full_poc_dashboard.json)**: 26 paneles detallados con filtros jerárquicos encadenados (`Entorno` $\rightarrow$ `Cluster` $\rightarrow$ `Servidor`).

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

## 🤖 Opción B: Despliegue y Sincronización de Seguridad con Ansible (`POC/ansible/`)

En [`POC/ansible/inventory.ini`](file:///Users/mario/repositorios/oratui/POC/ansible/inventory.ini) defines tus servidores y, bajo `[all:vars]`, las credenciales y el token que Ansible repartirá tanto a los demonios Oracle (`systemd`) como a los contenedores Docker (`.env`):

```ini
[all:vars]
flutter_web_user=admin
flutter_web_password=admin
grafana_admin_user=admin
grafana_admin_password=admin
oratui_api_token=
```

Y ejecutas:

```bash
cd POC/ansible
# Instala/actualiza el daemon ora-tui-otel (con sus credenciales/token) en [oracle_servers]
# y despliega Flutter Web (:8088) + Prometheus (:9090) + Grafana (:3000) en [monitoring_server]:
ansible-playbook -i inventory.ini site.yml
```

---

## 🏗️ Opción C: Gobierno de Flota con Jenkins ([`Jenkinsfile`](file:///Users/mario/repositorios/oratui/Jenkinsfile))

El [`Jenkinsfile`](file:///Users/mario/repositorios/oratui/Jenkinsfile) incluye parámetros de tipo `password` (`FLUTTER_WEB_PASSWORD` y `ORATUI_API_TOKEN`) para rotar contraseñas y tokens en toda la flota Oracle y en el contenedor Flutter Web con un solo clic. Consulta todos los detalles en **[`POC/jenkins/README.md`](file:///Users/mario/repositorios/oratui/POC/jenkins/README.md)**.

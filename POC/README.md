# 🚀 POC Multi-Servidor de Observabilidad Oracle & Linux (`ora-tui` + Prometheus + Grafana + Ansible + Jenkins)

Esta carpeta **`POC/`** proporciona una arquitectura completa para desplegar y mantener la observabilidad de **1 o 100 servidores Oracle** sin perder ni un solo detalle de las **26 gráficas por servidor**:

1. **Prometheus con Autodescubrimiento en Caliente (`file_sd_configs` en `:9090`)**:
   Lee dinámicamente el fichero [`prometheus/targets.yml`](file:///Users/mario/repositorios/oratui/POC/prometheus/targets.yml) cada 15 segundos sin necesidad de reiniciar el contenedor, y evalúa **15 reglas de alarma** críticas ([`prometheus/oracle_alerts.yml`](file:///Users/mario/repositorios/oratui/POC/prometheus/oracle_alerts.yml)).
2. **Grafana Multi-Servidor en 2 Niveles (`:3000`, usuario `admin` / `admin`)**:
   * **Nivel 1 (Home Dashboard) — [`01. Vista Global Multi-Servidor — Flota Oracle & SAN`](file:///Users/mario/repositorios/oratui/POC/grafana/dashboards/oratui_fleet_overview.json)**:
     Mapa semáforo de todas las máquinas de tu flota, tabla maestra de inventario en vivo y rankings `Top 15` de tablespaces, ASM, FRA e inodos más llenos de toda la empresa. **Haciendo clic sobre cualquier servidor se abre directamente su Dashboard de 26 paneles filtrado para esa máquina (*Drill-Down*)**.
   * **Nivel 2 (Detalle Completo por Servidor) — [`02. Detalle Completo de Servidor (26 Paneles)`](file:///Users/mario/repositorios/oratui/POC/grafana/dashboards/oratui_full_poc_dashboard.json)**:
     Conserva las **26 gráficas detalladas** pero añade arriba **3 desplegables jerárquicos encadenados (`1. Entorno` $\rightarrow$ `2. Cluster / BD` $\rightarrow$ `3. Servidor Oracle`)** con selección individual por defecto para que las gráficas de un servidor nunca se mezclen con las de otros, además de un botón arriba a la derecha para volver en 1 clic a la Vista Global de Flota.
3. **Automatización con Ansible ([`POC/ansible/`](file:///Users/mario/repositorios/oratui/POC/ansible))**:
   Un único inventario ([`ansible/inventory.ini`](file:///Users/mario/repositorios/oratui/POC/ansible/inventory.ini)) con `[oracle_servers]` y `[monitoring_server]`. El playbook [`ansible/site.yml`](file:///Users/mario/repositorios/oratui/POC/ansible/site.yml) instala/mantiene el daemon `ora-tui-otel` en todas las máquinas con Oracle y usa esa misma lista de máquinas para generar automáticamente `prometheus/targets.yml` e instalar/mantener Prometheus y Grafana en el servidor central.
4. **Pipeline CI/CD y Gestión de Flota en Jenkins ([`Jenkinsfile`](file:///Users/mario/repositorios/oratui/Jenkinsfile) y [`POC/jenkins/`](file:///Users/mario/repositorios/oratui/POC/jenkins/README.md))**:
   Permite compilar, desplegar con Ansible, verificar salud (`/healthz`) o dar de alta nuevos servidores Oracle desde un formulario web en Jenkins.

---

## ⚡ Opción A: Configuración Interactiva Rápida con `./configure-poc.sh`

Copia la carpeta `POC/` a tu servidor con Docker y ejecuta:

```bash
cd POC
chmod +x configure-poc.sh start-poc.sh
./configure-poc.sh
```

El script te mostrará un menú interactivo:
* **`[1]` Añadir o configurar un servidor Oracle interactivamente**: Te pregunta cada vez el `Nombre o IP del host Oracle`, el `Puerto` (`9161`) y el `Entorno` (`PROD`/`PRE`/`DES`), lo añade a `prometheus/targets.yml` sin borrar los anteriores, comprueba `/healthz` y levanta Docker.
* **`[2]` Reemplazar toda la lista con un único servidor Oracle**.
* **`[3]` Sincronizar la lista de servidores desde `ansible/inventory.ini`**.
* **`[4]` Ejecutar el playbook completo de Ansible (`ansible/site.yml`)**.

---

## 🤖 Opción B: Despliegue y Mantenimiento Automático con Ansible (`POC/ansible/`)

1. **Edita el inventario [`POC/ansible/inventory.ini`](file:///Users/mario/repositorios/oratui/POC/ansible/inventory.ini)** poniendo tus servidores con Oracle bajo `[oracle_servers]` y tu servidor Docker bajo `[monitoring_server]`:

   ```ini
   [oracle_servers]
   ora-prod-rac01.local  ansible_host=192.168.10.11  env=PROD  cluster_label=rac-prod  otel_port=9161
   ora-prod-rac02.local  ansible_host=192.168.10.12  env=PROD  cluster_label=rac-prod  otel_port=9161
   ora-pre-db01.local    ansible_host=192.168.20.11  env=PRE   cluster_label=single-pre otel_port=9161

   [monitoring_server]
   monitor-server.local  ansible_host=192.168.10.100
   ```

2. **Lanza el Playbook de Ansible**:
   ```bash
   cd POC/ansible

   # Despliegue completo (instala daemon ora-tui-otel en [oracle_servers] + Prometheus/Grafana en [monitoring_server]):
   ansible-playbook -i inventory.ini site.yml

   # Si solo has añadido/quitado servidores en inventory.ini y quieres refrescar Prometheus/Grafana:
   ansible-playbook -i inventory.ini site.yml --tags monitoring

   # Si solo quieres actualizar el binario/daemon ora-tui-otel en las máquinas Oracle:
   ansible-playbook -i inventory.ini site.yml --tags oracle
   ```

---

## 🏗️ Opción C: Gobierno "Para Nota" con Jenkins ([`Jenkinsfile`](file:///Users/mario/repositorios/oratui/Jenkinsfile))

Crea un Job de tipo **Pipeline** en Jenkins apuntando al repositorio `https://github.com/mario-ezquerro/oratui.git` (`Script Path: Jenkinsfile`).

Al pulsar **Build with Parameters**, dispondrás de:
* **`DEPLOY_ALL_ANSIBLE`**: Despliega/mantiene los daemons en `[oracle_servers]` y actualiza Prometheus + Grafana en `[monitoring_server]`.
* **`FULL_CI_CD_AND_DEPLOY`**: Compila el código Go (`CGO_ENABLED=0`), pasa los tests unitarios, construye los `.rpm` y `.deb`, y los despliega con Ansible.
* **`ADD_NEW_ORACLE_SERVER`**: Rellenas el nombre, IP y entorno del nuevo servidor Oracle en Jenkins; el pipeline lo añade automáticamente a `POC/ansible/inventory.ini`, le instala el daemon `ora-tui-otel` y actualiza `targets.yml` en Prometheus en caliente.
* **`UPDATE_MONITORING_TARGETS_ONLY`** y **`UPDATE_ORACLE_DAEMONS_ONLY`**.
* **`HEALTH_CHECK_FLEET`**: Verifica que el endpoint `/healthz` de todos los servidores Oracle responde `200 OK`.

Consulta todos los detalles en **[`POC/jenkins/README.md`](file:///Users/mario/repositorios/oratui/POC/jenkins/README.md)**.

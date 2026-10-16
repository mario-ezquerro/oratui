# 🏗️ Guía de Jenkins + Ansible para Gestión Multi-Servidor (`ora-tui`)

Este directorio (junto con el [`Jenkinsfile`](file:///Users/mario/repositorios/oratui/Jenkinsfile) de la raíz del repositorio y [`POC/ansible/`](file:///Users/mario/repositorios/oratui/POC/ansible)) proporciona una solución **CI/CD & GitOps completa** para instalar, mantener y escalar la monitorización de toda tu flota de servidores Oracle y tu servidor central de Prometheus + Grafana.

---

## 🔄 ¿Cómo funciona la integración entre Inventario, Ansible, Jenkins, Prometheus y Grafana?

```text
┌────────────────────────────────────────────────────────────────────────────┐
│ 1. INVENTARIO ÚNICO (POC/ansible/inventory.ini)                            │
│    • [oracle_servers]    -> Lista de todos los servidores con Oracle       │
│    • [monitoring_server] -> Máquina central con Docker (Prometheus/Grafana)│
└───────────────────────────────────────┬────────────────────────────────────┘
                                        │
                                        ▼
┌────────────────────────────────────────────────────────────────────────────┐
│ 2. PIPELINE DE JENKINS (Jenkinsfile) + ANSIBLE (POC/ansible/site.yml)      │
│    • Play 1 ([oracle_servers]):                                            │
│      - Instala/actualiza el paquete .rpm o .deb de ora-tui                 │
│      - Configura y arranca el daemon systemd "ora-tui-otel" (:9161)        │
│      - Abre el puerto 9161/tcp en firewalld y verifica /healthz            │
│    • Play 2 ([monitoring_server]):                                         │
│      - Genera automáticamente prometheus/targets.yml desde [oracle_servers]│
│      - Levanta Prometheus (:9090) con file_sd_configs (recarga en caliente)│
│      - Levanta Grafana (:3000) con los 2 Dashboards enlazados              │
└───────────────────────────────────────┬────────────────────────────────────┘
                                        │
                                        ▼
┌────────────────────────────────────────────────────────────────────────────┐
│ 3. GRAFANA MULTI-SERVIDOR EN 2 NIVELES                                     │
│    • Nivel 1 (Home): "01. Vista Global Multi-Servidor — Flota Oracle & SAN"│
│      Mapa y tabla maestra de todos los servidores con Drill-Down en 1 clic │
│    • Nivel 2 (Detalle): "02. Detalle Completo de Servidor (26 Paneles)"    │
│      Con selectores arriba (Entorno -> Cluster -> Servidor) sin mezclarlos │
└────────────────────────────────────────────────────────────────────────────┘
```

---

## 🛠️ Cómo configurar el Job en Jenkins en 2 minutos

1. En Jenkins, haz clic en **New Item (Nueva Tarea)** $\rightarrow$ escribe `ora-tui-fleet-manager` $\rightarrow$ selecciona **Pipeline** y pulsa **OK**.
2. En la sección **Pipeline**:
   * **Definition**: `Pipeline script from SCM`
   * **SCM**: `Git`
   * **Repository URL**: `https://github.com/mario-ezquerro/oratui.git`
   * **Branch Specifier**: `*/main`
   * **Script Path**: `Jenkinsfile`
3. Guarda y pulsa **Build with Parameters (Construir con parámetros)**.

---

## 🎛️ Modos de Operación disponibles en el Formulario de Jenkins (`ACTION`)

Al pulsar **Build with Parameters**, Jenkins te mostrará el desplegable `ACTION` con **6 operaciones listas**:

| Opción en `ACTION` | ¿Qué hace exactamente? |
| :--- | :--- |
| **`DEPLOY_ALL_ANSIBLE`** | Lee `POC/ansible/inventory.ini`, instala/mantiene el daemon `ora-tui-otel` en todas las máquinas de `[oracle_servers]` y actualiza Prometheus (`targets.yml`) + Grafana en `[monitoring_server]`. |
| **`FULL_CI_CD_AND_DEPLOY`** | Compila desde cero el binario estático en Go (`CGO_ENABLED=0`), pasa los tests unitarios, genera los nuevos paquetes `.rpm` y `.deb`, y los despliega en toda la flota con Ansible. |
| **`UPDATE_MONITORING_TARGETS_ONLY`** | Ejecuta únicamente `--tags monitoring`: regenera `prometheus/targets.yml` con la lista de servidores de `[oracle_servers]` y actualiza los dashboards de Grafana en caliente sin tocar los servidores Oracle. |
| **`UPDATE_ORACLE_DAEMONS_ONLY`** | Ejecuta únicamente `--tags oracle`: actualiza el paquete `ora-tui` y el servicio `ora-tui-otel` en las máquinas Oracle (puedes filtrar una sola máquina con `LIMIT_HOSTS`). |
| **`ADD_NEW_ORACLE_SERVER`** | **Alta automática desde Jenkins**: Rellenas `NEW_SERVER_HOSTNAME`, `NEW_SERVER_IP`, `NEW_SERVER_ENV` (`PROD`/`PRE`/`DES`) y `NEW_SERVER_CLUSTER`. Jenkins añade el servidor a `inventory.ini`, le instala el daemon `ora-tui-otel` por SSH y lo añade en caliente a Prometheus y Grafana. |
| **`HEALTH_CHECK_FLEET`** | Recorre todas las máquinas del inventario y comprueba con `curl` que `http://<IP>:9161/healthz` responde `200 OK`. |

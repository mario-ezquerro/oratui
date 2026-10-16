# 🏗️ Guía de Jenkins + Ansible para Gestión Multi-Servidor (`ora-tui`)

Tanto [`Jenkinsfile`](file:///Users/mario/repositorios/oratui/Jenkinsfile) (en la raíz del repositorio) como [`POC/jenkins/Jenkinsfile`](file:///Users/mario/repositorios/oratui/POC/jenkins/Jenkinsfile) contienen el **Pipeline Declarativo en Groovy nativo** para desplegar y mantener desde Jenkins toda la flota de servidores Oracle (`[oracle_servers]`) y el servidor central de Prometheus + Grafana (`[monitoring_server]`).

---

## 🛠️ Cómo configurar el Job en Jenkins

1. En Jenkins, pulsa **New Item (Nueva Tarea)** $\rightarrow$ escribe `ora-tui-fleet-manager` $\rightarrow$ selecciona **Pipeline** y pulsa **OK**.
2. En la sección **Pipeline**:
   * **Definition**: `Pipeline script from SCM`
   * **SCM**: `Git`
   * **Repository URL**: `https://github.com/mario-ezquerro/oratui.git`
   * **Branch Specifier**: `*/main`
   * **Script Path**: `Jenkinsfile` (o `POC/jenkins/Jenkinsfile`)
3. Guarda y pulsa **Build with Parameters (Construir con parámetros)**.

---

## 🎛️ Parámetros del Pipeline en Groovy

| Parámetro | Tipo | Descripción |
| :--- | :--- | :--- |
| **`ACTION`** | `choice` | • **`DEPLOY_ALL`**: Instala/mantiene el daemon `ora-tui-otel` en `[oracle_servers]` y actualiza Prometheus + Grafana en `[monitoring_server]`.<br>• **`UPDATE_MONITORING_ONLY`**: Solo regenera `targets.yml` y actualiza Prometheus + Grafana (`--tags monitoring`).<br>• **`UPDATE_ORACLE_DAEMONS_ONLY`**: Solo instala/actualiza el daemon `ora-tui-otel` en los servidores Oracle (`--tags oracle`).<br>• **`BUILD_PACKAGES_AND_DEPLOY`**: Compila Go (`CGO_ENABLED=0`), genera los `.rpm`/`.deb` y despliega todo con Ansible.<br>• **`HEALTH_CHECK_ONLY`**: Verifica en Groovy que el endpoint `/healthz` de todos los servidores responde `200 OK`. |
| **`INVENTORY_SOURCE`** | `choice` | • **`USE_REPO_INVENTORY_FILE`**: Usa el fichero [`POC/ansible/inventory.ini`](file:///Users/mario/repositorios/oratui/POC/ansible/inventory.ini) del repositorio Git.<br>• **`USE_JENKINS_PARAMETERS`**: Genera dinámicamente el `inventory.ini` en Groovy con las cajas de texto **`MONITORING_SERVER`** y **`ORACLE_SERVERS`** del propio formulario de Jenkins. |
| **`MONITORING_SERVER`** | `string` | Servidor donde se instala y mantiene Prometheus + Grafana (`monitor-server.local ansible_host=192.168.10.100`). |
| **`ORACLE_SERVERS`** | `text` (multilínea) | Listado de todas las máquinas con Oracle (una por línea con `ansible_host`, `env=PROD|PRE|DES` y `cluster_label`). |
| **`ANSIBLE_LIMIT`** | `string` | Permite limitar la ejecución a un servidor o grupo concreto (`all`, `oracle_servers`, `monitoring_server`, o el nombre de un host). |
| **`SSH_CREDENTIALS_ID`** | `string` | ID de la credencial SSH en Jenkins (`sshUserPrivateKey`). Si no existe o se deja vacía, usa la clave SSH por defecto del nodo Jenkins. |
| **`ANSIBLE_DRY_RUN`** | `boolean` | Si se marca, ejecuta Ansible en modo simulación (`--check --diff`) sin modificar los servidores. |

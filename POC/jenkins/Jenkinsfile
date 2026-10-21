#!/usr/bin/env groovy
// ==============================================================================
// Jenkinsfile — Pipeline Declarativo en Groovy para ora-tui (v1.6.0)
//
// Gestiona de extremo a extremo mediante Ansible:
//   1. Las máquinas con Oracle ([oracle_servers]): instala/actualiza el paquete
//      ora-tui (.rpm/.deb) y mantiene activo el daemon systemd "ora-tui-otel".
//   2. La máquina del servidor central ([monitoring_server]): instala/mantiene
//      Prometheus + Grafana y actualiza en caliente (file_sd_configs) el listado
//      de servidores Oracle monitorizados (targets.yml) y los 2 Dashboards.
// ==============================================================================

pipeline {
    agent any

    options {
        timestamps()
        ansiColor('xterm')
        buildDiscarder(logRotator(numToKeepStr: '30', artifactNumToKeepStr: '10'))
        disableConcurrentBuilds()
        timeout(time: 30, unit: 'MINUTES')
    }

    parameters {
        choice(
            name: 'ACTION',
            choices: [
                'DEPLOY_ALL',
                'UPDATE_MONITORING_ONLY',
                'UPDATE_ORACLE_DAEMONS_ONLY',
                'BUILD_PACKAGES_AND_DEPLOY',
                'HEALTH_CHECK_ONLY'
            ],
            description: 'DEPLOY_ALL: Despliega daemon en servidores Oracle y actualiza Prometheus/Grafana | UPDATE_MONITORING_ONLY: Solo actualiza targets.yml, Prometheus y Grafana | UPDATE_ORACLE_DAEMONS_ONLY: Solo actualiza el daemon en servidores Oracle | BUILD_PACKAGES_AND_DEPLOY: Compila Go + RPM/DEB y despliega todo | HEALTH_CHECK_ONLY: Verifica /healthz en toda la flota'
        )
        choice(
            name: 'INVENTORY_SOURCE',
            choices: [
                'USE_REPO_INVENTORY_FILE',
                'USE_JENKINS_PARAMETERS'
            ],
            description: 'USE_REPO_INVENTORY_FILE: Usa POC/ansible/inventory.ini del repositorio Git | USE_JENKINS_PARAMETERS: Genera el inventario usando los parámetros MONITORING_SERVER y ORACLE_SERVERS de abajo'
        )
        string(
            name: 'MONITORING_SERVER',
            defaultValue: 'monitor-server.local ansible_host=192.168.10.100',
            description: 'Servidor donde se instala/mantiene Prometheus y Grafana ([monitoring_server])'
        )
        text(
            name: 'ORACLE_SERVERS',
            defaultValue: '''ora-prod-rac01.local  ansible_host=192.168.10.11  env=PROD  cluster_label=rac-prod   otel_port=9161
ora-prod-rac02.local  ansible_host=192.168.10.12  env=PROD  cluster_label=rac-prod   otel_port=9161
ora-pre-db01.local    ansible_host=192.168.20.11  env=PRE   cluster_label=single-pre otel_port=9161
ora-des-db01.local    ansible_host=192.168.30.11  env=DES   cluster_label=single-des otel_port=9161''',
            description: 'Listado de máquinas con Oracle ([oracle_servers]). Una máquina por línea con su IP (ansible_host), entorno (env) y cluster (cluster_label).'
        )
        string(
            name: 'ANSIBLE_LIMIT',
            defaultValue: 'all',
            description: 'Limitar ejecución de Ansible a un servidor o grupo (ej. all, oracle_servers, monitoring_server, ora-prod-rac01.local)'
        )
        string(
            name: 'DEFAULT_OTEL_PORT',
            defaultValue: '9161',
            description: 'Puerto por defecto del exportador OpenTelemetry/Prometheus de ora-tui'
        )
        string(
            name: 'SCRAPE_INTERVAL',
            defaultValue: '10s',
            description: 'Intervalo de recolección del daemon ora-tui en cada servidor Oracle'
        )
        string(
            name: 'FLUTTER_WEB_USER',
            defaultValue: 'admin',
            description: 'Usuario de acceso para Flutter Web Command Center (:8088), Grafana (:3000) y API de los agentes Oracle (:9161)'
        )
        password(
            name: 'FLUTTER_WEB_PASSWORD',
            defaultValue: 'admin',
            description: 'Contraseña sincronizada para Flutter Web (:8088), Grafana (:3000) y agentes Oracle (:9161)'
        )
        password(
            name: 'ORATUI_API_TOKEN',
            defaultValue: '',
            description: 'Token compartido opcional (X-OraTui-Token) sincronizado entre Flutter Web y todos los servidores Oracle (:9161)'
        )
        string(
            name: 'SSH_CREDENTIALS_ID',
            defaultValue: 'oracle-ansible-ssh',
            description: 'ID de la credencial SSH (SSH Username with private key) en Jenkins para conectar a los servidores. Dejar vacío si el nodo Jenkins ya tiene claves SSH configuradas.'
        )
        booleanParam(
            name: 'ANSIBLE_DRY_RUN',
            defaultValue: false,
            description: 'Ejecutar Ansible en modo simulación (--check --diff) sin aplicar cambios en los servidores'
        )
    }

    environment {
        CGO_ENABLED    = '0'
        ANSIBLE_DIR    = "${WORKSPACE}/POC/ansible"
        INVENTORY_PATH = "${WORKSPACE}/POC/ansible/inventory.ini"
        PLAYBOOK_PATH  = "${WORKSPACE}/POC/ansible/site.yml"
        ANSIBLE_CONFIG = "${WORKSPACE}/POC/ansible/ansible.cfg"
        ANSIBLE_FORCE_COLOR = 'true'
    }

    stages {
        stage('1. Preparar y Validar Inventario') {
            steps {
                script {
                    echo ">>> Acción seleccionada: ${params.ACTION}"
                    echo ">>> Origen del inventario: ${params.INVENTORY_SOURCE}"

                    if (params.INVENTORY_SOURCE == 'USE_JENKINS_PARAMETERS') {
                        def oracleLines = params.ORACLE_SERVERS
                            .split('\n')
                            .collect { it.trim() }
                            .findAll { it && !it.startsWith('#') }

                        if (oracleLines.isEmpty()) {
                            error('El parámetro ORACLE_SERVERS no puede estar vacío cuando se usa USE_JENKINS_PARAMETERS.')
                        }
                        if (!params.MONITORING_SERVER?.trim()) {
                            error('El parámetro MONITORING_SERVER no puede estar vacío.')
                        }

                        def generatedInventory = """# Inventario generado automáticamente por Jenkins (Build #${env.BUILD_NUMBER})
[oracle_servers]
${oracleLines.join('\n')}

[monitoring_server]
${params.MONITORING_SERVER.trim()}

[all:vars]
ansible_user=root
default_otel_port=${params.DEFAULT_OTEL_PORT.trim()}
oratui_scrape_interval=${params.SCRAPE_INTERVAL.trim()}
poc_install_dir=/opt/oratui-poc
flutter_web_user=${params.FLUTTER_WEB_USER.trim()}
flutter_web_password=${params.FLUTTER_WEB_PASSWORD}
grafana_admin_user=${params.FLUTTER_WEB_USER.trim()}
grafana_admin_password=${params.FLUTTER_WEB_PASSWORD}
oratui_api_token=${params.ORATUI_API_TOKEN}
oratui_rpm_url=https://github.com/mario-ezquerro/oratui/raw/main/dist/ora-tui-1.9.0-1.x86_64.rpm
oratui_deb_url=https://github.com/mario-ezquerro/oratui/raw/main/dist/ora-tui_1.9.0-1_amd64.deb
"""
                        writeFile(file: env.INVENTORY_PATH, text: generatedInventory, encoding: 'UTF-8')
                    }

                    def currentInventory = readFile(file: env.INVENTORY_PATH, encoding: 'UTF-8')
                    echo "================ INVENTARIO ACTIVO ================"
                    echo currentInventory
                    echo "==================================================="
                }
            }
        }

        stage('2. Compilar Binario Go y Paquetes RPM/DEB') {
            when {
                expression { return params.ACTION == 'BUILD_PACKAGES_AND_DEPLOY' }
            }
            steps {
                sh '''
                    set -eu
                    echo ">>> Ejecutando tests unitarios y compilación estática (CGO_ENABLED=0)..."
                    make all
                    make packages
                    ls -lh bin/ dist/
                '''
            }
        }

        stage('3. Ansible: Instalar/Mantener Daemon en Servidores Oracle') {
            when {
                expression {
                    return params.ACTION in [
                        'DEPLOY_ALL',
                        'BUILD_PACKAGES_AND_DEPLOY',
                        'UPDATE_ORACLE_DAEMONS_ONLY'
                    ]
                }
            }
            steps {
                script {
                    runAnsiblePlaybook(
                        tags: 'oracle',
                        limit: params.ANSIBLE_LIMIT,
                        dryRun: params.ANSIBLE_DRY_RUN,
                        sshCredsId: params.SSH_CREDENTIALS_ID,
                        otelPort: params.DEFAULT_OTEL_PORT,
                        scrapeInterval: params.SCRAPE_INTERVAL
                    )
                }
            }
        }

        stage('4. Ansible: Instalar/Mantener Prometheus y Grafana en Servidor Central') {
            when {
                expression {
                    return params.ACTION in [
                        'DEPLOY_ALL',
                        'BUILD_PACKAGES_AND_DEPLOY',
                        'UPDATE_MONITORING_ONLY'
                    ]
                }
            }
            steps {
                script {
                    runAnsiblePlaybook(
                        tags: 'monitoring',
                        limit: params.ANSIBLE_LIMIT,
                        dryRun: params.ANSIBLE_DRY_RUN,
                        sshCredsId: params.SSH_CREDENTIALS_ID,
                        otelPort: params.DEFAULT_OTEL_PORT,
                        scrapeInterval: params.SCRAPE_INTERVAL
                    )
                }
            }
        }

        stage('5. Verificar Salud (/healthz) de Toda la Flota') {
            steps {
                script {
                    def inventoryText = readFile(file: env.INVENTORY_PATH, encoding: 'UTF-8')
                    def targets = parseInventoryEndpoints(inventoryText, params.DEFAULT_OTEL_PORT.trim())

                    echo ">>> Comprobando endpoints OpenTelemetry (/healthz) en ${targets.size()} servidores Oracle..."
                    for (t in targets) {
                        def targetUrl = "http://${t.ip}:${t.port}/healthz"
                        def status = sh(
                            script: "curl -fsS --connect-timeout 3 '${targetUrl}' >/dev/null 2>&1",
                            returnStatus: true
                        )
                        if (status == 0) {
                            echo "  [OK] ${t.name} (${targetUrl}) -> ONLINE"
                        } else {
                            echo "  [WARN] ${t.name} (${targetUrl}) -> SIN RESPUESTA"
                        }
                    }
                }
            }
        }
    }

    post {
        always {
            archiveArtifacts(
                artifacts: 'POC/ansible/inventory.ini, POC/prometheus/targets.yml, dist/*.rpm, dist/*.deb',
                allowEmptyArchive: true
            )
        }
        success {
            echo '✅ Pipeline finalizado correctamente: Daemons ora-tui-otel, Prometheus (targets.yml) y Grafana actualizados.'
        }
        failure {
            echo '❌ El pipeline ha fallado. Revisa el log de la etapa correspondiente.'
        }
    }
}

// ==============================================================================
// Funciones auxiliares en Groovy nativo para Jenkins Pipeline
// ==============================================================================

def runAnsiblePlaybook(Map cfg) {
    def extraArgs = []
    extraArgs << "-i '${env.INVENTORY_PATH}'"
    extraArgs << "'${env.PLAYBOOK_PATH}'"
    extraArgs << "--tags '${cfg.tags}'"

    if (cfg.limit?.trim()) {
        extraArgs << "--limit '${cfg.limit.trim()}'"
    }
    extraArgs << "-e 'default_otel_port=${cfg.otelPort.trim()}'"
    extraArgs << "-e 'oratui_scrape_interval=${cfg.scrapeInterval.trim()}'"

    if (cfg.dryRun) {
        extraArgs << '--check --diff'
    }

    def ansibleCmd = "ansible-playbook ${extraArgs.join(' ')}"

    // Si existe la credencial SSH en Jenkins, la inyectamos con withCredentials;
    // si no existe o está vacía, ejecutamos directamente con las claves del agente Jenkins.
    if (cfg.sshCredsId?.trim()) {
        try {
            withCredentials([
                sshUserPrivateKey(
                    credentialsId: cfg.sshCredsId.trim(),
                    keyFileVariable: 'ANSIBLE_SSH_KEY_FILE',
                    usernameVariable: 'ANSIBLE_SSH_USER'
                )
            ]) {
                sh """
                    set -eu
                    ${ansibleCmd} -u "\$ANSIBLE_SSH_USER" --private-key="\$ANSIBLE_SSH_KEY_FILE"
                """
            }
        } catch (Exception e) {
            echo ">>> Credencial '${cfg.sshCredsId}' no encontrada en Jenkins Credentials; ejecutando con la clave SSH por defecto del agente..."
            sh """
                set -eu
                ${ansibleCmd}
            """
        }
    } else {
        sh """
            set -eu
            ${ansibleCmd}
        """
    }
}

@NonCPS
List<Map> parseInventoryEndpoints(String content, String fallbackPort) {
    List<Map> result = []
    boolean inOracleSection = false

    for (String rawLine : content.split('\n')) {
        String line = rawLine.trim()
        if (!line || line.startsWith('#') || line.startsWith(';')) {
            continue
        }
        if (line.startsWith('[')) {
            inOracleSection = (line == '[oracle_servers]')
            continue
        }
        if (inOracleSection) {
            String[] tokens = line.split('\\s+')
            String hostName = tokens[0]
            String hostIp   = hostName
            String port     = fallbackPort

            for (int i = 1; i < tokens.length; i++) {
                if (tokens[i].startsWith('ansible_host=')) {
                    hostIp = tokens[i].substring('ansible_host='.length())
                } else if (tokens[i].startsWith('otel_port=')) {
                    port = tokens[i].substring('otel_port='.length())
                }
            }
            result.add([name: hostName, ip: hostIp, port: port])
        }
    }
    return result
}

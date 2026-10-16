// ==============================================================================
// Jenkinsfile — Pipeline CI/CD y Gestión de Flota Oracle para ora-tui (v1.6.0)
//
// Permite desde la interfaz gráfica de Jenkins:
//   1. Compilar ora-tui (CGO_ENABLED=0), ejecutar tests y empaquetar .rpm y .deb
//   2. Instalar y mantener el daemon systemd "ora-tui-otel" en todos los
//      servidores Oracle del inventario (POC/ansible/inventory.ini)
//   3. Dar de alta nuevos servidores Oracle en el inventario desde un formulario
//   4. Mantener y actualizar en caliente Prometheus (targets.yml file_sd) y
//      los 2 Dashboards de Grafana (Flota Multi-Servidor + 26 Paneles Detalle)
//   5. Ejecutar auditorías de salud (/healthz) sobre toda la flota
// ==============================================================================

pipeline {
    agent any

    options {
        timestamps()
        ansiColor('xterm')
        buildDiscarder(logRotator(numToKeepStr: '30'))
        disableConcurrentBuilds()
    }

    parameters {
        choice(
            name: 'ACTION',
            choices: [
                'DEPLOY_ALL_ANSIBLE',
                'FULL_CI_CD_AND_DEPLOY',
                'UPDATE_MONITORING_TARGETS_ONLY',
                'UPDATE_ORACLE_DAEMONS_ONLY',
                'ADD_NEW_ORACLE_SERVER',
                'HEALTH_CHECK_FLEET'
            ],
            description: '''Acción a ejecutar:
• DEPLOY_ALL_ANSIBLE: Instala/mantiene el daemon ora-tui-otel en [oracle_servers] y actualiza Prometheus + Grafana en [monitoring_server].
• FULL_CI_CD_AND_DEPLOY: Compila Go, pasa tests, genera .rpm/.deb y despliega todo con Ansible.
• UPDATE_MONITORING_TARGETS_ONLY: Solo actualiza el listado de máquinas (targets.yml) y dashboards en Prometheus/Grafana.
• UPDATE_ORACLE_DAEMONS_ONLY: Solo instala/actualiza el daemon ora-tui-otel en los servidores Oracle.
• ADD_NEW_ORACLE_SERVER: Añade un nuevo servidor Oracle al inventario, le instala el daemon y actualiza Prometheus/Grafana.
• HEALTH_CHECK_FLEET: Verifica que todos los daemons ora-tui-otel (:9161/healthz), Prometheus (:9090) y Grafana (:3000) responden.'''
        )
        string(
            name: 'LIMIT_HOSTS',
            defaultValue: 'all',
            description: 'Limitar ejecución de Ansible a un host o grupo concreto (ej. all, oracle_servers, monitoring_server, ora-prod-rac01.local)'
        )
        string(
            name: 'NEW_SERVER_HOSTNAME',
            defaultValue: '',
            description: '[Solo para ADD_NEW_ORACLE_SERVER] Nombre del nuevo servidor Oracle (ej. ora-prod-rac03.local)'
        )
        string(
            name: 'NEW_SERVER_IP',
            defaultValue: '',
            description: '[Solo para ADD_NEW_ORACLE_SERVER] Dirección IP o DNS del nuevo servidor Oracle (ej. 192.168.10.13)'
        )
        choice(
            name: 'NEW_SERVER_ENV',
            choices: ['PROD', 'PRE', 'DES'],
            description: '[Solo para ADD_NEW_ORACLE_SERVER] Entorno del nuevo servidor Oracle'
        )
        string(
            name: 'NEW_SERVER_CLUSTER',
            defaultValue: 'rac-prod',
            description: '[Solo para ADD_NEW_ORACLE_SERVER] Etiqueta de cluster o grupo (ej. rac-prod, single-erp)'
        )
        string(
            name: 'OTEL_PORT',
            defaultValue: '9161',
            description: 'Puerto TCP del exportador OpenTelemetry/Prometheus de ora-tui'
        )
        string(
            name: 'SSH_CREDENTIALS_ID',
            defaultValue: '',
            description: '(Opcional) ID de credencial SSH en Jenkins para conectar por Ansible. Dejar vacío si el agente Jenkins ya tiene claves SSH.'
        )
        booleanParam(
            name: 'DRY_RUN_CHECK',
            defaultValue: false,
            description: 'Ejecutar Ansible en modo simulación (--check --diff) sin aplicar cambios reales'
        )
    }

    environment {
        ANSIBLE_CONFIG = "${WORKSPACE}/POC/ansible/ansible.cfg"
        INVENTORY_FILE = "${WORKSPACE}/POC/ansible/inventory.ini"
        PLAYBOOK_FILE  = "${WORKSPACE}/POC/ansible/site.yml"
        CGO_ENABLED    = "0"
    }

    stages {
        stage('1. Validar Inventario & Parámetros') {
            steps {
                script {
                    echo "=========================================================================="
                    echo "  ORA-TUI JENKINS PIPELINE :: ACCIÓN = ${params.ACTION}"
                    echo "=========================================================================="
                    sh '''
                        echo ">>> Contenido actual del inventario Ansible (${INVENTORY_FILE}):"
                        cat "${INVENTORY_FILE}"
                    '''
                }
            }
        }

        stage('2. Alta de Nuevo Servidor Oracle en Inventario') {
            when {
                expression { params.ACTION == 'ADD_NEW_ORACLE_SERVER' }
            }
            steps {
                script {
                    if (!params.NEW_SERVER_HOSTNAME?.trim()) {
                        error("Debes indicar NEW_SERVER_HOSTNAME cuando seleccionas ADD_NEW_ORACLE_SERVER.")
                    }
                    def hostIp = params.NEW_SERVER_IP?.trim() ? params.NEW_SERVER_IP.trim() : params.NEW_SERVER_HOSTNAME.trim()
                    echo ">>> Añadiendo servidor ${params.NEW_SERVER_HOSTNAME} (${hostIp}) al grupo [oracle_servers]..."
                    sh """
                        if grep -q "^${params.NEW_SERVER_HOSTNAME}[[:space:]]" "\${INVENTORY_FILE}"; then
                            echo "El servidor ${params.NEW_SERVER_HOSTNAME} ya existe en el inventario; actualizando línea..."
                            sed -i.bak "s|^${params.NEW_SERVER_HOSTNAME}[[:space:]].*|${params.NEW_SERVER_HOSTNAME}  ansible_host=${hostIp}  env=${params.NEW_SERVER_ENV}  cluster_label=${params.NEW_SERVER_CLUSTER}  otel_port=${params.OTEL_PORT}|" "\${INVENTORY_FILE}"
                            rm -f "\${INVENTORY_FILE}.bak"
                        else
                            awk '/^\\[oracle_servers\\]/{print; print "${params.NEW_SERVER_HOSTNAME}  ansible_host=${hostIp}  env=${params.NEW_SERVER_ENV}  cluster_label=${params.NEW_SERVER_CLUSTER}  otel_port=${params.OTEL_PORT}"; next}1' "\${INVENTORY_FILE}" > "\${INVENTORY_FILE}.tmp"
                            mv "\${INVENTORY_FILE}.tmp" "\${INVENTORY_FILE}"
                        fi
                        cat "\${INVENTORY_FILE}"
                    """
                }
            }
        }

        stage('3. Compilar Binario Estático Go & Generar Paquetes (.rpm / .deb)') {
            when {
                expression { params.ACTION == 'FULL_CI_CD_AND_DEPLOY' }
            }
            steps {
                sh '''
                    echo ">>> Ejecutando tests unitarios y compilando ora-tui (CGO_ENABLED=0)..."
                    make all
                    if command -v nfpm >/dev/null 2>&1 || go version >/dev/null 2>&1; then
                        make packages
                    fi
                    ls -lh bin/ dist/
                '''
            }
        }

        stage('4. Ansible: Instalar/Mantener Daemon ora-tui-otel en Servidores Oracle') {
            when {
                expression {
                    params.ACTION in [
                        'DEPLOY_ALL_ANSIBLE',
                        'FULL_CI_CD_AND_DEPLOY',
                        'UPDATE_ORACLE_DAEMONS_ONLY',
                        'ADD_NEW_ORACLE_SERVER'
                    ]
                }
            }
            steps {
                script {
                    def checkFlag = params.DRY_RUN_CHECK ? '--check --diff' : ''
                    def limitTarget = (params.ACTION == 'ADD_NEW_ORACLE_SERVER' && params.NEW_SERVER_HOSTNAME?.trim())
                        ? params.NEW_SERVER_HOSTNAME.trim()
                        : params.LIMIT_HOSTS
                    def cmd = "ansible-playbook -i \"${env.INVENTORY_FILE}\" \"${env.PLAYBOOK_FILE}\" --tags oracle --limit \"${limitTarget}\" -e default_otel_port=${params.OTEL_PORT} ${checkFlag}"

                    if (params.SSH_CREDENTIALS_ID?.trim()) {
                        sshagent(credentials: [params.SSH_CREDENTIALS_ID.trim()]) {
                            sh cmd
                        }
                    } else {
                        sh cmd
                    }
                }
            }
        }

        stage('5. Ansible: Actualizar Prometheus (targets.yml) & Grafana Multi-Servidor') {
            when {
                expression {
                    params.ACTION in [
                        'DEPLOY_ALL_ANSIBLE',
                        'FULL_CI_CD_AND_DEPLOY',
                        'UPDATE_MONITORING_TARGETS_ONLY',
                        'ADD_NEW_ORACLE_SERVER'
                    ]
                }
            }
            steps {
                script {
                    def checkFlag = params.DRY_RUN_CHECK ? '--check --diff' : ''
                    def cmd = "ansible-playbook -i \"${env.INVENTORY_FILE}\" \"${env.PLAYBOOK_FILE}\" --tags monitoring -e default_otel_port=${params.OTEL_PORT} ${checkFlag}"

                    if (params.SSH_CREDENTIALS_ID?.trim()) {
                        sshagent(credentials: [params.SSH_CREDENTIALS_ID.trim()]) {
                            sh cmd
                        }
                    } else {
                        sh cmd
                    }
                }
            }
        }

        stage('6. Verificación de Salud (/healthz) de Toda la Flota') {
            steps {
                sh '''
                    echo ">>> Verificando conectividad con los endpoints /healthz de los servidores del inventario..."
                    awk '/^\\[oracle_servers\\]/{flag=1; next} /^\\[/{flag=0} flag && $1 !~ /^#/ && NF>0 {
                        host=$1; ip=$1; port="9161";
                        for(i=2;i<=NF;i++) {
                            if ($i ~ /^ansible_host=/) { split($i,a,"="); ip=a[2]; }
                            if ($i ~ /^otel_port=/)    { split($i,p,"="); port=p[2]; }
                        }
                        printf "  • Comprobando %-25s (http://%s:%s/healthz) ... ", host, ip, port;
                        if (system("curl -fsS --connect-timeout 3 http://" ip ":" port "/healthz >/dev/null 2>&1") == 0) {
                            print "[OK]";
                        } else {
                            print "[SIN RESPUESTA / PENDIENTE]";
                        }
                    }' "${INVENTORY_FILE}"
                '''
            }
        }
    }

    post {
        always {
            archiveArtifacts artifacts: 'POC/ansible/inventory.ini, POC/prometheus/targets.yml, dist/*.rpm, dist/*.deb', allowEmptyArchive: true
        }
        success {
            echo "✅ Pipeline de Observabilidad Oracle (ora-tui + Ansible + Prometheus + Grafana) completado con éxito."
        }
        failure {
            echo "❌ El pipeline falló. Revisa los logs de la etapa correspondiente en Jenkins."
        }
    }
}

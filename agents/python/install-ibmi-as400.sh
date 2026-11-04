#!/QOpenSys/usr/bin/sh
# ==============================================================================
# ORA-TUI - Instalador Nativo y Configurador de Daemon para IBM i (AS/400 PASE)
# ==============================================================================
# Instala el agente en /QOpenSys/opt/ora-tui/ora_tui_agent.py, crea el comando
# interactivo /QOpenSys/usr/bin/ora-tui y somete el trabajo batch (SBMJOB)
# ORATUI en el subsistema QUSRNOMAX / QSYSWRK.
# ==============================================================================

set -e

PORT="9161"
INTERVAL="10"
INSTALL_DIR="/QOpenSys/opt/ora-tui"
LOG_FILE="/var/log/ora-tui-otel.log"

while [ $# -gt 0 ]; do
    case "$1" in
        --port=*) PORT="${1#*=}" ;;
        --interval=*) INTERVAL="${1#*=}" ;;
    esac
    shift
done

echo "======================================================================"
echo " [IBM i / AS400] Instalando ORA-TUI Python Agent (PASE Environment)   "
echo "======================================================================"

# 1. Localizar Python 3 en PASE (/QOpenSys/pkgs/bin/python3)
PYTHON_BIN=""
for candidate in /QOpenSys/pkgs/bin/python3 /QOpenSys/usr/bin/python3 /usr/bin/python3; do
    if [ -x "$candidate" ]; then
        PYTHON_BIN="$candidate"
        break
    fi
done

if [ -z "$PYTHON_BIN" ]; then
    echo "[ERROR] No se encontró python3 en /QOpenSys/pkgs/bin/python3."
    echo "        Instálalo en PASE con: /QOpenSys/pkgs/bin/yum install python3"
    exit 1
fi

echo "[INFO] Python 3 detectado en: $PYTHON_BIN ($($PYTHON_BIN --version 2>&1))"

# 2. Copiar ficheros a /QOpenSys/opt/ora-tui
SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
mkdir -p "$INSTALL_DIR"
mkdir -p /var/log
cp "$SCRIPT_DIR/ora_tui_agent.py" "$INSTALL_DIR/ora_tui_agent.py"
chmod 755 "$INSTALL_DIR/ora_tui_agent.py"

# 3. Crear wrapper CLI interactivo en /QOpenSys/usr/bin/ora-tui
mkdir -p /QOpenSys/usr/bin
cat <<EOF > /QOpenSys/usr/bin/ora-tui
#!/QOpenSys/usr/bin/sh
export PATH=/QOpenSys/pkgs/bin:/QOpenSys/usr/bin:/usr/bin
exec "$PYTHON_BIN" "$INSTALL_DIR/ora_tui_agent.py" --platform=ibmi "\$@"
EOF
chmod 755 /QOpenSys/usr/bin/ora-tui
echo "[OK] Comando interactivo instalado en /QOpenSys/usr/bin/ora-tui"

# 4. Crear script de arranque del daemon en PASE
cat <<EOF > "$INSTALL_DIR/start-daemon-pase.sh"
#!/QOpenSys/usr/bin/sh
export PATH=/QOpenSys/pkgs/bin:/QOpenSys/usr/bin:/usr/bin
export QIBM_MULTI_THREADED=Y
exec "$PYTHON_BIN" "$INSTALL_DIR/ora_tui_agent.py" --platform=ibmi --opentelemetry --daemon --otel-port=${PORT} --interval=${INTERVAL} >> "$LOG_FILE" 2>&1
EOF
chmod 755 "$INSTALL_DIR/start-daemon-pase.sh"

# 5. Someter el trabajo batch (SBMJOB) en el subsistema QUSRNOMAX
if [ -x /QOpenSys/usr/bin/system ]; then
    echo "[INFO] Finalizando trabajo ORATUI previo si estuviera activo..."
    /QOpenSys/usr/bin/system "ENDJOB JOB(ORATUI) OPTION(*IMMED) DUPJOBOPT(*MSG)" >/dev/null 2>&1 || true
    sleep 1
    echo "[INFO] Sometiendo trabajo batch ORATUI via SBMJOB en QSYS/QUSRNOMAX..."
    /QOpenSys/usr/bin/system "SBMJOB CMD(QSH CMD('${INSTALL_DIR}/start-daemon-pase.sh')) JOB(ORATUI) JOBD(QGPL/QDFTJOBD) JOBQ(QSYS/QUSRNOMAX) ALWMLTTHD(*YES)"
    echo "[OK] Trabajo ORATUI sometido correctamente."
else
    echo "[WARN] Utilidad /QOpenSys/usr/bin/system no encontrada (entorno fuera de PASE)."
    echo "       Iniciando proceso en background estándar..."
    nohup "$INSTALL_DIR/start-daemon-pase.sh" >/dev/null 2>&1 &
fi

echo ""
echo "======================================================================"
echo " [OK] Instalación completada en IBM i (AS/400)"
echo "======================================================================"
echo "  - Ver trabajo activo    : system \"WRKACTJOB JOB(ORATUI)\""
echo "  - Parar Daemon          : system \"ENDJOB JOB(ORATUI) OPTION(*CNTRLD) DELAY(10)\""
echo "  - Arrancar Daemon       : system \"SBMJOB CMD(QSH CMD('${INSTALL_DIR}/start-daemon-pase.sh')) JOB(ORATUI) JOBQ(QSYS/QUSRNOMAX) ALWMLTTHD(*YES)\""
echo "  - Arranque en IPL (Boot): ADDJOBSCDE o ADDAJE SBSD(QSYS/QSYSWRK) JOB(ORATUI) JOBD(QGPL/ORTUIJOBD)"
echo "  - Consola TUI           : /QOpenSys/usr/bin/ora-tui"
echo "  - Endpoint Prometheus   : http://$(hostname):${PORT}/metrics"
echo "======================================================================"

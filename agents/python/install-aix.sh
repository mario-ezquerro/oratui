#!/bin/ksh
# ==============================================================================
# ORA-TUI - Instalador Nativo y Configurador de Daemon para IBM AIX (Oracle RAC)
# ==============================================================================
# Registra ora-tui como comando interactivo (/usr/bin/ora-tui) y como subsistema
# nativo del System Resource Controller (SRC) de AIX (oratui) con arranque
# automático en /etc/inittab.
# ==============================================================================

set -e

PORT="9161"
INTERVAL="5"
INSTALL_DIR="/opt/ora-tui"
LOG_FILE="/var/log/ora-tui-otel.log"
SUBSYS_NAME="oratui"

while [ $# -gt 0 ]; do
    case "$1" in
        --port=*) PORT="${1#*=}" ;;
        --interval=*) INTERVAL="${1#*=}" ;;
    esac
    shift
done

echo "======================================================================"
echo " [AIX] Instalando ORA-TUI Python Agent (Oracle Database RAC + SAN)   "
echo "======================================================================"

if [ "$(id -u)" != "0" ]; then
    echo "[ERROR] Este script debe ejecutarse como root en IBM AIX."
    exit 1
fi

# 1. Localizar binario Python 3 en AIX (AIX Toolbox / /usr/bin / /opt/freeware/bin)
PYTHON_BIN=""
for candidate in /usr/bin/python3 /opt/freeware/bin/python3 /usr/local/bin/python3; do
    if [ -x "$candidate" ]; then
        PYTHON_BIN="$candidate"
        break
    fi
done

if [ -z "$PYTHON_BIN" ]; then
    echo "[ERROR] No se encontró python3 en /usr/bin/python3 ni en /opt/freeware/bin/python3."
    echo "        Instálalo desde AIX Toolbox (dnf install python3) e inténtalo de nuevo."
    exit 1
fi

echo "[INFO] Python 3 detectado en: $PYTHON_BIN ($($PYTHON_BIN --version 2>&1))"

# 2. Copiar agente a /opt/ora-tui
SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
mkdir -p "$INSTALL_DIR"
cp "$SCRIPT_DIR/ora_tui_agent.py" "$INSTALL_DIR/ora_tui_agent.py"
chmod 755 "$INSTALL_DIR/ora_tui_agent.py"

# 3. Crear wrapper CLI interactivo en /usr/bin/ora-tui
cat <<EOF > /usr/bin/ora-tui
#!/bin/ksh
exec "$PYTHON_BIN" "$INSTALL_DIR/ora_tui_agent.py" "\$@"
EOF
chmod 755 /usr/bin/ora-tui
echo "[OK] Comando interactivo instalado en /usr/bin/ora-tui"

# 4. Crear wrapper del daemon para AIX SRC (System Resource Controller)
#    SRC requiere un proceso foreground que responda a SIGTERM (15) y SIGKILL (9)
cat <<EOF > "$INSTALL_DIR/run-src-daemon.ksh"
#!/bin/ksh
export PATH=/usr/bin:/etc:/usr/sbin:/usr/ucb:/usr/bin/X11:/sbin:/opt/freeware/bin
export LANG=C
exec "$PYTHON_BIN" "$INSTALL_DIR/ora_tui_agent.py" --opentelemetry --daemon --otel-port=${PORT} --interval=${INTERVAL} >> "$LOG_FILE" 2>&1
EOF
chmod 755 "$INSTALL_DIR/run-src-daemon.ksh"

# 5. Configurar el subsistema nativo en AIX SRC (System Resource Controller)
echo "[INFO] Configurando subsistema AIX SRC ($SUBSYS_NAME)..."
if lssrc -s "$SUBSYS_NAME" >/dev/null 2>&1; then
    echo "[INFO] Deteniendo y actualizando subsistema SRC existente ($SUBSYS_NAME)..."
    stopsrc -s "$SUBSYS_NAME" >/dev/null 2>&1 || true
    sleep 1
    rmssys -s "$SUBSYS_NAME" >/dev/null 2>&1 || true
fi

# mkssys:
#   -s oratui       : Nombre del subsistema
#   -p <path>       : Ejecutable
#   -u 0            : Ejecutar como root (UID 0) para acceso a ASM, MPIO y su - oracle
#   -S              : Usa señales (Signals) para parada limpia
#   -n 15           : SIGTERM en stopsrc normal
#   -f 9            : SIGKILL en stopsrc -f
#   -R              : Reiniciar automáticamente si el proceso cae (Respawn)
#   -G tcpip        : Grupo de subsistemas tcpip
mkssys -s "$SUBSYS_NAME" \
       -p "$INSTALL_DIR/run-src-daemon.ksh" \
       -u 0 \
       -S -n 15 -f 9 -R \
       -G tcpip

# 6. Registrar arranque automático en /etc/inittab tras el nivel multiusuario
if lsitab "$SUBSYS_NAME" >/dev/null 2>&1; then
    rmitab "$SUBSYS_NAME" >/dev/null 2>&1 || true
fi
mkitab "${SUBSYS_NAME}:2:once:/usr/bin/startsrc -s ${SUBSYS_NAME} > /dev/console 2>&1"
echo "[OK] Arranque automático registrado en /etc/inittab (mkitab ${SUBSYS_NAME})"

# 7. Arrancar el daemon con startsrc
startsrc -s "$SUBSYS_NAME"
sleep 2

echo ""
echo "======================================================================"
echo " [OK] Instalación completada en IBM AIX"
echo "======================================================================"
echo "  - Estado del Daemon SRC : lssrc -s $SUBSYS_NAME"
echo "  - Parar Daemon          : stopsrc -s $SUBSYS_NAME"
echo "  - Arrancar Daemon       : startsrc -s $SUBSYS_NAME"
echo "  - Consola TUI           : ora-tui"
echo "  - Endpoint Prometheus   : http://$(hostname):${PORT}/metrics"
echo "  - Log del Agente        : $LOG_FILE"
echo "======================================================================"
lssrc -s "$SUBSYS_NAME" || true

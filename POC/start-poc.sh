#!/usr/bin/env bash
set -euo pipefail

cd "$(dirname "$0")"

# Leer el último target guardado en .env si existe
DEFAULT_HOST="servidor-oracle.local"
DEFAULT_PORT="9161"
if [[ -f .env ]]; then
  SAVED_TARGET=$(grep -E '^ORACLE_TARGET=' .env | cut -d'=' -f2- | tr -d '"' | tr -d "'" || true)
  if [[ -n "$SAVED_TARGET" ]]; then
    DEFAULT_HOST="${SAVED_TARGET%%:*}"
    if [[ "$SAVED_TARGET" == *:* ]]; then
      DEFAULT_PORT="${SAVED_TARGET##*:}"
    fi
  fi
fi

echo "=========================================================================="
echo "  CONFIGURACIÓN DE LA POC ORA-TUI (PROMETHEUS + GRAFANA)"
echo "=========================================================================="
echo ""

# Pedir siempre el nombre o IP del host a monitorizar cada vez que se ejecuta el script
read -r -p "👉 Introduce el nombre o IP del host Oracle a monitorizar [${DEFAULT_HOST}]: " INPUT_HOST
TARGET_HOST="${INPUT_HOST:-$DEFAULT_HOST}"

# Si el usuario ya introdujo host:puerto (ej. 192.168.1.50:9161), separamos host y puerto
if [[ "$TARGET_HOST" == *:* ]]; then
  TARGET_PORT="${TARGET_HOST##*:}"
  TARGET_HOST="${TARGET_HOST%%:*}"
else
  read -r -p "👉 Introduce el puerto OpenTelemetry de ora-tui [${DEFAULT_PORT}]: " INPUT_PORT
  TARGET_PORT="${INPUT_PORT:-$DEFAULT_PORT}"
fi

TARGET="${TARGET_HOST}:${TARGET_PORT}"

echo ""
echo ">>> Generando configuración de Prometheus para monitorizar: ${TARGET} ..."

cat > .env <<EOF
ORACLE_TARGET=${TARGET}
PROMETHEUS_PORT=9090
GRAFANA_PORT=3000
GRAFANA_ADMIN_USER=admin
GRAFANA_ADMIN_PASSWORD=admin
EOF

cat > prometheus/prometheus.yml <<EOF
# ==============================================================================
# POC Prometheus Configuration - Generado por configure-poc.sh / start-poc.sh
# Host monitorizado: ${TARGET}
# ==============================================================================
global:
  scrape_interval: 10s
  evaluation_interval: 10s
  external_labels:
    monitor: 'oratui-poc'
    monitored_host: '${TARGET_HOST}'

rule_files:
  - "/etc/prometheus/oracle_alerts.yml"

scrape_configs:
  - job_name: 'oracle-oratui-poc'
    scrape_interval: 10s
    scrape_timeout: 8s
    metrics_path: '/metrics'
    static_configs:
      - targets:
          - '${TARGET}'
        labels:
          application: 'ora-tui'
          oracle_host: '${TARGET_HOST}'
EOF

# Comprobar conectividad rápida hacia el exportador ora-tui del servidor Oracle
echo ">>> Verificando conexión con http://${TARGET}/healthz ..."
if curl -fsS --connect-timeout 3 "http://${TARGET}/healthz" >/dev/null 2>&1; then
  echo "    [OK] Conexión exitosa con ora-tui en ${TARGET}."
else
  echo "    [AVISO] No se pudo conectar todavía a http://${TARGET}/healthz."
  echo "            Asegúrate de que en '${TARGET_HOST}' está corriendo:"
  echo "            sudo ora-tui --opentelemetry --daemon --otel-port=${TARGET_PORT}"
  echo "            (y que el puerto ${TARGET_PORT}/tcp está abierto en el firewall)."
fi

echo ""
echo ">>> Levantando / recargando contenedores Docker (Prometheus + Grafana)..."
if docker compose version >/dev/null 2>&1; then
  docker compose up -d --force-recreate
elif command -v docker-compose >/dev/null 2>&1; then
  docker-compose up -d --force-recreate
else
  echo "[ERROR] No se encontró 'docker compose' ni 'docker-compose' en este servidor."
  exit 1
fi

SERVER_IP=$(hostname -I 2>/dev/null | awk '{print $1}' || echo "localhost")
if [[ -z "$SERVER_IP" ]]; then
  SERVER_IP="localhost"
fi

echo ""
echo "=========================================================================="
echo "  ✅ POC ORA-TUI ACTIVA Y MONITORIZANDO: ${TARGET}"
echo "=========================================================================="
echo "  • Host Oracle Monitorizado : ${TARGET}"
echo "  • Grafana (Dashboard POC)  : http://${SERVER_IP}:3000"
echo "    Usuario / Contraseña     : admin / admin"
echo "  • Prometheus (Targets)     : http://${SERVER_IP}:9090/targets"
echo "  • Prometheus (15 Alertas)  : http://${SERVER_IP}:9090/alerts"
echo "=========================================================================="

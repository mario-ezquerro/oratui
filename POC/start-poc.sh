#!/usr/bin/env bash
set -euo pipefail

cd "$(dirname "$0")"

mkdir -p prometheus flutter-web/build/web flutter-web/web

DEFAULT_HOST="servidor-oracle.local"
DEFAULT_PORT="9161"
DEFAULT_ENV="PROD"

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
echo "  CONFIGURACIÓN INTERACTIVA MULTI-SERVIDOR POC ORA-TUI"
echo "  (Flutter Web Control & SRE Center + Prometheus + Grafana)"
echo "=========================================================================="
echo ""
echo "Servidores configurados actualmente en prometheus/targets.yml:"
if [[ -f prometheus/targets.yml ]]; then
  grep -E '^\s*-\s*"' prometheus/targets.yml || echo "  (ninguno)"
else
  echo "  (ninguno)"
fi
echo ""
echo "Modos de configuración:"
echo "  [1] Añadir o configurar un servidor Oracle interactivamente (por defecto)"
echo "  [2] Reemplazar toda la lista con un único servidor Oracle"
echo "  [3] Sincronizar la lista de servidores desde el inventario Ansible (ansible/inventory.ini)"
echo "  [4] Ejecutar el playbook completo de Ansible (instalar daemons + actualizar contenedores)"
read -r -p "👉 Selecciona una opción [1]: " MODE_CHOICE
MODE_CHOICE="${MODE_CHOICE:-1}"

if [[ "$MODE_CHOICE" == "4" ]]; then
  echo ">>> Ejecutando ansible-playbook -i ansible/inventory.ini ansible/site.yml ..."
  ansible-playbook -i ansible/inventory.ini ansible/site.yml
  exit 0
elif [[ "$MODE_CHOICE" == "3" ]]; then
  echo ">>> Generando prometheus/targets.yml a partir de [oracle_servers] en ansible/inventory.ini ..."
  awk '/^\[oracle_servers\]/{flag=1; next} /^\[/{flag=0} flag && $1 !~ /^#/ && NF>0 {
    host=$1; ip=$1; port="9161"; env="PROD"; cluster="oracle-db";
    for(i=2;i<=NF;i++) {
      if ($i ~ /^ansible_host=/)  { split($i,a,"="); ip=a[2]; }
      if ($i ~ /^otel_port=/)     { split($i,p,"="); port=p[2]; }
      if ($i ~ /^env=/)           { split($i,e,"="); env=e[2]; }
      if ($i ~ /^cluster_label=/) { split($i,c,"="); cluster=c[2]; }
    }
    printf "- targets:\n    - \"%s:%s\"\n  labels:\n    oracle_host: \"%s\"\n    env: \"%s\"\n    cluster_group: \"%s\"\n    application: \"ora-tui\"\n", ip, port, host, env, cluster;
  }' ansible/inventory.ini > prometheus/targets.yml
  echo ">>> Fichero prometheus/targets.yml actualizado desde ansible/inventory.ini:"
  cat prometheus/targets.yml
else
  # Modo 1 o 2: Pedir siempre el nombre o IP del host a monitorizar
  read -r -p "👉 Introduce el nombre o IP del host Oracle a monitorizar [${DEFAULT_HOST}]: " INPUT_HOST
  TARGET_HOST="${INPUT_HOST:-$DEFAULT_HOST}"

  if [[ "$TARGET_HOST" == *:* ]]; then
    TARGET_PORT="${TARGET_HOST##*:}"
    TARGET_HOST="${TARGET_HOST%%:*}"
  else
    read -r -p "👉 Introduce el puerto OpenTelemetry de ora-tui [${DEFAULT_PORT}]: " INPUT_PORT
    TARGET_PORT="${INPUT_PORT:-$DEFAULT_PORT}"
  fi

  read -r -p "👉 Introduce el entorno (PROD / PRE / DES) [${DEFAULT_ENV}]: " INPUT_ENV
  TARGET_ENV="${INPUT_ENV:-$DEFAULT_ENV}"

  TARGET="${TARGET_HOST}:${TARGET_PORT}"

  cat > .env <<EOF
ORACLE_TARGET=${TARGET}
PROMETHEUS_PORT=9090
GRAFANA_PORT=3000
FLUTTER_WEB_PORT=8088
GRAFANA_ADMIN_USER=admin
GRAFANA_ADMIN_PASSWORD=admin
FLUTTER_WEB_USER=admin
FLUTTER_WEB_PASSWORD=admin
ORATUI_API_TOKEN=
EOF

  if [[ "$MODE_CHOICE" == "2" ]] || [[ ! -f prometheus/targets.yml ]]; then
    cat > prometheus/targets.yml <<EOF
# Listado dinámico de servidores Oracle (file_sd_configs)
- targets:
    - "${TARGET}"
  labels:
    oracle_host: "${TARGET_HOST}"
    env: "${TARGET_ENV}"
    cluster_group: "oracle-db"
    application: "ora-tui"
EOF
  else
    # Si ya estaba ese target en targets.yml no lo duplicamos; si no, lo añadimos
    if grep -q "\"${TARGET}\"" prometheus/targets.yml 2>/dev/null; then
      echo ">>> El servidor ${TARGET} ya estaba en prometheus/targets.yml."
    else
      cat >> prometheus/targets.yml <<EOF
- targets:
    - "${TARGET}"
  labels:
    oracle_host: "${TARGET_HOST}"
    env: "${TARGET_ENV}"
    cluster_group: "oracle-db"
    application: "ora-tui"
EOF
      echo ">>> Servidor ${TARGET} (${TARGET_ENV}) añadido a prometheus/targets.yml."
    fi
  fi

  echo ">>> Verificando conexión con http://${TARGET}/healthz ..."
  if curl -fsS --connect-timeout 3 "http://${TARGET}/healthz" >/dev/null 2>&1; then
    echo "    [OK] Conexión exitosa con ora-tui en ${TARGET}."
  else
    echo "    [AVISO] No se pudo conectar todavía a http://${TARGET}/healthz."
    echo "            Asegúrate de que en '${TARGET_HOST}' está corriendo:"
    echo "            sudo ora-tui --opentelemetry --daemon --otel-port=${TARGET_PORT}"
  fi
fi

# Sincronizar automáticamente flutter-web/build/web/servers.json desde prometheus/targets.yml
if [[ -f prometheus/targets.yml ]]; then
  awk '
    BEGIN {
      print "{\n  \"default_port\": 9161,\n  \"servers\": ["
      count = 0
    }
    /- "[0-9a-zA-Z._-]+:[0-9]+"/ {
      gsub(/.*- "/, ""); gsub(/".*/, "");
      target = $0
    }
    /oracle_host:/ {
      gsub(/.*oracle_host: "/, ""); gsub(/".*/, "");
      host = $0
    }
    /env:/ {
      gsub(/.*env: "/, ""); gsub(/".*/, "");
      env = $0
      if (count > 0) printf ",\n"
      printf "    {\n      \"name\": \"%s (%s)\",\n      \"url\": \"http://%s\",\n      \"environment\": \"%s\",\n      \"site\": \"oracle-fleet\"\n    }", host, target, target, env
      count++
    }
    END {
      print "\n  ]\n}"
    }
  ' prometheus/targets.yml > flutter-web/build/web/servers.json
  cp flutter-web/build/web/servers.json flutter-web/web/servers.json 2>/dev/null || true
  echo ">>> Catálogo multi-servidor de Flutter Web (servers.json) sincronizado."
fi

echo ""
echo ">>> Levantando contenedores Docker (Flutter Web Command & SRE Center + Prometheus + Grafana)..."
if docker compose version >/dev/null 2>&1; then
  docker compose up -d --build
elif command -v docker-compose >/dev/null 2>&1; then
  docker-compose up -d --build
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
echo "  ✅ POC MULTI-SERVIDOR ORA-TUI ACTIVA"
echo "=========================================================================="
echo "  • 🚀 Flutter Web Command & SRE Center : http://${SERVER_IP}:8088"
echo "    Usuario / Contraseña (.env)         : admin / admin"
echo "  • 📊 Grafana (01. Vista Global Flota) : http://${SERVER_IP}:3000"
echo "  • 📊 Grafana (02. Detalle 26 Paneles) : http://${SERVER_IP}:3000/d/oratui-full-observability-poc"
echo "    Usuario / Contraseña (.env)         : admin / admin"
echo "  • 🔥 Prometheus (Targets Dinámicos)   : http://${SERVER_IP}:9090/targets"
echo "  • 🚨 Prometheus (15 Alertas)          : http://${SERVER_IP}:9090/alerts"
echo "=========================================================================="

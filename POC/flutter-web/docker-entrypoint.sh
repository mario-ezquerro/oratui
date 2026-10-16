#!/bin/sh
set -e

WEB_USER="${FLUTTER_WEB_USER:-admin}"
WEB_PASS="${FLUTTER_WEB_PASSWORD:-admin}"
API_TOKEN="${ORATUI_API_TOKEN:-}"

cat > /usr/share/nginx/html/auth-config.json <<EOF
{
  "auth_required": true,
  "username": "${WEB_USER}",
  "password": "${WEB_PASS}",
  "api_token": "${API_TOKEN}"
}
EOF

exec nginx -g "daemon off;"

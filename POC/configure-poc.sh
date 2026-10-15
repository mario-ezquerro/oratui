#!/usr/bin/env bash
set -euo pipefail

# Ejecuta el script interactivo de configuración y arranque de la POC
exec "$(dirname "$0")/start-poc.sh" "$@"

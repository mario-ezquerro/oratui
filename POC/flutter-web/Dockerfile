# ==============================================================================
# ORA-TUI Flutter Web Command & SRE Center — Contenedor Docker
# Muestra todas las gráficas de los servidores Oracle, ejecuta acciones remotas
# sobre BD/SO/SAN y gestiona SLOs + Alertas Críticas por Telegram y Email SMTP.
# ==============================================================================
FROM nginx:alpine

LABEL maintainer="ORA-TUI Project"
LABEL description="ORA-TUI Flutter Web Command & SRE Center (Reemplazo interactivo de Grafana con remediación remota y alertas Telegram/SMTP)"

COPY nginx.conf /etc/nginx/conf.d/default.conf
COPY build/web /usr/share/nginx/html

EXPOSE 80

CMD ["nginx", "-g", "daemon off;"]

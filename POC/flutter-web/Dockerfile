# ==============================================================================
# ORA-TUI Flutter Web Command & SRE Center — Contenedor Docker Blindado
# Muestra todas las gráficas de los servidores Oracle, ejecuta acciones remotas
# autenticadas sobre BD/SO/SAN y gestiona SLOs + Alertas por Telegram y Email.
# ==============================================================================
FROM nginx:alpine

LABEL maintainer="ORA-TUI Project"
LABEL description="ORA-TUI Flutter Web Command & SRE Center (Con Login Usuario/Contraseña desde .env, Gráficas en vivo, Acciones Remotas y SRE/SLOs)"

COPY nginx.conf /etc/nginx/conf.d/default.conf
COPY build/web /usr/share/nginx/html
COPY docker-entrypoint.sh /docker-entrypoint.sh
RUN chmod +x /docker-entrypoint.sh

EXPOSE 80

ENTRYPOINT ["/docker-entrypoint.sh"]

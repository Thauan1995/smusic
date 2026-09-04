#!/usr/bin/env bash
# Opção A: sobe o backend completo do smusic (Postgres, Redis, cmd/server,
# cmd/presence-server) localmente, popula um catálogo mínimo de teste, e
# builda + serve o app web para você testar no navegador.
#
# Uso:
#   ./scripts/run_local.sh up      # sobe tudo e abre o navegador (padrão)
#   ./scripts/run_local.sh down    # para os processos Go/proxy e os containers
#   ./scripts/run_local.sh status  # mostra o que está rodando
#
# Depois de rodar "up", a página web fica servida em foreground em
# http://localhost:5173 - Ctrl+C encerra só o servidor web; o backend
# continua rodando (rode "down" quando quiser parar tudo de vez, ou "up"
# de novo mais tarde para só reabrir o navegador sem reconstruir nada).
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
# shellcheck source=lib/common.sh
source "$SCRIPT_DIR/lib/common.sh"
# shellcheck source=lib/backend.sh
source "$SCRIPT_DIR/lib/backend.sh"

SMUSIC_WEB_PORT="${SMUSIC_WEB_PORT:-5173}"
SMUSIC_WEB_PIDFILE="$SMUSIC_RUN_DIR/web-server.pid"

cmd_up() {
  smusic_ensure_toolchains_on_path

  smusic_log "== Backend =="
  smusic_backend_up "localhost"
  smusic_backend_seed_demo_catalog

  smusic_log "== Frontend (web) =="
  (cd "$SMUSIC_ROOT/frontend" && melos bootstrap)

  smusic_log "Compilando o app web (aponta para http://localhost:$SMUSIC_PROXY_PORT)..."
  (
    cd "$SMUSIC_ROOT/frontend/app/smusic_web"
    flutter build web --dart-define=SMUSIC_API_BASE_URL="http://localhost:$SMUSIC_PROXY_PORT"
  )

  smusic_log ""
  smusic_log "Tudo pronto:"
  smusic_log "  API + WebSocket de presença (unificados): http://localhost:$SMUSIC_PROXY_PORT"
  smusic_log "  App web:                                  http://localhost:$SMUSIC_WEB_PORT"
  smusic_log ""
  smusic_log "Crie sua conta pelo próprio app (tela de signup) - já existe 1 faixa de teste no catálogo."
  smusic_log "Ctrl+C para fechar só o servidor web (o backend continua rodando)."
  smusic_log "Rode './scripts/run_local.sh down' para parar tudo."
  smusic_log ""

  if command -v xdg-open >/dev/null 2>&1; then
    (sleep 1 && xdg-open "http://localhost:$SMUSIC_WEB_PORT" >/dev/null 2>&1 &) || true
  fi

  cd "$SMUSIC_ROOT/frontend/app/smusic_web/build/web"
  echo $$ > "$SMUSIC_WEB_PIDFILE"
  exec python3 -m http.server "$SMUSIC_WEB_PORT"
}

cmd_down() {
  smusic_stop_pidfile "$SMUSIC_WEB_PIDFILE" "servidor web"
  smusic_backend_down
  smusic_log "Tudo parado."
}

cmd_status() {
  for entry in "cmd/server:$SMUSIC_SERVER_PIDFILE" "cmd/presence-server:$SMUSIC_PRESENCE_PIDFILE" "proxy local:$SMUSIC_PROXY_PIDFILE" "servidor web:$SMUSIC_WEB_PIDFILE"; do
    local label="${entry%%:*}" pidfile="${entry##*:}"
    if smusic_pid_alive "$pidfile"; then
      smusic_log "$label: rodando (pid $(cat "$pidfile"))"
    else
      smusic_log "$label: parado"
    fi
  done
  docker ps --filter "name=smusic-local" --format '  container {{.Names}}: {{.Status}}' || true
}

case "${1:-up}" in
  up) cmd_up ;;
  down) cmd_down ;;
  status) cmd_status ;;
  *) echo "Uso: $0 [up|down|status]"; exit 1 ;;
esac

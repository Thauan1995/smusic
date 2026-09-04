#!/usr/bin/env bash
# Backend lifecycle helpers shared by run_local.sh and build_android_demo.sh.
# Source scripts/lib/common.sh before this file.

SMUSIC_PG_CONTAINER="smusic-local-pg"
SMUSIC_REDIS_CONTAINER="smusic-local-redis"
SMUSIC_REST_PORT=8080
SMUSIC_PRESENCE_PORT=8081
SMUSIC_PROXY_PORT="${SMUSIC_PROXY_PORT:-8090}"
SMUSIC_WEB_ORIGIN="${SMUSIC_WEB_ORIGIN:-http://localhost:5173}"

SMUSIC_SERVER_PIDFILE="$SMUSIC_RUN_DIR/server.pid"
SMUSIC_PRESENCE_PIDFILE="$SMUSIC_RUN_DIR/presence-server.pid"
SMUSIC_PROXY_PIDFILE="$SMUSIC_RUN_DIR/path-proxy.pid"
SMUSIC_SEED_MARKER="$SMUSIC_RUN_DIR/seeded"

# Writes backend/.env if it doesn't exist yet, filling in real generated
# secrets (both cmd/server and cmd/presence-server read the same .env, so
# generating JWT_ED25519_SEED_HEX once here is what makes tokens minted by
# one process valid on the other - see backend/README.md).
smusic_backend_ensure_env() {
  local env_file="$SMUSIC_ROOT/backend/.env"
  if [ -f "$env_file" ]; then
    smusic_log "backend/.env já existe, mantendo como está."
    return
  fi
  smusic_log "Gerando backend/.env a partir do .env.example..."
  cp "$SMUSIC_ROOT/backend/.env.example" "$env_file"

  local jwt_seed pepper
  jwt_seed="$(openssl rand -hex 32)"
  pepper="$(openssl rand -hex 32)"

  # Portable in-place sed (GNU and BSD sed both accept this form with -i.bak).
  sed -i.bak \
    -e "s#^JWT_ED25519_SEED_HEX=.*#JWT_ED25519_SEED_HEX=$jwt_seed#" \
    -e "s#^PASSWORD_PEPPER_HEX=.*#PASSWORD_PEPPER_HEX=$pepper#" \
    "$env_file"
  rm -f "$env_file.bak"

  {
    echo ""
    echo "# Added by scripts/lib/backend.sh for local testing."
    echo "CORS_ALLOWED_ORIGINS=$SMUSIC_WEB_ORIGIN"
  } >> "$env_file"

  smusic_log "backend/.env criado com chaves geradas e CORS_ALLOWED_ORIGINS=$SMUSIC_WEB_ORIGIN."
}

# Starts Postgres + Redis. Tries docker compose first; falls back to plain
# `docker run` with named, reusable containers if compose isn't usable in
# this environment (a documented issue in some sandboxes, see
# backend/README.md).
smusic_backend_start_infra() {
  if docker ps --format '{{.Names}}' | grep -qx "$SMUSIC_PG_CONTAINER" \
    && docker ps --format '{{.Names}}' | grep -qx "$SMUSIC_REDIS_CONTAINER"; then
    smusic_log "Postgres/Redis já estão rodando (containers $SMUSIC_PG_CONTAINER/$SMUSIC_REDIS_CONTAINER)."
    return
  fi

  # Reuse stopped containers from a previous run if present.
  if docker ps -a --format '{{.Names}}' | grep -qx "$SMUSIC_PG_CONTAINER"; then
    smusic_log "Reiniciando containers existentes..."
    docker start "$SMUSIC_PG_CONTAINER" "$SMUSIC_REDIS_CONTAINER" >/dev/null
    return
  fi

  smusic_log "Tentando 'docker compose up -d'..."
  if (cd "$SMUSIC_ROOT/backend" && docker compose up -d) 2>/dev/null; then
    # docker-compose.yml's own container names may differ from ours; if it
    # worked, trust it and skip the docker-run fallback entirely.
    if docker ps --format '{{.Names}}' | grep -qE '(postgres|redis)'; then
      smusic_log "Postgres/Redis subiram via docker compose."
      return
    fi
  fi

  smusic_warn "docker compose indisponível/falhou neste ambiente - subindo Postgres/Redis via 'docker run' (mesmo workaround documentado em backend/README.md)."
  docker run -d --name "$SMUSIC_PG_CONTAINER" \
    -e POSTGRES_USER=smusic -e POSTGRES_PASSWORD=smusic -e POSTGRES_DB=smusic \
    -p 5432:5432 postgres:16 >/dev/null
  docker run -d --name "$SMUSIC_REDIS_CONTAINER" \
    -p 6379:6379 redis:7 >/dev/null
}

smusic_backend_wait_infra() {
  smusic_wait_for_tcp 127.0.0.1 5432 60 "Postgres"
  smusic_wait_for_tcp 127.0.0.1 6379 30 "Redis"
  # Postgres' TCP port can accept connections slightly before it's ready
  # to authenticate; give it a moment and let migrations retry via Go's
  # own connection retry if this isn't quite enough on a slow machine.
  sleep 2
}

smusic_backend_run_migrations() {
  smusic_log "Rodando migrations..."
  (cd "$SMUSIC_ROOT/backend" && go run ./cmd/migrate up)
}

# smusic_backend_start_server <bind_host>
# bind_host: "localhost" for Option A (local-only), "0.0.0.0" for Option B
# (reachable from a phone on the same LAN).
smusic_backend_start_server() {
  local bind_host="${1:-localhost}"
  if smusic_pid_alive "$SMUSIC_SERVER_PIDFILE"; then
    smusic_log "cmd/server já está rodando."
  else
    smusic_log "Subindo cmd/server em $bind_host:$SMUSIC_REST_PORT..."
    (
      cd "$SMUSIC_ROOT/backend"
      set -a
      # shellcheck disable=SC1091
      source .env
      set +a
      export HTTP_ADDR="$bind_host:$SMUSIC_REST_PORT"
      nohup go run ./cmd/server >"$SMUSIC_RUN_DIR/server.log" 2>&1 &
      echo $! > "$SMUSIC_SERVER_PIDFILE"
    )
  fi
  smusic_wait_for_http_ok "http://127.0.0.1:$SMUSIC_REST_PORT/healthz" 120 "cmd/server"
}

smusic_backend_start_presence_server() {
  local bind_host="${1:-localhost}"
  if smusic_pid_alive "$SMUSIC_PRESENCE_PIDFILE"; then
    smusic_log "cmd/presence-server já está rodando."
  else
    smusic_log "Subindo cmd/presence-server em $bind_host:$SMUSIC_PRESENCE_PORT..."
    (
      cd "$SMUSIC_ROOT/backend"
      set -a
      # shellcheck disable=SC1091
      source .env
      set +a
      export PRESENCE_HTTP_ADDR="$bind_host:$SMUSIC_PRESENCE_PORT"
      nohup go run ./cmd/presence-server >"$SMUSIC_RUN_DIR/presence-server.log" 2>&1 &
      echo $! > "$SMUSIC_PRESENCE_PIDFILE"
    )
  fi
  smusic_wait_for_tcp 127.0.0.1 "$SMUSIC_PRESENCE_PORT" 120 "cmd/presence-server"
}

# smusic_backend_start_proxy <bind_host>
# Unifies REST (8080) and presence WS (8081) behind one port, since the
# Flutter clients derive the presence URL from the same host:port as the
# REST base URL (see app/smusic_{mobile,web}/lib/main.dart's
# buildPresenceUri) - without this, the app can reach one or the other but
# never both.
smusic_backend_start_proxy() {
  local bind_host="${1:-localhost}"
  if smusic_pid_alive "$SMUSIC_PROXY_PIDFILE"; then
    smusic_log "Proxy local já está rodando na porta $SMUSIC_PROXY_PORT."
    return
  fi
  smusic_log "Subindo proxy local (unifica REST + presence numa porta só) em $bind_host:$SMUSIC_PROXY_PORT..."
  nohup python3 "$SMUSIC_ROOT/scripts/tools/path_proxy.py" \
    "$SMUSIC_PROXY_PORT" "$SMUSIC_REST_PORT" "$SMUSIC_PRESENCE_PORT" "$bind_host" \
    >"$SMUSIC_RUN_DIR/path-proxy.log" 2>&1 &
  echo $! > "$SMUSIC_PROXY_PIDFILE"
  smusic_wait_for_http_ok "http://127.0.0.1:$SMUSIC_PROXY_PORT/healthz" 20 "proxy local"
}

# smusic_backend_up <bind_host>
# Brings up infra + both Go processes + the unifying proxy. Idempotent -
# safe to call from both scripts, safe to call more than once.
smusic_backend_up() {
  local bind_host="${1:-localhost}"
  smusic_backend_ensure_env
  smusic_backend_start_infra
  smusic_backend_wait_infra
  smusic_backend_run_migrations
  smusic_backend_start_server "$bind_host"
  smusic_backend_start_presence_server "$bind_host"
  smusic_backend_start_proxy "$bind_host"
}

# Creates one demo artist + track through the real REST API (not direct
# SQL) so there's something to see/play immediately. Idempotent: skips if
# already seeded in a previous run of these scripts.
smusic_backend_seed_demo_catalog() {
  local base_url="http://127.0.0.1:$SMUSIC_PROXY_PORT"
  if [ -f "$SMUSIC_SEED_MARKER" ]; then
    smusic_log "Catálogo de teste já foi semeado antes (veja $SMUSIC_SEED_MARKER); pulando."
    return
  fi

  smusic_log "Criando um usuário e uma faixa de teste via API..."
  local email="seed-$(date +%s)@smusic.local"
  local signup_resp access_token artist_id
  signup_resp="$(curl -fsS -X POST "$base_url/v1/auth/signup" \
    -H 'Content-Type: application/json' \
    -d "{\"email\":\"$email\",\"password\":\"SuperSecret123\",\"display_name\":\"Seed User\"}")" \
    || { smusic_warn "Não consegui criar o usuário de seed (a API pode ainda não estar pronta). Você pode criar conteúdo manualmente pela API depois."; return; }

  access_token="$(python3 -c 'import json,sys; print(json.load(sys.stdin)["access_token"])' <<<"$signup_resp" 2>/dev/null || true)"
  if [ -z "$access_token" ]; then
    smusic_warn "Signup de seed não retornou access_token; pulando criação de catálogo."
    return
  fi

  artist_id="$(curl -fsS -X POST "$base_url/v1/catalog/artists" \
    -H "Authorization: Bearer $access_token" -H 'Content-Type: application/json' \
    -d '{"name":"smusic Demo Artist"}' \
    | python3 -c 'import json,sys; print(json.load(sys.stdin)["id"])' 2>/dev/null || true)"

  if [ -z "$artist_id" ]; then
    smusic_warn "Não consegui criar o artista de demonstração; pulando."
    return
  fi

  curl -fsS -X POST "$base_url/v1/catalog/tracks" \
    -H "Authorization: Bearer $access_token" -H 'Content-Type: application/json' \
    -d "{\"title\":\"smusic Demo Track\",\"duration_ms\":180000,\"artists\":[{\"artist_id\":\"$artist_id\",\"role\":\"primary\"}]}" \
    >/dev/null || smusic_warn "Não consegui criar a faixa de demonstração."

  touch "$SMUSIC_SEED_MARKER"
  smusic_log "Catálogo de teste criado (artista + 1 faixa). Crie sua própria conta no app para testar - o catálogo é compartilhado entre contas."
}

smusic_backend_down() {
  smusic_stop_pidfile "$SMUSIC_PROXY_PIDFILE" "proxy local"
  smusic_stop_pidfile "$SMUSIC_PRESENCE_PIDFILE" "cmd/presence-server"
  smusic_stop_pidfile "$SMUSIC_SERVER_PIDFILE" "cmd/server"
  if docker ps --format '{{.Names}}' | grep -qE "^($SMUSIC_PG_CONTAINER|$SMUSIC_REDIS_CONTAINER)\$"; then
    smusic_log "Parando containers Postgres/Redis (docker stop, não remove - dados ficam preservados para a próxima vez)..."
    docker stop "$SMUSIC_PG_CONTAINER" "$SMUSIC_REDIS_CONTAINER" >/dev/null 2>&1 || true
  fi
  (cd "$SMUSIC_ROOT/backend" && docker compose down 2>/dev/null) || true
}

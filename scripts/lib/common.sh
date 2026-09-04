#!/usr/bin/env bash
# Shared helpers for the smusic local-run scripts. Not meant to be run
# directly - sourced by scripts/run_local.sh and scripts/build_android_demo.sh.

# Resolve the repo root regardless of where this file is sourced from.
SMUSIC_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
SMUSIC_RUN_DIR="$SMUSIC_ROOT/.local-run"
mkdir -p "$SMUSIC_RUN_DIR"

smusic_log() {
  printf '\033[1;36m[smusic]\033[0m %s\n' "$*"
}

smusic_warn() {
  printf '\033[1;33m[smusic]\033[0m %s\n' "$*" >&2
}

smusic_die() {
  printf '\033[1;31m[smusic]\033[0m %s\n' "$*" >&2
  exit 1
}

# Adds common Go/Flutter/pub-cache install locations to PATH if the
# binaries aren't already reachable. Safe to call multiple times.
smusic_ensure_toolchains_on_path() {
  if ! command -v go >/dev/null 2>&1; then
    for candidate in /usr/local/go/bin "$HOME/go/bin" "$HOME/.local/go/bin"; do
      [ -x "$candidate/go" ] && export PATH="$PATH:$candidate"
    done
  fi
  if ! command -v flutter >/dev/null 2>&1; then
    for candidate in "$HOME/sdks/flutter/bin" "$HOME/flutter/bin" "$HOME/development/flutter/bin"; do
      [ -x "$candidate/flutter" ] && export PATH="$PATH:$candidate"
    done
  fi
  export PATH="$PATH:$HOME/.pub-cache/bin"

  command -v go >/dev/null 2>&1 || smusic_die "Go não encontrado no PATH. Instale o Go ou ajuste smusic_ensure_toolchains_on_path() em scripts/lib/common.sh com o caminho certo."
  command -v flutter >/dev/null 2>&1 || smusic_die "Flutter não encontrado no PATH. Instale o Flutter SDK ou ajuste smusic_ensure_toolchains_on_path() em scripts/lib/common.sh com o caminho certo."
  command -v docker >/dev/null 2>&1 || smusic_die "Docker não encontrado. É necessário para subir Postgres/Redis."
  command -v python3 >/dev/null 2>&1 || smusic_die "python3 não encontrado. É necessário para o proxy local e para servir o build web."
}

# smusic_wait_for_tcp <host> <port> <timeout_seconds> <label>
smusic_wait_for_tcp() {
  local host="$1" port="$2" timeout="$3" label="${4:-$1:$2}"
  local waited=0
  smusic_log "Aguardando $label ficar disponível..."
  while ! (exec 3<>"/dev/tcp/$host/$port") 2>/dev/null; do
    sleep 1
    waited=$((waited + 1))
    if [ "$waited" -ge "$timeout" ]; then
      smusic_die "$label não respondeu em ${timeout}s."
    fi
  done
  exec 3>&- 2>/dev/null || true
  smusic_log "$label disponível."
}

# smusic_wait_for_http_ok <url> <timeout_seconds> <label>
smusic_wait_for_http_ok() {
  local url="$1" timeout="$2" label="${3:-$1}"
  local waited=0
  smusic_log "Aguardando $label responder..."
  while ! curl -fsS -o /dev/null "$url" 2>/dev/null; do
    sleep 1
    waited=$((waited + 1))
    if [ "$waited" -ge "$timeout" ]; then
      smusic_die "$label não respondeu em ${timeout}s (url: $url)."
    fi
  done
  smusic_log "$label OK."
}

# smusic_pid_alive <pidfile>
smusic_pid_alive() {
  local pidfile="$1"
  [ -f "$pidfile" ] || return 1
  local pid
  pid="$(cat "$pidfile" 2>/dev/null || true)"
  [ -n "$pid" ] && kill -0 "$pid" 2>/dev/null
}

# smusic_stop_pidfile <pidfile> <label>
smusic_stop_pidfile() {
  local pidfile="$1" label="$2"
  if smusic_pid_alive "$pidfile"; then
    local pid
    pid="$(cat "$pidfile")"
    smusic_log "Parando $label (pid $pid)..."
    kill "$pid" 2>/dev/null || true
    for _ in 1 2 3 4 5; do
      kill -0 "$pid" 2>/dev/null || break
      sleep 1
    done
    kill -9 "$pid" 2>/dev/null || true
  fi
  rm -f "$pidfile"
}

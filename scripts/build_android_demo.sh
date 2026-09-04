#!/usr/bin/env bash
# Opção B: sobe o mesmo backend do smusic, mas acessível pela rede local
# (não só localhost), builda um APK debug do app Android apontando para o
# IP do seu computador, e instala automaticamente no celular se ele
# estiver conectado via USB com depuração ativada (adb).
#
# Uso:
#   ./scripts/build_android_demo.sh              # detecta o IP da rede sozinho
#   ./scripts/build_android_demo.sh --ip=192.168.1.50   # força um IP específico
#
# Pré-requisitos:
#   - Flutter com toolchain Android configurada (`flutter doctor`).
#   - Celular e computador na MESMA rede Wi-Fi.
#   - Porta escolhida (padrão 8090) liberada no firewall do computador.
#   - Para instalação automática: celular conectado por USB com "Depuração
#     USB" ativada (Opções do desenvolvedor) e `adb devices` mostrando o
#     aparelho. Sem isso, o script só gera o .apk para você transferir na mão.
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
# shellcheck source=lib/common.sh
source "$SCRIPT_DIR/lib/common.sh"
# shellcheck source=lib/backend.sh
source "$SCRIPT_DIR/lib/backend.sh"

LAN_IP=""
for arg in "$@"; do
  case "$arg" in
    --ip=*) LAN_IP="${arg#--ip=}" ;;
    *) smusic_die "Argumento desconhecido: $arg (uso: $0 [--ip=192.168.x.x])" ;;
  esac
done

smusic_detect_lan_ip() {
  # Tenta alguns métodos comuns em Linux; o usuário pode sempre sobrescrever
  # com --ip= se a detecção pegar a interface errada (ex: VPN, Docker bridge).
  local ip=""
  if command -v ip >/dev/null 2>&1; then
    ip="$(ip route get 1.1.1.1 2>/dev/null | awk '{for(i=1;i<=NF;i++) if ($i=="src") print $(i+1)}')"
  fi
  if [ -z "$ip" ] && command -v hostname >/dev/null 2>&1; then
    ip="$(hostname -I 2>/dev/null | awk '{print $1}')"
  fi
  echo "$ip"
}

smusic_ensure_toolchains_on_path

if [ -z "$LAN_IP" ]; then
  LAN_IP="$(smusic_detect_lan_ip)"
fi
[ -n "$LAN_IP" ] || smusic_die "Não consegui detectar o IP da rede local automaticamente. Rode com --ip=SEU_IP (veja com 'ip addr' ou 'hostname -I')."

smusic_log "Usando IP da rede local: $LAN_IP"
smusic_log "Lembrete: libere a porta $SMUSIC_PROXY_PORT no firewall do computador se a instalação travar ao conectar (ex: 'sudo ufw allow $SMUSIC_PROXY_PORT/tcp' no Linux Mint/Ubuntu)."

smusic_log "== Backend (acessível pela rede local) =="
SMUSIC_WEB_ORIGIN="http://localhost:5173" smusic_backend_up "0.0.0.0"
smusic_backend_seed_demo_catalog

smusic_log "== Frontend (Android) =="
(cd "$SMUSIC_ROOT/frontend" && melos bootstrap)

API_BASE_URL="http://$LAN_IP:$SMUSIC_PROXY_PORT"
smusic_log "Compilando o APK debug (aponta para $API_BASE_URL)..."
(
  cd "$SMUSIC_ROOT/frontend/app/smusic_mobile"
  flutter build apk --debug --dart-define=SMUSIC_API_BASE_URL="$API_BASE_URL"
)

APK_PATH="$SMUSIC_ROOT/frontend/app/smusic_mobile/build/app/outputs/flutter-apk/app-debug.apk"
[ -f "$APK_PATH" ] || smusic_die "Build terminou mas o APK não foi encontrado em $APK_PATH."

smusic_log ""
smusic_log "APK gerado em: $APK_PATH"

if command -v adb >/dev/null 2>&1 && adb devices | grep -qE '\bdevice$'; then
  smusic_log "Celular detectado via adb - instalando..."
  if adb install -r "$APK_PATH"; then
    smusic_log "Instalado! Abra o app 'smusic' no celular (mesma rede Wi-Fi que este computador)."
  else
    smusic_warn "adb install falhou - transfira o .apk manualmente (veja o caminho acima) e instale pelo próprio aparelho."
  fi
else
  smusic_log "Nenhum celular com depuração USB detectado (adb devices vazio)."
  smusic_log "Transfira o arquivo acima para o celular (cabo, e-mail, etc.) e instale manualmente"
  smusic_log "- o Android vai pedir para permitir 'instalar de fontes desconhecidas'."
fi

smusic_log ""
smusic_log "O backend continua rodando em background para o app conseguir conectar."
smusic_log "Rode './scripts/run_local.sh down' quando terminar de testar (para tudo, backend incluso)."

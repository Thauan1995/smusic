#!/usr/bin/env bash
# Redeploy do smusic nesta máquina: git pull + rebuild + migrate (só se
# necessário) + sobe os serviços atualizados.
#
# Uso (a partir de qualquer diretório, dentro do checkout do repo):
#   ./deploy/deploy.sh
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
REPO_ROOT="$(cd "$SCRIPT_DIR/.." && pwd)"
cd "$REPO_ROOT"

COMPOSE=(docker compose -f deploy/docker-compose.prod.yml --env-file deploy/.env.prod)

echo "== Atualizando o repositório =="
BEFORE_MIGRATIONS="$(git ls-files backend/migrations | sort)"
git pull
AFTER_MIGRATIONS="$(git ls-files backend/migrations | sort)"

echo "== Rebuildando imagens =="
"${COMPOSE[@]}" build

if [ "$BEFORE_MIGRATIONS" != "$AFTER_MIGRATIONS" ]; then
	echo "== Migrations novas detectadas, aplicando =="
	"${COMPOSE[@]}" run --rm migrate
else
	echo "== Nenhuma migration nova =="
fi

echo "== Subindo os serviços =="
"${COMPOSE[@]}" up -d

echo "== Status =="
"${COMPOSE[@]}" ps

SMUSIC_DOMAIN="$(grep -m1 '^SMUSIC_DOMAIN=' deploy/.env.prod | cut -d= -f2)"
echo "== Checando https://$SMUSIC_DOMAIN/healthz =="
sleep 3
if curl -fsS "https://$SMUSIC_DOMAIN/healthz"; then
	echo ""
	echo "OK: redeploy concluído."
else
	echo ""
	echo "AVISO: healthz não respondeu — confira os logs (docker compose -f deploy/docker-compose.prod.yml --env-file deploy/.env.prod logs)."
	exit 1
fi

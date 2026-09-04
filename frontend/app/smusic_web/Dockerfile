# Builds the Flutter web app + bundles it into the Caddy image that serves
# it and reverse-proxies to the Go backend (see deploy/Caddyfile,
# deploy/docker-compose.prod.yml).
#
# Build context is the REPO ROOT (not this directory) — see the "caddy"
# service in deploy/docker-compose.prod.yml — because this needs both the
# whole frontend/ monorepo (packages/core, packages/domain, packages/data
# are melos path-dependencies of smusic_web, linked via `melos bootstrap`)
# and deploy/Caddyfile, which lives outside frontend/.
FROM ghcr.io/cirruslabs/flutter:stable AS builder
WORKDIR /app
COPY frontend/ ./frontend/
WORKDIR /app/frontend
RUN dart pub global activate melos
ENV PATH="$PATH:/root/.pub-cache/bin"
RUN melos bootstrap

ARG SMUSIC_API_BASE_URL=http://localhost:8090
WORKDIR /app/frontend/app/smusic_web
RUN flutter build web --release --dart-define=SMUSIC_API_BASE_URL=${SMUSIC_API_BASE_URL}

FROM caddy:2-alpine
COPY --from=builder /app/frontend/app/smusic_web/build/web /srv/web
COPY deploy/Caddyfile /etc/caddy/Caddyfile

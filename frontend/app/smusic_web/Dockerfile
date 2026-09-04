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

# Custom Caddy build with the DuckDNS DNS provider plugin, so certificates
# can be issued via DNS-01 (proves domain ownership through a DNS TXT
# record) instead of HTTP-01/TLS-ALPN-01 — those require Let's Encrypt's
# validation servers to reach this box on 80/443, which isn't guaranteed
# on a home connection (ISP/router filtering, CGNAT, etc).
FROM caddy:2-builder-alpine AS caddy-builder
RUN xcaddy build --with github.com/caddy-dns/duckdns

FROM caddy:2-alpine
COPY --from=caddy-builder /usr/bin/caddy /usr/bin/caddy
COPY --from=builder /app/frontend/app/smusic_web/build/web /srv/web
COPY deploy/Caddyfile /etc/caddy/Caddyfile

# Decision Log
> Newest first. Updated automatically by the architect agent.

## 2026-09-04 — `fix-presence-rest-routing` implemented, validated locally, pending production redeploy

Fixed `deploy/Caddyfile`: `/v1/presence/connect` (WS) now has its own specific `handle` block before the general `/v1/*` rule; every other `/v1/presence/*` path (settings/consent/pause/resume/blocks) now correctly falls into `/v1/* -> server:8080` instead of the old catch-all `/v1/presence/* -> presence-server:8081` that 404'd them all.

Validated against a full local stack (Postgres+Redis via `docker run`, `cmd/server`/`cmd/presence-server` via `go run` on test ports, Caddy 2 in Docker with a routing-equivalent no-TLS test Caddyfile) — not against the real `smusic-dev.duckdns.org` host, which this session has no SSH/deploy access to (see `smusic-deploy-topology` memory). Reproduced the exact prior failure (404 on `GET /v1/presence/settings`) with the *old* logic, then confirmed the fix: `settings`/`consent`/`pause` all return 200, and the WS route still completes a real upgrade handshake (101 Switching Protocols) — no regression.

**Known follow-up, deliberately NOT fixed here (anti-scope in the spec)**: `scripts/tools/path_proxy.py` (local-dev proxy) has the identical bug — its docstring says it mirrors the Caddyfile's routing, and its code does route all `/v1/presence/*` to the presence port. Left untouched per the spec's anti-scope; worth a one-line follow-up spec if local-dev testing of presence REST endpoints (via the proxy) is ever needed.

**Still open**: production redeploy. This session cannot reach the actual host (no matching SSH config, no local Docker/Caddy process for it). User needs to `git pull` + restart the `caddy` service on the real machine (`docker compose -f deploy/docker-compose.prod.yml --env-file deploy/.env.prod up -d --build caddy` per `deploy/README.md`), then this spec's live DoD check (`curl https://smusic-dev.duckdns.org/v1/presence/settings` with a bearer token) can be re-run for final confirmation.

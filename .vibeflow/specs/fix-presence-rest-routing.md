# Spec: Fix production routing for presence REST control-plane endpoints

## Objective
Make `GET/PUT /v1/presence/settings`, `POST/DELETE /v1/presence/consent`, `POST /v1/presence/pause`, `POST /v1/presence/resume`, and `POST/DELETE /v1/presence/blocks/{user_id}` reachable through the public production domain, so users can actually control the proximity feature's privacy settings.

## Context
Confirmed live on 2026-09-04 against `https://smusic-dev.duckdns.org`: `GET /v1/presence/settings` (with a valid bearer token from a freshly created test user) returns `404 page not found`. Root cause: `deploy/Caddyfile` has
```
handle /v1/presence/* {
    reverse_proxy presence-server:8081
}
handle /v1/* {
    reverse_proxy server:8080
}
```
`presence-server` (per `backend/cmd/presence-server/main.go`) mounts **only** `/v1/presence/connect` (the WebSocket). Every other `/v1/presence/*` path — the entire REST privacy control plane, which `backend/README.md` explicitly documents as living on `cmd/server:8080` — matches the first `handle` block and is routed to the wrong process, which has no such route, hence 404.

This is not a cosmetic bug: `security.md` §1.1/§1.4 makes opt-in consent, revocation, pause, and block controls a hard (LGPD-driven) requirement for the whole proximity feature. As deployed, none of them are reachable by any real client hitting the public domain — only local dev (talking to `cmd/server` directly on `:8080`) exercises this surface today.

## Definition of Done
- [ ] `GET https://<domain>/v1/presence/settings` with a valid bearer token returns `200` with the settings payload (not 404), verified against the actual deployed Caddy config (not just unit-tested).
- [ ] `POST/DELETE /v1/presence/consent`, `POST /v1/presence/pause`, `POST /v1/presence/resume`, `POST/DELETE /v1/presence/blocks/{user_id}` are all reachable through the public domain with the same verification method.
- [ ] `WS /v1/presence/connect` still routes to `presence-server:8081` and continues to work (regression check — do not break the one route that currently works).
- [ ] `deploy/Caddyfile` has a comment explaining the split (which paths go where and why), so this doesn't regress silently on a future edit.
- [ ] No violation of `conventions.md` Don'ts — in particular, no coordinate/geohash field is introduced anywhere in this change (this spec only touches routing, not payloads).
- [ ] Change is validated against a live redeploy (`docker compose -f deploy/docker-compose.prod.yml ... up -d --build caddy` or equivalent), not just a Caddyfile diff review — re-run the same `curl` reproduction from this spec's Context section post-deploy.

## Scope
- `deploy/Caddyfile`: split the `/v1/presence/*` handling so only `/v1/presence/connect` (and any future WS-only routes `presence-server` actually owns) goes to `presence-server:8081`; everything else under `/v1/presence/*` goes to `server:8080`.
- Redeploy and live-verify against the actual `smusic-dev.duckdns.org` domain.

## Anti-scope
- Do NOT change which process owns which route (`cmd/server` vs `cmd/presence-server`) — that split is an intentional architecture decision (`backend-go.md` §1), only the reverse-proxy config is wrong.
- Do NOT touch `scripts/tools/path_proxy.py` (local-dev routing) unless it has the identical bug — check it as part of this spec's investigation, but this spec's DoD is scoped to `deploy/Caddyfile`/production only. If the same bug exists locally, file it as a one-line follow-up, don't silently expand scope here.
- Do NOT add authentication/authorization changes — this is purely a routing fix; the endpoints already enforce `RequireAuth`.

## Technical Decisions
- **Most direct fix**: reorder/narrow the Caddy `handle` blocks so `/v1/presence/connect` is matched specifically (Caddy's `handle` matches in file order, first match wins — an exact-path `handle /v1/presence/connect` block before the general `/v1/presence/*` block, or narrowing the general block's target to `server:8080` and adding a specific `/v1/presence/connect` block routed to `presence-server:8081`) — no application code changes needed, this is a deploy-config-only fix.

## Applicable Patterns
- None from `.vibeflow/patterns/` directly (this is infra/deploy config, not application code) — but the fix must preserve the process split documented in `backend-concurrency-presence.md`'s "Where" section.

## Risks
- **Risk**: reordering `handle` blocks accidentally makes `/v1/presence/connect` also match the general block first (Caddy `handle` short-circuits on first match, so block order matters). **Mitigation**: put the more specific path first, verify both routes explicitly post-deploy, not just one.
- **Risk**: this is a production deploy on a home-lab machine — a bad Caddyfile can take the whole domain down. **Mitigation**: validate the Caddyfile syntax (`caddy validate`) before restarting the `caddy` service, and confirm `/healthz` and the web root still respond after the change.

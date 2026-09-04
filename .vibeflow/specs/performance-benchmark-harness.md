# Spec: Build the load-test harness needed to actually judge the Spotify/YouTube Music performance targets

## Objective
Make it possible to objectively answer "does smusic meet the Spotify/YouTube Music performance bar the founding brief requires" — today that comparison cannot be made at all, because no load-test/benchmark tooling exists to measure the numeric targets `backend-go.md` §6 already defines.

## Context
The project's founding brief requires playback/library/performance to be judged by direct comparison against Spotify/YouTube Music benchmarks — "sem qualificador de performance solto." `backend-go.md` §6 already translated that into concrete, numbered targets (time-to-first-audio ≤300ms p50/≤800ms p95, seek latency ≤150ms p50/≤400ms p95, search ≤100ms p50/≤300ms p95, presence fanout ≤2s p95, ≥5,000 concurrent WS connections/replica, etc.) and explicitly says: "Essas metas são compromissos de arquitetura, não garantias automáticas... precisa ser validada por testes de carga (seção 7) antes de ir a produção." `backend-go.md` §7.5 names the intended tools: `k6`/`vegeta` for HTTP, a custom Go WS-client load tool for presence.

None of this exists in the repo today — no `k6`/`vegeta` scripts, no load-test results, no dashboard. This means any Auditor judgment so far ("Fatia 1 aprovada," "Fatia 2... auditoria geral contra os padrões de Spotify/YouTube Music") could only have been an architectural/code review, not a measurement against the founding brief's own stated benchmarks — since there is currently no way to produce a number to compare. This spec doesn't chase the targets themselves (that likely requires the still-deferred `media-edge-service`/real CDN, out of scope per `backend/README.md`'s deviation #8) — it builds the harness so the targets can be measured at all, against what's deployed today, establishing a real baseline.

## Definition of Done
- [ ] A `k6` (or `vegeta`) script exists that measures `GET /v1/catalog/search` p50/p95 latency under a defined concurrent load, runnable against any target URL (local or `smusic-dev.duckdns.org`).
- [ ] A `k6`/`vegeta` script exists that measures `POST /v1/playback/sessions/{id}/play` (time-to-signed-URL, the backend-controlled portion of "time to first audio") and `POST /v1/playback/sessions/{id}/seek` p50/p95 latency.
- [ ] A small Go (or `k6` WS-scripting-API) load tool connects N concurrent WS clients to `/v1/presence/connect`, sends periodic `update` frames, and measures fanout latency (time from one client's update to a nearby client's `nearby_update` frame) at increasing N, up to at least a few hundred concurrent connections (the ≥5,000/replica target from §6 is aspirational for a home-lab box — this DoD is about the harness working correctly and producing a real number, not about hitting 5,000 on a single old laptop).
- [ ] Running the harness against the current deployment produces a written report (a markdown file or equivalent, committed to the repo, e.g. `docs/architecture/performance-baseline.md`) stating the actual measured p50/p95 for each metric above, explicitly compared against `backend-go.md` §6's targets — pass/fail per metric, not vague.
- [ ] The report explicitly notes which targets cannot yet be meaningfully evaluated because the underlying infrastructure they depend on doesn't exist yet (e.g. real CDN-backed audio delivery) — so the gap is documented, not silently glossed over.
- [ ] No violation of `conventions.md` Don'ts.

## Scope
- Load-test scripts (`k6`/`vegeta`/small Go WS client) under a new `backend/loadtest/` (or repo-root `loadtest/`) directory.
- One committed baseline report comparing current measured numbers against `backend-go.md` §6's targets.
- Minimal `make` targets to re-run the harness (`make loadtest-http`, `make loadtest-presence` or similar) so this isn't a one-off script that bit-rots.

## Anti-scope
- Do NOT attempt to actually hit the ≥5,000 concurrent WS / ≥2,000 updates/s per-replica target on the current single-machine home-lab deployment — that's an infrastructure-scaling question (more replicas, real cloud hosting), not something this spec's harness needs to prove; the DoD only requires the harness to work and to report the real number achieved, whatever it is.
- Do NOT build this into CI as a blocking gate — `backend-go.md` §7.5 itself says load tests "não em CI de todo commit... mas obrigatórios em pipeline de release," i.e. a release-time check, not a per-commit one. Wiring it into a release pipeline is a reasonable follow-up, not this spec's job.
- Do NOT implement the deferred `media-edge-service`/real CDN/HLS pipeline to try to hit the time-to-first-audio target for real — that's explicitly out of scope for this slice per `backend/README.md`'s deviation #8; this spec only measures what's deployed today (including its known limitation of serving from `testdata/media/` directly).

## Technical Decisions
- **`k6` for HTTP, a small custom Go client for WS**: matches `backend-go.md` §7.5's own tool recommendation, and a custom Go WS client can reuse this codebase's existing `gorilla/websocket` dependency and presence protocol types directly (no need to reimplement frame parsing from scratch).

## Applicable Patterns
- None from `.vibeflow/patterns/` directly (load-test tooling is not application code) — but the custom Go WS client should reuse `internal/presence`'s existing frame types/protocol definitions rather than redefining them, to avoid drift between the test tool and the real protocol.

## Risks
- **Risk**: running a meaningful load test against a home-lab machine that's also serving real traffic (via `smusic-dev.duckdns.org`) could degrade the live service or look like a DDoS to anyone watching. **Mitigation**: run the harness against a local/dev instance first; only run a lighter, clearly-scoped confirmation pass against the public domain, and only with the user's explicit go-ahead given it's their home connection/hardware.
- **Risk**: numbers measured on an "notebook velho" are not representative of what a real production deployment would achieve, and could be misread as "the architecture fails its targets" when it's really a hardware/hosting constraint. **Mitigation**: the baseline report DoD item must explicitly caveat measured numbers against the deployment tier they were measured on — this is about establishing a harness and an honest baseline, not a final verdict on the architecture.

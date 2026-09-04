# Spec: Wire mandatory security CI gates (SAST, dependency scan, secret scan)

## Objective
Stand up the CI pipeline `security.md` §4/§6 mandates as a precondition for the project's own "zero critical vulnerabilities" acceptance criterion, so that criterion can ever be objectively verified instead of asserted.

## Context
`security.md` §4 requires CI to block merge/deploy on: SAST (gosec/semgrep) High/Critical findings, secret scanning (gitleaks) finding a committed secret, dependency scanning finding a Critical CVE without an approved exception, and infra-policy test failures. §6's "Verificação objetiva por Auditor" makes "CI logs showing SAST/dependency/secret-scan gates are active and blocking, with real execution history" one of exactly four conditions required before the founding brief's stop condition ("zero vulnerabilidades críticas identificadas") can be considered met by an independent auditor.

Current state (verified 2026-09-04): no `.github/workflows/` directory or equivalent CI config exists anywhere in the repo. `gosec`, `govulncheck`, `semgrep`, `gitleaks`, and `Trivy` are mentioned only in `docs/architecture/security.md` and `backend/README.md`'s prose — none appear in the `Makefile`, any script, or any config file. This is the single largest concrete gap between the founding brief's stop condition and the repo's actual state: whatever manual/Auditor review approved "Fatia 1"/"Fatia 2" in `00-overview.md`'s decision log, it did not have automated, continuously-enforced security gates behind it.

## Definition of Done
- [ ] A CI workflow (GitHub Actions, since the repo already uses `gh`-style conventions — or the CI system the user's deploy actually uses) runs on every push/PR and executes: `go vet`, `staticcheck`, `go test -race`, `govulncheck ./...` for the backend.
- [ ] The same workflow runs `gosec ./...` (or `semgrep` with a Go ruleset) against `backend/` and fails the build on any High/Critical finding.
- [ ] `gitleaks detect` (or equivalent) runs against the full diff/repo history and fails the build if a secret is found; run it once against current `HEAD` (and ideally full history) as part of this spec's implementation to confirm no secret was ever committed. Confirmed 2026-09-04: `backend/.env`/`deploy/.env.prod` are correctly gitignored (only `.env.example`/`.env.prod.example` are tracked) — this check is about proving that holds going forward, not fixing a known leak.
- [ ] `melos run analyze` and `dart pub outdated`/an equivalent Dart dependency-vulnerability check run for the frontend workspace.
- [ ] The workflow's status is visible (badge or equivalent) and at least one real run's logs are captured as evidence — this is what `security.md` §6 condition 2 explicitly asks an independent auditor to check ("not just configured, but with a history of real execution").
- [ ] No violation of `conventions.md` Don'ts.

## Scope
- CI workflow file(s) for backend (Go tooling) and frontend (Dart/Flutter tooling) gates listed above.
- A documented process for handling/exempting a finding (per `security.md` §4's "CVE Crítico sem exceção aprovada" language) — even a minimal `SECURITY-EXCEPTIONS.md` or PR-label convention is enough to satisfy the "approved exception" requirement; do not build tooling for this, just define the process.
- Fixing whatever gosec/govulncheck/gitleaks findings turn out to be Critical/High once the gates run for the first time is IN scope (the gates are useless if merged red and then ignored) — but only Critical/High, not every Medium/Low finding (see Anti-scope).

## Anti-scope
- Do NOT implement DAST (OWASP ZAP), Trivy container scanning, or SBOM generation in this spec — those need a staging environment and container registry this repo doesn't have configured yet; split into a follow-up spec once a staging deploy target exists.
- Do NOT fix every Medium/Low finding gosec/govulncheck surfaces — only block on Critical/High per `security.md` §6's own definition of "Crítico"/"Alto"; fixing everything is scope creep against a spec whose job is to get the *gate* working.
- Do NOT stand up a pentest or bug-bounty program — explicitly out of reach for a home-lab-hosted pre-launch project; `security.md` §6 already scopes that to "before public launch."
- Do NOT touch application code beyond what's needed to clear a Critical/High finding the new gates surface.

## Technical Decisions
- **CI platform**: GitHub Actions, since the repo is on GitHub-style tooling conventions already (the project uses `gh`-compatible workflows elsewhere in its tooling) and both `gosec`/`govulncheck`/`gitleaks`/Melos have first-class GitHub Actions or are trivially scriptable there. If the user's actual CI is elsewhere, swap the workflow syntax but keep the same gate list.
- **Fail-closed, not fail-open**: any Critical/High SAST or dependency finding, or any secret-scan hit, fails the build — matching `security.md` §4's explicit "pipeline bloqueia" language, not a warning-only gate.

## Applicable Patterns
- None from `.vibeflow/patterns/` (CI config, not application code) — but any code fixes made to clear findings must follow `backend-error-handling.md`/`backend-module-layout.md` as applicable.

## Risks
- **Risk**: gosec/govulncheck may surface a large first-run backlog that blocks all future merges immediately. **Mitigation**: triage findings once, fix Critical/High as part of this spec, file remaining Medium/Low as tracked follow-ups (not silently suppressed — matches `00-overview.md`'s "toda exclusão precisa de justificativa explícita" spirit).
- **Risk**: even though `backend/.env`/`deploy/.env.prod` are correctly gitignored today, a future contributor could `git add -f` one by accident. **Mitigation**: the gitleaks gate itself is the durable defense here — verify it actually catches a deliberately-staged fake secret in a throwaway branch before trusting it in CI.

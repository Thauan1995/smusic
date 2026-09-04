# Security gate exceptions

Process for exempting a CI security-gate finding — per
`docs/architecture/security.md` §4 ("CVE Crítico sem exceção aprovada"
blocks the pipeline) and `.vibeflow/specs/security-ci-gates.md`. Fixing
the finding is always the default; this document is for the rare case
where a finding is a genuine false positive or a risk explicitly accepted
by whoever owns the security review for this project.

## When an exception is allowed

- The finding is a **false positive** (the tool is wrong about the code),
  or
- The finding is **real but not exploitable in this codebase's actual
  usage** of the flagged code/dependency, or
- The finding requires a fix that is itself higher-risk than the finding
  (rare — needs explicit justification, not just "it's inconvenient").

An exception is never "we'll get to it later" for a genuine Critical/High
finding — that's a bug to fix, not an exception to grant.

## How to grant one

1. **In code** (gosec/staticcheck findings): use the tool's own inline
   suppression comment (`// #nosec G### -- <reason>` for gosec) directly
   above the flagged line, with a one-line reason a reviewer can evaluate
   without extra context. Never a bare suppression with no reason — see
   `internal/catalog/postgres/repo.go`'s `insertCredit` (`G101` false
   positive) and `internal/auth/password/argon2.go`'s `Verify` (`G115`,
   justified by the `maxDerivedKeyLen` bounds check immediately above it)
   for the pattern this project already follows.
2. **Dependency CVEs** (govulncheck/`dart pub outdated` findings): add a
   row to the table below. Required fields: which CVE/advisory, which
   dependency, why it's not exploitable in this codebase's actual call
   graph (govulncheck's own call-graph analysis is usually the evidence —
   "0 vulnerabilities... your code doesn't appear to call these" is a
   valid, checkable justification, not a hand-wave), and a re-review date
   (default: 90 days out, sooner for anything closer to security.md §6's
   "Crítico" bar).
3. Anything meeting security.md §6's explicit "Crítico" definition (CVSS
   ≥ 9.0, OR unauthenticated account takeover, OR third-party
   location/identity exposure without consent, OR RCE/cross-user data
   access) **cannot** be exempted here — it must be fixed before merge,
   full stop, per the founding brief's stop condition.

## Current exceptions

| Date | Finding | Dependency/location | Reason | Re-review by |
|---|---|---|---|---|
| 2026-09-04 | GO-2026-6303, -5932, -6354, -6355 (4 advisories, `golang.org/x/crypto/ssh`+`openpgp`) | Transitive dependency (not directly imported; this backend only uses `golang.org/x/crypto/argon2`) | `govulncheck ./...`'s call-graph analysis confirms 0 of these are reachable from any code this backend calls — see `.vibeflow/specs/security-ci-gates.md`'s evidence log. Re-run `govulncheck` whenever `go.sum` changes; this row is stale the moment that stops being true. (Updated 2026-09-04: `go mod tidy` after adding `testcontainers-go`/`pquerna/otp` bumped `golang.org/x/crypto` to a newer version, dropping the reported-but-unreachable advisory count from 17 to 4 — same non-exploitability reasoning applies to all of them.) | 2026-12-04 |

No code-level (`#nosec`) exceptions are tracked here — those two live as
inline comments per the "How to grant one" section above; this table is
only for dependency-level exceptions that can't be annotated at a single
line.

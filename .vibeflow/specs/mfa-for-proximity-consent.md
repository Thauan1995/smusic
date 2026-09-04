# Spec: Enforce mandatory MFA before enabling proximity discovery

## Objective
Require a verified second factor (TOTP) before a user can grant proximity-discovery consent, closing a hard requirement from `security.md` §2 that Fatia 2 shipped without.

## Context
`security.md` §2: "MFA: TOTP (RFC 6238) **obrigatório** para habilitar a feature de proximidade e para ações sensíveis (...); opcional (...) para login geral." `backend/README.md`'s Auth section originally justified shipping `internal/auth/mfa.NoopChallenger` by saying "no feature in this slice needs step-up auth yet — proximity, which does, is Fatia 2." Fatia 2 (presence/proximity) is now implemented and marked complete in `00-overview.md` §3, but `internal/auth/mfa.Challenger` is still wired as `NoopChallenger` in `cmd/server/main.go` (`_ = mfa.NoopChallenger{} // wired for future step-up flows`) — it is never called from `POST /v1/presence/consent` or anywhere in `internal/presence`. Any user can grant proximity consent today with only their normal password-based session, no second factor.

## Definition of Done
- [ ] `POST /v1/presence/consent` (granting consent) returns a distinct error (e.g. `403 mfa_required`) when the calling user has no verified TOTP factor enrolled, instead of succeeding.
- [ ] A TOTP enrollment endpoint exists (`internal/auth/mfa`'s real `Challenger` — replacing `NoopChallenger` for this path) allowing a user to enroll and verify a TOTP secret, following RFC 6238.
- [ ] Once enrolled and verified, `POST /v1/presence/consent` succeeds normally for that user.
- [ ] Unit tests cover: consent rejected without MFA, consent allowed after MFA enrollment+verification, using an in-memory fake `Challenger` (per `backend-testing.md`'s pattern — no real TOTP library needs a real clock/real secret in the unit tier, inject `Clock` as already done elsewhere in this codebase).
- [ ] `internal/auth/mfa`'s package doc / `backend/README.md`'s Auth section is updated to reflect that `NoopChallenger` is no longer wired for the proximity-consent path (it may remain the default for general login, which stays optional per `security.md` §2).
- [ ] No violation of `conventions.md` Don'ts — in particular, `Clock`/`IDGenerator` injection pattern is followed for the new TOTP secret generation, no direct `time.Now()`/crypto/rand call inside `service.go`.

## Scope
- Backend: a real `Challenger` implementation (TOTP enroll/verify) in `internal/auth/mfa`, wired into `presence.SettingsService`'s (or `NearbyService`'s, wherever consent-granting lives) consent-granting path only.
- A minimal enrollment REST surface (`POST /v1/auth/mfa/enroll`, `POST /v1/auth/mfa/verify` or similar, matching this codebase's existing REST shape) — reuse `backend-http-handlers.md`'s pattern.
- Frontend: minimal UI to enroll/verify TOTP, gated in front of the existing proximity opt-in value screen (`frontend-proximity-privacy-ui.md`'s flow) — do not redesign that flow, just add a step.

## Anti-scope
- Do NOT make MFA mandatory for general login — `security.md` §2 explicitly keeps that optional; only proximity-consent (and, per the same sentence, "ações sensíveis" like password/email change and session management) requires it. This spec covers proximity-consent only; a follow-up spec can cover the other sensitive actions if the user wants them prioritized.
- Do NOT implement SMS OTP as a factor — `security.md` §2 explicitly excludes it (SIM-swap risk).
- Do NOT build backup/recovery codes UX in this spec unless trivial — flag as a follow-up if it turns out non-trivial; the DoD above doesn't require it.
- Do NOT touch `oauth`'s `StubVerifier` — unrelated to this spec.

## Technical Decisions
- **TOTP library**: use a well-maintained Go TOTP library compatible with RFC 6238 (e.g. `pquerna/otp`) rather than hand-rolling HMAC-based OTP — this is exactly the kind of security-primitive code the project's own Argon2id/JWT choices show a "use a vetted library" preference for.
- **Where the gate lives**: enforce in `presence`'s service layer (`SettingsService`/`NearbyService`, wherever `GrantConsent` is implemented), not just in the HTTP handler — matches this codebase's existing pattern of putting business rules in `service.go`, testable without HTTP.

## Applicable Patterns
- `backend-module-layout.md` — new `Challenger` implementation follows the existing repo/service/postgres split.
- `backend-error-handling.md` — new sentinel error (e.g. `ErrMFARequired`) mapped in presence's existing `writeXError`.
- `backend-testing.md` — fake `Challenger` for unit tests, no real TOTP clock skew handling needed in the unit tier.
- `frontend-proximity-privacy-ui.md` — the new enrollment step composes with, not replaces, the existing opt-in flow.

## Risks
- **Risk**: TOTP clock-skew handling is a classic source of subtle bugs (valid codes rejected due to server/client clock drift). **Mitigation**: use the TOTP library's built-in skew-window support (the codebase's own `Clock` interface already models "injectable time" — reuse it for the verify window, don't hand-roll skew logic).
- **Risk**: this blocks any current test/QA user (including ones created for this analysis) from re-enabling proximity consent until they enroll MFA. **Mitigation**: expected and correct per spec — flag it explicitly in the PR description so it isn't mistaken for a regression during review.

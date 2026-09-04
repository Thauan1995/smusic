-- Real TOTP MFA (RFC 6238), replacing internal/auth/mfa's NoopChallenger
-- for one call site: security.md §2 mandates MFA before a user can enable
-- proximity discovery (Fatia 2). See .vibeflow/specs/mfa-for-proximity-consent.md.
--
-- One secret per user (a user enrolls at most one TOTP factor in this
-- slice — multiple factors/backup codes are a documented follow-up, not
-- part of this spec's DoD). `secret` is the base32 TOTP seed; like
-- MEDIA_SIGNING_KEY/JWT_ED25519_SEED_HEX elsewhere in this codebase, it is
-- stored as plaintext in Postgres in this slice (production key
-- management via Vault/KMS field-level encryption is the same documented
-- TODO already tracked for those secrets — see security.md §3 and
-- backend/README.md's Configuration table).
CREATE TABLE user_mfa_totp (
    user_id     UUID PRIMARY KEY REFERENCES users(id) ON DELETE CASCADE,
    secret      TEXT NOT NULL,
    verified_at TIMESTAMPTZ,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at  TIMESTAMPTZ NOT NULL DEFAULT now()
);

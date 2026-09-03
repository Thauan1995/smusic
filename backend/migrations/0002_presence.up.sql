-- Fatia 2 schema: privacy/consent settings, block list and the presence
-- access-audit log, per security.md §1 (the mandatory privacy policy for
-- the proximity-discovery feature) and data-architecture.md §4.5's schema
-- hooks.
--
-- Deliberately NOT part of this migration: any table holding a raw
-- lat/lng, geohash, or "who was near whom" tuple. Per security.md §1.5,
-- presence itself is efemeral-only (Redis, TTL 90s) and never persisted in
-- a durable relational table — see internal/presence/redisstore for that
-- side of the implementation. This migration only adds the durable
-- *configuration* (consent/settings, blocks) and the durable *audit trail*
-- of who queried whom (never the location/track data itself).
--
-- Deviations from data-architecture.md/security.md, documented:
--   * security.md §1.8 calls for a WORM store "separado do banco
--     operacional" for the audit log. No such infrastructure exists in
--     this phase (documented as an open question in security.md §7). This
--     migration substitutes an append-only table in the same operational
--     Postgres, enforced at the database level (not just "the application
--     promises not to UPDATE/DELETE") via BEFORE UPDATE/DELETE triggers
--     that unconditionally raise an exception — so even a bug or an ad-hoc
--     manual query cannot silently mutate/erase a row. This is a
--     documented, reviewable substitution, not a silent simplification.
--     TODO: migrate to real WORM/object-lock storage (e.g. S3 Object Lock)
--     when that infra exists; TODO: 180-day retention purge job (security.md
--     §1.8) is deliberately NOT implemented as a scheduled job in this
--     slice (task explicitly allows this to remain a documented TODO) — a
--     future purge job must run as a role that can temporarily
--     `ALTER TABLE presence_audit_log DISABLE TRIGGER trg_presence_audit_log_no_delete`
--     (or an equivalent bypass), since the immutability trigger below
--     applies to every role by default.
--   * presence_audit_log.requester_id/target_id are plain UUID columns
--     WITHOUT a foreign key to users(id). This is deliberate: a FK with
--     ON DELETE CASCADE would let an account deletion silently erase audit
--     history (defeating the append-only/immutability guarantee above —
--     LGPD account-deletion flows must not be able to double as an abuse
--     cover-up mechanism), and a FK without CASCADE would block account
--     deletion entirely once any audit row references the user. Decoupling
--     the audit log from users' referential lifecycle mirrors "separado do
--     banco operacional" in spirit even though it's physically the same
--     Postgres instance.

-- 1. Consent / privacy settings (security.md §1.1, §1.3, §1.4, §1.6;
--    data-architecture.md §4.5) --------------------------------------------

CREATE TABLE user_privacy_settings (
    user_id                      UUID PRIMARY KEY REFERENCES users(id) ON DELETE CASCADE,
    -- Default 'invisible' + paused_bool default true + consent disabled by
    -- default is what makes the feature "nasce desligada para todo
    -- usuário, inclusive contas novas" (security.md §1.1) true at the
    -- schema level, not just in application logic.
    presence_visibility          TEXT NOT NULL DEFAULT 'invisible'
                                      CHECK (presence_visibility IN ('invisible', 'friends_only', 'everyone')),
    presence_share_track         BOOLEAN NOT NULL DEFAULT false,
    proximity_consent_enabled    BOOLEAN NOT NULL DEFAULT false,
    proximity_consent_ts         TIMESTAMPTZ,
    proximity_consent_renew_due  TIMESTAMPTZ,
    -- security.md §1.3: slider steps 150/1000/5000/15000, floor 150m,
    -- ceiling 15000m, no "unlimited" option — enforced here too, not just
    -- in Go, so a direct SQL write can't violate the policy either.
    visibility_radius_m          SMALLINT NOT NULL DEFAULT 1000
                                      CHECK (visibility_radius_m IN (150, 1000, 5000, 15000)),
    -- security.md §1.6: 0 = anonymous (default), 1 = connections see
    -- name/avatar, 2 = opt-in "open discovery" (non-connections also see
    -- level-1 identity).
    reveal_level                 SMALLINT NOT NULL DEFAULT 0
                                      CHECK (reveal_level IN (0, 1, 2)),
    paused_bool                  BOOLEAN NOT NULL DEFAULT true,
    created_at                   TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at                   TIMESTAMPTZ NOT NULL DEFAULT now()
);

-- 2. Block list (security.md §1.4) -----------------------------------------

CREATE TABLE user_blocks (
    blocker_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    blocked_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    PRIMARY KEY (blocker_id, blocked_id),
    CHECK (blocker_id <> blocked_id)
);
CREATE INDEX idx_user_blocks_blocked_id ON user_blocks(blocked_id);

-- 3. Presence access audit log (security.md §1.8) ---------------------------
--
-- One append-only row per presence query where a requester actually
-- received a distance bucket about a target (see
-- internal/presence.NearbyService) — never exposed to the requester or the
-- target via any endpoint in this slice (Trust & Safety tooling is
-- explicitly out of scope here).

CREATE TABLE presence_audit_log (
    id              UUID PRIMARY KEY,
    requester_id    UUID NOT NULL,
    target_id       UUID NOT NULL,
    occurred_at     TIMESTAMPTZ NOT NULL DEFAULT now(),
    distance_bucket SMALLINT NOT NULL CHECK (distance_bucket BETWEEN 1 AND 4),
    endpoint        TEXT NOT NULL
);
CREATE INDEX idx_presence_audit_log_target_id ON presence_audit_log(target_id, occurred_at DESC);
CREATE INDEX idx_presence_audit_log_requester_id ON presence_audit_log(requester_id, occurred_at DESC);
CREATE INDEX idx_presence_audit_log_occurred_at ON presence_audit_log(occurred_at);

CREATE OR REPLACE FUNCTION presence_audit_log_immutable() RETURNS TRIGGER AS $$
BEGIN
    RAISE EXCEPTION 'presence_audit_log is append-only: % is not permitted (security.md §1.8)', TG_OP;
END;
$$ LANGUAGE plpgsql;

CREATE TRIGGER trg_presence_audit_log_no_update
    BEFORE UPDATE ON presence_audit_log
    FOR EACH ROW EXECUTE FUNCTION presence_audit_log_immutable();

CREATE TRIGGER trg_presence_audit_log_no_delete
    BEFORE DELETE ON presence_audit_log
    FOR EACH ROW EXECUTE FUNCTION presence_audit_log_immutable();

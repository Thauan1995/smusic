-- Fatia 1 schema: auth, catalog, library, playback-adjacent history/stats,
-- plus the tables data-architecture.md §1 lists as "central relationships"
-- that this slice's API surface doesn't yet exercise (plans, subscriptions,
-- family_plan_members, follows, user_devices, library_albums,
-- library_artists) — kept so the schema matches the target data model and
-- future slices don't need a disruptive migration to add them.
--
-- Deviations from data-architecture.md, documented:
--   * Added `refresh_tokens` (not in data-architecture.md's table list, but
--     required by security.md §2's revocable-opaque-refresh-token model).
--   * `tracks` does NOT have an `audio_asset_id` FK to
--     `track_audio_assets`, even though data-architecture.md §1.2's prose
--     mentions one: that would be circular with track_audio_assets.track_id
--     (the actual 1:N FK, "many assets per track"), which is the
--     relationship this migration implements. The "default" asset for a
--     track is selected by quality_tier at query time, not a hardcoded FK.
--   * play_events is RANGE-partitioned by played_at (monthly) per
--     data-architecture.md §5.1, with a DEFAULT partition so inserts never
--     fail if a monthly partition hasn't been pre-created yet — creating
--     future monthly partitions on a schedule is a documented TODO
--     (see README), not implemented as a DB job in this slice.

CREATE EXTENSION IF NOT EXISTS citext;
CREATE EXTENSION IF NOT EXISTS pg_trgm;
CREATE EXTENSION IF NOT EXISTS pgcrypto; -- gen_random_uuid()

-- 1.1 Identity and users -----------------------------------------------

CREATE TABLE users (
    id                UUID PRIMARY KEY,
    email             CITEXT UNIQUE NOT NULL,
    email_verified_at TIMESTAMPTZ,
    password_hash     TEXT,
    display_name      TEXT NOT NULL,
    handle            TEXT UNIQUE,
    avatar_url        TEXT,
    country_code      CHAR(2),
    status            TEXT NOT NULL DEFAULT 'active'
                          CHECK (status IN ('active', 'suspended', 'deleted')),
    created_at        TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at        TIMESTAMPTZ NOT NULL DEFAULT now(),
    deleted_at        TIMESTAMPTZ
);

CREATE TABLE user_auth_identities (
    id               UUID PRIMARY KEY,
    user_id          UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    provider         TEXT NOT NULL CHECK (provider IN ('google', 'apple', 'facebook', 'password')),
    provider_user_id TEXT NOT NULL,
    created_at       TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at       TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE (provider, provider_user_id)
);
CREATE INDEX idx_user_auth_identities_user_id ON user_auth_identities(user_id);

CREATE TABLE user_devices (
    id           UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id      UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    platform     TEXT NOT NULL CHECK (platform IN ('ios', 'android', 'web', 'desktop')),
    push_token   TEXT,
    last_seen_at TIMESTAMPTZ,
    app_version  TEXT,
    created_at   TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at   TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE (user_id, platform)
);

-- Addition vs. data-architecture.md's table list: required by security.md
-- §2's "refresh token opaco, armazenado hasheado ... revogável".
CREATE TABLE refresh_tokens (
    id           UUID PRIMARY KEY,
    user_id      UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    device_id    UUID REFERENCES user_devices(id) ON DELETE SET NULL,
    token_hash   TEXT NOT NULL UNIQUE,
    issued_at    TIMESTAMPTZ NOT NULL DEFAULT now(),
    expires_at   TIMESTAMPTZ NOT NULL,
    revoked_at   TIMESTAMPTZ,
    replaced_by  UUID REFERENCES refresh_tokens(id),
    created_at   TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at   TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX idx_refresh_tokens_user_id ON refresh_tokens(user_id);

-- 1.2 Catalog -------------------------------------------------------------

CREATE TABLE artists (
    id           UUID PRIMARY KEY,
    name         TEXT NOT NULL,
    slug         TEXT UNIQUE,
    bio          TEXT,
    image_url    TEXT,
    verified     BOOLEAN NOT NULL DEFAULT false,
    external_ids JSONB NOT NULL DEFAULT '{}'::jsonb,
    created_at   TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at   TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX idx_artists_name_trgm ON artists USING GIN (name gin_trgm_ops);

CREATE TABLE albums (
    id                UUID PRIMARY KEY,
    title             TEXT NOT NULL,
    primary_artist_id UUID REFERENCES artists(id) ON DELETE SET NULL,
    release_date      DATE,
    album_type        TEXT CHECK (album_type IN ('album', 'single', 'ep', 'compilation')),
    cover_url         TEXT,
    label             TEXT,
    created_at        TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at        TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX idx_albums_primary_artist_id ON albums(primary_artist_id);
CREATE INDEX idx_albums_title_trgm ON albums USING GIN (title gin_trgm_ops);

CREATE TABLE tracks (
    id                UUID PRIMARY KEY,
    title             TEXT NOT NULL,
    album_id          UUID REFERENCES albums(id) ON DELETE SET NULL,
    duration_ms       INTEGER NOT NULL CHECK (duration_ms > 0),
    track_number      SMALLINT,
    isrc              TEXT UNIQUE,
    explicit          BOOLEAN NOT NULL DEFAULT false,
    popularity_score  REAL NOT NULL DEFAULT 0,
    created_at        TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at        TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX idx_tracks_album_id_track_number ON tracks(album_id, track_number);
CREATE INDEX idx_tracks_title_trgm ON tracks USING GIN (title gin_trgm_ops);

CREATE TABLE track_artists (
    track_id  UUID NOT NULL REFERENCES tracks(id) ON DELETE CASCADE,
    artist_id UUID NOT NULL REFERENCES artists(id) ON DELETE CASCADE,
    role      TEXT NOT NULL CHECK (role IN ('primary', 'featured', 'producer', 'composer')),
    PRIMARY KEY (track_id, artist_id, role)
);
CREATE INDEX idx_track_artists_artist_id ON track_artists(artist_id);

CREATE TABLE track_audio_assets (
    id           UUID PRIMARY KEY,
    track_id     UUID NOT NULL REFERENCES tracks(id) ON DELETE CASCADE,
    storage_uri  TEXT NOT NULL,
    bitrate_kbps INTEGER,
    codec        TEXT CHECK (codec IN ('aac', 'opus', 'flac')),
    quality_tier TEXT CHECK (quality_tier IN ('low', 'normal', 'high', 'lossless')),
    created_at   TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at   TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX idx_track_audio_assets_track_id ON track_audio_assets(track_id);

CREATE TABLE genres (
    id   UUID PRIMARY KEY,
    name TEXT UNIQUE NOT NULL,
    slug TEXT UNIQUE
);

CREATE TABLE track_genres (
    track_id UUID NOT NULL REFERENCES tracks(id) ON DELETE CASCADE,
    genre_id UUID NOT NULL REFERENCES genres(id) ON DELETE CASCADE,
    PRIMARY KEY (track_id, genre_id)
);

-- 1.3 Playlists and biblioteca ---------------------------------------------

CREATE TABLE playlists (
    id          UUID PRIMARY KEY,
    owner_id    UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    title       TEXT NOT NULL,
    description TEXT,
    visibility  TEXT NOT NULL DEFAULT 'private'
                    CHECK (visibility IN ('private', 'unlisted', 'public', 'collaborative')),
    cover_url   TEXT,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at  TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX idx_playlists_owner_id ON playlists(owner_id);

CREATE TABLE playlist_tracks (
    id          UUID PRIMARY KEY,
    playlist_id UUID NOT NULL REFERENCES playlists(id) ON DELETE CASCADE,
    track_id    UUID NOT NULL REFERENCES tracks(id) ON DELETE CASCADE,
    position    NUMERIC NOT NULL,
    added_by    UUID REFERENCES users(id) ON DELETE SET NULL,
    added_at    TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX idx_playlist_tracks_playlist_id_position ON playlist_tracks(playlist_id, position);

CREATE TABLE library_tracks (
    user_id  UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    track_id UUID NOT NULL REFERENCES tracks(id) ON DELETE CASCADE,
    added_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    PRIMARY KEY (user_id, track_id)
);
CREATE INDEX idx_library_tracks_user_added ON library_tracks(user_id, added_at DESC, track_id DESC);

CREATE TABLE library_albums (
    user_id  UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    album_id UUID NOT NULL REFERENCES albums(id) ON DELETE CASCADE,
    added_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    PRIMARY KEY (user_id, album_id)
);

CREATE TABLE library_artists (
    user_id   UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    artist_id UUID NOT NULL REFERENCES artists(id) ON DELETE CASCADE,
    added_at  TIMESTAMPTZ NOT NULL DEFAULT now(),
    PRIMARY KEY (user_id, artist_id)
);

-- 1.4 Histórico de reprodução ----------------------------------------------

CREATE TABLE play_events (
    id           UUID NOT NULL,
    user_id      UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    track_id     UUID NOT NULL REFERENCES tracks(id) ON DELETE CASCADE,
    device_id    UUID REFERENCES user_devices(id) ON DELETE SET NULL,
    played_at    TIMESTAMPTZ NOT NULL,
    ms_played    INTEGER,
    context_type TEXT CHECK (context_type IN ('playlist', 'album', 'radio', 'search', 'nearby_discovery')),
    context_id   UUID,
    PRIMARY KEY (id, played_at)
) PARTITION BY RANGE (played_at);

-- Default partition catches any row outside explicitly created monthly
-- partitions, so writes never fail while waiting on a partition-creation
-- job (TODO, see README) to run.
CREATE TABLE play_events_default PARTITION OF play_events DEFAULT;

CREATE INDEX idx_play_events_user_id_played_at ON play_events(user_id, played_at DESC);

CREATE TABLE user_play_stats (
    user_id       UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    track_id      UUID NOT NULL REFERENCES tracks(id) ON DELETE CASCADE,
    play_count    INTEGER NOT NULL DEFAULT 0,
    last_played_at TIMESTAMPTZ,
    PRIMARY KEY (user_id, track_id)
);

-- 1.5 Assinaturas / planos --------------------------------------------------

CREATE TABLE plans (
    id                 UUID PRIMARY KEY,
    code               TEXT UNIQUE NOT NULL,
    price_cents        INTEGER NOT NULL,
    currency           CHAR(3) NOT NULL,
    max_devices        SMALLINT,
    audio_quality_tier TEXT
);

CREATE TABLE subscriptions (
    id                    UUID PRIMARY KEY,
    user_id               UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    plan_id               UUID NOT NULL REFERENCES plans(id),
    status                TEXT NOT NULL CHECK (status IN ('trialing', 'active', 'past_due', 'canceled', 'expired')),
    current_period_start  TIMESTAMPTZ,
    current_period_end    TIMESTAMPTZ,
    payment_provider      TEXT CHECK (payment_provider IN ('stripe', 'apple_iap', 'google_play')),
    payment_provider_ref  TEXT,
    created_at            TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at            TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX idx_subscriptions_user_id ON subscriptions(user_id);

CREATE TABLE family_plan_members (
    subscription_id UUID NOT NULL REFERENCES subscriptions(id) ON DELETE CASCADE,
    user_id         UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    PRIMARY KEY (subscription_id, user_id)
);

-- 1.6 Relacionamentos sociais (base) ---------------------------------------

CREATE TABLE follows (
    follower_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    followee_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT now(),
    PRIMARY KEY (follower_id, followee_id),
    CHECK (follower_id <> followee_id)
);

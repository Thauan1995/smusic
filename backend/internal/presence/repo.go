package presence

import "context"

// PrivacySettingsRepository persists PrivacySettings (Postgres
// user_privacy_settings, data-architecture.md §4.5).
type PrivacySettingsRepository interface {
	// Get returns ErrSettingsNotFound if userID has never had a row
	// written (callers should treat that the same as DefaultPrivacySettings,
	// never as an error condition toward the caller of the service layer).
	Get(ctx context.Context, userID string) (PrivacySettings, error)
	// Upsert creates or replaces the full settings row for s.UserID.
	Upsert(ctx context.Context, s PrivacySettings) error
}

// BlockRepository persists and queries the block list (Postgres
// user_blocks, security.md §1.4).
type BlockRepository interface {
	Block(ctx context.Context, blockerID, blockedID string) error
	// Unblock is idempotent: unblocking a pair that isn't blocked is not
	// an error.
	Unblock(ctx context.Context, blockerID, blockedID string) error
	// IsBlockedEitherWay reports whether a blocked b OR b blocked a —
	// security.md §1.4: blocking is symmetric in effect ("o bloqueado
	// nunca aparece para o bloqueador nem vice-versa").
	IsBlockedEitherWay(ctx context.Context, a, b string) (bool, error)
}

// FollowChecker answers the "are these two users mutual connections"
// question the reveal-level (security.md §1.6) and friends_only visibility
// (security.md §1.4/data-architecture.md §1.6) rules depend on. Implemented
// in internal/presence/postgres against the existing `follows` table
// (data-architecture.md §1.6: "base for the presence feature").
type FollowChecker interface {
	// IsMutualFollow reports whether a follows b AND b follows a.
	IsMutualFollow(ctx context.Context, a, b string) (bool, error)
}

// AuditLogRepository appends to the presence access audit log
// (security.md §1.8, Postgres presence_audit_log — append-only, enforced by
// a DB trigger, see migrations/0002_presence.up.sql). There is
// deliberately no Read/List method on this interface: this slice exposes
// no endpoint for reading the audit log to anyone (a Trust & Safety tool
// is explicitly out of scope here), so the interface the rest of the
// application depends on shouldn't even offer the capability to wire up
// such an endpoint by accident.
type AuditLogRepository interface {
	Append(ctx context.Context, entry AuditLogEntry) error
}

// MFAChecker answers "does this user have a verified second factor" —
// security.md §2's hard requirement before proximity consent can be
// granted (see SettingsService.GrantConsent). Implemented by *auth.Service
// (HasVerifiedMFA), wired only in cmd/server/main.go per backend-go.md
// §1's module-boundary rule: presence never imports auth or touches its
// tables directly.
type MFAChecker interface {
	HasVerifiedMFA(ctx context.Context, userID string) (bool, error)
}

// Profile is the minimal identity information the presence module needs to
// render a Nível 1 (security.md §1.6) result — display name and avatar,
// nothing else. Deliberately not User itself: presence must never depend on
// (or be able to leak) any field of auth.User beyond what proximity
// disclosure actually needs.
type Profile struct {
	DisplayName string
	AvatarURL   string
}

// ProfileResolver hydrates identity for a small, already-privacy-filtered
// set of user IDs. Presence never queries this for a candidate that hasn't
// already passed every other check (block, consent, radius, rate limit) —
// data-architecture.md §4.3: "minimizando quantos perfis completos são
// 'tocados' por consulta." Implemented by an adapter in cmd/server (wired
// against auth.Service), per backend-go.md §1's module-boundary rule:
// presence doesn't import auth or touch its tables directly.
type ProfileResolver interface {
	Resolve(ctx context.Context, userIDs []string) (map[string]Profile, error)
}

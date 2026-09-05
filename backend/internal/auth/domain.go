// Package auth implements signup/login/refresh/logout (security.md §2) as a
// self-contained domain module, per backend-go.md §1's package-boundary
// rule: other modules never reach into auth's tables directly, only
// through the interfaces this package (or its api subpackage) exposes.
package auth

import (
	"errors"
	"time"
)

// User statuses, mirroring data-architecture.md §1.1's users.status enum.
const (
	UserStatusActive    = "active"
	UserStatusSuspended = "suspended"
	UserStatusDeleted   = "deleted"
)

// Roles (.vibeflow/specs/catalog-write-authorization.md). RoleUser is
// every account's default; RoleCatalogCurator is granted manually (no
// admin UI in this slice — see the spec's anti-scope) and is currently
// the only gate beyond RoleUser, checked by
// internal/platform/middleware.RequireRole via Service.HasRole.
const (
	RoleUser           = "user"
	RoleCatalogCurator = "catalog_curator"
)

// User is the auth module's view of a user: enough to authenticate and
// identify, not the full social/profile record (that's a product-facing
// concern other modules can layer on top via the same user_id).
type User struct {
	ID           string
	Email        string
	PasswordHash string // empty if the user only ever authenticated via OAuth
	DisplayName  string
	Handle       string
	Status       string
	Role         string // RoleUser or RoleCatalogCurator; defaults to RoleUser at the DB level
	CreatedAt    time.Time
	UpdatedAt    time.Time
}

// Identity links a third-party OAuth/OIDC subject to a User
// (data-architecture.md §1.1 user_auth_identities).
type Identity struct {
	ID             string
	UserID         string
	Provider       string
	ProviderUserID string
	CreatedAt      time.Time
}

// Device is a client device a user has authenticated from
// (data-architecture.md §1.1 user_devices). Refresh tokens are optionally
// bound to a device so "log out of this device" (security.md §2) is
// possible.
type Device struct {
	ID         string
	UserID     string
	Platform   string
	PushToken  string
	AppVersion string
	LastSeenAt time.Time
}

// RefreshToken is the persisted (hashed) record backing an opaque refresh
// token (security.md §2). The plaintext is returned to the client exactly
// once, at issuance, and never stored.
type RefreshToken struct {
	ID         string
	UserID     string
	DeviceID   string // "" if not tied to a specific device
	TokenHash  string
	IssuedAt   time.Time
	ExpiresAt  time.Time
	RevokedAt  *time.Time
	ReplacedBy string // "" until rotated
}

// Sentinel errors. Handlers map these to HTTP status codes; service tests
// assert on them directly via errors.Is (backend-go.md §7).
var (
	ErrInvalidInput        = errors.New("auth: invalid input")
	ErrEmailTaken          = errors.New("auth: email already registered")
	ErrInvalidCredentials  = errors.New("auth: invalid email or password")
	ErrUserNotFound        = errors.New("auth: user not found")
	ErrInvalidRefreshToken = errors.New("auth: invalid refresh token")
	ErrRefreshTokenExpired = errors.New("auth: refresh token expired")
	ErrRefreshTokenRevoked = errors.New("auth: refresh token revoked")
	ErrRefreshTokenReused  = errors.New("auth: refresh token reuse detected, session revoked")
	ErrIdentityNotLinked   = errors.New("auth: oauth identity not linked to any user")
)

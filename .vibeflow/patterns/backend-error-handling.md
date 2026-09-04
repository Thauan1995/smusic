---
tags: [go, error-handling, sentinel-errors, http-status-mapping]
modules: [backend/internal/auth/, backend/internal/presence/, backend/internal/catalog/, backend/internal/library/, backend/internal/playback/]
applies_to: [services, handlers]
confidence: inferred
---
# Pattern: Sentinel errors + centralized HTTP status mapping

<!-- vibeflow:auto:start -->
## What
Every module declares a fixed set of sentinel errors in `domain.go` (`var Err... = errors.New(...)`). Services return/wrap these with `fmt.Errorf("%w: ...", ErrX)`; callers and tests check with `errors.Is`. Each module's `api` package has exactly one `writeXError(w, err)` function that is the single place translating sentinel errors to HTTP status codes — domain/service code never imports `net/http` or knows about status codes.

## Where
Every `domain.go` (error declarations) and every `api/handlers.go` (the `writeXError` switch) across `auth`, `catalog`, `library`, `playback`, `presence`.

## The Pattern
```go
// domain.go
var (
	ErrInvalidInput       = errors.New("auth: invalid input")
	ErrEmailTaken         = errors.New("auth: email already registered")
	ErrInvalidCredentials = errors.New("auth: invalid email or password")
)
```
```go
// service.go — wrap with context, never swallow
if len(in.Password) < 8 {
	return AuthResult{}, fmt.Errorf("%w: password must be at least 8 characters", ErrInvalidInput)
}
```
```go
// api/handlers.go — the ONLY place that maps errors to status codes
func writeAuthError(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, auth.ErrInvalidInput):
		httpx.WriteError(w, http.StatusBadRequest, "invalid_input", err.Error())
	case errors.Is(err, auth.ErrEmailTaken):
		httpx.WriteError(w, http.StatusConflict, "email_taken", "email already registered")
	...
	default:
		httpx.WriteError(w, http.StatusInternalServerError, "internal_error", "internal error")
	}
}
```

## Rules
- No panics in business logic (explicitly documented, e.g. `presence/domain.go`: "no panics anywhere in this package").
- Tests assert on errors via `errors.Is`, never string-matching `err.Error()`.
- A default `case` always maps to `500 internal_error` with a generic message — internal error details are never leaked to the client (defense against information disclosure).
- Security-sensitive errors are deliberately merged: `Login` returns the *same* `ErrInvalidCredentials` for "user not found" and "wrong password" so the API never reveals whether an email is registered (`auth/service.go` comment, security.md §2/§5 account-takeover threat model). Apply the same "don't leak existence" principle to any new auth-adjacent error path.
- `Logout` treats an unknown/already-revoked refresh token as success (idempotent), not an error — never leak whether a token ever existed.

## Examples from this codebase
File: `backend/internal/presence/domain.go`
```go
var (
	ErrConsentRequired    = errors.New("presence: proximity consent not granted")
	ErrConsentExpired     = errors.New("presence: proximity consent expired, renewal required")
	ErrIngestSaturated    = errors.New("presence: ingest pipeline saturated, slow down update frequency")
)
```

File: `backend/internal/auth/service.go` (existence-hiding pattern)
```go
user, err := s.users.GetByEmail(ctx, email)
if err != nil {
	if errors.Is(err, ErrUserNotFound) {
		// Deliberately identical error to "wrong password" below.
		return AuthResult{}, ErrInvalidCredentials
	}
	...
}
```
<!-- vibeflow:auto:end -->

## Anti-patterns
None found in the modules sampled.

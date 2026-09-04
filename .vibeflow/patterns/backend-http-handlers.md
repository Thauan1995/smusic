---
tags: [go, http, chi, handlers, rest]
modules: [backend/internal/auth/api/, backend/internal/catalog/api/, backend/internal/library/api/, backend/internal/playback/api/, backend/internal/presence/api/]
applies_to: [handlers, routes]
confidence: inferred
---
# Pattern: Thin chi HTTP handlers

<!-- vibeflow:auto:start -->
## What
Every module's `api` package exposes a `Handler` struct wrapping a narrow, locally-declared `Service` interface (not the concrete service type), a `NewHandler(svc Service) *Handler` constructor, and a `Mount(r chi.Router, authr middleware.Authenticator, ...)` method that registers `/v1/<module>/...` routes with `chimw`/custom middleware groups. Handlers do decode → call service → map response, nothing else.

## Where
`backend/internal/*/api/handlers.go`. Routes are assembled centrally in `backend/cmd/server/main.go`'s `buildRouter`.

## The Pattern
```go
type Service interface { // re-declared narrow interface, not the concrete *auth.Service
	SignUp(ctx context.Context, in auth.SignUpInput) (auth.AuthResult, error)
	...
}

type Handler struct{ svc Service }

func NewHandler(svc Service) *Handler { return &Handler{svc: svc} }

func (h *Handler) Mount(r chi.Router, authr middleware.Authenticator, loginRateLimit func(http.Handler) http.Handler) {
	r.Group(func(r chi.Router) {
		if loginRateLimit != nil {
			r.Use(loginRateLimit)
		}
		r.Post("/v1/auth/signup", h.signUp)
		r.Post("/v1/auth/login", h.login)
	})
	r.Group(func(r chi.Router) {
		r.Use(middleware.RequireAuth(authr))
		r.Get("/v1/auth/me", h.me)
	})
}

func (h *Handler) signUp(w http.ResponseWriter, r *http.Request) {
	var req credentialsRequest
	if err := httpx.DecodeJSON(r, &req); err != nil {
		httpx.WriteError(w, http.StatusBadRequest, "invalid_request", err.Error())
		return
	}
	result, err := h.svc.SignUp(r.Context(), auth.SignUpInput{...})
	if err != nil {
		writeAuthError(w, err)
		return
	}
	httpx.WriteJSON(w, http.StatusCreated, toAuthResponse(result))
}
```
Route table (all under `/v1/`, mounted in `main.go#buildRouter`):
`auth`: `POST /signup`, `POST /login`, `POST /refresh`, `POST /logout`, `GET /me` (auth), `POST /logout-all` (auth).
`catalog`: `GET /search`, `GET /artists/{id}`, `GET /albums/{id}`, `GET /tracks/{id}` (public); `POST /artists|albums|tracks` (auth — no role check found, see Anti-patterns).
`library`: playlists + saved-tracks CRUD, all under `/v1/library/me/...` (auth).
`playback`: session create/state/play/pause/seek/next/queue (auth).
`presence`: settings/consent/pause/resume/block (auth) — the real-time nearby feed itself is WebSocket, not REST (see `internal/presence/ws/`).

## Rules
- Request/response DTOs (`credentialsRequest`, `authResponse`, etc.) are private structs local to `api/handlers.go`, converted to/from domain types via small `toX`/`.toInput()` helpers — domain types never carry `json` tags themselves.
- Auth-required routes are always wrapped in `r.Group(func(r chi.Router) { r.Use(middleware.RequireAuth(authr)); ... })`, never an ad-hoc per-handler check.
- `middleware.UserID(r.Context())` is the only way a handler reads the authenticated caller; never re-parse the Authorization header in a handler.
- Rate limiting (when applicable) is applied as router middleware (`loginRateLimit`), not inside the handler body.

## Examples from this codebase
File: `backend/internal/auth/api/handlers.go:51-67` (Mount, shown above)
File: `backend/internal/presence/api/handlers.go:51-58` — same `r.Group` + `RequireAuth` shape for every presence settings/consent route.
<!-- vibeflow:auto:end -->

## Anti-patterns
- `POST /v1/catalog/artists|albums|tracks` (`backend/internal/catalog/api/handlers.go:55-57`) is gated only by `RequireAuth` (any logged-in user), with no visible role/admin check in the handler or service layer sampled. For a production catalog-ingestion endpoint this is a likely gap — any authenticated user can currently create catalog entries. Flag for the security spec.

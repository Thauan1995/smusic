package api

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/go-chi/chi/v5"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"smusic/backend/internal/auth"
	"smusic/backend/internal/auth/oauth"
)

type fakeService struct {
	signUpFn     func(ctx context.Context, in auth.SignUpInput) (auth.AuthResult, error)
	loginFn      func(ctx context.Context, in auth.LoginInput) (auth.AuthResult, error)
	loginOAuthFn func(ctx context.Context, provider oauth.Provider, idToken, displayName string, device *auth.DeviceInput) (auth.AuthResult, error)
	refreshFn    func(ctx context.Context, refreshToken string) (auth.AuthResult, error)
	logoutFn     func(ctx context.Context, refreshToken string) error
	logoutAllFn  func(ctx context.Context, userID string) error
	meFn         func(ctx context.Context, userID string) (auth.User, error)
}

func (f *fakeService) SignUp(ctx context.Context, in auth.SignUpInput) (auth.AuthResult, error) {
	return f.signUpFn(ctx, in)
}
func (f *fakeService) Login(ctx context.Context, in auth.LoginInput) (auth.AuthResult, error) {
	return f.loginFn(ctx, in)
}
func (f *fakeService) LoginWithOAuth(ctx context.Context, provider oauth.Provider, idToken, displayName string, device *auth.DeviceInput) (auth.AuthResult, error) {
	return f.loginOAuthFn(ctx, provider, idToken, displayName, device)
}
func (f *fakeService) Refresh(ctx context.Context, refreshToken string) (auth.AuthResult, error) {
	return f.refreshFn(ctx, refreshToken)
}
func (f *fakeService) Logout(ctx context.Context, refreshToken string) error {
	return f.logoutFn(ctx, refreshToken)
}
func (f *fakeService) LogoutAll(ctx context.Context, userID string) error {
	return f.logoutAllFn(ctx, userID)
}
func (f *fakeService) Me(ctx context.Context, userID string) (auth.User, error) {
	return f.meFn(ctx, userID)
}

type fakeAuthenticator struct{ userID string }

func (f fakeAuthenticator) Authenticate(token string) (string, error) {
	if token != "valid" {
		return "", errors.New("invalid")
	}
	return f.userID, nil
}

func newTestRouter(svc *fakeService) chi.Router {
	r := chi.NewRouter()
	NewHandler(svc).Mount(r, fakeAuthenticator{userID: "user-1"}, nil)
	return r
}

func doRequest(r chi.Router, method, path, body string, headers map[string]string) *httptest.ResponseRecorder {
	req := httptest.NewRequest(method, path, strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	for k, v := range headers {
		req.Header.Set(k, v)
	}
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	return w
}

func TestSignUp_Success(t *testing.T) {
	svc := &fakeService{signUpFn: func(ctx context.Context, in auth.SignUpInput) (auth.AuthResult, error) {
		assert.Equal(t, "a@b.com", in.Email)
		require.NotNil(t, in.Device)
		assert.Equal(t, "ios", in.Device.Platform)
		return auth.AuthResult{UserID: "u1", AccessToken: "at", RefreshToken: "rt"}, nil
	}}
	w := doRequest(newTestRouter(svc), http.MethodPost, "/v1/auth/signup",
		`{"email":"a@b.com","password":"supersecret","display_name":"A","device":{"platform":"ios","app_version":"1.0"}}`, nil)

	assert.Equal(t, http.StatusCreated, w.Code)
	assert.Contains(t, w.Body.String(), `"user_id":"u1"`)
}

func TestSignUp_NoDevice(t *testing.T) {
	svc := &fakeService{signUpFn: func(ctx context.Context, in auth.SignUpInput) (auth.AuthResult, error) {
		assert.Nil(t, in.Device)
		return auth.AuthResult{UserID: "u1"}, nil
	}}
	w := doRequest(newTestRouter(svc), http.MethodPost, "/v1/auth/signup", `{"email":"a@b.com","password":"supersecret","display_name":"A"}`, nil)
	assert.Equal(t, http.StatusCreated, w.Code)
}

func TestSignUp_InvalidBody(t *testing.T) {
	svc := &fakeService{}
	w := doRequest(newTestRouter(svc), http.MethodPost, "/v1/auth/signup", `not json`, nil)
	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestSignUp_ServiceError(t *testing.T) {
	svc := &fakeService{signUpFn: func(ctx context.Context, in auth.SignUpInput) (auth.AuthResult, error) {
		return auth.AuthResult{}, auth.ErrEmailTaken
	}}
	w := doRequest(newTestRouter(svc), http.MethodPost, "/v1/auth/signup", `{"email":"a@b.com","password":"supersecret","display_name":"A"}`, nil)
	assert.Equal(t, http.StatusConflict, w.Code)
}

func TestSignUp_OAuthDelegates(t *testing.T) {
	svc := &fakeService{loginOAuthFn: func(ctx context.Context, provider oauth.Provider, idToken, displayName string, device *auth.DeviceInput) (auth.AuthResult, error) {
		assert.Equal(t, oauth.Provider("google"), provider)
		assert.Equal(t, "tok", idToken)
		return auth.AuthResult{UserID: "u1"}, nil
	}}
	w := doRequest(newTestRouter(svc), http.MethodPost, "/v1/auth/signup", `{"oauth_provider":"google","oauth_token":"tok"}`, nil)
	assert.Equal(t, http.StatusOK, w.Code)
}

func TestMount_AppliesLoginRateLimitOnlyToSignupAndLogin(t *testing.T) {
	svc := &fakeService{
		signUpFn: func(ctx context.Context, in auth.SignUpInput) (auth.AuthResult, error) { return auth.AuthResult{}, nil },
		loginFn:  func(ctx context.Context, in auth.LoginInput) (auth.AuthResult, error) { return auth.AuthResult{}, nil },
		refreshFn: func(ctx context.Context, refreshToken string) (auth.AuthResult, error) {
			return auth.AuthResult{}, nil
		},
	}
	var calls []string
	limiter := func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			calls = append(calls, r.URL.Path)
			next.ServeHTTP(w, r)
		})
	}
	r := chi.NewRouter()
	NewHandler(svc).Mount(r, fakeAuthenticator{userID: "user-1"}, limiter)

	doRequest(r, http.MethodPost, "/v1/auth/signup", `{"email":"a@b.com","password":"supersecret","display_name":"A"}`, nil)
	doRequest(r, http.MethodPost, "/v1/auth/login", `{"email":"a@b.com","password":"x"}`, nil)
	doRequest(r, http.MethodPost, "/v1/auth/refresh", `{"refresh_token":"rt"}`, nil)

	assert.ElementsMatch(t, []string{"/v1/auth/signup", "/v1/auth/login"}, calls, "rate limit must apply to signup/login only, never refresh")
}

func TestSignUp_OAuthMissingProvider(t *testing.T) {
	svc := &fakeService{}
	w := doRequest(newTestRouter(svc), http.MethodPost, "/v1/auth/signup", `{"oauth_token":"tok"}`, nil)
	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestLogin_Success(t *testing.T) {
	svc := &fakeService{loginFn: func(ctx context.Context, in auth.LoginInput) (auth.AuthResult, error) {
		return auth.AuthResult{UserID: "u1"}, nil
	}}
	w := doRequest(newTestRouter(svc), http.MethodPost, "/v1/auth/login", `{"email":"a@b.com","password":"x"}`, nil)
	assert.Equal(t, http.StatusOK, w.Code)
}

func TestLogin_InvalidBody(t *testing.T) {
	svc := &fakeService{}
	w := doRequest(newTestRouter(svc), http.MethodPost, "/v1/auth/login", `{`, nil)
	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestLogin_InvalidCredentials(t *testing.T) {
	svc := &fakeService{loginFn: func(ctx context.Context, in auth.LoginInput) (auth.AuthResult, error) {
		return auth.AuthResult{}, auth.ErrInvalidCredentials
	}}
	w := doRequest(newTestRouter(svc), http.MethodPost, "/v1/auth/login", `{"email":"a@b.com","password":"x"}`, nil)
	assert.Equal(t, http.StatusUnauthorized, w.Code)
}

func TestLogin_OAuthDelegates(t *testing.T) {
	svc := &fakeService{loginOAuthFn: func(ctx context.Context, provider oauth.Provider, idToken, displayName string, device *auth.DeviceInput) (auth.AuthResult, error) {
		return auth.AuthResult{}, oauth.ErrNotImplemented
	}}
	w := doRequest(newTestRouter(svc), http.MethodPost, "/v1/auth/login", `{"oauth_provider":"apple","oauth_token":"tok"}`, nil)
	assert.Equal(t, http.StatusNotImplemented, w.Code)
}

func TestRefresh_Success(t *testing.T) {
	svc := &fakeService{refreshFn: func(ctx context.Context, refreshToken string) (auth.AuthResult, error) {
		assert.Equal(t, "rt", refreshToken)
		return auth.AuthResult{UserID: "u1"}, nil
	}}
	w := doRequest(newTestRouter(svc), http.MethodPost, "/v1/auth/refresh", `{"refresh_token":"rt"}`, nil)
	assert.Equal(t, http.StatusOK, w.Code)
}

func TestRefresh_InvalidBody(t *testing.T) {
	svc := &fakeService{}
	w := doRequest(newTestRouter(svc), http.MethodPost, "/v1/auth/refresh", `{`, nil)
	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestRefresh_Error(t *testing.T) {
	svc := &fakeService{refreshFn: func(ctx context.Context, refreshToken string) (auth.AuthResult, error) {
		return auth.AuthResult{}, auth.ErrRefreshTokenExpired
	}}
	w := doRequest(newTestRouter(svc), http.MethodPost, "/v1/auth/refresh", `{"refresh_token":"rt"}`, nil)
	assert.Equal(t, http.StatusUnauthorized, w.Code)
}

func TestLogout_Success(t *testing.T) {
	svc := &fakeService{logoutFn: func(ctx context.Context, refreshToken string) error { return nil }}
	w := doRequest(newTestRouter(svc), http.MethodPost, "/v1/auth/logout", `{"refresh_token":"rt"}`, nil)
	assert.Equal(t, http.StatusNoContent, w.Code)
}

func TestLogout_InvalidBody(t *testing.T) {
	svc := &fakeService{}
	w := doRequest(newTestRouter(svc), http.MethodPost, "/v1/auth/logout", `{`, nil)
	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestLogout_Error(t *testing.T) {
	svc := &fakeService{logoutFn: func(ctx context.Context, refreshToken string) error { return errors.New("boom") }}
	w := doRequest(newTestRouter(svc), http.MethodPost, "/v1/auth/logout", `{"refresh_token":"rt"}`, nil)
	assert.Equal(t, http.StatusInternalServerError, w.Code)
}

func TestMe_RequiresAuth(t *testing.T) {
	svc := &fakeService{}
	w := doRequest(newTestRouter(svc), http.MethodGet, "/v1/auth/me", "", nil)
	assert.Equal(t, http.StatusUnauthorized, w.Code)
}

func TestMe_Success(t *testing.T) {
	svc := &fakeService{meFn: func(ctx context.Context, userID string) (auth.User, error) {
		assert.Equal(t, "user-1", userID)
		return auth.User{ID: "user-1", Email: "a@b.com", DisplayName: "A", Status: auth.UserStatusActive}, nil
	}}
	w := doRequest(newTestRouter(svc), http.MethodGet, "/v1/auth/me", "", map[string]string{"Authorization": "Bearer valid"})
	assert.Equal(t, http.StatusOK, w.Code)
	assert.Contains(t, w.Body.String(), `"email":"a@b.com"`)
}

func TestMe_NotFound(t *testing.T) {
	svc := &fakeService{meFn: func(ctx context.Context, userID string) (auth.User, error) {
		return auth.User{}, auth.ErrUserNotFound
	}}
	w := doRequest(newTestRouter(svc), http.MethodGet, "/v1/auth/me", "", map[string]string{"Authorization": "Bearer valid"})
	assert.Equal(t, http.StatusNotFound, w.Code)
}

func TestLogoutAll_Success(t *testing.T) {
	svc := &fakeService{logoutAllFn: func(ctx context.Context, userID string) error {
		assert.Equal(t, "user-1", userID)
		return nil
	}}
	w := doRequest(newTestRouter(svc), http.MethodPost, "/v1/auth/logout-all", "", map[string]string{"Authorization": "Bearer valid"})
	assert.Equal(t, http.StatusNoContent, w.Code)
}

func TestLogoutAll_RequiresAuth(t *testing.T) {
	svc := &fakeService{}
	w := doRequest(newTestRouter(svc), http.MethodPost, "/v1/auth/logout-all", "", nil)
	assert.Equal(t, http.StatusUnauthorized, w.Code)
}

func TestLogoutAll_Error(t *testing.T) {
	svc := &fakeService{logoutAllFn: func(ctx context.Context, userID string) error { return errors.New("boom") }}
	w := doRequest(newTestRouter(svc), http.MethodPost, "/v1/auth/logout-all", "", map[string]string{"Authorization": "Bearer valid"})
	assert.Equal(t, http.StatusInternalServerError, w.Code)
}

func TestWriteAuthError_AllBranches(t *testing.T) {
	cases := []struct {
		err    error
		status int
	}{
		{auth.ErrInvalidInput, http.StatusBadRequest},
		{auth.ErrEmailTaken, http.StatusConflict},
		{auth.ErrInvalidCredentials, http.StatusUnauthorized},
		{auth.ErrUserNotFound, http.StatusNotFound},
		{auth.ErrInvalidRefreshToken, http.StatusUnauthorized},
		{auth.ErrRefreshTokenExpired, http.StatusUnauthorized},
		{auth.ErrRefreshTokenRevoked, http.StatusUnauthorized},
		{auth.ErrRefreshTokenReused, http.StatusUnauthorized},
		{oauth.ErrNotImplemented, http.StatusNotImplemented},
		{oauth.ErrUnsupportedProvider, http.StatusBadRequest},
		{errors.New("unmapped"), http.StatusInternalServerError},
	}
	for _, tc := range cases {
		svc := &fakeService{meFn: func(ctx context.Context, userID string) (auth.User, error) {
			return auth.User{}, tc.err
		}}
		w := doRequest(newTestRouter(svc), http.MethodGet, "/v1/auth/me", "", map[string]string{"Authorization": "Bearer valid"})
		require.Equal(t, tc.status, w.Code, "err=%v", tc.err)
	}
}

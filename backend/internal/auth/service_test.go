package auth

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"smusic/backend/internal/auth/oauth"
	"smusic/backend/internal/platform/clock"
	"smusic/backend/internal/platform/idgen"
)

const refreshTTL = 30 * 24 * time.Hour

type deps struct {
	users         *fakeUserRepo
	identities    *fakeIdentityRepo
	devices       *fakeDeviceRepo
	refreshTokens *fakeRefreshTokenRepo
	hasher        *fakeHasher
	signer        *fakeSigner
	refreshGen    *fakeRefreshGen
	oauthV        *fakeOAuthVerifier
	mfa           *fakeMFAProvider
	clock         *clock.Frozen
}

func newTestService(t *testing.T) (*Service, *deps) {
	t.Helper()
	clk := clock.NewFrozen(time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC))
	d := &deps{
		users:         newFakeUserRepo(),
		identities:    newFakeIdentityRepo(),
		devices:       &fakeDeviceRepo{},
		refreshTokens: newFakeRefreshTokenRepo(),
		hasher:        &fakeHasher{verifyOK: true},
		signer:        &fakeSigner{clock: clk, ttl: 15 * time.Minute},
		refreshGen:    &fakeRefreshGen{},
		oauthV:        &fakeOAuthVerifier{},
		mfa:           newFakeMFAProvider(),
		clock:         clk,
	}
	svc := NewService(d.users, d.identities, d.devices, d.refreshTokens, d.hasher, d.signer, d.refreshGen, d.oauthV, d.mfa, clk, idgen.NewSequential("id"), refreshTTL)
	return svc, d
}

var errBoom = errors.New("boom")

// --- SignUp ---

func TestSignUp_Success(t *testing.T) {
	svc, d := newTestService(t)

	result, err := svc.SignUp(context.Background(), SignUpInput{
		Email: " Alice@Example.com ", Password: "supersecret", DisplayName: "Alice",
	})
	require.NoError(t, err)
	assert.NotEmpty(t, result.UserID)
	assert.NotEmpty(t, result.AccessToken)
	assert.NotEmpty(t, result.RefreshToken)
	assert.Equal(t, d.clock.Now().Add(refreshTTL), result.RefreshTokenExpiresAt)

	stored, err := d.users.GetByEmail(context.Background(), "alice@example.com")
	require.NoError(t, err)
	assert.Equal(t, result.UserID, stored.ID)
	assert.Equal(t, "Alice", stored.DisplayName)
	assert.Equal(t, UserStatusActive, stored.Status)
}

func TestSignUp_MissingEmail(t *testing.T) {
	svc, _ := newTestService(t)
	_, err := svc.SignUp(context.Background(), SignUpInput{Password: "supersecret", DisplayName: "A"})
	assert.ErrorIs(t, err, ErrInvalidInput)
}

func TestSignUp_ShortPassword(t *testing.T) {
	svc, _ := newTestService(t)
	_, err := svc.SignUp(context.Background(), SignUpInput{Email: "a@b.com", Password: "short", DisplayName: "A"})
	assert.ErrorIs(t, err, ErrInvalidInput)
}

func TestSignUp_MissingDisplayName(t *testing.T) {
	svc, _ := newTestService(t)
	_, err := svc.SignUp(context.Background(), SignUpInput{Email: "a@b.com", Password: "supersecret"})
	assert.ErrorIs(t, err, ErrInvalidInput)
}

func TestSignUp_DuplicateEmail(t *testing.T) {
	svc, _ := newTestService(t)
	in := SignUpInput{Email: "a@b.com", Password: "supersecret", DisplayName: "A"}
	_, err := svc.SignUp(context.Background(), in)
	require.NoError(t, err)

	_, err = svc.SignUp(context.Background(), in)
	assert.ErrorIs(t, err, ErrEmailTaken)
}

func TestSignUp_HasherError(t *testing.T) {
	svc, d := newTestService(t)
	d.hasher.hashErr = errBoom
	_, err := svc.SignUp(context.Background(), SignUpInput{Email: "a@b.com", Password: "supersecret", DisplayName: "A"})
	require.Error(t, err)
	assert.False(t, errors.Is(err, ErrInvalidInput))
}

func TestSignUp_CreateUserOtherError(t *testing.T) {
	svc, d := newTestService(t)
	d.users.createErr = errBoom
	_, err := svc.SignUp(context.Background(), SignUpInput{Email: "a@b.com", Password: "supersecret", DisplayName: "A"})
	require.Error(t, err)
	assert.False(t, errors.Is(err, ErrEmailTaken))
}

func TestSignUp_WithDevice(t *testing.T) {
	svc, d := newTestService(t)
	_, err := svc.SignUp(context.Background(), SignUpInput{
		Email: "a@b.com", Password: "supersecret", DisplayName: "A",
		Device: &DeviceInput{Platform: "ios", AppVersion: "1.0"},
	})
	require.NoError(t, err)
	assert.Equal(t, "ios", d.devices.last.Platform)
	assert.Equal(t, 1, d.devices.n)
}

func TestSignUp_DeviceUpsertError(t *testing.T) {
	svc, d := newTestService(t)
	d.devices.upsertErr = errBoom
	_, err := svc.SignUp(context.Background(), SignUpInput{
		Email: "a@b.com", Password: "supersecret", DisplayName: "A",
		Device: &DeviceInput{Platform: "ios"},
	})
	require.Error(t, err)
}

func TestSignUp_RefreshGenError(t *testing.T) {
	svc, d := newTestService(t)
	d.refreshGen.newErr = errBoom
	_, err := svc.SignUp(context.Background(), SignUpInput{Email: "a@b.com", Password: "supersecret", DisplayName: "A"})
	require.Error(t, err)
}

func TestSignUp_SignerError(t *testing.T) {
	svc, d := newTestService(t)
	d.signer.signErr = errBoom
	_, err := svc.SignUp(context.Background(), SignUpInput{Email: "a@b.com", Password: "supersecret", DisplayName: "A"})
	require.Error(t, err)
}

func TestSignUp_StoreRefreshTokenError(t *testing.T) {
	svc, d := newTestService(t)
	d.refreshTokens.storeErr = errBoom
	_, err := svc.SignUp(context.Background(), SignUpInput{Email: "a@b.com", Password: "supersecret", DisplayName: "A"})
	require.Error(t, err)
}

// --- Login ---

func TestLogin_Success(t *testing.T) {
	svc, _ := newTestService(t)
	_, err := svc.SignUp(context.Background(), SignUpInput{Email: "a@b.com", Password: "supersecret", DisplayName: "A"})
	require.NoError(t, err)

	result, err := svc.Login(context.Background(), LoginInput{Email: "A@B.com", Password: "supersecret"})
	require.NoError(t, err)
	assert.NotEmpty(t, result.AccessToken)
}

func TestLogin_MissingCredentials(t *testing.T) {
	svc, _ := newTestService(t)
	_, err := svc.Login(context.Background(), LoginInput{Email: "", Password: ""})
	assert.ErrorIs(t, err, ErrInvalidInput)
}

func TestLogin_UnknownEmail(t *testing.T) {
	svc, _ := newTestService(t)
	_, err := svc.Login(context.Background(), LoginInput{Email: "nope@b.com", Password: "supersecret"})
	assert.ErrorIs(t, err, ErrInvalidCredentials)
}

func TestLogin_WrongPassword(t *testing.T) {
	svc, d := newTestService(t)
	_, err := svc.SignUp(context.Background(), SignUpInput{Email: "a@b.com", Password: "supersecret", DisplayName: "A"})
	require.NoError(t, err)

	d.hasher.verifyOK = false
	_, err = svc.Login(context.Background(), LoginInput{Email: "a@b.com", Password: "wrong"})
	assert.ErrorIs(t, err, ErrInvalidCredentials)
}

func TestLogin_InactiveUser(t *testing.T) {
	svc, d := newTestService(t)
	now := d.clock.Now()
	require.NoError(t, d.users.Create(context.Background(), User{
		ID: "u1", Email: "a@b.com", PasswordHash: "hashed:x", DisplayName: "A",
		Status: UserStatusSuspended, CreatedAt: now, UpdatedAt: now,
	}))

	_, err := svc.Login(context.Background(), LoginInput{Email: "a@b.com", Password: "x"})
	assert.ErrorIs(t, err, ErrInvalidCredentials)
}

func TestLogin_OAuthOnlyUserNoPassword(t *testing.T) {
	svc, d := newTestService(t)
	now := d.clock.Now()
	require.NoError(t, d.users.Create(context.Background(), User{
		ID: "u1", Email: "a@b.com", PasswordHash: "", DisplayName: "A",
		Status: UserStatusActive, CreatedAt: now, UpdatedAt: now,
	}))

	_, err := svc.Login(context.Background(), LoginInput{Email: "a@b.com", Password: "x"})
	assert.ErrorIs(t, err, ErrInvalidCredentials)
}

func TestLogin_GetByEmailOtherError(t *testing.T) {
	svc, d := newTestService(t)
	d.users.getByEmailErr = errBoom
	_, err := svc.Login(context.Background(), LoginInput{Email: "a@b.com", Password: "x"})
	require.Error(t, err)
	assert.False(t, errors.Is(err, ErrInvalidCredentials))
}

func TestLogin_VerifyError(t *testing.T) {
	svc, d := newTestService(t)
	_, err := svc.SignUp(context.Background(), SignUpInput{Email: "a@b.com", Password: "supersecret", DisplayName: "A"})
	require.NoError(t, err)

	d.hasher.verifyErr = errBoom
	_, err = svc.Login(context.Background(), LoginInput{Email: "a@b.com", Password: "supersecret"})
	require.Error(t, err)
	assert.False(t, errors.Is(err, ErrInvalidCredentials))
}

// --- LoginWithOAuth ---

func TestLoginWithOAuth_MissingToken(t *testing.T) {
	svc, _ := newTestService(t)
	_, err := svc.LoginWithOAuth(context.Background(), oauth.ProviderGoogle, "", "", nil)
	assert.ErrorIs(t, err, ErrInvalidInput)
}

func TestLoginWithOAuth_VerifierError(t *testing.T) {
	svc, d := newTestService(t)
	d.oauthV.err = oauth.ErrNotImplemented
	_, err := svc.LoginWithOAuth(context.Background(), oauth.ProviderGoogle, "sometoken", "", nil)
	assert.ErrorIs(t, err, oauth.ErrNotImplemented)
}

func TestLoginWithOAuth_ExistingIdentity(t *testing.T) {
	svc, d := newTestService(t)
	now := d.clock.Now()
	require.NoError(t, d.users.Create(context.Background(), User{ID: "u1", Email: "a@b.com", DisplayName: "A", Status: UserStatusActive, CreatedAt: now, UpdatedAt: now}))
	require.NoError(t, d.identities.Link(context.Background(), Identity{ID: "i1", UserID: "u1", Provider: "google", ProviderUserID: "sub-1"}))
	d.oauthV.subject = "sub-1"
	d.oauthV.email = "a@b.com"

	result, err := svc.LoginWithOAuth(context.Background(), oauth.ProviderGoogle, "sometoken", "", nil)
	require.NoError(t, err)
	assert.Equal(t, "u1", result.UserID)
}

func TestLoginWithOAuth_NewUserFindOrCreate(t *testing.T) {
	svc, d := newTestService(t)
	d.oauthV.subject = "sub-2"
	d.oauthV.email = "new@b.com"

	result, err := svc.LoginWithOAuth(context.Background(), oauth.ProviderApple, "sometoken", "", nil)
	require.NoError(t, err)
	assert.NotEmpty(t, result.UserID)

	stored, err := d.users.GetByID(context.Background(), result.UserID)
	require.NoError(t, err)
	assert.Equal(t, "new@b.com", stored.Email)
	assert.Equal(t, "new@b.com", stored.DisplayName, "falls back to email when no display name supplied")

	userID, err := d.identities.GetUserIDByProvider(context.Background(), "apple", "sub-2")
	require.NoError(t, err)
	assert.Equal(t, result.UserID, userID)
}

func TestLoginWithOAuth_NewUserWithDisplayName(t *testing.T) {
	svc, d := newTestService(t)
	d.oauthV.subject = "sub-3"
	d.oauthV.email = "new2@b.com"

	result, err := svc.LoginWithOAuth(context.Background(), oauth.ProviderApple, "sometoken", "  Bob  ", nil)
	require.NoError(t, err)

	stored, err := d.users.GetByID(context.Background(), result.UserID)
	require.NoError(t, err)
	assert.Equal(t, "Bob", stored.DisplayName)
}

func TestLoginWithOAuth_IdentityLookupOtherError(t *testing.T) {
	svc, d := newTestService(t)
	d.identities.getErr = errBoom
	_, err := svc.LoginWithOAuth(context.Background(), oauth.ProviderGoogle, "sometoken", "", nil)
	require.Error(t, err)
	assert.False(t, errors.Is(err, ErrIdentityNotLinked))
}

func TestLoginWithOAuth_CreateUserError(t *testing.T) {
	svc, d := newTestService(t)
	d.users.createErr = errBoom
	_, err := svc.LoginWithOAuth(context.Background(), oauth.ProviderGoogle, "sometoken", "", nil)
	require.Error(t, err)
}

func TestLoginWithOAuth_LinkIdentityError(t *testing.T) {
	svc, d := newTestService(t)
	d.identities.linkErr = errBoom
	_, err := svc.LoginWithOAuth(context.Background(), oauth.ProviderGoogle, "sometoken", "", nil)
	require.Error(t, err)
}

// --- Refresh ---

func signUp(t *testing.T, svc *Service) AuthResult {
	t.Helper()
	result, err := svc.SignUp(context.Background(), SignUpInput{Email: "a@b.com", Password: "supersecret", DisplayName: "A"})
	require.NoError(t, err)
	return result
}

func TestRefresh_Success_Rotates(t *testing.T) {
	svc, _ := newTestService(t)
	initial := signUp(t, svc)

	rotated, err := svc.Refresh(context.Background(), initial.RefreshToken)
	require.NoError(t, err)
	assert.Equal(t, initial.UserID, rotated.UserID)
	assert.NotEqual(t, initial.RefreshToken, rotated.RefreshToken)
	assert.NotEqual(t, initial.AccessToken, rotated.AccessToken)

	// the new token works too
	rotated2, err := svc.Refresh(context.Background(), rotated.RefreshToken)
	require.NoError(t, err)
	assert.NotEqual(t, rotated.RefreshToken, rotated2.RefreshToken)
}

func TestRefresh_MissingToken(t *testing.T) {
	svc, _ := newTestService(t)
	_, err := svc.Refresh(context.Background(), "")
	assert.ErrorIs(t, err, ErrInvalidInput)
}

func TestRefresh_NotFound(t *testing.T) {
	svc, _ := newTestService(t)
	_, err := svc.Refresh(context.Background(), "does-not-exist")
	assert.ErrorIs(t, err, ErrInvalidRefreshToken)
}

func TestRefresh_GetByHashOtherError(t *testing.T) {
	svc, d := newTestService(t)
	d.refreshTokens.getByHashErr = errBoom
	_, err := svc.Refresh(context.Background(), "whatever")
	require.Error(t, err)
	assert.False(t, errors.Is(err, ErrInvalidRefreshToken))
}

func TestRefresh_Expired(t *testing.T) {
	svc, d := newTestService(t)
	initial := signUp(t, svc)
	d.clock.Advance(refreshTTL + time.Hour)

	_, err := svc.Refresh(context.Background(), initial.RefreshToken)
	assert.ErrorIs(t, err, ErrRefreshTokenExpired)
}

func TestRefresh_Revoked(t *testing.T) {
	svc, _ := newTestService(t)
	initial := signUp(t, svc)
	require.NoError(t, svc.Logout(context.Background(), initial.RefreshToken))

	_, err := svc.Refresh(context.Background(), initial.RefreshToken)
	assert.ErrorIs(t, err, ErrRefreshTokenRevoked)
}

func TestRefresh_ReuseDetected(t *testing.T) {
	svc, d := newTestService(t)
	initial := signUp(t, svc)

	rotated, err := svc.Refresh(context.Background(), initial.RefreshToken)
	require.NoError(t, err)

	// Present the already-rotated (stale) token again: reuse detected.
	_, err = svc.Refresh(context.Background(), initial.RefreshToken)
	assert.ErrorIs(t, err, ErrRefreshTokenReused)

	// The rotated (otherwise-valid) token must now be revoked too.
	_, err = svc.Refresh(context.Background(), rotated.RefreshToken)
	assert.ErrorIs(t, err, ErrRefreshTokenRevoked)
	_ = d
}

func TestRefresh_RevokeAllErrorOnReuse(t *testing.T) {
	svc, d := newTestService(t)
	initial := signUp(t, svc)
	_, err := svc.Refresh(context.Background(), initial.RefreshToken)
	require.NoError(t, err)

	d.refreshTokens.revokeAllErr = errBoom
	_, err = svc.Refresh(context.Background(), initial.RefreshToken)
	require.Error(t, err)
	assert.False(t, errors.Is(err, ErrRefreshTokenReused))
}

func TestRefresh_SignerErrorDuringRotation(t *testing.T) {
	svc, d := newTestService(t)
	initial := signUp(t, svc)

	d.signer.signErr = errBoom
	_, err := svc.Refresh(context.Background(), initial.RefreshToken)
	require.Error(t, err)
}

func TestRefresh_StoreNewTokenError(t *testing.T) {
	svc, d := newTestService(t)
	initial := signUp(t, svc)

	d.refreshTokens.storeErr = errBoom
	_, err := svc.Refresh(context.Background(), initial.RefreshToken)
	require.Error(t, err)
}

func TestRefresh_MarkReplacedError(t *testing.T) {
	svc, d := newTestService(t)
	initial := signUp(t, svc)

	d.refreshTokens.markReplacedErr = errBoom
	_, err := svc.Refresh(context.Background(), initial.RefreshToken)
	require.Error(t, err)
}

// --- Logout / LogoutAll ---

func TestLogout_Success(t *testing.T) {
	svc, _ := newTestService(t)
	initial := signUp(t, svc)

	require.NoError(t, svc.Logout(context.Background(), initial.RefreshToken))

	_, err := svc.Refresh(context.Background(), initial.RefreshToken)
	assert.ErrorIs(t, err, ErrRefreshTokenRevoked)
}

func TestLogout_MissingToken(t *testing.T) {
	svc, _ := newTestService(t)
	err := svc.Logout(context.Background(), "")
	assert.ErrorIs(t, err, ErrInvalidInput)
}

func TestLogout_UnknownTokenIsIdempotent(t *testing.T) {
	svc, _ := newTestService(t)
	err := svc.Logout(context.Background(), "does-not-exist")
	assert.NoError(t, err)
}

func TestLogout_GetByHashOtherError(t *testing.T) {
	svc, d := newTestService(t)
	d.refreshTokens.getByHashErr = errBoom
	err := svc.Logout(context.Background(), "whatever")
	require.Error(t, err)
}

func TestLogout_RevokeError(t *testing.T) {
	svc, d := newTestService(t)
	initial := signUp(t, svc)
	d.refreshTokens.revokeErr = errBoom
	err := svc.Logout(context.Background(), initial.RefreshToken)
	require.Error(t, err)
}

func TestLogoutAll_Success(t *testing.T) {
	svc, _ := newTestService(t)
	initial := signUp(t, svc)

	require.NoError(t, svc.LogoutAll(context.Background(), initial.UserID))

	_, err := svc.Refresh(context.Background(), initial.RefreshToken)
	assert.ErrorIs(t, err, ErrRefreshTokenRevoked)
}

func TestLogoutAll_MissingUserID(t *testing.T) {
	svc, _ := newTestService(t)
	err := svc.LogoutAll(context.Background(), "")
	assert.ErrorIs(t, err, ErrInvalidInput)
}

func TestLogoutAll_RepoError(t *testing.T) {
	svc, d := newTestService(t)
	d.refreshTokens.revokeAllErr = errBoom
	err := svc.LogoutAll(context.Background(), "u1")
	require.Error(t, err)
}

// --- Me ---

func TestMe_Success(t *testing.T) {
	svc, _ := newTestService(t)
	initial := signUp(t, svc)

	user, err := svc.Me(context.Background(), initial.UserID)
	require.NoError(t, err)
	assert.Equal(t, "a@b.com", user.Email)
}

func TestMe_NotFound(t *testing.T) {
	svc, _ := newTestService(t)
	_, err := svc.Me(context.Background(), "does-not-exist")
	assert.ErrorIs(t, err, ErrUserNotFound)
}

func TestMe_OtherError(t *testing.T) {
	svc, d := newTestService(t)
	d.users.getByIDErr = errBoom
	_, err := svc.Me(context.Background(), "u1")
	require.Error(t, err)
	assert.False(t, errors.Is(err, ErrUserNotFound))
}

// --- MFA (.vibeflow/specs/mfa-for-proximity-consent.md) ---

func TestEnrollMFA_Success(t *testing.T) {
	svc, _ := newTestService(t)
	secret, uri, err := svc.EnrollMFA(context.Background(), "u1")
	require.NoError(t, err)
	assert.NotEmpty(t, secret)
	assert.NotEmpty(t, uri)
}

func TestEnrollMFA_MissingUserID(t *testing.T) {
	svc, _ := newTestService(t)
	_, _, err := svc.EnrollMFA(context.Background(), "")
	assert.ErrorIs(t, err, ErrInvalidInput)
}

func TestEnrollMFA_ProviderError(t *testing.T) {
	svc, d := newTestService(t)
	d.mfa.err = errBoom
	_, _, err := svc.EnrollMFA(context.Background(), "u1")
	require.Error(t, err)
}

func TestVerifyMFA_Success(t *testing.T) {
	svc, _ := newTestService(t)
	_, _, err := svc.EnrollMFA(context.Background(), "u1")
	require.NoError(t, err)

	ok, err := svc.VerifyMFA(context.Background(), "u1", "good-code")
	require.NoError(t, err)
	assert.True(t, ok)
}

func TestVerifyMFA_WrongCode(t *testing.T) {
	svc, _ := newTestService(t)
	_, _, err := svc.EnrollMFA(context.Background(), "u1")
	require.NoError(t, err)

	ok, err := svc.VerifyMFA(context.Background(), "u1", "wrong-code")
	require.NoError(t, err)
	assert.False(t, ok)
}

func TestVerifyMFA_ProviderError(t *testing.T) {
	svc, d := newTestService(t)
	d.mfa.err = errBoom
	_, err := svc.VerifyMFA(context.Background(), "u1", "123456")
	require.Error(t, err)
}

func TestVerifyMFA_MissingArgs(t *testing.T) {
	svc, _ := newTestService(t)
	_, err := svc.VerifyMFA(context.Background(), "", "123456")
	assert.ErrorIs(t, err, ErrInvalidInput)
	_, err = svc.VerifyMFA(context.Background(), "u1", "")
	assert.ErrorIs(t, err, ErrInvalidInput)
}

// TestHasVerifiedMFA_ReflectsEnrollAndVerify is the exact sequence
// presence.SettingsService.GrantConsent depends on (auth.Service satisfies
// presence.MFAChecker structurally): unverified until enrolled AND a
// correct code has been submitted once.
func TestHasVerifiedMFA_ReflectsEnrollAndVerify(t *testing.T) {
	svc, _ := newTestService(t)

	ok, err := svc.HasVerifiedMFA(context.Background(), "u1")
	require.NoError(t, err)
	assert.False(t, ok, "not enrolled yet")

	_, _, err = svc.EnrollMFA(context.Background(), "u1")
	require.NoError(t, err)
	ok, err = svc.HasVerifiedMFA(context.Background(), "u1")
	require.NoError(t, err)
	assert.False(t, ok, "enrolled but not yet verified")

	_, err = svc.VerifyMFA(context.Background(), "u1", "good-code")
	require.NoError(t, err)
	ok, err = svc.HasVerifiedMFA(context.Background(), "u1")
	require.NoError(t, err)
	assert.True(t, ok, "verified")
}

func TestHasVerifiedMFA_MissingUserID(t *testing.T) {
	svc, _ := newTestService(t)
	_, err := svc.HasVerifiedMFA(context.Background(), "")
	assert.ErrorIs(t, err, ErrInvalidInput)
}

package mfa

import (
	"context"
	"testing"
	"time"

	"github.com/pquerna/otp/totp"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"smusic/backend/internal/platform/clock"
)

// fakeSecretRepository is an in-memory SecretRepository (backend-go.md
// §7: real-I/O-free unit tests) — internal/auth/mfa/postgres implements
// the real one, exercised separately per
// .vibeflow/specs/backend-integration-test-coverage.md.
type fakeSecretRepository struct {
	secrets map[string]Secret
	getErr  error
}

func newFakeSecretRepository() *fakeSecretRepository {
	return &fakeSecretRepository{secrets: map[string]Secret{}}
}

func (f *fakeSecretRepository) Get(_ context.Context, userID string) (Secret, error) {
	if f.getErr != nil {
		return Secret{}, f.getErr
	}
	s, ok := f.secrets[userID]
	if !ok {
		return Secret{}, ErrSecretNotFound
	}
	return s, nil
}

func (f *fakeSecretRepository) Upsert(_ context.Context, s Secret) error {
	f.secrets[s.UserID] = s
	return nil
}

func (f *fakeSecretRepository) MarkVerified(_ context.Context, userID string, verifiedAt time.Time) error {
	s, ok := f.secrets[userID]
	if !ok {
		return ErrSecretNotFound
	}
	t := verifiedAt
	s.VerifiedAt = &t
	f.secrets[userID] = s
	return nil
}

func newTestChallenger(t *testing.T) (*TOTPChallenger, *fakeSecretRepository, *clock.Frozen) {
	t.Helper()
	repo := newFakeSecretRepository()
	clk := clock.NewFrozen(time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC))
	accountName := func(_ context.Context, userID string) (string, error) { return userID + "@smusic.test", nil }
	return NewTOTPChallenger(repo, clk, accountName), repo, clk
}

func TestTOTPChallenger_Enroll_PersistsUnverifiedSecret(t *testing.T) {
	c, repo, _ := newTestChallenger(t)
	secret, err := c.Enroll(context.Background(), "u1")
	require.NoError(t, err)
	assert.NotEmpty(t, secret)

	stored, err := repo.Get(context.Background(), "u1")
	require.NoError(t, err)
	assert.Equal(t, secret, stored.Value)
	assert.Nil(t, stored.VerifiedAt, "enrolling alone must not activate the factor")
}

func TestTOTPChallenger_EnrollURI_ReturnsOtpauthURL(t *testing.T) {
	c, _, _ := newTestChallenger(t)
	secret, uri, err := c.EnrollURI(context.Background(), "u1")
	require.NoError(t, err)
	assert.NotEmpty(t, secret)
	assert.Contains(t, uri, "otpauth://totp/")
	assert.Contains(t, uri, "issuer="+Issuer)
}

func TestTOTPChallenger_Verify_CorrectCodeActivatesFactor(t *testing.T) {
	c, repo, clk := newTestChallenger(t)
	secret, err := c.Enroll(context.Background(), "u1")
	require.NoError(t, err)

	code, err := totp.GenerateCode(secret, clk.Now())
	require.NoError(t, err)

	ok, err := c.Verify(context.Background(), "u1", code)
	require.NoError(t, err)
	assert.True(t, ok)

	stored, err := repo.Get(context.Background(), "u1")
	require.NoError(t, err)
	require.NotNil(t, stored.VerifiedAt)
	assert.Equal(t, clk.Now(), *stored.VerifiedAt)

	hasVerified, err := c.HasVerified(context.Background(), "u1")
	require.NoError(t, err)
	assert.True(t, hasVerified)
}

func TestTOTPChallenger_Verify_WrongCode(t *testing.T) {
	c, _, _ := newTestChallenger(t)
	_, err := c.Enroll(context.Background(), "u1")
	require.NoError(t, err)

	ok, err := c.Verify(context.Background(), "u1", "000000")
	require.NoError(t, err)
	assert.False(t, ok)

	hasVerified, err := c.HasVerified(context.Background(), "u1")
	require.NoError(t, err)
	assert.False(t, hasVerified)
}

func TestTOTPChallenger_Verify_NotEnrolled(t *testing.T) {
	c, _, _ := newTestChallenger(t)
	_, err := c.Verify(context.Background(), "ghost", "123456")
	assert.ErrorIs(t, err, ErrSecretNotFound)
}

func TestTOTPChallenger_Verify_AlreadyVerifiedDoesNotReVerify(t *testing.T) {
	// A second correct code must not error or re-trigger MarkVerified —
	// it should just keep reporting success (a user re-authenticating
	// with a fresh code, e.g. for a later sensitive action, is normal
	// usage, not a bug).
	c, repo, clk := newTestChallenger(t)
	secret, err := c.Enroll(context.Background(), "u1")
	require.NoError(t, err)

	code1, err := totp.GenerateCode(secret, clk.Now())
	require.NoError(t, err)
	ok, err := c.Verify(context.Background(), "u1", code1)
	require.NoError(t, err)
	require.True(t, ok)

	firstVerifiedAt := *mustGet(t, repo, "u1").VerifiedAt

	clk.Advance(30 * time.Second)
	code2, err := totp.GenerateCode(secret, clk.Now())
	require.NoError(t, err)
	ok, err = c.Verify(context.Background(), "u1", code2)
	require.NoError(t, err)
	assert.True(t, ok)

	// VerifiedAt is set once, at first activation — not bumped on every
	// subsequent successful check.
	assert.Equal(t, firstVerifiedAt, *mustGet(t, repo, "u1").VerifiedAt)
}

func TestTOTPChallenger_HasVerified_NotEnrolled(t *testing.T) {
	c, _, _ := newTestChallenger(t)
	ok, err := c.HasVerified(context.Background(), "ghost")
	require.NoError(t, err)
	assert.False(t, ok)
}

func TestTOTPChallenger_HasVerified_RepoError(t *testing.T) {
	c, repo, _ := newTestChallenger(t)
	repo.getErr = mfaAssertErr("boom")
	_, err := c.HasVerified(context.Background(), "u1")
	require.Error(t, err)
}

type mfaAssertErr string

func (e mfaAssertErr) Error() string { return string(e) }

func mustGet(t *testing.T, repo *fakeSecretRepository, userID string) Secret {
	t.Helper()
	s, err := repo.Get(context.Background(), userID)
	require.NoError(t, err)
	return s
}

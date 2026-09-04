package presence

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"smusic/backend/internal/platform/clock"
)

// newTestSettingsService wires an MFAChecker that reports every user as
// already MFA-verified — existing tests here predate the MFA gate
// (.vibeflow/specs/mfa-for-proximity-consent.md) and exercise other
// behavior; TestSettingsService_GrantConsent_RequiresMFA and its
// siblings below construct a SettingsService directly instead, with an
// explicit fakeMFAChecker, to test the gate itself.
func newTestSettingsService(t *testing.T) (*SettingsService, *fakePrivacySettingsRepo, *fakeBlockRepo, *fakeGeoIndex, *clock.Frozen) {
	t.Helper()
	settings := newFakePrivacySettingsRepo()
	blocks := newFakeBlockRepo()
	geo := newFakeGeoIndex()
	clk := clock.NewFrozen(time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC))
	return NewSettingsService(settings, blocks, geo, newFakeMFAChecker(true), clk), settings, blocks, geo, clk
}

func TestSettingsService_Get_DefaultsWhenMissing(t *testing.T) {
	svc, _, _, _, _ := newTestSettingsService(t)
	s, err := svc.Get(context.Background(), "u1")
	require.NoError(t, err)
	assert.Equal(t, DefaultPrivacySettings("u1"), s)
}

func TestSettingsService_Get_EmptyUserID(t *testing.T) {
	svc, _, _, _, _ := newTestSettingsService(t)
	_, err := svc.Get(context.Background(), "")
	assert.ErrorIs(t, err, ErrInvalidInput)
}

func TestSettingsService_Update_EmptyUserID(t *testing.T) {
	svc, _, _, _, _ := newTestSettingsService(t)
	_, err := svc.Update(context.Background(), "", UpdateSettingsInput{})
	assert.ErrorIs(t, err, ErrInvalidInput)
}

func TestSettingsService_Update_ValidatesRadius(t *testing.T) {
	svc, _, _, _, _ := newTestSettingsService(t)
	bad := 999
	_, err := svc.Update(context.Background(), "u1", UpdateSettingsInput{VisibilityRadiusM: &bad})
	assert.ErrorIs(t, err, ErrInvalidRadius)
}

func TestSettingsService_Update_ValidatesRevealLevel(t *testing.T) {
	svc, _, _, _, _ := newTestSettingsService(t)
	bad := 5
	_, err := svc.Update(context.Background(), "u1", UpdateSettingsInput{RevealLevel: &bad})
	assert.ErrorIs(t, err, ErrInvalidRevealLevel)
}

func TestSettingsService_Update_ValidatesVisibility(t *testing.T) {
	svc, _, _, _, _ := newTestSettingsService(t)
	bad := "public"
	_, err := svc.Update(context.Background(), "u1", UpdateSettingsInput{PresenceVisibility: &bad})
	assert.ErrorIs(t, err, ErrInvalidVisibility)
}

func TestSettingsService_Update_AppliesValidChanges(t *testing.T) {
	svc, repo, _, _, _ := newTestSettingsService(t)
	radius := 5000
	level := 1
	visibility := VisibilityEveryone
	shareTrack := true
	paused := false

	s, err := svc.Update(context.Background(), "u1", UpdateSettingsInput{
		PresenceVisibility: &visibility,
		PresenceShareTrack: &shareTrack,
		VisibilityRadiusM:  &radius,
		RevealLevel:        &level,
		Paused:             &paused,
	})
	require.NoError(t, err)
	assert.Equal(t, VisibilityEveryone, s.PresenceVisibility)
	assert.True(t, s.PresenceShareTrack)
	assert.Equal(t, 5000, s.VisibilityRadiusM)
	assert.Equal(t, 1, s.RevealLevel)
	assert.False(t, s.Paused)

	stored, err := repo.Get(context.Background(), "u1")
	require.NoError(t, err)
	assert.Equal(t, s, stored)
}

// TestSettingsService_Update_Pause_RemovesFromIndexImmediately is the
// privacy invariant from security.md §1.4: pausing must have an IMMEDIATE
// effect on the live index, not wait for TTL.
func TestSettingsService_Update_Pause_RemovesFromIndexImmediately(t *testing.T) {
	svc, _, _, geo, _ := newTestSettingsService(t)
	geo.entries["u1"] = fakeGeoEntry{PresenceEntry: PresenceEntry{UserID: "u1"}, pos: GeoPosition{}}

	paused := true
	_, err := svc.Update(context.Background(), "u1", UpdateSettingsInput{Paused: &paused})
	require.NoError(t, err)

	_, ok, _ := geo.Touch(context.Background(), "u1", time.Minute)
	assert.False(t, ok, "user should have been removed from the index immediately on pause")
}

func TestSettingsService_Update_InvisibleVisibility_RemovesFromIndex(t *testing.T) {
	svc, _, _, geo, _ := newTestSettingsService(t)
	geo.entries["u1"] = fakeGeoEntry{PresenceEntry: PresenceEntry{UserID: "u1"}, pos: GeoPosition{}}

	invisible := VisibilityInvisible
	notPaused := false
	_, err := svc.Update(context.Background(), "u1", UpdateSettingsInput{PresenceVisibility: &invisible, Paused: &notPaused})
	require.NoError(t, err)

	_, ok, _ := geo.Touch(context.Background(), "u1", time.Minute)
	assert.False(t, ok)
}

func TestSettingsService_GrantConsent(t *testing.T) {
	svc, _, _, _, clk := newTestSettingsService(t)
	s, err := svc.GrantConsent(context.Background(), "u1")
	require.NoError(t, err)
	assert.True(t, s.ProximityConsentEnabled)
	require.NotNil(t, s.ProximityConsentTS)
	require.NotNil(t, s.ProximityConsentRenewDue)
	assert.Equal(t, clk.Now(), *s.ProximityConsentTS)
	assert.Equal(t, clk.Now().Add(ConsentValidityPeriod), *s.ProximityConsentRenewDue)
	assert.True(t, s.HasActiveConsent(clk.Now()))
}

// TestSettingsService_GrantConsent_DoesNotAutoEnableDiscovery: granting
// consent alone must not make a user discoverable — they remain paused
// and invisible until they separately choose to be visible.
func TestSettingsService_GrantConsent_DoesNotAutoEnableDiscovery(t *testing.T) {
	svc, _, _, _, _ := newTestSettingsService(t)
	s, err := svc.GrantConsent(context.Background(), "u1")
	require.NoError(t, err)
	assert.True(t, s.Paused)
	assert.Equal(t, VisibilityInvisible, s.PresenceVisibility)
}

// TestSettingsService_GrantConsent_RequiresMFA:
// .vibeflow/specs/mfa-for-proximity-consent.md — security.md §2 mandates a
// verified TOTP factor before consent can be granted.
func TestSettingsService_GrantConsent_RequiresMFA(t *testing.T) {
	settings := newFakePrivacySettingsRepo()
	blocks := newFakeBlockRepo()
	clk := clock.NewFrozen(time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC))
	svc := NewSettingsService(settings, blocks, nil, newFakeMFAChecker(false), clk)

	_, err := svc.GrantConsent(context.Background(), "u1")
	assert.ErrorIs(t, err, ErrMFARequired)

	// Consent must not have been partially applied.
	s, err := svc.Get(context.Background(), "u1")
	require.NoError(t, err)
	assert.False(t, s.ProximityConsentEnabled)
}

// TestSettingsService_GrantConsent_SucceedsAfterMFAVerified mirrors a real
// enroll -> verify -> grant-consent flow: the fake starts unverified, is
// flipped to verified (as TOTPChallenger.Verify does on the first correct
// code), and only then does GrantConsent succeed.
func TestSettingsService_GrantConsent_SucceedsAfterMFAVerified(t *testing.T) {
	settings := newFakePrivacySettingsRepo()
	blocks := newFakeBlockRepo()
	clk := clock.NewFrozen(time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC))
	mfa := newFakeMFAChecker(false)
	svc := NewSettingsService(settings, blocks, nil, mfa, clk)

	_, err := svc.GrantConsent(context.Background(), "u1")
	assert.ErrorIs(t, err, ErrMFARequired)

	mfa.setVerified("u1", true)

	s, err := svc.GrantConsent(context.Background(), "u1")
	require.NoError(t, err)
	assert.True(t, s.ProximityConsentEnabled)
}

// TestSettingsService_GrantConsent_MFACheckError: a transient failure
// checking MFA status must surface as an error, never silently treated as
// "verified" (that would defeat the gate) or "not verified" (that would
// wrongly block a legitimately-verified user on infra hiccups — the
// correct behavior is "fail the request, let the client retry").
func TestSettingsService_GrantConsent_MFACheckError(t *testing.T) {
	settings := newFakePrivacySettingsRepo()
	blocks := newFakeBlockRepo()
	clk := clock.NewFrozen(time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC))
	mfa := &fakeMFAChecker{err: errBoomPresence}
	svc := NewSettingsService(settings, blocks, nil, mfa, clk)

	_, err := svc.GrantConsent(context.Background(), "u1")
	require.Error(t, err)
	assert.False(t, errors.Is(err, ErrMFARequired), "an infra error must not be reported as ErrMFARequired")
}

func TestSettingsService_RevokeConsent_ImmediateAndForcesPause(t *testing.T) {
	svc, _, _, geo, clk := newTestSettingsService(t)
	_, err := svc.GrantConsent(context.Background(), "u1")
	require.NoError(t, err)
	visible := VisibilityEveryone
	notPaused := false
	_, err = svc.Update(context.Background(), "u1", UpdateSettingsInput{PresenceVisibility: &visible, Paused: &notPaused})
	require.NoError(t, err)

	geo.entries["u1"] = fakeGeoEntry{PresenceEntry: PresenceEntry{UserID: "u1"}, pos: GeoPosition{}}

	s, err := svc.RevokeConsent(context.Background(), "u1")
	require.NoError(t, err)
	assert.False(t, s.ProximityConsentEnabled)
	assert.True(t, s.Paused)
	assert.False(t, s.HasActiveConsent(clk.Now()))

	_, ok, _ := geo.Touch(context.Background(), "u1", time.Minute)
	assert.False(t, ok, "revoking consent must remove the user from the live index immediately")
}

func TestSettingsService_SetPaused(t *testing.T) {
	svc, _, _, _, _ := newTestSettingsService(t)
	s, err := svc.SetPaused(context.Background(), "u1", false)
	require.NoError(t, err)
	assert.False(t, s.Paused)

	s, err = svc.SetPaused(context.Background(), "u1", true)
	require.NoError(t, err)
	assert.True(t, s.Paused)
}

func TestSettingsService_NilGeoIndex_DoesNotPanic(t *testing.T) {
	settings := newFakePrivacySettingsRepo()
	blocks := newFakeBlockRepo()
	clk := clock.NewFrozen(time.Now())
	svc := NewSettingsService(settings, blocks, nil, newFakeMFAChecker(true), clk)

	paused := true
	_, err := svc.Update(context.Background(), "u1", UpdateSettingsInput{Paused: &paused})
	require.NoError(t, err)

	_, err = svc.RevokeConsent(context.Background(), "u1")
	require.NoError(t, err)
}

func TestSettingsService_Block(t *testing.T) {
	svc, _, blocks, _, _ := newTestSettingsService(t)
	require.NoError(t, svc.Block(context.Background(), "a", "b"))
	blocked, err := blocks.IsBlockedEitherWay(context.Background(), "a", "b")
	require.NoError(t, err)
	assert.True(t, blocked)
}

func TestSettingsService_Block_CannotBlockSelf(t *testing.T) {
	svc, _, _, _, _ := newTestSettingsService(t)
	err := svc.Block(context.Background(), "a", "a")
	assert.ErrorIs(t, err, ErrCannotBlockSelf)
}

func TestSettingsService_Block_EmptyIDs(t *testing.T) {
	svc, _, _, _, _ := newTestSettingsService(t)
	assert.ErrorIs(t, svc.Block(context.Background(), "", "b"), ErrInvalidInput)
	assert.ErrorIs(t, svc.Block(context.Background(), "a", ""), ErrInvalidInput)
}

func TestSettingsService_Unblock(t *testing.T) {
	svc, _, blocks, _, _ := newTestSettingsService(t)
	require.NoError(t, svc.Block(context.Background(), "a", "b"))
	require.NoError(t, svc.Unblock(context.Background(), "a", "b"))
	blocked, err := blocks.IsBlockedEitherWay(context.Background(), "a", "b")
	require.NoError(t, err)
	assert.False(t, blocked)
}

func TestSettingsService_Unblock_EmptyIDs(t *testing.T) {
	svc, _, _, _, _ := newTestSettingsService(t)
	assert.ErrorIs(t, svc.Unblock(context.Background(), "", "b"), ErrInvalidInput)
}

func TestSettingsService_Get_RepoError(t *testing.T) {
	svc, repo, _, _, _ := newTestSettingsService(t)
	repo.getErr = errBoomPresence
	_, err := svc.Get(context.Background(), "u1")
	require.Error(t, err)
	assert.False(t, assertIsSentinel(err))
}

func TestSettingsService_Update_GetError(t *testing.T) {
	svc, repo, _, _, _ := newTestSettingsService(t)
	repo.getErr = errBoomPresence
	paused := true
	_, err := svc.Update(context.Background(), "u1", UpdateSettingsInput{Paused: &paused})
	require.Error(t, err)
}

func TestSettingsService_GrantConsent_GetError(t *testing.T) {
	svc, repo, _, _, _ := newTestSettingsService(t)
	repo.getErr = errBoomPresence
	_, err := svc.GrantConsent(context.Background(), "u1")
	require.Error(t, err)
}

func TestSettingsService_Update_UpsertError(t *testing.T) {
	svc, repo, _, _, _ := newTestSettingsService(t)
	repo.upsertErr = errBoomPresence
	paused := true
	_, err := svc.Update(context.Background(), "u1", UpdateSettingsInput{Paused: &paused})
	require.Error(t, err)
}

func TestSettingsService_GrantConsent_UpsertError(t *testing.T) {
	svc, repo, _, _, _ := newTestSettingsService(t)
	repo.upsertErr = errBoomPresence
	_, err := svc.GrantConsent(context.Background(), "u1")
	require.Error(t, err)
}

func TestSettingsService_RevokeConsent_GetError(t *testing.T) {
	svc, repo, _, _, _ := newTestSettingsService(t)
	repo.getErr = errBoomPresence
	_, err := svc.RevokeConsent(context.Background(), "u1")
	require.Error(t, err)
}

func TestSettingsService_RevokeConsent_UpsertError(t *testing.T) {
	svc, repo, _, _, _ := newTestSettingsService(t)
	repo.upsertErr = errBoomPresence
	_, err := svc.RevokeConsent(context.Background(), "u1")
	require.Error(t, err)
}

func TestSettingsService_Block_RepoError(t *testing.T) {
	svc, _, blocks, _, _ := newTestSettingsService(t)
	blocks.err = errBoomPresence
	err := svc.Block(context.Background(), "a", "b")
	require.Error(t, err)
}

func TestSettingsService_Unblock_RepoError(t *testing.T) {
	svc, _, blocks, _, _ := newTestSettingsService(t)
	blocks.err = errBoomPresence
	err := svc.Unblock(context.Background(), "a", "b")
	require.Error(t, err)
}

func TestSettingsService_GrantConsent_EmptyUserID(t *testing.T) {
	svc, _, _, _, _ := newTestSettingsService(t)
	_, err := svc.GrantConsent(context.Background(), "")
	assert.ErrorIs(t, err, ErrInvalidInput)
}

func TestSettingsService_RevokeConsent_EmptyUserID(t *testing.T) {
	svc, _, _, _, _ := newTestSettingsService(t)
	_, err := svc.RevokeConsent(context.Background(), "")
	assert.ErrorIs(t, err, ErrInvalidInput)
}

// assertIsSentinel is a tiny helper used only to make TestSettingsService_Get_RepoError's
// intent explicit without importing errors just for one call site.
func assertIsSentinel(err error) bool {
	return err == ErrSettingsNotFound
}

var errBoomPresence = &boomErr{"boom"}

type boomErr struct{ msg string }

func (b *boomErr) Error() string { return b.msg }

package presence

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestDefaultPrivacySettings_IsSafeByDefault(t *testing.T) {
	s := DefaultPrivacySettings("u1")
	assert.Equal(t, VisibilityInvisible, s.PresenceVisibility)
	assert.True(t, s.Paused)
	assert.False(t, s.ProximityConsentEnabled)
	assert.Equal(t, RevealLevelAnonymous, s.RevealLevel)
	assert.False(t, s.HasActiveConsent(time.Now()))
}

func TestHasActiveConsent(t *testing.T) {
	now := time.Date(2026, 6, 1, 0, 0, 0, 0, time.UTC)
	future := now.Add(24 * time.Hour)
	past := now.Add(-24 * time.Hour)

	cases := []struct {
		name string
		s    PrivacySettings
		want bool
	}{
		{"disabled", PrivacySettings{ProximityConsentEnabled: false, ProximityConsentRenewDue: &future}, false},
		{"enabled, no renew due (fail closed)", PrivacySettings{ProximityConsentEnabled: true, ProximityConsentRenewDue: nil}, false},
		{"enabled, renew due in future", PrivacySettings{ProximityConsentEnabled: true, ProximityConsentRenewDue: &future}, true},
		{"enabled, renew due in past (expired)", PrivacySettings{ProximityConsentEnabled: true, ProximityConsentRenewDue: &past}, false},
		{"enabled, renew due exactly now (not before -> expired)", PrivacySettings{ProximityConsentEnabled: true, ProximityConsentRenewDue: &now}, false},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			assert.Equal(t, c.want, c.s.HasActiveConsent(now))
		})
	}
}

func TestIsValidRadius(t *testing.T) {
	for _, r := range AllowedRadiiM {
		assert.True(t, IsValidRadius(r))
	}
	assert.False(t, IsValidRadius(0))
	assert.False(t, IsValidRadius(151))
	assert.False(t, IsValidRadius(20000))
	assert.False(t, IsValidRadius(-150))
}

func TestIsValidRevealLevel(t *testing.T) {
	assert.True(t, IsValidRevealLevel(0))
	assert.True(t, IsValidRevealLevel(1))
	assert.True(t, IsValidRevealLevel(2))
	assert.False(t, IsValidRevealLevel(3))
	assert.False(t, IsValidRevealLevel(-1))
}

func TestIsValidVisibility(t *testing.T) {
	assert.True(t, IsValidVisibility(VisibilityInvisible))
	assert.True(t, IsValidVisibility(VisibilityFriendsOnly))
	assert.True(t, IsValidVisibility(VisibilityEveryone))
	assert.False(t, IsValidVisibility("public"))
	assert.False(t, IsValidVisibility(""))
}

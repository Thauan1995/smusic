package oauth

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestStubVerifier_SupportedProviders(t *testing.T) {
	v := StubVerifier{}
	for _, p := range []Provider{ProviderGoogle, ProviderApple} {
		_, _, err := v.Verify(context.Background(), p, "some-token")
		assert.ErrorIs(t, err, ErrNotImplemented)
	}
}

func TestStubVerifier_UnsupportedProvider(t *testing.T) {
	v := StubVerifier{}
	_, _, err := v.Verify(context.Background(), Provider("facebook"), "token")
	assert.ErrorIs(t, err, ErrUnsupportedProvider)
}

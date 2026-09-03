package mfa

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestNoopChallenger(t *testing.T) {
	c := NoopChallenger{}

	_, err := c.Enroll(context.Background(), "user-1")
	assert.ErrorIs(t, err, ErrNotImplemented)

	ok, err := c.Verify(context.Background(), "user-1", "123456")
	assert.False(t, ok)
	assert.ErrorIs(t, err, ErrNotImplemented)
}

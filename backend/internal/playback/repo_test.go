package playback

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestNoopPlayEventRecorder(t *testing.T) {
	var r NoopPlayEventRecorder
	require.NoError(t, r.Record(context.Background(), PlayEvent{}))
}

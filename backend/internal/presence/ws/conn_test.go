package ws

import (
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gorilla/websocket"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"smusic/backend/internal/presence"
)

func testLogger() *slog.Logger {
	return slog.New(slog.NewTextHandler(io.Discard, &slog.HandlerOptions{}))
}

// dialTestConn spins up a bare WS echo-free server and returns a *conn
// wrapping the server-side connection, plus a client-side *websocket.Conn
// the test can read from directly.
func dialTestConn(t *testing.T, userID string) (*conn, *websocket.Conn, func()) {
	t.Helper()
	var upgrader websocket.Upgrader
	var serverConn *conn
	ready := make(chan struct{})

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		wsConn, err := upgrader.Upgrade(w, r, nil)
		require.NoError(t, err)
		serverConn = newConn(userID, wsConn, testLogger())
		go serverConn.writePump()
		close(ready)
		// keep the handler alive until the test is done via the client closing
		for {
			if _, _, err := wsConn.ReadMessage(); err != nil {
				return
			}
		}
	}))

	wsURL := "ws" + srv.URL[len("http"):]
	clientConn, _, err := websocket.DefaultDialer.Dial(wsURL, nil)
	require.NoError(t, err)
	<-ready

	cleanup := func() {
		_ = clientConn.Close()
		srv.Close()
	}
	return serverConn, clientConn, cleanup
}

func TestConn_Send_DeliversFrameToClient(t *testing.T) {
	c, client, cleanup := dialTestConn(t, "u1")
	defer cleanup()

	require.NoError(t, c.Send(presence.Frame{Type: presence.FrameNearbyUpdate}))

	_ = client.SetReadDeadline(time.Now().Add(2 * time.Second))
	_, data, err := client.ReadMessage()
	require.NoError(t, err)
	assert.Contains(t, string(data), presence.FrameNearbyUpdate)
}

func TestConn_UserID(t *testing.T) {
	c, _, cleanup := dialTestConn(t, "alice")
	defer cleanup()
	assert.Equal(t, "alice", c.UserID())
}

func TestConn_Send_FullBuffer_NoWritePump(t *testing.T) {
	var upgrader websocket.Upgrader
	var serverConn *conn
	ready := make(chan struct{})
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		wsConn, err := upgrader.Upgrade(w, r, nil)
		require.NoError(t, err)
		serverConn = newConn("u1", wsConn, testLogger())
		close(ready)
		for {
			if _, _, err := wsConn.ReadMessage(); err != nil {
				return
			}
		}
	}))
	defer srv.Close()

	wsURL := "ws" + srv.URL[len("http"):]
	clientConn, _, err := websocket.DefaultDialer.Dial(wsURL, nil)
	require.NoError(t, err)
	defer clientConn.Close()
	<-ready

	// No writePump running: the outbound buffer fills after outboundBuffer sends.
	var lastErr error
	for i := 0; i < outboundBuffer+2; i++ {
		lastErr = serverConn.Send(presence.Frame{Type: presence.FrameNearbyUpdate})
	}
	assert.ErrorIs(t, lastErr, errOutboundBufferFull)
}

func TestConn_Close_Idempotent(t *testing.T) {
	c, _, cleanup := dialTestConn(t, "u1")
	defer cleanup()

	c.close()
	c.close() // must not panic

	err := c.Send(presence.Frame{Type: presence.FrameDrain})
	assert.ErrorIs(t, err, errConnClosed)
}

func TestConn_WritePump_ExitsOnWriteFailure(t *testing.T) {
	c, client, _ := dialTestConn(t, "u1")
	_ = client.Close() // force the next server-side write to fail

	// Give the read side of the closed connection a moment, then attempt a
	// send; writePump should notice the failure and close the connection.
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		if err := c.Send(presence.Frame{Type: presence.FrameNearbyUpdate}); err != nil {
			return // connection closed as expected
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatal("expected writePump to eventually close the connection after a write failure")
}

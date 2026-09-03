package httpx

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type payload struct {
	Name string `json:"name"`
}

func TestDecodeJSON_Success(t *testing.T) {
	r := httptest.NewRequest(http.MethodPost, "/", strings.NewReader(`{"name":"track"}`))
	var p payload
	require.NoError(t, DecodeJSON(r, &p))
	assert.Equal(t, "track", p.Name)
}

func TestDecodeJSON_UnknownField(t *testing.T) {
	r := httptest.NewRequest(http.MethodPost, "/", strings.NewReader(`{"name":"track","bogus":1}`))
	var p payload
	err := DecodeJSON(r, &p)
	require.Error(t, err)
}

func TestDecodeJSON_TrailingData(t *testing.T) {
	r := httptest.NewRequest(http.MethodPost, "/", strings.NewReader(`{"name":"a"}{"name":"b"}`))
	var p payload
	err := DecodeJSON(r, &p)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "trailing data")
}

func TestDecodeJSON_MalformedBody(t *testing.T) {
	r := httptest.NewRequest(http.MethodPost, "/", strings.NewReader(`not json`))
	var p payload
	err := DecodeJSON(r, &p)
	require.Error(t, err)
}

func TestDecodeJSON_TooLarge(t *testing.T) {
	big := bytes.Repeat([]byte("a"), maxBodyBytes+10)
	body := `{"name":"` + string(big) + `"}`
	r := httptest.NewRequest(http.MethodPost, "/", strings.NewReader(body))
	var p payload
	err := DecodeJSON(r, &p)
	require.ErrorIs(t, err, ErrBodyTooLarge)
}

func TestWriteJSON(t *testing.T) {
	w := httptest.NewRecorder()
	WriteJSON(w, http.StatusCreated, payload{Name: "x"})

	assert.Equal(t, http.StatusCreated, w.Code)
	assert.Equal(t, "application/json; charset=utf-8", w.Header().Get("Content-Type"))
	assert.JSONEq(t, `{"name":"x"}`, w.Body.String())
}

func TestWriteJSON_NilBody(t *testing.T) {
	w := httptest.NewRecorder()
	WriteJSON(w, http.StatusNoContent, nil)

	assert.Equal(t, http.StatusNoContent, w.Code)
	assert.Empty(t, w.Body.String())
}

func TestWriteError(t *testing.T) {
	w := httptest.NewRecorder()
	WriteError(w, http.StatusBadRequest, "invalid_input", "email is required")

	assert.Equal(t, http.StatusBadRequest, w.Code)
	assert.JSONEq(t, `{"error":{"code":"invalid_input","message":"email is required"}}`, w.Body.String())
}

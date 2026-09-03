// Package httpx holds small, transport-only helpers (JSON decode/encode,
// standard error envelope) shared by every module's api package. Keeping
// these here — instead of duplicating them per module — is what lets
// handlers stay "thin" (backend-go.md §7): a handler's job is parse request
// -> call service -> map result/error to a response, nothing else.
package httpx

import (
	"encoding/json"
	"errors"
	"io"
	"net/http"
)

// ErrBodyTooLarge is returned by DecodeJSON when the request body exceeds
// the configured limit.
var ErrBodyTooLarge = errors.New("httpx: request body too large")

// maxBodyBytes caps request bodies to defend against a client (or bug)
// sending an unbounded stream, per security.md §4 (abuse via oversized
// payloads); 1 MiB is generous for this API's JSON payloads.
const maxBodyBytes = 1 << 20

// DecodeJSON reads and decodes a JSON request body into dst. It rejects
// unknown fields (catches client/contract typos early) and bodies over
// maxBodyBytes.
func DecodeJSON(r *http.Request, dst any) error {
	r.Body = http.MaxBytesReader(nil, r.Body, maxBodyBytes)
	dec := json.NewDecoder(r.Body)
	dec.DisallowUnknownFields()
	if err := dec.Decode(dst); err != nil {
		var maxErr *http.MaxBytesError
		if errors.As(err, &maxErr) {
			return ErrBodyTooLarge
		}
		return err
	}
	// Reject trailing garbage after the JSON value (e.g. two concatenated
	// objects) instead of silently ignoring it.
	if err := dec.Decode(&struct{}{}); err != io.EOF {
		return errors.New("httpx: unexpected trailing data after JSON body")
	}
	return nil
}

// WriteJSON writes v as a JSON response with the given status code.
func WriteJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	if v == nil {
		return
	}
	_ = json.NewEncoder(w).Encode(v)
}

// ErrorEnvelope is the standard shape for every error response body.
type ErrorEnvelope struct {
	Error ErrorBody `json:"error"`
}

// ErrorBody carries a machine-readable code and a human-readable message.
type ErrorBody struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

// WriteError writes a standardized error envelope with the given status,
// machine-readable code, and human-readable message.
func WriteError(w http.ResponseWriter, status int, code, message string) {
	WriteJSON(w, status, ErrorEnvelope{Error: ErrorBody{Code: code, Message: message}})
}

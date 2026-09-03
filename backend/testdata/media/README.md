# Local test media asset

`sample.mp3` in this directory is a placeholder file, **not real audio**.
It exists so `internal/playback/media.LocalResolver` (backend-go.md §2's
TODO for the CDN/media-edge-service path) has something to point a signed
URL at during local development and manual testing, per the task's
instruction to serve a local test file instead of standing up a real CDN
for this slice.

To test actual playback in a client, replace `sample.mp3` with a real
short MP3 file — the server serves whatever bytes are here via
`GET /media/sample.mp3` regardless of content.

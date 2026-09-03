# Local test media asset

`sample.mp3` in this directory is a real, decodable audio file (a ~3s,
44.1kHz mono 440Hz tone, encoded with `lameenc` — see
`frontend/README.md`'s E2E test section for how it was generated and why:
the original placeholder here was a text stub, which is enough to exercise
the network path but makes a real browser's `<audio>`/Web Audio decoder
fail, which would have masked the difference between "playback broke
because of network/CORS" and "playback broke because the test fixture
isn't audio" during real-browser E2E validation
(`frontend/app/smusic_web/integration_test/`). It exists so
`internal/playback/media.LocalResolver` (backend-go.md §2's TODO for the
CDN/media-edge-service path) has something real to point a signed URL at
during local development, manual testing, and the Flutter web E2E
integration test, per the task's instruction to serve a local test file
instead of standing up a real CDN for this slice.

The server serves whatever bytes are here via `GET /media/sample.mp3`
regardless of content — replace this file with any other short MP3 if a
different fixture is needed.

/// frontend-flutter.md section 4.3. Deliberately a distinct type from
/// `core_networking`'s `SocketConnectionPhase` (generic transport phase,
/// core has no feature dependency) - `social_proximity_data`'s repository
/// maps one onto the other: `connected` -> [connected], `reconnecting` ->
/// [reconnecting] (this is what drives the "reconnecting…" banner after a
/// previously-live session drops, frontend-flutter.md section 4.3), and
/// both `connecting` (never-yet-connected first attempt) and `disconnected`
/// (stopped) -> [offline] - the very first connect attempt intentionally
/// does *not* show the "reconnecting…" banner (nothing was lost yet; the
/// screen's own loading skeleton covers that case instead).
enum ProximityConnectionState { connected, reconnecting, offline }

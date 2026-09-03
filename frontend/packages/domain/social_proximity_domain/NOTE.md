# social_proximity_domain — not implemented in Fatia 1

This directory is a placeholder for the proximity/presence domain package
described in `docs/architecture/frontend-flutter.md` section 1.2 and
`docs/architecture/00-overview.md` section 3 ("Proximidade fica fora da
Fatia 1"). It intentionally has no `pubspec.yaml` yet, so `melos bootstrap`
does not pick it up as a workspace package.

When Fatia 2 starts, this package should contain:
- `ProximityFeedRepository` interface (frontend-flutter.md section 4.1).
- `NearbyListener`, `ProximityConnectionState`, `LocationPermissionState`
  entities.
- The `AsyncNotifier<NearbyFeedState>` combining socket/permission/feed
  state described in section 4.1.

It must be built against `security.md`'s finalized privacy model (bucketed
distance + jitter, opt-in, `invisible` default) from day one - see
`docs/architecture/00-overview.md` section 1, never a simplified version
that could expose raw coordinates.

/// security.md section 1.6's "níveis de revelação". The *effective* level
/// for a given viewer of a given nearby listener is computed server-side
/// (stranger vs. mutual connection) and arrives per-[NearbyListener] as
/// `reveal_level` on the wire (`social_proximity_data`'s DTO maps it onto
/// [NearbyListener]'s `displayName`/`avatarUrl` being null-or-not - see that
/// class's doc comment for the client-side defense-in-depth rule). This
/// enum is *also* reused, distinctly, as the user's own **configured
/// ceiling** in [ProximityPrivacySettings.maxRevealLevel]: "what level am I
/// willing to be shown at to people who are not a mutual connection of
/// mine". [level2] there specifically means the user has opted in to
/// security.md 1.6's "descoberta aberta" - a second, separate consent from
/// the base feature opt-in (security.md section 1.6: "Precisa de segundo
/// consentimento explícito, separado do consentimento de ativar a
/// feature").
enum RevealLevel {
  /// security.md 1.6: existence + track + distance bucket only, no name/
  /// avatar. Default for strangers, and the only sane default ceiling for
  /// [ProximityPrivacySettings.maxRevealLevel].
  level0,

  /// security.md 1.6: display name (possibly a proximity-only pseudonym)
  /// + avatar + distance bucket. Always granted to mutual connections
  /// server-side, regardless of the user's [ProximityPrivacySettings.
  /// maxRevealLevel] - that setting only governs what *non-connections*
  /// see.
  level1,

  /// security.md 1.6: level 1 information shown to non-connections too
  /// ("descoberta aberta") - requires the second explicit consent
  /// described above.
  level2,
}

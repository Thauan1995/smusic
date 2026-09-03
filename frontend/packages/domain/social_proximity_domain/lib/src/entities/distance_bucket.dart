/// The **only** geographic information that ever reaches the client about
/// another user - security.md section 1.2's authoritative decision (not the
/// illustrative examples in frontend-flutter.md section 4.2 or
/// backend-go.md section 4, both of which show different placeholder
/// strings and are explicitly marked there as pending privacy-team
/// validation; security.md section 1's table is the actual signed-off
/// policy and is what this enum encodes).
///
/// There is deliberately **no numeric field anywhere on this type or on
/// [NearbyListener]** - a bucket is rendered from [label] alone. This is a
/// structural (compile-time) defense in depth against ever displaying a
/// metric distance to the user, on top of whatever the backend does or does
/// not send (see `DistanceBucketMapper.fromWire` in this same file for the
/// runtime half of that defense).
enum DistanceBucket {
  /// security.md 1.2: < 150 m.
  veryClose,

  /// security.md 1.2: 150 m - 1 km.
  neighborhood,

  /// security.md 1.2: 1 km - 5 km.
  region,

  /// security.md 1.2: 5 km - 15 km.
  city;

  /// Portuguese labels are security.md section 1.2's actual product copy
  /// ("Bem pertinho" / "No seu bairro" / "Na sua região" / "Na sua cidade"),
  /// not a placeholder - this is the one piece of literal UI string this
  /// domain package owns, because getting a bucket's label wrong is a
  /// privacy defect, not a cosmetic one, so it is defined once next to the
  /// enum it describes rather than duplicated in `social_proximity_ui`.
  String get label {
    switch (this) {
      case DistanceBucket.veryClose:
        return 'Bem pertinho';
      case DistanceBucket.neighborhood:
        return 'No seu bairro';
      case DistanceBucket.region:
        return 'Na sua região';
      case DistanceBucket.city:
        return 'Na sua cidade';
    }
  }
}

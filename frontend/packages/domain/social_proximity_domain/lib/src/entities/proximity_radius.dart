/// security.md section 1.3: "Slider com passos: 150 m / 1 km / 5 km / 15
/// km" - the same thresholds as [DistanceBucket]'s table, by design (the
/// radius is the ceiling of "who can see me", the buckets are how far-away
/// people are described - both are anchored to the same 4 distances so the
/// UI's slider steps line up 1:1 with the labels a viewer might see).
enum ProximityRadius {
  m150,
  km1,
  km5,
  km15;

  int get meters {
    switch (this) {
      case ProximityRadius.m150:
        return 150;
      case ProximityRadius.km1:
        return 1000;
      case ProximityRadius.km5:
        return 5000;
      case ProximityRadius.km15:
        return 15000;
    }
  }

  String get label {
    switch (this) {
      case ProximityRadius.m150:
        return '150 m';
      case ProximityRadius.km1:
        return '1 km';
      case ProximityRadius.km5:
        return '5 km';
      case ProximityRadius.km15:
        return '15 km';
    }
  }

  /// security.md 1.3: "Padrão ao ativar a feature: 1 km."
  static const ProximityRadius defaultValue = ProximityRadius.km1;
}

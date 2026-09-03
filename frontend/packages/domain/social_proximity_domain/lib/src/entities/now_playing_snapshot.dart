/// What a nearby listener is currently playing, as much as reveal level 0
/// is allowed to show (security.md section 1.6: "Alguém por perto está
/// ouvindo *[Faixa]*" - track only, no artist required by that copy, but
/// artist is included here since reveal levels 1/2 show it alongside the
/// name and it costs nothing to carry it for level 0's card too, where the
/// UI simply chooses not to render it).
class NowPlayingSnapshot {
  const NowPlayingSnapshot({required this.trackTitle, this.artistName});

  final String trackTitle;
  final String? artistName;

  @override
  bool operator ==(Object other) =>
      other is NowPlayingSnapshot &&
      other.trackTitle == trackTitle &&
      other.artistName == artistName;

  @override
  int get hashCode => Object.hash(trackTitle, artistName);
}

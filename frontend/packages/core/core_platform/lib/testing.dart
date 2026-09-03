/// Test-only fakes for `core_platform` interfaces, per
/// docs/architecture/frontend-flutter.md section 5.2 ("FakeNativeAudioEngine
/// ... permite simular buffering, erro de rede, fim de faixa, sem tocar
/// áudio de verdade"). Exported from a separate entry point so production
/// code never accidentally imports test doubles.
library core_platform.testing;

export 'src/audio_engine/native_audio_engine.dart';
export 'src/location/location_provider.dart';
export 'src/testing/fake_location_provider.dart';
export 'src/testing/fake_native_audio_engine.dart';

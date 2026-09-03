# smusic — Frontend (Flutter, Web + Mobile)

Implementation of **Fatia 1** (auth, catalog/library, basic playback — see
`docs/architecture/00-overview.md` section 3) of the architecture described
in `docs/architecture/frontend-flutter.md`. Social/proximity discovery
(Fatia 2) is explicitly out of scope here.

## Monorepo layout

Melos workspace, feature-first within each layer (section 1.2 of the spec):

```
frontend/
  packages/
    core/
      core_platform/        # NativeAudioEngine (just_audio), LocationProvider
                             # (interface only), OfflineStorage (Noop only)
      core_networking/      # dio ApiClient, auth/retry interceptors
      core_design_system/   # theme, tokens, skeletons, shared widgets
    domain/
      auth_domain/           library_domain/           player_domain/
      social_proximity_domain/   # placeholder only, see NOTE.md — Fatia 2
    data/
      auth_data/              library_data/              player_data/
      social_proximity_data/ # placeholder only, see NOTE.md — Fatia 2
    presentation/
      auth_ui/  library_ui/  player_ui/  shared_navigation/
      social_proximity_ui/   # placeholder only, see NOTE.md — Fatia 2
  app/
    smusic_app_shared/  # the single SmusicApp widget (root + theme + router)
    smusic_mobile/       # thin entrypoint (iOS/Android)
    smusic_web/          # thin entrypoint (Web)
  tool/
    check_layer_deps.sh  # grep-based domain/presentation/data layering check
  melos.yaml
```

`social_proximity_*` directories contain only a `NOTE.md` and no
`pubspec.yaml`, so `melos bootstrap` does not treat them as packages yet —
per the task instruction, the directory shape is left in place for Fatia 2
without implementing anything inside it.

## Running it

Prerequisites: Flutter SDK (stable channel), Melos.

```bash
# from frontend/
dart pub global activate melos   # if not already installed
melos bootstrap                  # resolves + links all 16 workspace packages
```

Run the mobile app (needs an Android/iOS toolchain and device/emulator):

```bash
cd app/smusic_mobile
flutter run
# point at a non-default backend:
flutter run --dart-define=SMUSIC_API_BASE_URL=https://api.staging.smusic.dev
```

Run the web app (needs Chrome):

```bash
cd app/smusic_web
flutter run -d chrome
```

Both entrypoints default `SMUSIC_API_BASE_URL` to `http://localhost:8080`
(the Go backend's default dev port per `docs/architecture/backend-go.md`).

## Testing

```bash
# whole workspace
melos run analyze        # flutter analyze in every package
melos run test           # flutter test --coverage in every package with test/
melos run check-layers   # architecture layering check (see below)

# one package
cd packages/domain/auth_domain
dart test --coverage=coverage   # pure-Dart packages: dart test
# or, in a Flutter package:
flutter test --coverage
```

`smusic_mobile`/`smusic_web` have no `test/` directory: both are wiring-only
`main.dart` files (see "Desvios da spec" below) — exercising them would
mean mocking `flutter_secure_storage`'s and `just_audio`'s platform
channels for a file with no branching logic of its own, which
`docs/architecture/00-overview.md` section 2's exclusion policy treats the
same way `backend-go.md` treats `main.go`: infra wiring, not business
logic. Every concrete class they instantiate (`SecureTokenStorage`,
`HttpAuthRepository`, `JustAudioPlaybackAdapter`, ...) is unit-tested at
100% in its own package instead.

## Real results (last full run, all 16 packages)

`flutter analyze` — **clean, zero issues, in every package** (`melos run
analyze` / `melos exec -- flutter analyze`, no infos/warnings/errors
suppressed).

`melos bootstrap` — **succeeds**, 16/16 packages bootstrapped.

`tool/check_layer_deps.sh` (`melos run check-layers`) — **passes**: no
`domain/*` file imports Flutter or a `data`/`presentation` package; no
`presentation/*` file imports a `data` package directly.

Tests — **338/338 passing**, **100% line coverage of hand-written code**
in every one of the 14 packages that have a `test/` directory (excluding
`*.g.dart`/`*.freezed.dart` — none exist in this codebase, see "Desvios da
spec" — and the documented exclusions listed below, per
`docs/architecture/00-overview.md` section 2's "cada exclusão precisa de
justificativa explícita e revisável, nunca silenciosa"):

| Package | Tests | Lines covered |
|---|---:|---:|
| `core_networking` | 25 | 85/85 |
| `core_platform` | 25 | 57/57 |
| `core_design_system` | 18 | 57/57 |
| `auth_domain` | 35 | 85/85 |
| `library_domain` | 39 | 108/108 |
| `player_domain` | 24 | 56/56 |
| `auth_data` | 25 | 80/80 |
| `library_data` | 26 | 85/85 |
| `player_data` | 42 | 168/168 |
| `auth_ui` | 13 | 91/91 |
| `library_ui` | 24 | 89/89 |
| `player_ui` | 24 | 107/107 |
| `shared_navigation` | 16 | 67/67 |
| `smusic_app_shared` | 2 | 12/12 |
| **Total** | **338** | **1147/1147** |

`smusic_mobile`/`smusic_web`: no test suite (see rationale above);
`flutter analyze` clean in both.

Coverage was measured with `flutter test --coverage` /
`dart test --coverage` + `package:coverage`'s `format_coverage
--check-ignore`, which honors `// coverage:ignore-line` and
`// coverage:ignore-start`/`-end` markers. Every marker used in this
codebase is listed under "Documented coverage exclusions" below — none are
silent.

### Documented coverage exclusions

1. **`JustAudioNativeEngine`'s instance methods**
   (`core_platform/lib/src/audio_engine/just_audio_native_audio_engine.dart`).
   This is the one class in the whole monorepo that imports
   `package:just_audio`; its methods are thin bindings onto a real platform
   channel (`JustAudioPlatform.instance`), which throws
   `MissingPluginException` under plain `flutter test` — the same category
   of exclusion `backend-go.md` section 7 grants `main.go`/DI wiring. The
   one piece of real logic in that file (`ProcessingState` →
   `PlaybackEngineState` mapping) was extracted to a top-level function and
   *is* fully unit-tested. Verified instead by manual `flutter run` smoke
   test.
2. **Private no-op constructors** (`ClassName._()`) on static-only utility/
   token classes (`SmusicSpacing`, `SmusicColors`, `Breakpoints`,
   `SmusicTheme`, `AuthDtos`, `LibraryDtos`, `PlaybackDtos`) — they exist
   only to block instantiation and are never meant to run.
3. **One unreachable branch** in `library_domain`'s `SearchNotifier`
   (`search_notifier.dart`): the `loading:` callback passed to
   `AsyncValue.guard(...)`'s result `.when(...)` — `AsyncValue.guard` only
   ever produces `AsyncData`/`AsyncError`, never `AsyncLoading`, so that
   callback is required by `.when()`'s exhaustiveness but never actually
   invoked.

A related, non-exclusion methodological note for future maintainers: several
test files deliberately call a constructor **without** `const` even when the
production code marks it `const` (with a comment saying so). A `const Foo(x)`
literal is canonicalized at compile time, and canonicalized construction
never shows as a "hit" line to `package:coverage` — even though the
constructor genuinely ran. This is not a real coverage exclusion (no
`// coverage:ignore` marker is used); it is just how the test invocation is
written so the line legitimately registers as covered.

## Desvios da spec (com justificativa)

1. **Riverpod escrito à mão, não `@riverpod` code-gen**
   (`riverpod_generator`/`riverpod_annotation`, spec section 1.1). Toda
   lógica de domínio ainda vive em `Notifier`/`AsyncNotifier` puros
   (`AuthSessionNotifier`, `LibraryPlaylistsNotifier`, `SearchNotifier`),
   só que declarados manualmente. Evita um passo de `build_runner` no
   loop de teste/CI desta fatia; a superfície pública (providers
   observáveis, overrides no `ProviderScope`) é a mesma que o code-gen
   geraria, então migrar depois é um refactor mecânico, não uma mudança de
   arquitetura.
2. **`PlayerState` como sealed class escrita à mão, não `freezed`**
   (spec section 2.1 já mostra a assinatura ilustrativa como sealed
   class). Dart 3 tem sealed classes + pattern matching exaustivo nativos;
   com 5 variantes fixas e sem unions aninhadas, o ganho de `freezed` não
   paga um novo `build_runner` nesta fatia.
3. **`PlaybackQueueController` ganhou `pause()`/`resume()`**, ausentes no
   snippet ilustrativo da seção 2.1 (que só lista `playFromQueue`/
   `skipNext`/`skipPrevious`/`seekTo` + streams). Extensão aditiva
   necessária para o escopo concreto "play/pause/seek/next/previous
   funcionando fim a fim" — nenhum método do snippet original foi
   removido ou alterado.
4. **Dois pacotes não nomeados explicitamente na árvore da seção 1.2**:
   `presentation/auth_ui` (segue o mesmo padrão de `library_ui`/
   `player_ui` — cada feature com seu próprio pacote de apresentação) e
   `app/smusic_app_shared` (hospeda o único `SmusicApp` compartilhado
   exigido pela seção 1.3 — a árvore da seção 1.2 não mostra onde esse
   widget mora, e colocá-lo em `shared_navigation` misturaria "config de
   rotas" com "raiz do app/tema", que a própria seção 1.2 escopa
   separadamente).
5. **Redirect do `go_router` simplificado**: `isAuthenticated` é um
   snapshot síncrono (`ref.read`), não uma escuta reativa completa via
   `refreshListenable` sozinho. A transição interativa login/signup →
   app é disparada explicitamente pelas próprias telas
   (`LoginScreen.onLoggedIn`/`SignUpScreen.onSignedUp`), não pelo
   `redirect`. Para o caso "abrir o app já com sessão salva" (onde não há
   nenhuma ação explícita do usuário para disparar navegação),
   `GoRouterRefreshListenable` escuta `authSessionProvider` e força uma
   reavaliação do `redirect` quando a sessão restaurada resolve — ver
   comentários em `shared_navigation/lib/src/app_router.dart` e
   `go_router_refresh_listenable.dart`.
6. **Golden test com a API nativa do Flutter (`matchesGoldenFile`)**, não
   `golden_toolkit`/`alchemist` (mencionados como opções na seção 5.2) —
   evita uma dependência extra para o único componente golden exigido
   nesta fatia (`SmusicPrimaryButton`, 2 variantes: tema claro habilitado,
   tema escuro carregando).
7. **Enforcement de camadas via script grep** (`tool/check_layer_deps.sh`,
   rodado por `melos run check-layers`), não um lint customizado completo
   via `dart_dependency_validator`/`custom_lint` (seção 1.2 menciona
   ambos como possibilidade). Documentado como TODO explícito para CI
   real — ver comentário no topo do script.
8. **Prefetch de uma faixa à frente (seção 2.2, "prefetch antecipado") está
   implementado**: `JustAudioPlaybackAdapter._prefetchNext()` resolve e
   chama `NativeAudioEngine.setNextSource` para `queue[currentIndex + 1]`
   logo após `playFromQueue`/`skipNext`/`skipPrevious` carregarem a faixa
   atual (com melhor esforço — uma falha na resolução do prefetch nunca
   derruba a faixa que já está tocando), e `skipNext()` reaproveita essa
   resolução já quente em vez de refazer a chamada de rede quando ela bate
   com a faixa de destino. **Prefetch preditivo de 3 faixas** (segundo
   bullet da seção 2.2), **seleção adaptativa de bitrate**, e **crossfade**
   (seção 2.3) continuam como TODO, autorizado pelo escopo da tarefa.
   `TrackSourceResolver` (resolução local-primeiro, seção 2.5) não existe —
   toda fonte é uma URL de streaming. Gapless real (transição no nível do
   engine, seção 2.3) depende também de `JustAudioNativeEngine.load`
   carregar um `ConcatenatingAudioSource` — hoje ele carrega uma
   `ja.AudioSource.uri` avulsa, então `setNextSource` no engine real é um
   no-op documentado (ver comentário em
   `core_platform/lib/src/audio_engine/just_audio_native_audio_engine.dart`);
   isso é uma mudança no nível do engine (arquivo com exclusão de
   cobertura, sem lógica de branching testável), não no adapter, e fica
   fora do escopo desta correção pontual.
   **Suposição sinalizada para o especialista de backend**: `play()` é o
   único resolvedor de `PlaybackSessionRepository` endereçável por
   `trackId`, e por `backend-go.md` seção 4 ele também marca a faixa como
   "now playing" no servidor — não existe um endpoint de "resolver sem
   marcar como tocando agora". `_prefetchNext()` reusa `play()` mesmo assim,
   o que move o ponteiro "now playing" do backend para N+1 um pouco antes
   do áudio local realmente chegar lá; invisível nesta fatia (não há UI de
   sincronização entre dispositivos ainda — ver item 11), mas deve ser
   revisitado quando essa UI existir.
9. **Busca "instantânea local" (seção 3.3) não implementada** — não há
   índice local de biblioteca/cache para consultar; a busca em
   `SearchNotifier` é 100% remota (debounce de 300ms + paginação por
   cursor funcionando).
10. **Sem telas de detalhe de faixa/álbum/playlist** — `onPlaylistTap`/
    `onResultTap` são pontos de extensão (`void Function(...)?`) já
    conectados em `buildAppRouter`, mas sem destino de navegação real
    nesta fatia (não estavam no escopo concreto da tarefa). Histórico de
    reprodução (`GET /v1/library/me/history`) também não foi
    implementado, pelo mesmo motivo.
11. **`deviceId` de sessão de playback é gerado por lançamento**
    (`'mobile-$microsecondsSinceEpoch'`/`'web-$microsecondsSinceEpoch'`),
    não um id persistido por instalação. Sem consequência funcional nesta
    fatia (sync entre dispositivos tipo "Connect" não está no escopo),
    mas passaria a criar uma sessão de backend nova a cada abertura do
    app quando esse escopo chegar — sinalizado com TODO no `main.dart` de
    cada app.
12. **Suposições sobre lacunas do contrato do backend** (cada uma também
    comentada no código-fonte relevante, com "flagged for the backend
    specialist"):
    - `POST /v1/auth/signup` recebe um campo `display_name` adicional,
      não mostrado na assinatura ilustrativa de `backend-go.md` seção 4
      (mas necessário — `GET /v1/auth/me` devolve um).
    - `POST /v1/auth/refresh` pode ou não devolver um novo
      `refresh_token`; se ausente, o cliente reusa o antigo.
    - Nenhuma resposta de auth traz expiração explícita do access token;
      aproximada como `agora + 15min` no cliente (o refresh reativo em
      401 do `AuthInterceptor` é a rede de segurança real caso a
      estimativa erre).
    - Cada item de `GET /v1/catalog/search`'s `results[]` é assumido como
      `{ id, type, title, subtitle }`.
    - `POST .../play`'s resposta não ecoa `track_id` (o cliente já sabe
      qual pediu); `POST .../next`'s resposta não tem `expires_at`
      (assumido `agora + 5min`, a extremidade conservadora da janela de
      "5-10 min" citada na seção 2 de `backend-go.md`).
13. **`core_platform.LocationProvider`/`OfflineStorage`**: interfaces
    existem (conforme pedido — "pode ficar como interface só"), mas
    nenhum provider Riverpod as referencia nesta fatia; `OfflineStorage`
    só tem `NoopOfflineStorage` (retorna sempre "não suportado").

## Arquitetura de camadas (enforcement)

Ver `docs/architecture/frontend-flutter.md` seção 1.2. Regras impostas por
`tool/check_layer_deps.sh`:

- `domain/*` nunca importa Flutter, nem `data/*`, nem `presentation/*`.
- `presentation/*` nunca importa `data/*` diretamente (a ligação
  `data → domain` acontece só em `app/smusic_mobile`/`app/smusic_web`, via
  overrides de `ProviderScope`).

`domain/player_domain` importa `core_platform` (que depende de Flutter,
por causa do `just_audio`) — isso é **permitido explicitamente** pela
spec ("domain depende só de Dart puro + core/core_platform (interfaces)"),
não uma violação: nenhum arquivo de `player_domain` importa
`package:flutter/...` diretamente, ele só usa os tipos de vocabulário do
`NativeAudioEngine` (`AudioSource`, `PlaybackEngineState`, ...) que
`core_platform` expõe.

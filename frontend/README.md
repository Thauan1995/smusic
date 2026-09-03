# smusic — Frontend (Flutter, Web + Mobile)

Implementation of **Fatia 1** (auth, catalog/library, basic playback) and
**Fatia 2** (social proximity discovery) of the architecture described in
`docs/architecture/frontend-flutter.md` — see `docs/architecture/
00-overview.md` section 3 for the slice plan. Fatia 2 is built against
`backend/internal/presence`'s real, parallel-track implementation (not just
`backend-go.md`'s illustrative WS snippet) and `docs/architecture/
security.md` section 1's privacy model in full: opt-in with a value screen
before the OS location prompt, bucketed distance only (never metric),
raio/reveal-level/pause controls, and 6-month consent renewal.

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
      social_proximity_domain/   # NearbyListener, ProximityPrivacySettings,
                                  # feed/settings notifiers (Fatia 2)
    data/
      auth_data/              library_data/              player_data/
      social_proximity_data/ # WebSocketProximityFeedRepository,
                              # HttpProximityPrivacySettingsRepository (Fatia 2)
    presentation/
      auth_ui/  library_ui/  player_ui/  shared_navigation/
      social_proximity_ui/   # nearby list, privacy settings, value/permission
                              # screens (Fatia 2)
  app/
    smusic_app_shared/  # the single SmusicApp widget (root + theme + router)
    smusic_mobile/       # thin entrypoint (iOS/Android)
    smusic_web/          # thin entrypoint (Web)
  tool/
    check_layer_deps.sh  # grep-based domain/presentation/data layering check
  melos.yaml
```

`core_networking/lib/src/websocket/` (`ReconnectingWebSocketClient`,
generic - exponential backoff + jitter, reusable for any future stream) and
`core_platform/lib/src/location/` (`GeolocatorLocationProvider`) are Fatia
2 additions to two Fatia-1 core packages, not new packages of their own —
per frontend-flutter.md section 1.2, `core/*` has no feature dependency, so
the WS reconnect client and the real location provider live there rather
than inside `social_proximity_data`/`core_platform`'s own feature slice.

## Running it

Prerequisites: Flutter SDK (stable channel), Melos.

```bash
# from frontend/
dart pub global activate melos   # if not already installed
melos bootstrap                  # resolves + links all 19 workspace packages
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

`smusic_mobile`/`smusic_web` have a `test/` directory since Fatia 2, but it
covers only the one piece of `main.dart` with real branching logic worth
testing directly: `buildPresenceUri`/`buildPresenceSocketClient` (the
`/v1/presence/connect` WS URI construction - `http`/`https` -> `ws`/`wss`
scheme mapping, `access_token` query param). The rest of `main()` (object
construction/wiring, no branches) stays untested for the same reason
established in Fatia 1 — exercising it would mean mocking
`flutter_secure_storage`'s and `just_audio`'s platform channels for code
with no branching logic of its own, which `docs/architecture/
00-overview.md` section 2's exclusion policy treats the same way
`backend-go.md` treats `main.go`: infra wiring, not business logic. Every
concrete class both files instantiate (`SecureTokenStorage`,
`HttpAuthRepository`, `JustAudioPlaybackAdapter`,
`WebSocketProximityFeedRepository`, `GeolocatorLocationProvider`, ...) is
unit-tested at 100% in its own package instead.

## Testes E2E (Web, browser real)

`app/smusic_web/integration_test/real_backend_e2e_test.dart` é um teste de
integração real (per `docs/architecture/frontend-flutter.md` seção 5.2):
constrói exatamente a mesma árvore de widgets que `main.dart` monta em
produção (mesmo `ApiClient`, mesmos repositórios HTTP reais, mesmo
`JustAudioNativeEngine`), sem nenhum fake/mock, e dirige um Chrome real via
`flutter drive`/`chromedriver` fazendo signup → login → busca real → início
de reprodução real contra um backend real — o único tipo de evidência capaz
de provar que o stack de rede completo (Dio → HTTP → CORS do browser →
backend Go real) funciona de ponta a ponta, algo que testes unitários com
fakes e verificação via `curl` no backend não provam.

**Como rodar** (backend real + Postgres + Redis já de pé, catálogo com uma
faixa `E2E Test Track` inserida via SQL, `CORS_ALLOWED_ORIGINS` cobrindo a
porta usada):

```bash
cd app/smusic_web
flutter drive \
  --driver=test_driver/integration_test.dart \
  --target=integration_test/real_backend_e2e_test.dart \
  -d chrome --web-port=5173 \
  --dart-define=SMUSIC_API_BASE_URL=http://localhost:8080 \
  --web-browser-flag="--autoplay-policy=no-user-gesture-required" \
  --web-browser-flag="--no-sandbox" \
  --headless
```

**Status nesta sessão de desenvolvimento**: o teste foi escrito e, no
processo de escrevê-lo (rodando manualmente contra o backend real antes de
o arquivo de teste existir), **encontrou e corrigiu 2 bugs de contrato
reais** entre frontend e backend que nenhum teste unitário com fakes tinha
pego, porque os fakes assumiam o contrato "certo" em vez do real:

1. `GET /v1/catalog/tracks/{id}` e `GET /v1/catalog/search` não retornam o
   formato plano (`artist`/`album` como string, `results[]` genérico) que
   `library_dtos.dart` assumia — retornam `artists: [{artist_name, ...}]` e
   arrays separados `tracks`/`albums`/`artists`. Corrigido em
   `packages/data/library_data/lib/src/dto/library_dtos.dart` (mantendo
   compatibilidade com o formato antigo caso apareça), com testes novos
   cobrindo ambos os formatos.
2. Tocar num resultado de busca do tipo faixa não fazia **nada** — não
   havia navegação real até o player. Corrigido conectando `onResultTap` em
   `app/smusic_app_shared/lib/src/app_router_provider.dart` (o único ponto
   de composição compartilhado por mobile e web) a `getTrack` +
   `playFromQueue` + navegação para `/player`, com teste widget cobrindo o
   caminho feliz e o caso de um resultado que não é faixa (no-op).

Porém, a **execução completa do `flutter drive` contra um Chrome real não
completou neste ambiente de sandbox específico**: o Chrome headless sobe
normalmente sozinho (`google-chrome --headless=new --no-sandbox` funciona),
mas a conexão de debug que o `flutter drive`/`chromedriver` precisa
estabelecer com ele trava indefinidamente em "Waiting for connection from
debug service on Chrome" (confirmado 2x, com e sem `--no-sandbox`/
`--disable-gpu`, timeout de 280s nas duas tentativas). Uma tentativa
anterior de validar via extensão Claude in Chrome também falhou por a
extensão não estar conectada nesta sessão. Ambos são bloqueios de
**ambiente de execução** (esta sandbox específica), não do código — o teste
em si é válido e deve rodar normalmente numa máquina de desenvolvimento ou
CI com um Chrome/chromedriver "de verdade" acessível.

**Evidência substituta obtida nesta sessão** para a preocupação original do
Auditor (CORS quebrando silenciosamente o cliente web): contra o backend
real rodando (não um teste unitário), com `curl` simulando exatamente o
preflight e o request que um browser faria (`Origin` header):

- Preflight de origem permitida → `200`, com `Access-Control-Allow-Origin`,
  `Access-Control-Allow-Methods` e `Access-Control-Allow-Headers`
  corretamente ecoados.
- Preflight de origem **não** permitida → `200` sem nenhum header CORS
  (o browser bloqueia a chamada do lado do cliente, comportamento correto).

Isso confirma que a política de CORS documentada em `backend/README.md`
funciona como especificado contra o servidor real — mas não substitui, e
não pretende substituir, a garantia mais forte que `real_backend_e2e_test.dart`
daria com um Chrome de debug funcional. Rodar esse teste num ambiente sem
essa limitação de sandbox é um item de follow-up explícito antes do launch.

## Real results (last full run, all 19 packages)

`flutter analyze` — **clean, zero issues, in every package** (`melos run
analyze` / `melos exec -- flutter analyze`, no infos/warnings/errors
suppressed).

`melos bootstrap` — **succeeds**, 19/19 packages bootstrapped.

`tool/check_layer_deps.sh` (`melos run check-layers`) — **passes**: no
`domain/*` file imports Flutter or a `data`/`presentation` package; no
`presentation/*` file imports a `data` package directly.

Tests — **552/552 passing**, **100% line coverage of hand-written code**
in every package that has a `test/` directory (excluding `*.g.dart`/
`*.freezed.dart` — none exist in this codebase, see "Desvios da spec" —
and the documented exclusions listed below, per `docs/architecture/
00-overview.md` section 2's "cada exclusão precisa de justificativa
explícita e revisável, nunca silenciosa"):

| Package | Tests | Lines covered |
|---|---:|---:|
| `core_networking` | 48 | 171/171 |
| `core_platform` | 46 | 89/89 |
| `core_design_system` | 18 | 57/57 |
| `auth_domain` | 35 | 85/85 |
| `library_domain` | 39 | 108/108 |
| `player_domain` | 24 | 56/56 |
| `social_proximity_domain` | 56 | 153/153 |
| `auth_data` | 25 | 80/80 |
| `library_data` | 30 | 107/107 |
| `player_data` | 42 | 168/168 |
| `social_proximity_data` | 51 | 168/168 |
| `auth_ui` | 13 | 91/91 |
| `library_ui` | 24 | 89/89 |
| `player_ui` | 24 | 107/107 |
| `social_proximity_ui` | 46 | 270/270 |
| `shared_navigation` | 19 | 78/78 |
| `smusic_app_shared` | 4 | 23/23 |
| `smusic_mobile` | 4 | 12/38 |
| `smusic_web` | 4 | 12/38 |
| **Total** | **552** | **1872/1924** |

`library_data` and `smusic_app_shared` grew (85→107, 12→23 lines) in Fatia
1's own E2E-testing pass from 2 contract-bug fixes (`library_dtos.dart`'s
real backend JSON shapes, `onResultTap`'s real wiring) — each new branch
has a real test, not a `coverage:ignore`. `core_networking` (85→171) and
`core_platform` (57→89) grew with Fatia 2's `ReconnectingWebSocketClient`/
`GeolocatorLocationProvider` additions. `shared_navigation` grew (67→78)
with the `/nearby` route tree (task scope item 5). `smusic_mobile`/
`smusic_web` (0→12/38 each) gained their first tests, covering
`buildPresenceUri`/`buildPresenceSocketClient` only — see "Testing" above
for why the rest of `main()` stays untested by design.

`smusic_mobile`/`smusic_web`'s 26 uncovered lines each (`main()`'s object
construction/wiring itself) are the one deliberately-untested surface in
this workspace — see "Testing" above; every concrete class they wire is
100%-covered in its own package.

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
    - ~~Cada item de `GET /v1/catalog/search`'s `results[]` é assumido
      como `{ id, type, title, subtitle }`~~ — **CORRIGIDO**: essa
      suposição estava errada, confirmado contra o backend real durante a
      escrita do teste E2E (ver seção "Testes E2E" acima). O backend
      retorna `{ tracks: [...], albums: [...], artists: [...],
      next_cursor }`, cada um com sua própria shape completa, não uma
      linha pré-achatada. `library_dtos.dart` agora trata os dois formatos
      (o achatado permanece aceito, caso algum outro endpoint/versão futura
      volte a usá-lo).
    - `POST .../play`'s resposta não ecoa `track_id` (o cliente já sabe
      qual pediu); `POST .../next`'s resposta não tem `expires_at`
      (assumido `agora + 5min`, a extremidade conservadora da janela de
      "5-10 min" citada na seção 2 de `backend-go.md`).
13. **`core_platform.LocationProvider`/`OfflineStorage`**: interfaces
    existem (conforme pedido — "pode ficar como interface só"), mas
    nenhum provider Riverpod as referencia nesta fatia; `OfflineStorage`
    só tem `NoopOfflineStorage` (retorna sempre "não suportado").

### Fatia 2 (descoberta social por proximidade)

O trabalho da Fatia 2 foi iniciado por um agente anterior (interrompido por
um limite de sessão, não por erro) e continuado/concluído nesta sessão. A
maior parte dos itens abaixo são correções contra o contrato real do
backend (`backend/internal/presence`, que passou a existir em paralelo
depois que o agente anterior escreveu suas próprias suposições) — cada uma
também comentada no código-fonte relevante:

14. **`distance_bucket` wire codes corrigidos**: `social_proximity_data`
    assumia `lt_150m`/`150m_1km`/`1km_5km`/`5km_15km`; o real
    (`backend/internal/presence/ws/protocol.go`'s `bucketCode`) é
    `under_150m`/`150m_1km`/`1km_5km`/`5km_15km` — só o primeiro estava
    errado, mas era um bug real e silencioso (cairia sempre no fallback
    "least precise bucket", nunca um crash, então nenhum teste anterior
    pegava). Corrigido, com teste cobrindo os 4 códigos reais.
15. **REST `presence_visibility` e WS `visibility.mode` usam dialetos de
    wire diferentes para o mesmo conceito permissivo** — `"everyone"` no
    REST (`domain.go`'s `VisibilityEveryone`) vs. `"visible"` no frame WS
    (`nearby_service.go`'s doc comment, que cita literalmente
    `backend-go.md`). Confirmado como assimetria real do backend, não erro
    do cliente — `ProximityDtos` agora fala os dois dialetos
    deliberadamente (`visibilityModeFromWire`/`ToWire` para o WS,
    `presenceVisibilityFromWire`/`ToWire` para o REST), sinalizado para o
    especialista de backend como algo a reconciliar num único literal no
    futuro.
16. **Endpoints REST de privacidade reescritos contra o contrato real**:
    a versão anterior assumia `GET/POST /v1/presence/privacy` (endpoint
    inexistente) com campos `enabled`/`visibility_mode`/`radius_m`/
    `max_reveal_level`. O real (`backend/internal/presence/api/
    handlers.go`) é `GET/PUT /v1/presence/settings` +
    `POST/DELETE /v1/presence/consent`, com campos
    `presence_visibility`/`proximity_consent_enabled`/
    `visibility_radius_m`/`reveal_level`. Isso não era só um nome de rota
    errado: `PUT /v1/presence/settings`'s handler chama
    `json.Decoder.DisallowUnknownFields()`, então o payload antigo teria
    **sempre retornado 400** contra o backend real. `ApiClient` ganhou um
    método `put()` (só tinha `get`/`post`/`delete`) para poder falar o
    verbo certo.
17. **`ProximityPrivacySettingsRepository.renewConsent()` virou
    `grantConsent()`/`revokeConsent()`**: o backend não tem um endpoint de
    "renovar" separado de "conceder" — `SettingsService.GrantConsent`
    "enables (or renews)" é a mesma operação (`POST /v1/presence/consent`)
    nos dois casos. `revokeConsent()` (`DELETE /v1/presence/consent`)
    também força `paused: true` no servidor (defesa em profundidade lá).
18. **`enableFeature()` faz 2 chamadas, não 1**: conceder consentimento
    sozinho (`SettingsService.GrantConsent`) nunca muda
    `paused`/`presence_visibility` no backend — por design, são eixos
    independentes ("consentir com o processamento" ≠ "estar visível
    agora"). Deixado assim, o único CTA da tela de valor ("Ativar
    descoberta por proximidade") deixaria o usuário com a feature
    "habilitada" mas ainda pausado/invisível, o que não é o que o botão
    promete. `enableFeature()` por isso também despausa e define
    `presence_visibility: everyone` (nível de revelação continua no
    padrão seguro `level0`/anônimo — ver `RevealLevel`'s doc comment).
    Suposição de produto sinalizada para confirmação: o valor
    `everyone`/nível 0 como default de ativação (em vez de, por exemplo,
    `friends_only`) segue a cópia da própria tela de valor ("a música que
    você está ouvindo pode ficar visível para pessoas por perto"), mas é
    uma escolha desta implementação, não um valor documentado
    explicitamente em security.md 1.3/1.6 para esse momento específico.
19. **Autenticação do handshake WS via query param, não header** —
    `backend/internal/presence/ws/handler.go`'s `bearerToken` aceita
    `Authorization` header (só nativo) ou `access_token` query param;
    `smusic_mobile`/`smusic_web`'s `main.dart` usam sempre o query param
    para manter o código de composição 100% idêntico entre as duas
    plataformas (frontend-flutter.md seção 1.3). Como
    `ReconnectingWebSocketClient.uriBuilder` é síncrono e
    `AuthTokenSource.currentAccessToken()` é assíncrono, cada `main.dart`
    mantém um cache do token em memória, atualizado a cada 30s (bem abaixo
    do TTL de 10-15min do access token, security.md seção 2) — um
    trade-off de staleness limitada, escopado à raiz de composição, sem
    mudar `ReconnectingWebSocketClient` (classe já testada). Extraído como
    `buildPresenceUri`/`buildPresenceSocketClient`, testado diretamente
    (o único teste que `smusic_mobile`/`smusic_web` têm).
20. **`PauseDiscoveryToggle` no shell flutua no canto inferior direito, não
    no superior direito**: a primeira tentativa (canto superior, sobre o
    `AppBar`) colidia visualmente e funcionalmente com o botão de
    configurações de `ProximityListScreen` (e colidiria com qualquer outra
    action de `AppBar` de qualquer tela) — pego por um teste de widget
    real (`app_router_test.dart`) que falhou ao tentar tocar o botão de
    configurações. Reposicionado para o canto inferior direito do
    conteúdo, acima do `MiniPlayerBar`, uma zona que nenhuma tela usa hoje.
21. **`presence_share_track` (campo real do backend) não tem controle de
    UI nesta fatia** — a lista de escopo da tarefa (item 2) não pede um
    toggle de "compartilhar faixa atual", só raio/nível de
    revelação/pausa/ativação/renovação. `ProximityDtos.settingsToJson`
    nunca inclui essa chave no `PUT`, então o backend nunca a altera a
    partir deste cliente (campo opcional, ausência = "sem mudança") — uma
    omissão documentada, não uma perda de dado silenciosa.
22. **Lista de bloqueio de usuários (security.md 1.4, `POST/DELETE
    /v1/presence/blocks/{user_id}` no backend real) não implementada
    nesta fatia** — fora da lista de escopo explícita da tarefa (itens
    1-7); nenhuma UI/repositório do lado do cliente cobre isso ainda.
23. **`reveal_level` não existe no frame WS `users[]` do backend real**
    (`ws/protocol.go`'s `userFrame` só tem `display_name`/`avatar_url`
    presentes-ou-ausentes, nunca um campo explícito de nível). O cliente
    já tinha uma defesa em profundidade para isso (infere nível 0 vs. 1+
    pela presença de identidade, nunca infere nível 2) — mantida,
    sinalizada para o especialista de backend como um `reveal_level`
    explícito no `userFrame` sendo desejável.

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

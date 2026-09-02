# smusic — Arquitetura Frontend (Flutter, Web + Mobile)

Status: **Fase de planejamento.** Este documento é uma spec de arquitetura, não uma descrição de código existente. Nenhuma implementação foi feita ainda. Trechos de Dart são assinaturas ilustrativas, não código de produção.

Escopo: cliente Flutter compartilhado entre Web e Mobile (iOS/Android), com meta declarada de igualar ou superar Spotify e YouTube Music (YTM) em reprodução, biblioteca e performance, e de entregar um diferencial competitivo próprio — descoberta social em tempo real por proximidade física.

Restrição não negociável do projeto: **100% de reuso de UI e lógica de negócio entre web e mobile**. Fork de implementação por plataforma é proibido; é permitido isolar apenas a fina camada de binding nativo (plugins, APIs de plataforma) atrás de interfaces comuns.

---

## 1. Arquitetura do app

### 1.1 Gerenciamento de estado: Riverpod (code-gen, `riverpod_generator` + `riverpod_annotation`)

**Decisão: Riverpod sobre Bloc.**

Justificativa:

- **Independência de `BuildContext`.** Riverpod providers são acessíveis fora da árvore de widgets (services, background isolates, callbacks de plugins nativos como `audio_service`). Isso é decisivo para o player: o estado de reprodução precisa ser lido/escrito a partir de um `AudioHandler` rodando em contexto de serviço em background, sem widget tree disponível. Com Bloc isso exige gambiarras (singletons de `BlocProvider` fora do contexto, `GetIt`); com Riverpod é o caso de uso padrão (`ProviderContainer` global único, injetado no `AudioHandler`).
- **Composição declarativa de dependências.** A tela "quem está por perto" depende simultaneamente de: stream de WebSocket, permissão de localização, posição do GPS, e estado de autenticação. Com Riverpod isso é um grafo de `Provider`s que se recombinam automaticamente (`ref.watch` em cascata); com Bloc seria um `BlocListener`/`MultiBlocProvider` aninhado manualmente, com mais boilerplate de coordenação.
- **Testabilidade sem widget.** `ProviderContainer` permite testar lógica de domínio (ex.: cálculo de próxima faixa no gapless queue) sem montar nenhum widget — relevante para a meta de cobertura de linha alta (seção 5).
- **Code-gen (`@riverpod`) reduz boilerplate** e dá tipagem forte, aproximando a ergonomia de Bloc (que tem boilerplate explícito por design) sem abrir mão da flexibilidade de composição do Riverpod puro.
- Bloc não é uma escolha ruim — é mais opinativo e força uma disciplina de eventos/estados que ajuda em times grandes — mas o requisito concreto de "estado do player acessível fora da árvore de widgets, compartilhado entre engines nativas diferentes" pesa a favor de Riverpod. Bloc é aceitável reavaliar se o time tiver forte experiência prévia com ele; não é bloqueante de arquitetura.

Regras de uso:
- Toda lógica de negócio vive em `Notifier`/`AsyncNotifier` gerados (`@riverpod`), nunca em `StatefulWidget`.
- Widgets são `ConsumerWidget`/`HookConsumerWidget` (usamos `flutter_hooks` + `hooks_riverpod` para controllers efêmeros de UI como `TextEditingController`, `ScrollController` — mantém widgets stateless mesmo com estado local de UI).
- `Provider`s de domínio nunca importam Flutter (`package:flutter/material.dart`); isso é verificado por lint customizado (ver 1.3).

### 1.2 Estrutura em camadas

Monorepo único (Melos + workspaces do Dart/Flutter), organizado por **feature-first** dentro de cada camada, não por camada dentro de feature — isso evita que "player" vire uma pasta gigante misturando domain/data/UI.

```
smusic/
  packages/
    core/                      # sem dependência de features; usado por todas
      core_platform/           # abstrações de plataforma (ver 1.3)
      core_networking/         # dio/http client, interceptors, retry
      core_design_system/      # tema, tokens, componentes visuais compartilhados
      core_analytics/          # abstração de telemetria
    domain/
      player_domain/           # entidades, use cases, interfaces de repositório do player
      library_domain/
      social_proximity_domain/
      auth_domain/
    data/
      player_data/             # implementações de repositório, DTOs, mappers
      library_data/
      social_proximity_data/
      auth_data/
    presentation/
      player_ui/                # widgets + Notifiers de apresentação
      library_ui/
      social_proximity_ui/
      shared_navigation/         # go_router config, shell de navegação
  app/
    smusic_mobile/               # entrypoint mobile (main.dart fino)
    smusic_web/                  # entrypoint web (main.dart fino)
```

Regras de dependência (impostas por `melos` + `import_lint`/`dart_dependency_validator` em CI):

- `domain/*` não depende de `data/*` nem de `presentation/*` nem de Flutter. Depende só de Dart puro + `core/core_platform` (interfaces).
- `data/*` depende de `domain/*` (implementa as interfaces de repositório) e de `core/core_networking`.
- `presentation/*` depende de `domain/*` (via providers que chamam use cases) e de `core/core_design_system`. **Nunca depende diretamente de `data/*`** — a ligação `data → domain` é resolvida por injeção de dependência no nível do app (`app/smusic_mobile` e `app/smusic_web` fazem o wiring de `ProviderScope(overrides: [...])`).
- `app/smusic_mobile` e `app/smusic_web` são os **únicos** lugares onde pode existir código condicional de plataforma real (registro de implementações concretas de plugin). Cada um tem menos de ~150 linhas de `main.dart` + bootstrap.

Isso garante 100% de reuso por construção: todo o código em `domain/`, `data/` (exceto os pontos de binding explícitos descritos em 1.3) e `presentation/` é Dart/Flutter puro e compila para as duas plataformas a partir do mesmo pacote.

### 1.3 Mecanismo de reuso 100% web/mobile: abstração de plataforma via interface + injeção

Princípio: **nenhuma feature ou widget importa um plugin nativo diretamente.** Toda capacidade que difere entre web e mobile é definida como uma interface abstrata em `core_platform`, com uma implementação por plataforma registrada via `dart:io`/`kIsWeb` **apenas no ponto de composição** (`app/*`), nunca espalhada pelas features.

```dart
// packages/core/core_platform/lib/src/location/location_provider.dart
abstract interface class LocationProvider {
  Stream<GeoPosition> watchPosition({required LocationAccuracy accuracy});
  Future<LocationPermissionStatus> requestPermission();
  Future<LocationPermissionStatus> checkPermission();
}

// packages/core/core_platform/lib/src/audio_engine/native_audio_engine.dart
abstract interface class NativeAudioEngine {
  Future<void> load(AudioSource source);
  Future<void> play();
  Future<void> pause();
  Future<void> seek(Duration position);
  Stream<PlaybackPositionEvent> get positionStream;
  Stream<PlaybackEngineState> get engineStateStream;
  Future<void> setNextSource(AudioSource source); // suporte a gapless
}

// packages/core/core_platform/lib/src/downloads/offline_storage.dart
abstract interface class OfflineStorage {
  Future<void> saveTrack(TrackId id, Stream<List<int>> bytes);
  Future<File?> getLocalFile(TrackId id); // null no Web -> feature de download é no-op
  Future<void> deleteTrack(TrackId id);
  Stream<double> downloadProgress(TrackId id);
}
```

Implementações concretas (uma por plataforma, cada uma em seu próprio pacote `*_mobile`/`*_web` dentro de `core_platform`):

| Interface | Implementação mobile | Implementação web |
|---|---|---|
| `NativeAudioEngine` | `just_audio` (AVPlayer/ExoPlayer por trás) | `just_audio` com backend `just_audio_web` (MediaElement/Web Audio API) — **mesmo pacote Dart, backend nativo diferente por trás, mesma API Dart exposta** |
| `LocationProvider` | `geolocator` | `geolocator` (usa Geolocation API do browser) — mesmo pacote, funciona nos dois; fallback de precisão degradada tratado dentro da implementação, não na feature |
| `OfflineStorage` | `path_provider` + filesystem | `NoopOfflineStorage` (retorna sempre "não suportado"); feature de download detecta via `OfflineCapability.isSupported` e esconde a UI de download no shell web — **decisão de produto, não de arquitetura**: offline é mobile-only por natureza do Web, mas a feature não faz fork de código, só reage a uma capability flag exposta pela mesma interface |
| Background/lockscreen controls | `audio_service` (MPNowPlayingInfoCenter/MediaSession Android) | `audio_service_web` / MediaSession Web API | 
| Notificações push (novo "amigo por perto") | `firebase_messaging` | `firebase_messaging` (service worker) |

Wiring acontece só em `app/*`:

```dart
// app/smusic_mobile/lib/main.dart
void main() {
  runApp(ProviderScope(
    overrides: [
      locationProviderImpl.overrideWithValue(GeolocatorLocationProvider()),
      offlineStorageImpl.overrideWithValue(FilesystemOfflineStorage()),
    ],
    child: const SmusicApp(),
  ));
}

// app/smusic_web/lib/main.dart
void main() {
  runApp(ProviderScope(
    overrides: [
      locationProviderImpl.overrideWithValue(GeolocatorLocationProvider()), // mesma impl
      offlineStorageImpl.overrideWithValue(NoopOfflineStorage()),
    ],
    child: const SmusicApp(),
  ));
}
```

`SmusicApp` (widget raiz, roteamento `go_router`, tema, todas as telas) é **um único pacote compartilhado**, zero diferença de código-fonte entre as duas entradas. CI roda um teste de arquitetura (`dependency_validator` custom rule) que falha o build se qualquer arquivo fora de `core_platform/*_mobile` ou `core_platform/*_web` importar um pacote de plugin nativo diretamente (allowlist checada por path glob) — isso é o mecanismo de enforcement automatizado do requisito de 100% de reuso, não apenas uma convenção documentada.

Responsividade (não confundir com fork de plataforma): layout adapta por **breakpoint de largura de tela** (`LayoutBuilder`/`core_design_system.Breakpoints`), não por `Platform.isX`. Um mobile web em tablet e um app mobile em telefone dividem os mesmos breakpoints; um desktop web e um tablet grande convergem para o layout "wide". Isso é responsabilidade de `presentation/*`, com os mesmos widgets renderizando árvores diferentes conforme `MediaQuery`, nunca conforme SO.

---

## 2. Arquitetura de reprodução de áudio

Meta: paridade ou superioridade perceptível frente a Spotify/YTM em latência de início de reprodução, ausência de gaps entre faixas, robustez em background, e downloads offline confiáveis.

### 2.1 Engine e abstração

`just_audio` como engine base nas duas plataformas (ver tabela 1.3), por já encapsular ExoPlayer (Android), AVPlayer (iOS/macOS) e Web Audio/MediaElement (Web) atrás de uma única API Dart, e por suportar nativamente `ConcatenatingAudioSource` (fila) e `setAudioSource(..., preload: true)` (prefetch). Isso evita reimplementar abstração de engine do zero — reduz risco na meta de reuso 100%.

Estado do player é modelado como uma máquina de estados única em `player_domain`, **independente de engine**:

```dart
sealed class PlayerState {
  const factory PlayerState.idle() = PlayerIdle;
  const factory PlayerState.buffering(QueueItem current) = PlayerBuffering;
  const factory PlayerState.playing(QueueItem current, Duration position) = PlayerPlaying;
  const factory PlayerState.paused(QueueItem current, Duration position) = PlayerPaused;
  const factory PlayerState.error(PlayerError error) = PlayerErrorState;
}

abstract interface class PlaybackQueueController {
  Future<void> playFromQueue(List<QueueItem> queue, {required int startIndex});
  Future<void> skipNext();
  Future<void> skipPrevious();
  Future<void> seekTo(Duration position);
  Stream<PlayerState> get stateStream;
  Stream<QueueItem?> get nowPlayingStream;
}
```

`PlaybackQueueController` é a única superfície que `presentation/player_ui` conhece. A implementação (`data/player_data`) traduz para chamadas de `NativeAudioEngine`. Isso garante que trocar de engine (ex.: migrar para `media_kit` no futuro) não toca em UI nem em domain.

### 2.2 Buffering e prefetch de próximas faixas

- **Prefetch antecipado**: ao iniciar reprodução da faixa N, o `PlaybackQueueController` imediatamente chama `NativeAudioEngine.setNextSource` para a faixa N+1 (via `ConcatenatingAudioSource` do `just_audio`), independente de rede — o engine nativo já baixa/bufferiza o próximo item em paralelo.
- **Prefetch preditivo estendido**: um `QueuePrefetchNotifier` mantém sempre as próximas **3 faixas** com pelo menos os primeiros ~15s decodificáveis em cache (usando `just_audio`'s cache de HTTP range requests + um `LruAudioCache` próprio em `player_data` para o manifesto de metadados/artwork, não só o áudio).
- **Qualidade adaptativa**: seleção de bitrate por faixa é resolvida no momento do prefetch com base em: tipo de conexão (`connectivity_plus`), preferência do usuário (Wi-Fi only para downloads, "Data Saver" para streaming), e sinal de qualidade vindo do backend (assumindo CDN com múltiplos bitrates — dependência declarada na seção 7).
- **Buffer alvo**: 30s de áudio decodificado à frente do playhead em condições normais (config `bufferForPlaybackDuration`/`bufferForPlaybackAfterRebufferDuration` do ExoPlayer no Android; equivalentes configurados via `just_audio` nas outras plataformas), ajustável dinamicamente para redes lentas (fallback até 8s para começar a tocar mais rápido, com refill agressivo em paralelo).

### 2.3 Gapless e crossfade

- **Gapless por padrão** entre faixas do mesmo álbum/contexto de fila, via `ConcatenatingAudioSource` do `just_audio`, que faz a transição no nível do engine nativo (sem re-render Flutter, sem gap perceptível — mesma técnica usada por ExoPlayer nativamente).
- **Crossfade opcional** (configurável pelo usuário, 0–12s, default off para paridade com o comportamento padrão do Spotify): implementado com dois `AudioPlayer` (`just_audio`) sobrepostos e fade de volume via `AnimationController` de domínio (não widget) nos últimos N segundos da faixa atual enquanto a próxima já está tocando em paralelo. Encapsulado inteiramente em `data/player_data`; `PlaybackQueueController` expõe só `setCrossfadeDuration(Duration)`.
- Testado com golden/integration test que mede timestamps de start/stop dos dois streams simulados (fake engine, ver seção 5) para garantir que não há silêncio nem sobreposição de áudio bruto fora da janela configurada.

### 2.4 Controle em background e lockscreen

- **Mobile**: `audio_service` provê o `AudioHandler` que roda como foreground service (Android) / background audio session (iOS), integrado com `MediaSession`/`MPRemoteCommandCenter` para controles no lockscreen, notificação persistente, e Bluetooth/CarPlay/Android Auto.
- **Web**: MediaSession API do browser (via `audio_service_web` ou binding direto de `navigator.mediaSession`) para os controles de teclado multimídia e a UI que o navegador expõe (Chrome/Edge mostram controles na barra de mídia do SO).
- Ambos os `AudioHandler`s (mobile e web) implementam a mesma interface `PlaybackQueueController` — o `AudioHandler` **é** uma das implementações concretas registradas em `app/*`, não uma camada paralela. Isso significa que pressionar "next" no lockscreen e pressionar "next" na UI do app disparam exatamente o mesmo caminho de código em `player_domain`.
- Estado do player sobrevive a kill do processo em background via persistência de "current queue + position" (ver 2.5) — ao processo ser recriado, o `AudioHandler` restaura a fila do último estado salvo antes de aceitar novos comandos.

### 2.5 Downloads offline (mobile only, ver 1.3)

- `OfflineStorage.saveTrack` baixa o arquivo de áudio completo (bitrate escolhido) mais metadados/artwork para armazenamento local (`path_provider` + SQLite/Isar para o índice de faixas baixadas — decisão de storage local detalhada no doc de arquitetura de dados, dependência declarada na seção 7).
- Fila de download é gerenciada por um `DownloadQueueNotifier` em `player_domain`/`player_data`, com retomada após perda de conectividade (download parcial persistido, retomado via HTTP range request).
- Reprodução prioriza arquivo local: `AudioSource` resolvido por um `TrackSourceResolver` que checa `OfflineStorage.getLocalFile` antes de cair para streaming de rede — transparente para `PlaybackQueueController`.
- DRM/licenciamento de conteúdo baixado é uma dependência explícita do time de dados/backend/legal (seção 7) — este documento assume que o backend entrega uma URL de download já autorizada e não define esquema de proteção de conteúdo.

### 2.6 Estado do player compartilhado entre engines nativas diferentes

O ponto central da seção 2 é este: **`player_domain` nunca vê `just_audio`, `ExoPlayer`, `AVPlayer` ou Web Audio API.** Ele vê apenas `PlayerState`, `QueueItem`, `PlaybackQueueController`. A tradução "estado nativo do engine → `PlayerState` de domínio" é responsabilidade exclusiva de uma classe (`JustAudioPlaybackAdapter` em `data/player_data`) que existe uma única vez no código-fonte e roda nas duas plataformas (porque `just_audio` já abstrai o engine nativo por trás dela). Isso é o que torna verdadeiro o requisito de 100% de reuso especificamente para player: não há "adapter mobile" e "adapter web" — há um adapter, que delega para um pacote (`just_audio`) que por sua vez tem plugins nativos por baixo, invisíveis a três das quatro camadas do app.

---

## 3. Arquitetura de UI de biblioteca e navegação

Meta: performance percebida equivalente a Spotify/YTM ao rolar bibliotecas com milhares de itens (playlists, "Sua Biblioteca", resultados de busca).

### 3.1 Virtualização de listas grandes

- Toda lista de biblioteca usa `ListView.builder`/`SliverList.builder` (nunca `ListView(children: [...])`) — virtualização nativa do Flutter, itens fora da viewport não são construídos.
- Para listas muito longas com necessidade de "jump to letter"/scrollbar arrastável (padrão em bibliotecas de música), usamos `scrollable_positioned_list` ou `CustomScrollView` com `SliverList` custom + um índice de fast-scroll construído a partir dos dados já carregados (evita rebuild de todos os itens ao arrastar o scrollbar).
- Paginação: dados vêm do backend paginados (cursor-based — dependência do time de dados/API, seção 7); `library_data` implementa um `PagingController`-like (Riverpod `AsyncNotifier` com `loadMore()`) que busca a próxima página ~2 telas antes do fim da lista atual (prefetch de dados, análogo ao prefetch de áudio).
- Itens de lista são `const` sempre que possível e usam `RepaintBoundary` ao redor de artwork (isolamos repaint do texto/controles do repaint da imagem, que muda com mais frequência por causa do cache assíncrono).

### 3.2 Cache de imagens/artwork

- `cached_network_image` (com `CacheManager` customizado) para cache em disco + memória de artworks, com **redimensionamento no servidor via URL de thumbnail** (assumindo backend expõe variantes de tamanho — dependência seção 7) para nunca decodificar uma imagem maior que o `cacheWidth`/`cacheHeight` necessário ao slot da UI (evita jank de decode de imagem grande em lista).
- `precacheImage` disparado proativamente para artworks das próximas ~10 posições visíveis durante scroll rápido (throttle por `Timer` para não disparar uma requisição por frame).
- Placeholder: skeleton (ver 3.4), nunca spinner central — reduz percepção de "travamento".

### 3.3 Busca com debounce

- `SearchNotifier` (`@riverpod`) com debounce de 300ms sobre o texto digitado (`Stream.debounceTime` via `rxdart` ou implementação manual com `Timer`), cancelando requisição em voo anterior (`CancelToken` do `dio`) a cada novo termo.
- Busca local instantânea (sem debounce) contra um índice em memória do que já está em cache local (biblioteca do usuário, downloads, histórico recente) é mostrada **imediatamente** enquanto a busca remota (catálogo completo) ainda está em voo — resultado local aparece em <50ms, resultado remoto substitui/complementa quando chega. Isso é o mesmo padrão usado por Spotify (resultados instantâneos de "sua biblioteca" antes dos resultados de catálogo).

### 3.4 Skeleton / loading states

- Todo `AsyncNotifier` de listagem expõe estados `loading`/`data`/`error` (via `AsyncValue` nativo do Riverpod); `presentation/*` mapeia `loading` para skeletons com o **mesmo layout/dimensões** do conteúdo final (shimmer via pacote leve tipo `shimmer` ou implementação própria com `AnimatedContainer` + gradiente), nunca `CircularProgressIndicator` full-screen para conteúdo que já tem forma conhecida (lista de faixas, grid de álbuns).
- Skeletons são widgets em `core_design_system`, reusados por todas as features — garante consistência visual e reduz custo de manter golden tests (uma skeleton, N telas).

### 3.5 Navegação

`go_router` (declarativo, suporta deep link e URL real no Web — requisito direto do reuso 100%: navegação por push/pop imperativo do `Navigator` clássico não mapeia bem para URLs de browser, `go_router` resolve isso com uma única árvore de rotas para as duas plataformas). Shell de navegação (bottom nav mobile / rail ou sidebar web) é o **mesmo `ShellRoute`**, renderizando `NavigationBar` ou `NavigationRail` conforme breakpoint (ver 1.3), não conforme plataforma.

---

## 4. Integração em tempo real: descoberta social por proximidade

Este é o diferencial competitivo do produto — tratado com mais rigor de resiliência que features "nice to have".

### 4.1 Consumo do stream (WebSocket)

- Abstração de domínio: `ProximityFeedRepository` (interface em `social_proximity_domain`) expõe `Stream<List<NearbyListener>>` — a feature nunca toca em `WebSocketChannel` diretamente.
- Implementação (`social_proximity_data`) usa `web_socket_channel` (funciona em mobile e web sobre a mesma API) conectado a um endpoint do backend que emite eventos `nearby_listener_update`/`nearby_listener_left` (protocolo exato — dependência do backend/dados, seção 7; assumimos aqui um protocolo de eventos incrementais, não snapshot completo a cada tick, para escalar com muitos usuários próximos).
- Localização do próprio usuário é enviada ao servidor com throttle (ex.: a cada 15–30s de movimento significativo, `distanceFilter` do `geolocator`) — não a cada atualização de GPS bruta, para poupar bateria e reduzir tráfego. O trade-off latência-vs-bateria é uma decisão de produto explícita a validar com o time de backend/dados (protocolo pode preferir push do cliente vs. pull periódico).
- Estado combinado num `AsyncNotifier<NearbyFeedState>` que funde: lista de ouvintes próximos + status de conexão do socket + status de permissão de localização — a UI reage a um único estado, não a três fontes cruas.

### 4.2 UI: lista e mapa

- **Modo lista** (default, menor custo de implementação e de performance — sem SDK de mapa): cards por usuário próximo com avatar, nome, faixa/artista atual, e distância aproximada (bucket: "muito perto", "~500m", "~1km" — nunca metragem exata por privacidade, decisão a confirmar com segurança/privacidade, seção 7). Lista virtualizada como qualquer outra (seção 3.1), ordenada por proximidade.
- **Modo mapa** (opcional/fase 2 dentro do próprio front, não bloqueante do MVP): usa `google_maps_flutter`/`mapbox_gl` — **estes SDKs não têm suporte Web equivalente ao nativo hoje**, então o modo mapa é o único ponto de UI onde uma diferença real de capability entre plataformas é esperada; tratado com a mesma técnica de capability-flag da seção 1.3 (`MapCapability.isSupported`), com fallback automático para modo lista no Web se necessário, decisão a revalidar quando a implementação começar (SDKs de mapa web evoluem rápido; ver `maplibre_gl` como alternativa cross-platform mais madura em Web).
- Atualizações incrementais da lista (chegada/saída de um ouvinte próximo) são animadas com `AnimatedList`/`implicit animations`, não rebuild completo — reforça a sensação de "ao vivo".

### 4.3 Reconexão

- `ProximityFeedRepository` implementa reconexão com backoff exponencial (ex.: 1s, 2s, 4s, 8s, cap em 30s) e jitter, via um `ReconnectingWebSocketClient` genérico em `core_networking` (reusável para qualquer outro stream futuro, não só proximidade).
- Estado de conexão exposto explicitamente à UI (`ProximityConnectionState`: `connected | reconnecting | offline`), renderizado como um indicador discreto (não bloqueante) — nunca deixamos a lista "congelada" parecendo atualizada quando na verdade está stale; um banner sutil indica "reconectando..." após ~5s desconectado.
- App lifecycle: socket é pausado/fechado ao app ir para background (mobile) prolongado, reaberto ao voltar ao foreground (`AppLifecycleListener` do Flutter, que funciona uniformemente em mobile e Web — Web mapeia para visibilitychange).

### 4.4 Permissão de localização

- `LocationPermissionState`: `notRequested | granted | deniedOnce | deniedForever | restricted` — máquina de estados de domínio, não booleano.
- Fluxo: feature de proximidade é **opt-in explícito** com tela de explicação de valor antes do prompt de permissão do SO (padrão recomendado por Apple/Google, melhora taxa de aceite). Se negado, a UI mostra estado vazio explicando o motivo, com CTA para abrir configurações do SO (`geolocator.openAppSettings()`), nunca insiste com prompt repetido.
- Web: permissão de geolocalização do browser é per-origin e pode ser revogada silenciosamente pelo usuário; `LocationProvider` web trata erro de permissão retornando o mesmo enum de estados, então `social_proximity_ui` não sabe se está rodando em mobile ou Web.
- Retenção/precisão dos dados de localização enviados ao servidor (arredondamento, TTL, se o backend persiste histórico) é uma decisão que depende do time de segurança/privacidade — declarada como pergunta aberta (seção 7); o frontend está desenhado para nunca enviar coordenada bruta de alta precisão sem essa decisão (o `LocationProvider` já expõe um parâmetro de `LocationAccuracy` que pode ser reduzido a nível de bairro/quarteirão se exigido).

---

## 5. Estratégia de testes

Meta declarada pelo usuário: eventualmente 100% de cobertura de linha no frontend. Tratamento honesto abaixo.

### 5.1 O que "100% de cobertura de linha" realmente implica

- É uma meta atingível **em código escrito à mão com lógica de negócio** (domain, data, notifiers de apresentação) porque essas camadas são unit-testáveis sem widget e sem plataforma real.
- É **cara ou artificial** em: código gerado (`*.g.dart`, `*.freezed.dart` de `riverpod_generator`/`freezed`) — deve ser **excluído** da métrica de cobertura via configuração do `coverage`/`lcov` (`--exclude-coverage='**/*.g.dart,**/*.freezed.dart'`), pois testá-lo é testar o gerador, não o produto; e em telas puramente declarativas/triviais (uma tela "Sobre" estática) — cobrimos com um golden test único, que naturalmente cobre 100% das linhas por ser tão simples, sem esforço desproporcional.
- Alguns branches são difíceis de cobrir de forma honesta sem mocks elaborados: erros de hardware (GPS indisponível, engine de áudio nativo retornando erro de codec) e race conditions de rede real. Resolvemos isso com **fakes determinísticos de infraestrutura** (abaixo), não pulando a linha — mas é importante que o Auditor saiba que "100%" aqui significa 100% de linhas exercitadas por testes determinísticos, não 100% de cenários reais de produção cobertos.
- 100% de cobertura **não é sinônimo de ausência de bugs** (não cobre lógica que não existe, nem interação real entre componentes) — tratamos como piso de qualidade e rede de segurança contra regressão, não como prova de corretude. Isso deve ser comunicado como expectativa realista.

### 5.2 Camadas de teste

**Unit tests (domain + data)** — maior volume, mais barato, maior contribuição para cobertura de linha:
- 100% das use cases/`Notifier`s de `*_domain` testados com `ProviderContainer` isolado, sem Flutter widget.
- `data/*` testado contra fakes de `core_networking` (nenhum teste de unidade bate em rede real).
- Player: um `FakeNativeAudioEngine` (implementa `NativeAudioEngine`) controlado por teste, permite simular buffering, erro de rede, fim de faixa, sem tocar áudio de verdade — condição para testar gapless/crossfade/prefetch deterministicamente (seção 2.3).
- Proximidade: `FakeReconnectingWebSocketClient` que simula desconexão/reconexão/latência sob controle do teste (`StreamController` manual).

**Widget tests** — cada widget de `presentation/*` testado isoladamente com `ProviderScope(overrides: [...])` injetando estado fake; cobre estados de loading/error/empty/data de cada tela (os skeletons da seção 3.4 tornam esse teste barato porque o shape é compartilhado).

**Golden tests** (`golden_toolkit`/`alchemist`) — para `core_design_system` (todo componente visual compartilhado) e para as telas-chave (player expandido, biblioteca, feed de proximidade), rodando em ao menos 2 tamanhos de tela (mobile e wide/web) e nos dois temas (claro/escuro) — pega regressão visual, não é fonte primária de cobertura de linha (goldens tendem a cobrir pouca lógica nova por teste), mas é essencial para a meta de "igualar ou superar" visualmente os concorrentes.

**Integration/E2E** (`integration_test` do Flutter, rodando em device real/emulador para mobile e em `flutter drive`/chromedriver para Web — mesmos arquivos de teste, dois runners, reforça reuso): fluxos críticos ponta a ponta — login → tocar uma faixa → verificar áudio audível (via mock de engine controlado, já que CI não tem áudio de verdade) → next/previous → fechar app e checar lockscreen; permitir localização → ver item aparecer na lista de proximidade → simular perda de conexão → checar banner de reconexão. `patrol` é considerado como alternativa para permissões nativas reais (ex.: aceitar diálogo de permissão de localização do SO, que `integration_test` puro não controla bem) — decisão de adotar `patrol` especificamente para os testes que precisam interagir com diálogos nativos do SO, mantendo `integration_test` puro para o resto.

**CI de cobertura**: `flutter test --coverage`, merge de lcov por pacote do monorepo, gate de PR com **cobertura mínima por pacote** (ex.: 90% em `domain`/`data`, 75% em `presentation` inicialmente, subindo por etapas até a meta de 100% combinada excluindo gerado) — meta de 100% tratada como objetivo de longo prazo com trajetória incremental declarada, não como gate imediato de todo PR desde o dia 1 (evitaria travar o time cedo).

---

## 6. Metas de performance concretas (comparadas a Spotify/YTM)

**Todas as métricas abaixo são estimativas baseadas em conhecimento público sobre o comportamento desses apps e em características típicas de apps Flutter de porte equivalente — não são benchmarks medidos, e devem ser validadas com profiling real assim que houver build. Servem como alvo de design, não como SLA já comprovado.**

| Métrica | Spotify (estimado, público) | YTM (estimado, público) | Meta smusic | Racional |
|---|---|---|---|---|
| Cold start até tela interativa (mobile, dispositivo médio) | ~1.5–2.5s | ~2–3s | **≤ 2.5s** | Flutter com `deferred components`/lazy init de features não críticas no primeiro frame (proximidade e download só inicializam após a tela inicial estar interativa); paridade, não vitória clara — apps nativos legados tendem a vencer cold start puro. |
| Tempo até primeiro áudio audível após tap em uma faixa (rede boa) | ~200–400ms | ~300–500ms | **≤ 400ms** (rede boa), **≤ 1.2s** (rede 3G/ruim) | Depende do buffer mínimo antes de iniciar playback (seção 2.2, fallback a 8s de buffer-target vira início mais rápido) e de prefetch da faixa já estar quente quando o usuário toca em algo da lista visível (seção 2.2/3.1 prefetch combinado). |
| FPS de scroll em lista de biblioteca (1000+ itens, dispositivo médio) | ~60fps (nativo) | ~55–60fps | **≥ 58fps médio, sem frames > 32ms (jank) em > 1% dos frames** | Meta explicitamente definida em termos de frame budget do Flutter (`flutter_driver`/`integration_test` com `traceAction` medindo `FrameTiming`), não só "fps subjetivo"; depende de virtualização estrita (3.1) e de não decodificar imagem maior que o slot (3.2). |
| Tamanho do bundle web (initial load, gzip) | Spotify Web Player: histórico de ser um app grande (Electron-like SPA, vários MB) | YTM Web: também SPA pesada | **≤ 3–4 MB gzip no first paint interativo**, com o restante em `deferred components`/code-splitting por rota | Flutter Web historicamente gera bundles maiores que SPAs JS equivalentes; meta é ser competitivo com as versões web desses concorrentes (que também não são leves), não com um SPA minimalista — usar renderer `CanvasKit` apenas se necessário para paridade visual de texto/gráficos, avaliar `skwasm`/HTML renderer para reduzir peso caso a fidelidade visual permita. |
| Tempo até lista de "quem está por perto" popular após permissão concedida | (sem equivalente direto nos concorrentes — feature própria) | — | **≤ 2s** desde permissão concedida até primeiro resultado renderizado (assumindo backend responde no mesmo intervalo) | Definido como meta própria de produto, já que não há benchmark de concorrente para essa feature específica — reforça que a auditoria dessa métrica depende do SLA do backend (seção 7). |

---

## 7. Perguntas em aberto para outros especialistas

**Arquitetura de dados / backend:**
1. Protocolo exato do WebSocket de proximidade: snapshot completo periódico vs. eventos incrementais (`joined`/`left`/`updated`)? Formato do payload (schema de `NearbyListener`)? Frequência máxima de envio de posição do cliente aceita pelo servidor?
2. O backend expõe múltiplos bitrates/variantes de áudio por faixa (para seleção adaptativa, seção 2.2) e variantes de tamanho de artwork (para cache de imagem, seção 3.2)? Se não, o frontend precisa fazer downscale client-side, o que muda a arquitetura de cache.
3. Paginação de biblioteca/catálogo: cursor-based ou offset-based? Existe endpoint de busca com resultados combinados (biblioteca local + catálogo) ou o frontend precisa fazer merge de duas chamadas?
4. Esquema de autorização de download offline (URLs assinadas com TTL? DRM real?) — impacta diretamente o design de `OfflineStorage`/`DownloadQueueNotifier`.
5. Existe (ou haverá) endpoint de "resume playback" cross-device (retomar no celular o que estava tocando no web)? Isso adicionaria um requisito de sincronização de `PlayerState` que hoje não está coberto neste documento.

**Segurança / privacidade:**
6. Qual a política de precisão/retenção da localização enviada para a feature de proximidade (arredondamento, TTL no servidor, direito de exclusão)? O frontend já está desenhado para poder degradar `LocationAccuracy`, mas a decisão de qual precisão usar por padrão é de vocês.
7. A distância mostrada ao usuário deve ser sempre "bucketizada" (ex.: faixas de distância) ou existe algum contexto em que distância exata é aceitável? Isso muda o design da UI de lista/mapa (seção 4.2).
8. Modelo de bloqueio/opt-out: um usuário pode aparecer para "amigos" mas não para "estranhos"? Isso adiciona estado de domínio (visibilidade por audiência) não coberto aqui.
9. Requisitos de compliance (LGPD/GDPR) sobre dado de geolocalização armazenado localmente no dispositivo (cache do `ProximityFeedRepository`) — por quanto tempo pode persistir em disco vs. só em memória?

**Produto / design:**
10. Prioridade real do "modo mapa" (seção 4.2) — se for prioridade alta desde o MVP, vale investir agora em avaliar `maplibre_gl` (melhor suporte Web) em vez de `google_maps_flutter`, decisão que fica mais cara de reverter depois.
11. Confirmação da política de crossfade default (off, como Spotify) e da UX de permissão de localização opt-in (tela de valor antes do prompt do SO) alinhada com o time de produto/growth.

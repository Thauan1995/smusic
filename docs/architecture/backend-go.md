# smusic — Arquitetura de Backend (Go)

Status: FASE DE PLANEJAMENTO — este documento é a especificação de arquitetura do backend. Não contém código de produção; trechos de Go são ilustrativos (assinaturas de interface, padrões de concorrência) para comunicar decisões de design.

Diferencial de produto: descoberta social em tempo real (quem perto de mim está ouvindo o quê, agora). Barra de qualidade: reprodução, biblioteca e performance devem igualar ou superar Spotify e YouTube Music, com metas numéricas (seção 6).

---

## 1. Topologia de serviços: monolito modular, com dois serviços extraídos desde o dia 1

### Decisão

**Monolito modular** para o núcleo do produto (auth, catálogo, biblioteca, playlists, reprodução/estado de fila), decomposto internamente em módulos Go com fronteiras de pacote rígidas (`internal/<domínio>`), comunicação apenas via interfaces explícitas — nunca acesso direto a tabelas de outro domínio.

Dois serviços são **extraídos como processos separados desde o início**, não por dogma de microsserviço, mas porque têm perfis de carga, escalonamento e falha fundamentalmente diferentes do resto:

1. **`presence-service`** — feed de presença/proximidade em tempo real. Milhares de conexões long-lived, fanout O(N²) potencial por região geográfica, precisa escalar horizontalmente de forma independente do resto (picos de presença não correlacionam com picos de catálogo/biblioteca) e precisa de deploys/rollbacks frequentes sem arriscar o caminho crítico de reprodução.
2. **`media-edge-service`** — o caminho de entrega de áudio (range requests, assinatura de URLs de CDN, transcodificação sob demanda/pré-processada). Isolado porque é o único componente com requisitos de latência de cauda extremamente agressivos (seção 6) e porque times de dados vão querer instrumentá-lo pesadamente sem tocar no monolito de domínio.

Tudo o mais (auth, catálogo, biblioteca, social graph estático — seguir/seguidores, playlists compartilhadas) fica no monólito modular, atrás de um único binário deployável (com possibilidade de rodar múltiplas réplicas stateless atrás de um load balancer).

### Justificativa

- **Time pequeno/médio em fase inicial.** Microsserviços completos (N serviços, N bancos, service mesh, contratos versionados entre times) impõem custo de coordenação que não se paga antes de product-market fit. Um monolito modular dá 90% do benefício de isolamento (testabilidade, fronteiras de domínio claras, deploy de features independente via feature flags) com 10% do custo operacional.
- **Módulos com fronteiras de pacote = extração futura barata.** Se `catalog` ou `library` precisar virar serviço próprio quando o time de dados quiser um pipeline de recomendação separado, a fronteira de interface já existe — o corte vira "mover pacote + trocar chamada de função por chamada de rede", não um redesenho.
- **Exceção deliberada para presença e mídia**, porque adiar a extração desses dois custaria mais do que adiantá-la: presença tem um padrão de concorrência (milhões de writes pequenos e frequentes, fanout geoespacial) que não deve competir por recursos (goroutines, conexões DB, memória) com requests de catálogo/auth; e o edge de mídia precisa escalar por CDN/PoP geográfico de forma desacoplada do resto.
- **Considerando que times de dados e segurança também vão atuar no sistema:** um monolito modular com fronteiras de domínio explícitas é mais fácil de auditar (segurança) e de instrumentar para extração de eventos analíticos (dados) do que um "big ball of mud". Cada módulo de domínio expõe:
  - um **event log de domínio** (outbox pattern — seção 3/5) que o time de dados consome sem tocar no código de request path;
  - um **boundary de autorização único** (middleware central de authz, não checagens espalhadas) que o time de segurança pode revisar e testar isoladamente.
- **O que evitamos:** microsserviços "por domínio" (auth-service, catalog-service, library-service, playlist-service...) desde o início. Isso multiplicaria por 5-8x o número de deploys, bancos, e superfícies de rede internas, sem ganho de escalabilidade real — catálogo e biblioteca compartilham o mesmo perfil de leitura pesada / escrita rara, e forçar uma fronteira de rede entre eles hoje só adicionaria latência (chamadas RPC internas) e pontos de falha distribuída (timeouts, retries, circuit breakers) para um problema que não existe ainda.

```
┌─────────────────────────────────────────────────────────┐
│                    smusic-core (monólito modular)         │
│  ┌────────┐ ┌─────────┐ ┌──────────┐ ┌─────────────────┐ │
│  │  auth  │ │ catalog │ │ library  │ │ playback-state   │ │
│  │        │ │         │ │(playlists│ │ (fila, posição,  │ │
│  │        │ │         │ │ /histór.)│ │  seek)           │ │
│  └────────┘ └─────────┘ └──────────┘ └─────────────────┘ │
│  cada módulo: internal/<domínio>/{api,service,repo}       │
│  authz middleware central · outbox de eventos de domínio  │
└─────────────────────────────────────────────────────────┘
        │ gRPC interno            │ gRPC interno
        ▼                         ▼
┌──────────────────────┐  ┌──────────────────────────┐
│  presence-service      │  │  media-edge-service        │
│  WS/SSE fanout          │  │  range requests, URLs      │
│  geo-index em memória   │  │  assinadas, ABR manifest   │
│  escala por réplica      │  │  escala por PoP/CDN        │
└──────────────────────┘  └──────────────────────────┘
```

Regra de re-avaliação: se algum módulo do monólito ultrapassar consistentemente >40% do tempo de CPU do processo agregado, ou se seu time crescer a ponto de precisar de deploys independentes diários, ele vira candidato a extração — não antes.

---

## 2. Stack técnica

### HTTP / gRPC

- **gRPC** para toda comunicação **interna** entre `smusic-core`, `presence-service` e `media-edge-service`. Motivo: contratos fortemente tipados via protobuf (facilita auditoria de segurança de payloads), streaming bidirecional nativo (útil para presença), e overhead menor que REST+JSON em chamadas internas de alta frequência.
- **HTTP/JSON (REST)** para a API pública voltada a clientes (mobile/web/desktop), via **gRPC-Gateway** gerado a partir das mesmas definições `.proto` do core — um único contrato fonte-da-verdade, evitando duplicação e drift entre o schema interno e o externo.
- Router HTTP: **`chi`** (`go-chi/chi`) para as rotas REST/gateway e endpoints que não passam pelo gRPC-Gateway (ex.: endpoints de streaming de mídia com controle fino de headers de range request, upload de assets). Motivo: idiomático (compõe com `net/http` padrão, sem framework "mágico"), middleware chain explícita, e não introduz um runtime HTTP próprio — importante para manter observabilidade e testabilidade simples.
- **WebSocket**: biblioteca `nhooyr.io/websocket` (ou `gorilla/websocket` como alternativa madura) para o feed de presença exposto a clientes — ver justificativa de protocolo abaixo.

### Entrega de áudio

- **HTTP Range Requests obrigatórios** (`Accept-Ranges: bytes`) em todo endpoint de streaming de mídia — pré-requisito para seek instantâneo e para os players nativos de iOS/Android/web (`<audio>`, `AVPlayer`, `ExoPlayer`) que dependem de range requests para buffering incremental.
- **Bitrate adaptativo**: HLS (HTTP Live Streaming) com manifests `.m3u8` gerando variantes pré-transcodificadas em pelo menos 3 níveis (ex.: ~64kbps AAC para condição de rede ruim, ~128kbps padrão, ~256/320kbps para Wi-Fi/qualidade alta — espelhando as faixas usadas por Spotify/YT Music). Transcodificação é feita **offline/assíncrona no ingest** (pipeline de processamento, fora do `media-edge-service`), não sob demanda — sob demanda introduziria latência de cold-start incompatível com a meta de "tempo até o primeiro áudio" (seção 6).
- **CDN é o caminho primário de entrega**, não fallback. `media-edge-service` nunca serve bytes de áudio diretamente ao cliente final em produção — ele:
  1. autentica/autoriza o pedido de reprodução;
  2. gera URLs assinadas e de curta duração (ex.: 5-10 min) apontando para os objetos na CDN (CloudFront/Cloudflare/Fastly — decisão final depende de contrato de infra, não bloqueia este documento);
  3. registra o evento de "play iniciado" para dados/billing.
  Isso mantém o processo Go fora do caminho de I/O de bytes pesados (deixa o Go fazer o que faz bem: lógica de negócio e concorrência de controle, não proxy de payload binário), e dá aos times de dados/segurança um ponto único para instrumentar acesso e revogação de acesso a mídia.
- Origem dos arquivos: armazenamento de objetos (S3-compatível) com replicação multi-região atrás da CDN.

### Protocolo do feed de presença em tempo real

Comparativo:

| Critério | WebSocket | SSE | gRPC streaming |
|---|---|---|---|
| Bidirecional (cliente envia localização + recebe feed) | Sim, nativo | Não (precisa de POST separado para upload) | Sim, nativo |
| Suporte nativo em browser sem lib extra | Sim | Sim | Não (requer grpc-web + proxy) |
| Reconexão automática | Manual | Nativa (`EventSource`) | Manual |
| Overhead por mensagem | Baixo (frame binário) | Médio (texto, HTTP/1.1 chunked) | Baixo (binário, HTTP/2 multiplexado) |
| Funciona bem atrás de proxies/firewalls corporativos e redes móveis com NAT agressivo | Bom, com fallback | Ótimo | Médio (depende de suporte HTTP/2 completo) |
| Maturidade para mobile (iOS/Android) | Ótima | Fraca (sem `EventSource` nativo robusto) | Boa (grpc-swift/grpc-java), porém maior custo de integração |

**Decisão: WebSocket para o feed de presença cliente-facing.**

Justificativa:
- O feed de presença é **inerentemente bidirecional**: o cliente precisa enviar (localização aproximada, estado de reprodução atual) e receber (quem está por perto, ouvindo o quê) na mesma sessão, com baixa latência nos dois sentidos. SSE resolveria só metade (servidor→cliente), forçando um segundo canal HTTP para o upload de estado, o que dobra a complexidade de coordenação sem ganho real.
- gRPC streaming seria a escolha técnica "mais pura" para o *interno* (`presence-service` ↔ `smusic-core`, onde já usamos gRPC), mas para o *cliente móvel/web* o custo de integração (grpc-web + proxy Envoy para browsers, ou libs gRPC nativas em cada plataforma mobile) não se paga frente ao ganho marginal sobre WebSocket, que já é suportado nativamente em toda plataforma alvo.
- Mitigamos a fraqueza de reconexão manual do WebSocket com um protocolo de aplicação leve por cima: heartbeat/ping-pong a cada 15s, reconexão exponencial no client SDK, e **resync de estado completo** (não apenas deltas) na reconexão, para tolerar perda de mensagens durante quedas de rede — comum em cenários de descoberta social "andando na rua".
- **Internamente**, `presence-service` fala gRPC streaming com `smusic-core` (para autorização, resolução de identidade social — grafo de quem segue quem — e persistência assíncrona de eventos de presença), mantendo o protocolo cliente-facing (WS) desacoplado do protocolo interno.

---

## 3. Modelo de concorrência — presença em tempo real

### Requisitos de carga

Milhares de clientes conectados simultaneamente, cada um enviando atualizações de localização aproximada + estado de reprodução em intervalo curto (ex.: a cada 10-20s, ou em eventos discretos: play/pause/skip). O `presence-service` precisa:

1. Ingerir esses updates sem bloquear.
2. Recalcular e distribuir, para cada usuário conectado, a lista de "pessoas próximas ouvindo algo agora" — um fanout que é potencialmente O(N) por update, não O(N²) ingênuo, graças a indexação geoespacial (ver abaixo).
3. Fazer isso com backpressure explícito — nunca deixar uma goroutine de fanout lento derrubar o ingest, nem deixar o ingest sem limite estourar memória.

### Arquitetura de concorrência

**Padrão: pipeline de estágios com channels, um goroutine-pool por estágio, geo-index particionado.**

```go
// Esboço ilustrativo — não é código de produção.

// Estágio 1: ingest. Uma goroutine por conexão WS ativa (padrão idiomático Go:
// 1 goroutine de leitura + 1 de escrita por conexão), mas todo trabalho de
// negócio é despachado para os estágios seguintes via channel — a goroutine
// da conexão nunca faz cálculo geoespacial nem bloqueia em I/O de terceiros.
type PresenceUpdate struct {
    UserID    string
    Lat, Lon  float64
    Track     TrackState // o que está tocando agora, ou nil
    Timestamp time.Time
}

type IngestPipeline struct {
    updates chan PresenceUpdate // buffered, com capacidade dimensionada por carga
}

func (p *IngestPipeline) Enqueue(u PresenceUpdate) error {
    select {
    case p.updates <- u:
        return nil
    default:
        // Backpressure explícito: canal cheio => rejeita e sinaliza ao
        // cliente para reduzir a frequência de envio, em vez de bloquear
        // a goroutine da conexão (o que acumularia latência em cascata)
        // ou de crescer o buffer sem limite (o que estouraria memória).
        return ErrIngestSaturated
    }
}

// Estágio 2: worker pool fixo (dimensionado por GOMAXPROCS/carga observada,
// não "uma goroutine por update") consome o channel, atualiza o geo-index
// particionado por célula (ex.: geohash) e calcula o delta de vizinhança.
func (p *IngestPipeline) runWorkers(ctx context.Context, n int, idx *GeoIndex, out chan<- FanoutJob) {
    for i := 0; i < n; i++ {
        go func() {
            for {
                select {
                case u := <-p.updates:
                    delta := idx.ApplyAndDiff(u) // O(1) amortizado por célula
                    select {
                    case out <- delta:
                    case <-ctx.Done():
                        return
                    default:
                        metrics.FanoutDropped.Inc() // backpressure observável
                    }
                case <-ctx.Done():
                    return
                }
            }
        }()
    }
}
```

Decisões-chave:

- **Geo-index particionado (geohash/S2 cells) mantido em memória por réplica**, com atualização O(1) amortizada por update e diffing incremental — evita recomputar toda a vizinhança de todo mundo a cada tick. Cada célula geográfica é uma unidade de sharding natural: réplicas do `presence-service` podem ser particionadas por região (consistent hashing sobre célula geográfica) para escalar horizontalmente sem coordenação global.
- **Worker pool de tamanho fixo por estágio**, não goroutine-por-update — evita o anti-padrão de explosão de goroutines sob pico de carga, que é a forma mais comum de um serviço Go "idiomaticamente ingênuo" cair sob carga real.
- **Backpressure em três camadas**:
  1. Canal de ingest com capacidade limitada + rejeição explícita (não bloqueio) quando saturado — o cliente recebe um erro/sinal para reduzir frequência de envio (client SDK implementa backoff).
  2. Canal de fanout com métrica de drop explícita — perder um delta de presença ocasionalmente é aceitável (é um "now-state", não um evento crítico que precisa garantia de entrega); a próxima atualização periódica corrige o estado.
  3. Rate limiting por usuário na borda (seção 5) — limita updates/segundo por client_id antes mesmo de chegar ao pipeline.
- **Sem locks globais.** Cada célula geográfica do geo-index tem seu próprio mutex (ou é gerenciada por uma única goroutine "dona" da célula, recebendo comandos via channel — padrão "goroutine como dono de estado" idiomático em Go) — contenção fica localizada, nunca global.
- **Fanout para conexões WS**: cada conexão tem sua própria goroutine de escrita com buffer pequeno; se o buffer de saída de um cliente específico enche (cliente lento/rede ruim), aquele cliente especificamente perde updates de presença (nunca afeta outros clientes) e recebe um resync completo no próximo heartbeat.

### Graceful shutdown

```go
func (s *PresenceService) Shutdown(ctx context.Context) error {
    s.stopAcceptingNewConnections() // fecha listener, novas conexões => 503
    s.broadcastDrainNotice()        // avisa clientes conectados: reconectar em outra réplica
    close(s.ingest.updates)         // sinaliza fim de novos updates aos workers
    // aguarda workers em voo terminarem, respeitando o timeout do ctx
    // (deadline de shutdown do orquestrador, ex. 30s no k8s)
    done := make(chan struct{})
    go func() { s.wg.Wait(); close(done) }()
    select {
    case <-done:
        return nil
    case <-ctx.Done():
        return fmt.Errorf("shutdown incompleto no prazo: %w", ctx.Err())
    }
}
```

- Usa `context.Context` propagado desde `main()` com `signal.NotifyContext(os.Interrupt, syscall.SIGTERM)` — padrão idiomático Go para shutdown coordenado.
- Conexões WS recebem um frame de "drain" antes do fechamento, permitindo que o client SDK reconecte proativamente a outra réplica (via load balancer) sem que o usuário perceba uma falha — crítico para não degradar a experiência de descoberta social durante deploys.
- Nenhum estado de presença é persistido de forma síncrona e crítica: é um cache "best-effort" reconstruível a partir do próximo heartbeat de cada cliente, o que simplifica drasticamente o shutdown (não há transação de banco a fechar no caminho quente).

---

## 4. Contratos de API de alto nível

Definidos como `.proto` (fonte da verdade), expostos via gRPC internamente e via gRPC-Gateway/REST externamente. Abaixo, formato simplificado (não é o `.proto` completo).

### Autenticação

```
POST   /v1/auth/signup            { email, password | oauth_token }         -> { user_id, access_token, refresh_token }
POST   /v1/auth/login              { email, password | oauth_token }         -> { access_token, refresh_token }
POST   /v1/auth/refresh            { refresh_token }                          -> { access_token }
POST   /v1/auth/logout             { refresh_token }                          -> 204
GET    /v1/auth/me                 (bearer token)                             -> { user_id, display_name, ... }
```
- Tokens: JWT de acesso de vida curta (~15 min) + refresh token opaco de vida longa, rotacionado a cada uso (mitigação de replay — detalhe de política final pertence ao especialista em segurança).

### Catálogo / Biblioteca

```
GET    /v1/catalog/search?q=&type=track|album|artist|playlist&limit=&cursor=  -> { results[], next_cursor }
GET    /v1/catalog/tracks/{id}                                                  -> { id, title, artist, album, duration_ms, available_bitrates[] }
GET    /v1/catalog/albums/{id}                                                  -> { id, title, tracks[] }

GET    /v1/library/me/playlists                                                -> { playlists[] }
POST   /v1/library/me/playlists                { name, is_public }             -> { playlist_id }
POST   /v1/library/me/playlists/{id}/tracks     { track_id, position? }        -> 204
DELETE /v1/library/me/playlists/{id}/tracks/{track_id}                         -> 204
POST   /v1/library/me/saved-tracks              { track_id }                   -> 204
GET    /v1/library/me/history?limit=&cursor=                                   -> { plays[], next_cursor }
```
- Busca com cursor-based pagination (não offset — offset degrada em catálogos grandes e é inconsistente sob escrita concorrente).

### Reprodução (play / seek / fila)

```
POST   /v1/playback/sessions                { device_id, context? }           -> { session_id, playback_url_manifest }
POST   /v1/playback/sessions/{id}/play      { track_id, position_ms? }        -> { stream_url (assinada), expires_at }
POST   /v1/playback/sessions/{id}/pause                                        -> 204
POST   /v1/playback/sessions/{id}/seek      { position_ms }                    -> 204
POST   /v1/playback/sessions/{id}/next                                         -> { track_id, stream_url }
POST   /v1/playback/sessions/{id}/queue     { track_ids[], position?: "next"|"end" } -> 204
GET    /v1/playback/sessions/{id}/state                                        -> { track_id, position_ms, is_playing, queue[] }
```
- `stream_url` é sempre uma URL assinada de curta duração apontando para a CDN (via `media-edge-service`), nunca um endpoint do monólito.
- Estado de sessão de reprodução é mantido no módulo `playback-state`, sincronizado entre dispositivos do mesmo usuário via o mesmo mecanismo de push usado pela presença (reaproveitando a infraestrutura de WS/fanout, mas em canal lógico separado — "Connect"-like, no estilo Spotify Connect).

### Presença social / proximidade

```
WS     /v1/presence/connect                                                    -> upgrade para WebSocket

Cliente -> Servidor (frames):
  { type: "update", lat, lon, accuracy_m, now_playing?: { track_id, position_ms } }
  { type: "heartbeat" }
  { type: "visibility", mode: "visible" | "invisible" | "friends_only" }

Servidor -> Cliente (frames):
  { type: "nearby_update", users: [ { user_id, display_name, distance_bucket, now_playing } ] }
  { type: "resync_full", users: [...] }
  { type: "drain", reconnect_hint: "..." }
```
- `distance_bucket` (ex.: "<50m", "50-200m", "200m-1km"), nunca coordenadas exatas de terceiros — decisão de privacidade que precisa validação final do especialista em segurança (ver seção de perguntas em aberto).
- `visibility` dá ao usuário controle explícito de opt-in/opt-out granular — requisito não-negociável para um feature de geolocalização social.
- REST complementar para consultas não realtime:
```
GET    /v1/social/me/followers
GET    /v1/social/me/following
POST   /v1/social/follow/{user_id}
```

---

## 5. Cache, rate limiting e observabilidade

### Cache

- **Redis** como cache distribuído compartilhado entre réplicas do monólito, camadas:
  - Catálogo (metadados de faixa/álbum/artista): TTL longo (horas), invalidação por evento de escrita (outbox → invalidação assíncrona), já que catálogo é majoritariamente leitura.
  - Resultados de busca: TTL curto (minutos), chave por query normalizada.
  - Estado de sessão de reprodução (`playback-state`): Redis como store primário de estado efêmero (não Postgres) — é acessado a cada seek/play, precisa de latência de sub-milissegundo, e não precisa de durabilidade forte (perda de estado de reprodução = pior caso, o cliente re-sincroniza).
- **Cache em memória local (in-process, `sync.Map` ou LRU tipo `ristretto`)** para dados extremamente quentes e pequenos (ex.: feature flags, configuração de bitrate por região) — evita round-trip a Redis no caminho mais quente do sistema. Invalidado por pub/sub Redis quando muda.
- **CDN como cache de bytes de mídia** (seção 2) — cache-control agressivo, já que os objetos de áudio processado são imutáveis (versionados por hash de conteúdo, não por ID mutável).

### Rate limiting

- **Rate limiting na borda** (API gateway / middleware `chi`) usando **token bucket por usuário autenticado e por IP** (para endpoints não autenticados como signup/login), implementado sobre Redis (`INCR` + `EXPIRE`, ou algoritmo de sliding window log para os endpoints sensíveis a abuso como login) para ser consistente entre réplicas.
- Limites diferenciados por classe de endpoint:
  - Auth (login/signup): limite agressivo por IP (mitigação de brute-force — política final com segurança).
  - Presença (`update` via WS): limitado no nível de aplicação dentro do `presence-service` (não faz sentido tratar como HTTP request-response), ver seção 3.
  - Playback (play/seek): limite generoso (uso normal não deve nunca esbarrar nele), mas presente para conter clientes com bug/loop.
  - Busca: limite médio, com backoff sugerido no header de resposta (`Retry-After`).
- Resposta padronizada `429` com header `Retry-After` — nunca falha silenciosa.

### Observabilidade

- **Métricas**: Prometheus (`client_golang`), expostas em `/metrics` por serviço. Métricas obrigatórias por serviço: latência de request (histogram, por rota+método+status), taxa de erro, saturação de canais internos (tamanho de buffer de canais críticos do pipeline de presença — sinal direto de backpressure), goroutines ativas, GC pause time. Dashboards em Grafana.
- **Tracing distribuído**: OpenTelemetry SDK para Go, exportando para um backend compatível (Tempo/Jaeger/Honeycomb — decisão de infra final em aberto). Trace obrigatório atravessando: request do cliente → monólito → chamada gRPC interna → media-edge/presence-service, com propagação de `trace_id` também nos frames de WebSocket (campo no envelope da mensagem) para correlacionar eventos de presença de ponta a ponta.
- **Logging estruturado**: `slog` (biblioteca padrão Go, desde 1.21) com formato JSON em produção, texto legível em dev. Todo log carrega `trace_id`, `user_id` (quando aplicável, com PII minimizada), `service`, `module`. Nunca logar payload de senha/token; nunca logar coordenadas exatas de localização em nível INFO (apenas em DEBUG local, nunca em produção) — decisão que cruza com segurança/privacidade.
- **SLOs formais** por serviço (ligados às metas da seção 6), com alerting em erro-budget burn rate (não apenas limiar estático) — evita alert fatigue e formaliza o compromisso de "igualar Spotify/YT Music" em termos operacionais, não só de intenção.

---

## 6. Metas de performance (mensuráveis, comparadas a Spotify/YouTube Music)

**Fonte das estimativas de concorrentes**: não há números oficiais publicados por Spotify/Google para estas métricas; os valores abaixo são estimativas baseadas em benchmarks públicos de UX de apps de streaming de referência amplamente reportados por analistas de performance mobile e por medições informais de comunidade (blogs de engenharia de streaming, discussões técnicas públicas sobre CDN/HLS para áudio) — tratá-los como **piso competitivo estimado**, não como números contratuais exatos dos concorrentes. Cada meta abaixo é declarada com a fonte do racional.

| Métrica | Meta smusic | Referência estimada (Spotify/YT Music) | Fonte do racional |
|---|---|---|---|
| Tempo até o primeiro áudio (cold start, tap em "play" até som audível) | **≤ 300ms p50 / ≤ 800ms p95** em rede boa (Wi-Fi/4G+) | Spotify/YT Music tipicamente respondem em algo entre 200-600ms em condições boas | Padrão de UX de apps de streaming de referência: manifests pré-buscados, primeiro segmento HLS pequeno (~2-4s) já em cache de borda; benchmark comum de "instant play" na indústria de áudio é <1s em rede boa |
| Tempo até o primeiro áudio, rede ruim (3G/latência alta) | **≤ 1.5s p95** | Estimado 1.5-2.5s para apps de referência | Uso de variante de bitrate mais baixa (~64kbps) como primeiro segmento sempre que a detecção de rede indicar condição ruim (fast-start heurístico), técnica documentada publicamente em engenharia de ABR de streaming |
| Latência de seek (arrastar barra → áudio no novo ponto) | **≤ 150ms p50 / ≤ 400ms p95** | Apps de referência tipicamente <200-300ms | Depende de range requests bem indexados na CDN + segmentos HLS curtos (2-4s), evitando re-buffer completo |
| Tempo de resposta de busca na biblioteca/catálogo (server-side, até primeiro byte de resultado) | **≤ 100ms p50 / ≤ 300ms p95** | Apps de referência tipicamente 100-250ms | Busca em índice invertido dedicado (ex. Elasticsearch/Meilisearch) fora do caminho de escrita transacional, com cache de queries frequentes |
| Latência de fanout de presença (update de um usuário → visível para vizinhos conectados) | **≤ 2s p95** | Não há benchmark de mercado direto (feature não existe hoje da mesma forma) — meta definida internamente | Compromisso deliberado: presença é "near-real-time", não "real-time" no sentido de jogos; 2s é imperceptível para o caso de uso de descoberta social ambiente, e dá folga para backpressure gracioso sob pico |
| Throughput do feed de presença por réplica | **≥ 5.000 conexões WS concorrentes e ≥ 2.000 updates/s processados por réplica**, escalando horizontalmente linear via sharding geográfico | N/A (interno) | Dimensionamento a partir do padrão de worker-pool + geo-index em memória (seção 3); validado por teste de carga antes de produção, não é uma garantia teórica |
| Disponibilidade do caminho crítico de reprodução (play/seek/queue) | **≥ 99.9% mensal** (≈43min de indisponibilidade/mês) | Apps de referência operam com SLAs internos tipicamente ≥99.9-99.95% | Padrão de indústria para serviços de consumo em escala; declarado como SLO com error budget, não aspiração |
| Disponibilidade do feed de presença | **≥ 99.5% mensal** | N/A (feature diferenciada, sem benchmark direto) | Tolerância maior aceita deliberadamente: presença é "best-effort" por design (seção 3) — degradar presença nunca deve degradar reprodução |
| Tempo de cold start de novo device/app (login até biblioteca navegável) | **≤ 1.2s p95** (dados em cache local após primeiro login) | Apps de referência tipicamente ≤1-1.5s | Estratégia de cache local no client (fora do escopo deste doc de backend, mas a API precisa suportar sync incremental/ETag para viabilizar isso) |

Essas metas são **compromissos de arquitetura, não garantias automáticas** — cada uma implica decisões já tomadas acima (CDN como caminho primário, segmentos HLS curtos, geo-index em memória, índice de busca dedicado) e precisa ser validada por testes de carga (seção 7) antes de ir a produção, com dashboards de SLO (seção 5) monitorando desvio contínuo.

---

## 7. Estratégia de testes

### Postura sobre a meta de 100% de cobertura de linha

Sendo direto: **100% de cobertura de linha, literal e permanente, não é uma meta saudável e não deve ser tratada como critério de aceite absoluto** — mas a arquitetura abaixo é desenhada para viabilizar o **máximo de cobertura significativa possível** (na prática, esperar 90-97% real de código testável, com o restante documentado e justificado, não escondido).

Riscos reais de perseguir 100% literal, sem meias-palavras:
- **Cobertura de linha não mede qualidade de teste.** É trivial chegar a 100% com testes que executam código sem afirmar (`assert`) nada relevante sobre seu comportamento — isso daria uma falsa sensação de segurança pior do que não ter a métrica.
- **Código de infraestrutura fina (main, wiring de DI, `main.go`, geração de config) tem retorno marginal decrescente** ao ser coberto por teste unitário — muitas vezes é melhor coberto por teste de integração/smoke ou nem coberto, com essas linhas explicitamente excluídas e documentadas (`// coverage:ignore` + motivo), em vez de escrever testes artificiais só para bater o número.
- **Branches de erro defensivo "impossíveis" (ex.: `if err != nil` num `json.Marshal` de struct estático que nunca falha)** são um caso clássico onde forçar cobertura significa injetar falhas artificiais via mocks só para exercitar uma linha que nunca vai rodar em produção — custo de manutenção sem benefício de detecção de bug real.
- **Concorrência é o ponto mais difícil de cobrir de forma significativa**: cobrir uma goroutine linha-a-linha não prova ausência de race condition ou deadlock. Isso exige uma categoria de teste diferente (abaixo), não apenas mais cobertura de linha.

Decisão de arquitetura: perseguir o teto prático mais alto possível através de **testabilidade por design**, e ser transparente sobre o número real e o porquê de qualquer gap, em vez de prometer "100%" e depois inflar a métrica com testes vazios.

### Testabilidade por design (pré-requisito arquitetural)

- **Injeção de dependência explícita via interfaces em todo módulo**, sem singletons globais nem `init()` com efeito colateral de I/O. Todo módulo de domínio (`internal/catalog`, `internal/library`, etc.) expõe um construtor que recebe suas dependências (repositório, cache, publisher de eventos) como interfaces:
```go
type TrackRepository interface {
    GetByID(ctx context.Context, id string) (Track, error)
    Search(ctx context.Context, q SearchQuery) ([]Track, error)
}

type CatalogService struct {
    repo  TrackRepository
    cache CacheClient
}

func NewCatalogService(repo TrackRepository, cache CacheClient) *CatalogService {
    return &CatalogService{repo: repo, cache: cache}
}
```
  Isso permite testar toda lógica de negócio com fakes/in-memory implementations das interfaces, sem subir Postgres/Redis reais no teste unitário — condição necessária para cobertura alta e testes rápidos (unit suite completa deve rodar em segundos, não minutos).
- **Isolamento de I/O nas bordas.** Chamadas a rede, disco, relógio (`time.Now()`) e geração de aleatoriedade (tokens, IDs) são sempre injetadas via interface (`Clock`, `IDGenerator`) — nunca chamadas diretamente dentro de lógica de domínio. Isso elimina a categoria de código "não testável por natureza" (tempo real, I/O real) do núcleo de negócio.
- **Nenhum uso de `panic` para controle de fluxo** (conforme requisito do projeto) — todo erro é um valor `error` retornado explicitamente e testável por asserção direta (`errors.Is`/`errors.As` com sentinel errors e tipos de erro de domínio), o que por si só facilita testar caminhos de erro sem `recover()` artificial em testes.
- **Handlers HTTP/gRPC finos.** Handlers apenas fazem parsing/validação de request e chamam o service layer — a lógica de negócio testável vive inteiramente no service layer, desacoplada do transporte. Isso permite cobrir a lógica de negócio sem precisar de testes HTTP end-to-end para cada branch.

### Pirâmide de testes

1. **Unit tests** (base da pirâmide, maioria dos testes): cobrem service layer, lógica de domínio, validação, cálculo (ex.: geo-index diffing, cálculo de distance bucket). Usam `testing` padrão + `testify/assert` para legibilidade; fakes/in-memory para todas as interfaces de I/O. Meta: rodar toda a suíte unitária em <30s no CI.
2. **Testes de concorrência dedicados**: além de unit tests, todo código que usa goroutines/channels (pipeline de presença, worker pools) roda sob `go test -race` obrigatoriamente no CI (falha de build se detectar data race), e testes específicos de propriedade para backpressure (ex.: "sob N updates simultâneos além da capacidade do buffer, o sistema rejeita explicitamente em vez de bloquear indefinidamente" — verificado com timeout no próprio teste). Fuzzing (`go test -fuzz`) aplicado a parsers de payload de entrada (parsing de frames WS, validação de request) para achar panics/edge cases automaticamente.
3. **Integration tests**: sobem dependências reais via containers efêmeros (`testcontainers-go` para Postgres/Redis), testando o módulo de domínio completo (service + repo real) contra um banco real — cobre migrations, queries SQL reais, comportamento real do driver. Rodam em CI em paralelo, isolados por schema/container por suíte.
4. **Contract tests** entre `smusic-core`, `presence-service` e `media-edge-service`: testes que validam que os `.proto` e o comportamento real de cada lado do contrato gRPC interno permanecem compatíveis — evita quebra silenciosa na fronteira entre os processos extraídos (seção 1).
5. **Load/performance tests**: `k6` ou `vegeta` para HTTP/REST, ferramenta de carga customizada em Go (usando o próprio client SDK de WS) para o feed de presença, simulando milhares de conexões concorrentes para validar as metas numéricas da seção 6 antes de cada release major. Rodam em ambiente de staging dedicado, não em CI de todo commit (custo/tempo), mas obrigatórios em pipeline de release.
6. **Chaos/graceful-shutdown tests**: teste automatizado que envia SIGTERM ao `presence-service` sob carga simulada e verifica que nenhuma conexão é derrubada abruptamente (todas recebem o frame de `drain`) e que o processo termina dentro do timeout configurado — valida o comportamento descrito na seção 3, não apenas por inspeção de código.

### Enforcement no CI

- Cobertura de linha medida (`go test -coverprofile`) e reportada por módulo, com **limiar mínimo obrigatório por módulo** (ex.: 90% em service layers de domínio) que falha o build se regredir — mas **sem bloquear merge por não atingir 100%** globalmente; gaps abaixo do limiar em código legitimamente difícil de testar (wiring, `main.go`) exigem justificativa explícita em comentário/PR, revisável por outro engenheiro, não supressão silenciosa.
- `go vet`, `staticcheck` e `go test -race` são gates obrigatórios de CI, não opcionais — fazem parte da definição de "testável" tanto quanto a métrica de cobertura em si.

---

## Perguntas em aberto para outros especialistas

**Para o especialista em arquitetura de dados:**
1. Modelo de dados definitivo para o catálogo (schema relacional normalizado vs. desnormalizado para leitura) e escolha de motor de busca dedicado (Elasticsearch, Meilisearch, ou extensão de busca full-text do Postgres) — este documento assume um índice de busca separado do banco transacional, mas não define qual.
2. Estratégia de particionamento/sharding do banco transacional principal (Postgres presumido) conforme a base de usuários cresce — este documento não assume um número de usuários-alvo.
3. Pipeline de ingestão de eventos de domínio (outbox pattern mencionado na seção 1/5) — qual message broker (Kafka, NATS, SQS) alimenta os times de dados a partir dos eventos emitidos pelo monólito, e qual a garantia de entrega exigida (at-least-once é assumido aqui, mas não confirmado).
4. Retenção e agregação de histórico de reprodução para features de recomendação/analytics — volume e janela de retenção não foram definidos neste documento.
5. Consistência do estado de presença entre réplicas do `presence-service` quando um usuário está na fronteira entre duas células geográficas/regiões (o documento assume sharding por célula, mas o tratamento de borda entre shards precisa de desenho conjunto).

**Para o especialista em segurança:**
1. Política final de tokens (duração exata de access/refresh token, rotação, revogação em logout de todos os dispositivos, detecção de reuse de refresh token roubado).
2. Modelo de privacidade de localização: este documento assume `distance_bucket` (nunca coordenadas exatas de terceiros) e opt-in explícito via `visibility`, mas a granularidade exata dos buckets, o raciocínio contra ataques de triangulação (um usuário malicioso inferindo posição exata combinando múltiplos buckets/ângulos) e a política de retenção de dados de localização brutos (quanto tempo o `presence-service` guarda lat/lon antes de descartar) precisam de revisão dedicada.
3. Assinatura e expiração de URLs de mídia (`media-edge-service`): algoritmo de assinatura, janela de validade exata, e proteção contra hotlinking/compartilhamento de URL assinada antes da expiração.
4. Modelo de ameaça para o WebSocket de presença: rate limiting de abuso (um usuário enviando localização falsa/spoofed em alta frequência para "stalkear" alguém), e se há necessidade de verificação de plausibilidade de movimento (ex.: rejeitar updates que implicariam velocidade de deslocamento impossível).
5. Requisitos de compliance (LGPD, dado o produto ser social/geolocalização) que podem impor restrições adicionais sobre o que este documento assume ser "best-effort" (ex.: direito ao esquecimento sobre histórico de presença, mesmo sendo dado efêmero).

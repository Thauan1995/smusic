# Arquitetura de Dados — smusic

**Status:** Planejamento (pré-implementação)
**Autor:** Especialista em Arquitetura de Dados
**Escopo:** Modelagem de dados, seleção de tecnologia de armazenamento, estratégia de escalabilidade e compliance de dados para a plataforma smusic (streaming musical + descoberta social em tempo real por proximidade).

---

## 0. Visão geral e princípios

O smusic tem duas cargas de trabalho fundamentalmente diferentes, que **não devem compartilhar o mesmo motor de banco de dados**:

| Carga de trabalho | Características | Categoria |
|---|---|---|
| Catálogo, usuários, playlists, assinaturas, histórico agregado | Forte consistência, relacionamentos ricos, transações ACID, leitura pesada mas previsível, dado de longa duração | **Transacional / relacional** |
| Presença em tempo real ("quem está ouvindo o quê, perto de mim, agora") | Escrita altíssima frequência (heartbeats a cada poucos segundos por usuário ativo), expiração automática (TTL), consultas geoespaciais de baixíssima latência, dado efêmero (segundos a minutos) | **Presença / geoespacial / tempo real** |

Princípio orientador: **poliglota por desenho, não por acidente**. Cada armazenamento é escolhido pelo padrão de acesso que ele serve, e a integração entre eles é feita via IDs estáveis (ex: `user_id`, `track_id` do Postgres referenciados em registros efêmeros do Redis) — nunca duplicando a fonte de verdade.

---

## 1. Schema relacional — dados transacionais

Banco relacional (justificativa na seção 2). Convenções: chaves primárias `UUID` (geradas na aplicação Go via UUIDv7 para manter localidade temporal de índice — ver seção 5), timestamps `TIMESTAMPTZ`, soft-delete via `deleted_at` onde a retenção/LGPD exigir (seção 6), toda tabela com `created_at`/`updated_at`.

### 1.1 Identidade e usuários

**`users`**
- `id UUID PK`
- `email CITEXT UNIQUE NOT NULL`
- `email_verified_at TIMESTAMPTZ`
- `password_hash TEXT` (nulo se login apenas via OAuth)
- `display_name TEXT NOT NULL`
- `handle TEXT UNIQUE` (ex: `@thauan`, para perfis públicos/compartilhamento)
- `avatar_url TEXT`
- `country_code CHAR(2)` — relevante para catálogo licenciado por território
- `status ENUM('active','suspended','deleted')`
- `created_at`, `updated_at`, `deleted_at`

**`user_auth_identities`** (OAuth/social login, separado de `users` para permitir múltiplos provedores)
- `id UUID PK`, `user_id FK→users`, `provider ENUM('google','apple','facebook','password')`, `provider_user_id TEXT`, `UNIQUE(provider, provider_user_id)`

**`user_devices`** (necessário para sessão, push notification e para correlacionar com presença — ver seção 4)
- `id UUID PK`, `user_id FK`, `platform ENUM('ios','android','web','desktop')`, `push_token TEXT`, `last_seen_at TIMESTAMPTZ`, `app_version TEXT`

### 1.2 Catálogo

**`artists`**
- `id UUID PK`, `name TEXT NOT NULL`, `slug TEXT UNIQUE`, `bio TEXT`, `image_url TEXT`, `verified BOOLEAN DEFAULT false`, `external_ids JSONB` (ISNI, Spotify ID de migração, etc.)

**`albums`**
- `id UUID PK`, `title TEXT NOT NULL`, `primary_artist_id FK→artists`, `release_date DATE`, `album_type ENUM('album','single','ep','compilation')`, `cover_url TEXT`, `label TEXT`

**`tracks`**
- `id UUID PK`, `title TEXT NOT NULL`, `album_id FK→albums NULL` (single pode não ter álbum formal), `duration_ms INTEGER NOT NULL`, `track_number SMALLINT`, `isrc TEXT UNIQUE`, `explicit BOOLEAN`, `audio_asset_id FK→track_audio_assets`, `popularity_score REAL DEFAULT 0` (cache denormalizado, recalculado por job assíncrono a partir do histórico de reprodução; ver seção 5 sobre por que isso não é calculado on-the-fly)
- Índice composto `(album_id, track_number)`; `GIN` trigram em `title` como fallback de busca (ver seção 5 sobre FTS dedicado)

**`track_artists`** (N:N — feats, colaborações)
- `track_id FK`, `artist_id FK`, `role ENUM('primary','featured','producer','composer')`, PK composta

**`track_audio_assets`**
- `id UUID PK`, `track_id FK`, `storage_uri TEXT` (referência a objeto em blob storage/CDN, não o binário), `bitrate_kbps INTEGER`, `codec ENUM('aac','opus','flac')`, `quality_tier ENUM('low','normal','high','lossless')` — modelada como tabela separada (1:N por faixa) para suportar múltiplas qualidades/codecs sem alterar a tabela `tracks`

**`genres`** e **`track_genres`** (N:N) — taxonomia para recomendação e navegação.

### 1.3 Playlists e biblioteca

**`playlists`**
- `id UUID PK`, `owner_id FK→users`, `title TEXT`, `description TEXT`, `visibility ENUM('private','unlisted','public','collaborative')`, `cover_url TEXT`

**`playlist_tracks`**
- `id UUID PK` (não usar PK composta `(playlist_id, track_id)` pois a mesma faixa pode aparecer 2x na playlist), `playlist_id FK`, `track_id FK`, `position INTEGER NOT NULL` (reordenação — usar espaçamento fracionário ou `position` do tipo `NUMERIC` para evitar reindexação em massa a cada drag-and-drop), `added_by FK→users`, `added_at`
- Índice `(playlist_id, position)`

**`library_tracks`** (favoritos / "Músicas Curtidas")
- `user_id FK`, `track_id FK`, `added_at`, PK composta `(user_id, track_id)`

**`library_albums`**, **`library_artists`** (seguir/salvar) — mesmo padrão de tabela de associação simples.

### 1.4 Histórico de reprodução

Alto volume de escrita (toda reprodução gera evento), mas consultado majoritariamente em agregações (não linha a linha). Duas camadas:

**`play_events`** (fato bruto, particionada por tempo — ver seção 5)
- `id UUID PK`, `user_id FK`, `track_id FK`, `device_id FK→user_devices`, `played_at TIMESTAMPTZ NOT NULL`, `ms_played INTEGER`, `context_type ENUM('playlist','album','radio','search','nearby_discovery')`, `context_id UUID NULL` — o valor `'nearby_discovery'` é o gancho que liga reprodução a uma descoberta feita via presença social, permitindo métricas de produto ("X% das reproduções vieram de descoberta por proximidade") sem acoplar o histórico ao armazenamento de presença efêmero.
- Particionada mensalmente por `played_at` (partição declarativa Postgres).

**`user_play_stats`** (agregado, recalculado por job — não em tempo real)
- `user_id FK`, `track_id FK`, `play_count INTEGER`, `last_played_at`, PK `(user_id, track_id)` — evita fazer `COUNT()` sobre `play_events` para telas de "mais tocadas".

### 1.5 Assinaturas / planos

**`plans`**
- `id UUID PK`, `code TEXT UNIQUE` (`free`, `premium_individual`, `premium_family`, `premium_student`), `price_cents INTEGER`, `currency CHAR(3)`, `max_devices SMALLINT`, `audio_quality_tier`

**`subscriptions`**
- `id UUID PK`, `user_id FK`, `plan_id FK`, `status ENUM('trialing','active','past_due','canceled','expired')`, `current_period_start`, `current_period_end`, `payment_provider ENUM('stripe','apple_iap','google_play')`, `payment_provider_ref TEXT` (id externo, nunca armazenar dado de cartão)

**`family_plan_members`** — `subscription_id FK`, `user_id FK`, para planos família/duo.

### 1.6 Relacionamentos sociais (base para a feature de presença)

**`follows`** — `follower_id FK→users`, `followee_id FK→users`, PK composta. Usado para eventualmente priorizar "amigos próximos" sobre "estranhos próximos" na feature de descoberta.

**`user_privacy_settings`** — ver seção 4.5; campo-ponte entre o modelo relacional e a política de privacidade de presença.

---

## 2. Escolha do banco relacional: PostgreSQL

**Decisão: PostgreSQL (versão atual estável, 16/17) como banco transacional primário.**

Justificativa frente a alternativas:

- **vs. MySQL**: Postgres tem suporte nativo superior a `JSONB` indexável (útil para `external_ids`, metadados de catálogo variáveis por fonte de ingestão), particionamento declarativo maduro, `CITEXT` para e-mail case-insensitive sem lógica na aplicação, e extensões relevantes (`pg_trgm` para busca fallback, `pgcrypto`/`uuid-ossp`). O ecossistema Go tem drivers de primeira classe (`pgx`) com melhor suporte a tipos nativos do Postgres que os drivers MySQL equivalentes.
- **vs. bancos NoSQL (MongoDB/DynamoDB) como base primária**: o domínio (usuários, playlists, faixas, assinaturas) é **fortemente relacional** — playlist→faixas→álbuns→artistas, assinatura→plano→pagamento — com integridade referencial crítica (não pode existir `playlist_tracks` apontando para `track_id` inexistente, não pode haver assinatura sem plano válido). Modelar isso em um documento desnormalizado obriga a aplicação a reimplementar consistência que o banco relacional já garante via FKs e transações ACID. NoSQL se justificaria se o padrão de acesso fosse dominado por leitura de agregados isolados sem necessidade de joins ad-hoc — não é o caso aqui (ex: "faixas da playlist X que também estão na biblioteca do usuário Y" é uma consulta relacional natural).
- **vs. CockroachDB/Yugabyte (SQL distribuído)**: adiam complexidade operacional (consenso distribuído, latência de escrita cross-região) que não se justifica na fase atual. Postgres com réplicas de leitura (seção 5) atende a escala inicial e intermediária; a migração para SQL distribuído fica como caminho de evolução, não como ponto de partida — reduz risco operacional na fase de lançamento.
- Postgres tem histórico comprovado de escalar bibliotecas musicais em produção (é a base declarada publicamente por diversos serviços de streaming em algum ponto de sua stack) e ecossistema de observability/backup/tooling maduro para operação por equipe pequena.

**Não** usar um único banco Postgres para presença em tempo real — ver seção 3.

---

## 3. Armazenamento de presença em tempo real / geolocalização: Redis

**Decisão: Redis (com módulo de geoespacial nativo `GEOADD`/`GEOSEARCH` + Pub/Sub, ou Redis Cluster para escala) como armazenamento primário da feature de descoberta por proximidade.**

### Por que um banco relacional tradicional é inadequado aqui

1. **Padrão de escrita**: se cada usuário ativo emite um heartbeat de localização/faixa a cada 10-30s, com centenas de milhares de usuários simultâneos isso é uma taxa de escrita de milhares a dezenas de milhares de `UPSERT`s por segundo, cada um substituindo o valor anterior (não é um log append-only). Postgres pagaria custo de MVCC (cada update gera uma nova versão de linha + tombstone para vacuum) para dados que serão descartados em segundos — geraria bloat e pressão de autovacuum incompatível com a carga transacional do catálogo compartilhando o mesmo cluster.
2. **Expiração**: a feature exige que registros de presença expirem automaticamente (usuário parou de ouvir → deve sumir do radar de proximidade em segundos, não minutos). Postgres não tem TTL nativo por linha; simular isso exige jobs de limpeza (`DELETE WHERE expires_at < now()`) rodando com alta frequência, competindo por I/O com o workload transacional. Redis tem `EXPIRE`/`PEXPIRE` nativo por chave, O(1), sem necessidade de job de limpeza.
3. **Consulta geoespacial de baixa latência**: "quem está num raio de N metros de mim" precisa responder em dezenas de milissegundos para não travar a UI. Redis `GEOSEARCH` (baseado em geohash + sorted set) resolve isso em memória; a extensão `PostGIS` do Postgres faz consultas geoespaciais corretamente mas é otimizada para dados espaciais relativamente estáveis com índices GiST — reconstruir/atualizar esses índices na frequência de heartbeat de presença é um padrão de uso que o PostGIS não foi desenhado para servir com a mesma taxa de escrita/expiração.
4. **Modelo de consistência**: presença é, por natureza, um dado "best effort" e eventualmente consistente — não precisa de ACID nem de durabilidade em disco garantida (perder um heartbeat de 10s não é uma falha de negócio). Isso é exatamente o perfil que justifica trocar durabilidade forte por velocidade — o oposto do que se quer para uma tabela de assinaturas ou biblioteca.
5. **Pub/Sub embutido**: a notificação em tempo real ("alguém entrou no seu raio") se beneficia do Pub/Sub nativo do Redis (ou Redis Streams para garantir entrega/replay), evitando polling do cliente contra o banco relacional.

### Alternativas consideradas e descartadas

- **Banco de séries temporais dedicado (InfluxDB/TimescaleDB)**: bom para métricas agregadas ao longo do tempo, mas não é o encaixe natural para "estado atual mais recente por usuário com busca geoespacial" — a pergunta não é "qual foi a série histórica de posição", é "quem está aqui agora". TimescaleDB é uma extensão Postgres, herdando parte das limitações do item 1-2 acima para este padrão de escrita/expiração específico.
- **Elasticsearch com tipo `geo_point`**: suporta geoespacial, mas é otimizado para busca/agregação sobre grandes volumes relativamente estáveis, com custo de indexação mais alto por escrita do que Redis — inadequado para um dado que muda a cada heartbeat e expira em segundos. (Elasticsearch continua sendo a escolha correta para busca textual do catálogo — seção 5, é um recurso separado.)
- **Manter presença só em memória do processo Go (sem store externo)**: inviável assim que houver mais de uma instância do serviço atrás de um load balancer — usuários conectados a instâncias diferentes não veriam uns aos outros. Redis serve como o "estado compartilhado" entre instâncias stateless do backend Go.

---

## 4. Modelo de dados da feature de presença

### 4.1 Representação de "usuário X está a Y metros ouvindo faixa Z agora"

Estrutura proposta em Redis (chaves ilustrativas, não é DDL de implementação):

- **Índice geoespacial**: um sorted set geoespacial por célula de região/cidade (sharding lógico — ver 4.4), ex. `geo:presence:{region_shard}`, populado via `GEOADD` com `member = user_id` (ou um `presence_id` opaco — ver 4.5 sobre privacidade) e coordenadas.
- **Registro de presença detalhado**: hash separado `presence:{user_id}` contendo:
  - `track_id` (FK lógica para `tracks.id` no Postgres — nunca duplicar metadados da faixa aqui, só o ID)
  - `started_at` (epoch)
  - `geohash` (não lat/lng exato — ver 4.3)
  - `visibility_flag` (ver 4.5)
  - `device_id`
- Separar o índice geoespacial (para busca "quem está perto") do hash de detalhe (para "o que essa pessoa está ouvindo") permite expirar/consultar cada um com granularidade própria e permite que o índice geoespacial guarde apenas um identificador opaco, sem vazar automaticamente "o que" a pessoa ouve para quem só está fazendo a varredura geoespacial — a leitura do "o que" exige um segundo passo explícito, que é o ponto de controle de privacidade (seção 4.5).

### 4.2 TTL / expiração

- Cada chave de presença (`presence:{user_id}` e a entrada correspondente no sorted set geoespacial) recebe TTL curto, proposto em **60–90 segundos**, renovado a cada heartbeat do cliente (app envia heartbeat a cada 20-30s; 2-3 heartbeats perdidos = usuário considerado offline e some do radar).
- Isso é uma **proposta técnica de ordem de grandeza**, não uma política fechada — o valor exato de TTL e a frequência de heartbeat são uma escolha conjunta de UX (quão "ao vivo" a feature parece) e de política de privacidade (quanto tempo um dado de localização/escuta de um terceiro permanece consultável mesmo que efemeramente) — sinalizado como pergunta em aberto na seção 7.
- Ao expirar, o registro simplesmente desaparece (sem tombstone, sem necessidade de job de limpeza) — reforça a escolha do Redis sobre Postgres para este dado.

### 4.3 Granularidade de localização — proposta técnica concreta

**Proposta: nunca armazenar lat/lng exato do usuário no dado de presença consultável por terceiros. Usar geohash truncado.**

- Um geohash completo (ex: 9+ caracteres) equivale a precisão de ~metros, suficiente para identificar uma residência exata — inaceitável para um dado exposto a terceiros por padrão.
- Proposta: truncar o geohash usado no índice de proximidade para **6 caracteres (~1.2km x 0.6km de célula)** como granularidade padrão de *exposição a terceiros*, mesmo que a posição bruta capturada do GPS do dispositivo tenha mais precisão internamente para fins de cálculo de distância aproximada.
- A distância exibida ao usuário final ("a ~800m de você") deve ser **arredondada em faixas** (ex: "menos de 500m", "500m–1km", "1-2km") em vez de um valor contínuo preciso, para evitar que múltiplas leituras ao longo do tempo permitam a um usuário malicioso triangular a posição exata de outro por trilateração.
- O `geohash` truncado é o único campo de localização gravado no registro de presença (`presence:{user_id}.geohash`) — a coordenada bruta, se necessária para cálculo de ordenação por proximidade, deve ser processada no momento da escrita (no serviço Go) e descartada, não persistida em texto claro associada ao `user_id` em nenhum armazenamento de longa duração.
- Consulta "quem está próximo de mim": o backend Go calcula o geohash truncado do requisitante, faz `GEOSEARCH` no shard de região correspondente com raio configurável (ex: 1-5km), filtra os `visibility_flag` (seção 4.5) antes de retornar qualquer resultado, e resolve os `user_id`/`track_id` retornados contra o Postgres (dados relativamente estáveis: nome de exibição, avatar, título da faixa) só para o conjunto final já filtrado — minimizando quantos perfis completos são "tocados" por consulta.

### 4.4 Estratégia de consulta "quem está próximo de mim" e sharding lógico

- Chaves geoespaciais particionadas por região grosseira (ex: por país/estado, ou por célula de geohash de 2-3 caracteres) — `geo:presence:{coarse_region}` — para (a) permitir que o Redis Cluster distribua a carga por região sem uma única sorted set gigante global, e (b) já alinhar com eventuais requisitos de residência de dados por país.
- Fluxo de consulta: app do usuário requisitante envia sua posição aproximada → serviço Go resolve a região grosseira → `GEOSEARCH` na(s) partição(ões) relevante(s) (considerar borda de região: consultar região vizinha se o usuário estiver perto do limite) → aplica filtro de visibilidade e de bloqueio/follow (se a política de privacidade decidir que "amigos" têm regra diferente de "estranhos") → hidrata resultado com dados do Postgres → retorna ao cliente.

### 4.5 Ponto de decisão de privacidade — suporte de schema (decisão de política é do especialista em segurança)

Esta feature expõe, por padrão, a localização aproximada e a faixa que um terceiro está ouvindo — dado sensível sob a LGPD (dado de geolocalização é tratado com sensibilidade elevada em orientações da ANPD, e hábitos de consumo podem compor perfil comportamental). A arquitetura de dados propõe os **pontos de controle no schema** para suportar qualquer decisão de política que o especialista em segurança/privacidade tomar, mas **não define a política em si**:

- `user_privacy_settings` (Postgres, fonte de verdade de configuração persistente):
  - `presence_visibility ENUM('invisible','friends_only','everyone')` — proposta de default é uma pergunta em aberto (ver seção 7); tecnicamente suportamos qualquer default.
  - `presence_share_track BOOLEAN` — permite a um usuário aparecer no radar de proximidade sem revelar *o que* está ouvindo (só presença, não a faixa).
  - `presence_location_granularity_override SMALLINT NULL` — permite que a política decida, no futuro, oferecer ao usuário um controle de granularidade mais grosseiro que o default (ex: usuário mais cauteloso escolhe célula de geohash de 4 caracteres em vez de 6).
- `visibility_flag` replicado no registro efêmero do Redis (`presence:{user_id}.visibility_flag`) é uma **cópia de leitura rápida** da configuração do Postgres, sincronizada no momento em que o heartbeat é processado — evita que toda consulta de proximidade precise ir ao Postgres para checar a preferência de cada candidato retornado pelo `GEOSEARCH`. Qualquer mudança de configuração do usuário deve invalidar/atualizar essa cópia imediatamente (não esperar o próximo heartbeat), para que "modo invisível" tenha efeito perceptualmente instantâneo.
- O schema também deixa espaço para um requisito comum em features desse tipo — **bloqueio unilateral** (`user_blocks: blocker_id, blocked_id`, tabela Postgres) — a ser consultado no filtro de resultado da busca de proximidade, caso a política de segurança determine que usuários bloqueados nunca devem se ver mutuamente no radar independentemente da visibilidade geral.

---

## 5. Estratégia de escalabilidade

O objetivo declarado é igualar/superar Spotify/YTM em performance de biblioteca e reprodução — isso orienta as escolhas abaixo para os dois eixos: **latência de leitura do catálogo/biblioteca** e **capacidade de escrita/consulta de presença**.

### 5.1 Particionamento / sharding

- **`play_events`**: particionamento declarativo por `played_at` (mensal). Justificativa: é a tabela de maior volume de escrita e crescimento ilimitado no domínio transacional; particionar permite `DROP PARTITION` eficiente quando a política de retenção (seção 6) exigir expurgo de dados antigos, em vez de `DELETE` em massa.
- Catálogo (`tracks`, `albums`, `artists`) não precisa de particionamento neste estágio — volume (milhões de faixas) é gerenciável por um único cluster Postgres bem indexado; particionamento aqui adicionaria complexidade sem ganho real na escala inicial/intermediária.
- Redis: sharding lógico por região geográfica já descrito (4.4); tecnicamente viabilizado por Redis Cluster (hash slots) quando o volume de usuários simultâneos exceder a capacidade de uma única instância/réplica primária.

### 5.2 Réplicas de leitura

- Postgres com topologia primário + N réplicas de leitura (streaming replication). Tráfego de leitura de catálogo (buscar álbum, carregar playlist, listar biblioteca) é ordens de magnitude maior que escrita e tolera a leve defasagem de réplica — direcionar via connection pooling no backend Go (ver seção 7, dependência do time de backend).
- Escritas que exigem consistência imediata pós-escrita (ex: usuário acabou de criar uma playlist e espera vê-la na tela seguinte) devem ler do primário ou usar "read-your-writes" via roteamento explícito — decisão de implementação a ser refinada com o backend, mas o schema não impõe obstáculo a isso.

### 5.3 Cache

- **Redis como cache de catálogo quente**, camada adicional (instância/cluster logicamente separado do Redis de presença, para não competir por memória/CPU com um workload de latência crítica de UX social): cache de faixas/álbuns mais populares, resultados de "Top charts", metadados de playlists públicas populares — reduz carga nas réplicas de leitura do Postgres para os itens de acesso mais desproporcional (efeito long-tail: poucas faixas concentram a maioria dos plays).
- Invalidação por TTL curto + invalidação ativa em eventos de escrita relevantes (ex: playlist pública editada) — a cargo do backend definir a estratégia exata (write-through vs. cache-aside), mas a arquitetura de dados recomenda **cache-aside com TTL curto** como padrão inicial por ser mais simples de operar corretamente.

### 5.4 Busca full-text — Elasticsearch/Meilisearch dedicado vs. Postgres FTS

**Decisão: motor de busca dedicado (Meilisearch ou Elasticsearch) para a busca do catálogo voltada ao usuário final (buscar faixa/artista/álbum/playlist), com Postgres FTS/`pg_trgm` mantido apenas como fallback interno de baixo tráfego.**

Justificativa:
- Igualar a experiência de busca do Spotify/YTM exige tolerância a erro de digitação, busca fonética, ranking por relevância combinando popularidade + correspondência textual, busca multi-campo (artista+título+álbum simultaneamente) e latência sub-100ms mesmo sob carga alta — isso é o caso de uso central de motores de busca dedicados, não um requisito acessório.
- Postgres FTS (`tsvector`/`tsquery`) é competente para buscas estruturadas simples, mas fica claramente atrás em fuzzy matching, tolerância a erro de digitação e tuning de relevância — recursos que se tornariam trabalho de engenharia significativo para replicar manualmente, quando um motor dedicado já resolve isso nativamente.
- **Meilisearch** é a recomendação default para a fase inicial: mais simples de operar, latência muito baixa fora da caixa, ótimo para relevância "tipo Spotify" (fuzzy + ranking por atributo, incluindo boost por `popularity_score`) com bem menos overhead operacional que um cluster Elasticsearch. Elasticsearch permanece como alternativa se, na escala, forem necessários recursos de agregação/analytics que o Meilisearch não cobre bem (nesse caso, poderia coexistir: Meilisearch para busca de catálogo, Elasticsearch para analytics/logs, se necessário).
- Pipeline de indexação: o Postgres continua a fonte de verdade; alterações em `tracks`/`albums`/`artists`/`playlists públicas` propagam para o índice de busca via CDC (ex: `pgoutput`/Debezium) ou job incremental — a busca nunca escreve direto no motor de busca a partir da aplicação sem passar pelo Postgres primeiro, evitando divergência de fonte de verdade.

### 5.5 Chaves primárias e localidade de índice

Uso de **UUIDv7** (timestamp-ordenado) em vez de UUIDv4 aleatório para todas as PKs de alto volume de inserção (`play_events`, `presence` quando aplicável, tabelas de associação de alto volume) — evita fragmentação de índice B-tree que UUIDv4 aleatório causa em cargas de escrita intensa, mantendo a vantagem de geração distribuída sem round-trip ao banco (relevante para o backend Go gerar IDs client-side).

---

## 6. Migração de schema e retenção/exclusão de dados (LGPD)

### 6.1 Versionamento e migração de schema

- Migrações versionadas, incrementais e reversíveis, aplicadas via ferramenta de migração (ex: `golang-migrate` ou `atlas`, escolha final com o time de backend Go) — cada mudança de schema é um arquivo numerado sequencialmente, aplicado em pipeline de CI/CD antes do deploy do binário Go que depende dela.
- Regra de compatibilidade: mudanças de schema em produção seguem o padrão *expand/contract* (adicionar coluna nova nullable → deploy do código que escreve nos dois formatos → backfill → deploy do código que só lê o novo formato → remover coluna antiga) para permitir deploy sem downtime com múltiplas instâncias do backend rodando versões adjacentes durante o rollout.
- Alterações nas chaves/estruturas do Redis (ex: mudança do formato de `presence:{user_id}`) não têm "migração" no sentido tradicional (dado efêmero) — mudanças de formato só precisam de compatibilidade transitória entre versões do serviço durante um rolling deploy, dado o TTL curto natural dos dados.

### 6.2 Retenção e exclusão — compliance LGPD

- **Direito de exclusão (Art. 18, LGPD)**: ao usuário solicitar exclusão de conta, a tabela `users` recebe `deleted_at` (soft delete imediato, bloqueia login e visibilidade), seguido de um job assíncrono que:
  - anonimiza/expurga `play_events` do usuário (ou agrega e descarta o vínculo com `user_id`, mantendo apenas estatística agregada anônima para métricas de produto, se a base legal permitir),
  - remove entradas em `library_tracks`, `follows`, `user_privacy_settings`,
  - remove qualquer registro de presença remanescente no Redis imediatamente (não espera o TTL natural),
  - mantém apenas o mínimo exigido por obrigação legal/contábil (ex: histórico de transações de assinatura, por prazo fiscal) — este ponto de exceção deve ser confirmado com jurídico/segurança quanto ao prazo exato.
- **Minimização de dado de presença**: como já não persistimos coordenada exata (seção 4.3) e o TTL é curto, a superfície de exclusão para o dado mais sensível (localização) já é pequena por desenho — o dado praticamente se autodestrói antes que uma solicitação de exclusão precise agir sobre ele. Isso deve ser citado explicitamente em qualquer relatório de impacto à proteção de dados (RIPD/DPIA) que o especialista de segurança/privacidade precise preparar para esta feature.
- **Retenção de `play_events`**: proposta de retenção detalhada (por `user_id`) por um período definido pela política de produto/privacidade (ex: 12-24 meses), após o qual as partições mensais mais antigas são agregadas em estatística anônima e a partição bruta é dropada (`DROP PARTITION`, operação barata graças ao particionamento da seção 5.1). Prazo exato é uma decisão de política, não técnica — sinalizado na seção 7.
- **Portabilidade de dados**: schema relacional facilita atender a um eventual pedido de exportação (Art. 18, IV/V) via um job que faz `SELECT` das tabelas relevantes por `user_id` e serializa para um formato exportável (JSON) — não requer desenho adicional além do já modelado.

---

## 7. Perguntas em aberto para outros especialistas

### Para o especialista em segurança / privacidade (decisão de política — a arquitetura de dados só propõe suporte técnico)

1. **Modelo de visibilidade padrão da presença**: qual deve ser o default de `presence_visibility` para um novo usuário — `invisible` (opt-in explícito para aparecer), `friends_only`, ou `everyone`? Isso tem implicação direta em base legal LGPD (opt-in explícito é o caminho mais seguro para dado sensível, mas reduz o efeito de rede do produto).
2. **Existência e comportamento de "modo invisível"** temporário (ex: usuário visível por padrão mas pode se ocultar por sessão) — o schema já suporta via `presence_visibility` e `visibility_flag`, mas a UX/regra de negócio (é por sessão? por dispositivo? global?) precisa vir da política de privacidade.
3. **Granularidade final de localização exposta**: a proposta técnica desta doc é geohash truncado a 6 caracteres (~1km) com distância exibida em faixas, não valor contínuo — isso precisa de validação/aprovação como adequado (ou o especialista pode exigir granularidade ainda mais grosseira, ou granularidade configurável pelo usuário).
4. **TTL exato de presença** (proposto 60-90s nesta doc) — confirmar se esse prazo é aceitável do ponto de vista de exposição de dado ou se deve ser mais curto.
5. **Prazo de retenção de `play_events`** (histórico de escuta) por usuário antes de agregação/expurgo — decisão de política de produto + privacidade, não técnica.
6. **Regra de bloqueio/lista de exclusão** para a busca de proximidade (usuários bloqueados nunca se veem?) e se deve haver algum mecanismo de "denunciar" que suspenda a visibilidade de presença de um usuário preventivamente.
7. Necessidade formal de **RIPD/DPIA** para a feature de presença antes do lançamento (o schema já foi desenhado pensando em minimização de dado, mas a decisão formal e o documento são de responsabilidade de segurança/privacidade).

### Para o backend em Go

1. Confirmação do driver Postgres (`pgx` é a recomendação implícita desta arquitetura pelo suporte superior a tipos nativos e por performance, mas a decisão final e a estratégia de connection pooling — pool interno do `pgx` vs. PgBouncer externo — cabe ao time de backend, especialmente considerando o número de instâncias stateless que abrirão conexões contra primário/réplicas).
2. Estratégia de client para Redis (`go-redis` é o padrão de mercado) e decisão sobre Redis Cluster vs. Redis Sentinel para a camada de presença, considerando a topologia de deploy (multi-região?) que só o backend/infra pode definir.
3. Mecanismo exato de CDC do Postgres para o motor de busca (Debezium completo vs. job incremental mais simples via polling de `updated_at`) — depende da complexidade operacional que o time está disposto a assumir na fase de lançamento.
4. Confirmação de que o padrão de heartbeat de presença (cliente móvel envia posição a cada 20-30s) é viável do ponto de vista de consumo de bateria/dados no app — isso é decisão de produto/mobile, mas afeta diretamente o TTL e a carga de escrita do Redis assumidos nesta doc.

### Para o frontend / apps móveis

1. Confirmação de que distância exibida em faixas (não valor contínuo) é aceitável para a experiência de descoberta pretendida, dado que é uma restrição de privacidade recomendada nesta doc (seção 4.3).
2. Necessidade de indicar visualmente ao usuário, em tempo real, quando seu próprio status de presença está ativo/invisível — depende da UX que a política de privacidade definir.

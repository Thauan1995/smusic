# smusic — Visão Geral da Arquitetura (Síntese)

Status: **Fase de planejamento concluída (rodada 1).** Este documento consolida as decisões dos 4 especialistas (Go/backend, Flutter/frontend, arquitetura de dados, segurança) e resolve/expõe os pontos de atrito entre eles. Detalhes completos em cada documento:

- [`backend-go.md`](./backend-go.md)
- [`frontend-flutter.md`](./frontend-flutter.md)
- [`data-architecture.md`](./data-architecture.md)
- [`security.md`](./security.md)

---

## 1. Consistência entre especialistas — checagem cruzada

Os 4 documentos foram produzidos em paralelo, sem ver o resultado uns dos outros, e convergiram de forma consistente nos pontos centrais:

| Decisão | Dados | Segurança | Backend | Frontend |
|---|---|---|---|---|
| Presença nunca em banco durável | ✅ Redis, TTL 60-90s | ✅ Redis, TTL 90s, "nunca persistida" | ✅ Redis, "best-effort", sem transação síncrona | ✅ assume backend não persiste bruto |
| Localização exposta ao cliente | Propôs geohash truncado (6 chars, ~1km) como *suporte técnico interno* | **Decisão final**: cliente nunca recebe geohash/coordenada — só 4 buckets relativos + jitter ±75m | Já assumiu `distance_bucket` no protocolo WS (convergência espontânea) | Já assumiu bucket, nunca metragem exata |
| Opt-in explícito | Propôs `presence_visibility` enum como gancho de schema | **Decisão final**: opt-in explícito, off por padrão, renovação semestral | Protocolo WS já inclui frame `visibility` opt-in/opt-out | Fluxo de permissão já desenhado como opt-in com tela de valor antes do prompt do SO |
| Bloqueio | Propôs tabela `user_blocks` | Confirma: bloqueio silencioso, avaliado server-side | — | Sinalizou como pergunta aberta (já respondida por segurança) |

**Não há contradição bloqueante.** A arquitetura de dados propôs corretamente o "suporte técnico" (geohash interno, TTL, flag de visibilidade) sem decidir a política — exatamente como pedido — e segurança fechou a política em cima desse suporte. Falta apenas formalizar 3 ajustes finos no schema de dados (abaixo).

### Ajustes a aplicar no schema de dados (não requer nova rodada de especialista, são refinamentos diretos)

1. `presence:{user_id}.geohash` (proposto pelos dados) deve ser substituído/complementado por lat/lng efêmero em memória de processo (não Redis) só durante o cálculo do bucket + jitter — segurança exige que a coordenada "limpa" nunca fique acessível a um serviço além do cálculo de bucket. **Efeito prático**: o campo persistido em Redis deve ser `distance_bucket` pré-calculado por par de consulta, não um geohash reconsultável — ou, se o geohash truncado permanecer como índice interno do `GEOSEARCH`, o resultado bruto do `GEOSEARCH` nunca deve trafegar para fora do `presence-service` sem passar pelo jitter+bucketing.
2. `user_privacy_settings.presence_visibility` default = **`invisible`** (não `friends_only` nem `everyone`) — decisão de segurança (feature nasce desligada).
3. Adicionar campos de consentimento explícitos que dados listou como pendentes: `proximity_consent_enabled`, `proximity_consent_ts`, `proximity_consent_renew_due` (renovação a cada 6 meses), `visibility_radius_m` (enum 150/1000/5000/15000), `reveal_level` (0/1/2), `paused_bool` — já especificados por segurança, ficam formalmente incorporados ao schema de dados.

---

## 2. Ponto de atrito real: meta de 100% de cobertura de testes literal

Você definiu como critério de parada do loop do Auditor: **100% de cobertura de linha, literal**. Dois dos quatro especialistas (Go e Flutter) — de forma independente — desaconselharam tratar isso como meta global forçada, pelos mesmos motivos técnicos (cobertura de linha ≠ qualidade de teste; código gerado/boilerplate/branches defensivos "impossíveis" custam caro para cobrir sem benefício real; ambos recomendam excluir código gerado da métrica e usar limiares por módulo).

Isso é uma tensão real entre o critério de parada que você definiu e a recomendação técnica unânime dos dois especialistas de implementação. Preciso da sua decisão explícita antes de configurar o loop do Auditor, porque o Auditor vai literalmente aprovar ou rejeitar com base nesse número.

---

## 3. Plano de MVP incremental (proposta, não iniciado)

Com base nos 4 documentos, o corte de MVP mais barato que já exercita a arquitetura ponta a ponta (auth → catálogo mínimo → reprodução → presença com privacidade real, não uma versão fake) seria:

**Fatia 1 — esqueleto vertical fino:**
- Backend: monólito modular Go (auth + catalog mínimo + playback-state), sem ainda extrair `media-edge-service`/`presence-service` como processos separados (podem começar como módulos internos e ser extraídos quando o documento de backend recomenda — regra dos >40% CPU).
- Dados: Postgres com schema de `users`, `tracks`, `albums`, `artists`, `library_tracks`, `play_events` (sem particionamento ainda — YAGNI até volume real).
- Frontend: monorepo Melos com a estrutura de camadas completa desde o início (é mais barato começar certo do que migrar depois), auth + player básico (sem crossfade/offline ainda) + biblioteca com virtualização.
- Segurança: auth completo (JWT curto + refresh revogável, Argon2id, MFA opcional) desde o dia 1 — não é algo para adicionar depois.
- **Proximidade fica fora da Fatia 1** — é a feature de maior risco de privacidade/segurança; entra na Fatia 2 já com o modelo completo de opt-in/buckets/jitter/auditoria, nunca uma versão simplificada "temporária" que exporia coordenadas.

**Fatia 2 — diferencial competitivo:**
- `presence-service` extraído, Redis geoespacial, protocolo WS completo com o modelo de privacidade final de segurança.
- `media-edge-service` extraído, CDN, HLS adaptativo.

Cada fatia é implementada pelos 4 especialistas em paralelo novamente (cada um no seu domínio), seguida de auditoria.

---

## 4. Próximo passo

Conforme sua escolha original, esta síntese volta para sua aprovação antes de qualquer implementação. Preciso de decisão sobre a tensão da seção 2, e confirmação (ou ajuste) do plano de fatias da seção 3 antes de começar a implementar e de configurar o loop do Auditor.

# smusic — Visão Geral da Arquitetura (Síntese)

Status: **Fase de planejamento concluída. Fatia 1 (base) implementada, corrigida e aprovada pelo Auditor. Fatia 2 (proximidade social) em andamento.** Detalhes completos em cada documento:

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

Ajustes de schema já incorporados na implementação da Fatia 2 (ver seção 5): `distance_bucket` pré-calculado nunca reconsultável como geohash bruto fora do serviço de presença; `presence_visibility` default `invisible`; campos de consentimento (`proximity_consent_enabled/ts/renew_due`, `visibility_radius_m`, `reveal_level`, `paused_bool`) formalizados no schema.

---

## 2. Decisão: política de cobertura de testes (RESOLVIDO)

**Decisão do usuário: 100% de cobertura em todo código escrito à mão (lógica de domínio/negócio), excluindo código gerado automaticamente (`*.g.dart`, `*.freezed.dart`, wiring de `main.go`) e branches defensivos documentados como impossíveis — cada exclusão precisa de justificativa explícita e revisável, nunca silenciosa.**

Esse é o critério que o Auditor usa para aprovar/rejeitar cobertura de testes. Uma exclusão sem justificativa documentada no código/PR conta como cobertura não atingida.

---

## 3. Plano de MVP incremental

**Fatia 1 — esqueleto vertical fino: CONCLUÍDA E APROVADA.**
Backend Go (monólito modular: auth, catalog, library, playback-state), Postgres, Flutter (monorepo Melos completo, auth, player, biblioteca). Auditado 2x (aprovação inicial com 4 ressalvas → todas fechadas → reauditoria aprovou avançar).

Dívida técnica não-bloqueante registrada pela reauditoria (rastrear, não esquecer):
1. `JustAudioNativeEngine.setNextSource` é no-op em produção — falta `ConcatenatingAudioSource` para gapless real funcionar de ponta a ponta (o prefetch já busca/resolve a próxima faixa corretamente, só o engine não usa isso ainda).
2. `frontend/app/smusic_web/integration_test/real_backend_e2e_test.dart` nunca rodou até o fim (bloqueio de ambiente de sandbox confirmado 2x de forma independente, inclusive pelo próprio Auditor). Rodar em CI/máquina com Chrome debug funcional antes do lançamento público.

**Fatia 2 — diferencial competitivo (proximidade social): EM ANDAMENTO.**
- `presence-service` (processo separado, per backend-go.md seção 1), Redis geoespacial (GEOADD/GEOSEARCH + Pub/Sub, TTL 90s), protocolo WS completo.
- Modelo de privacidade completo de security.md seção 1: opt-in explícito (off por padrão, renovação a cada 6 meses), 4 buckets de distância relativa (nunca geohash/coordenada ao cliente) com jitter espacial ±75m renovado a cada heartbeat, raio de visibilidade configurável (150m-15km, interseção mútua), modo invisível/pausa, bloqueio silencioso, 3 níveis de revelação de identidade, k-anonimato ≥20 em agregados, log de auditoria de acessos (imutável, 180 dias, Trust & Safety only), rate limiting anti-triangulação (1 consulta/par/30s, 200/dia).
- `media-edge-service`/CDN real: ainda fora de escopo (Fatia 3+), mantém `LocalResolver`.

---

## 4. Decisão: ordem de implementação (RESOLVIDO)

**Decisão do usuário: Fatia 1 (base) primeiro — auth, catálogo, biblioteca, reprodução — sem a feature de proximidade. Fatia 2 (proximidade social) só depois, com o modelo de privacidade completo de `security.md` já pronto.**

## 5. Status

Fatia 1 aprovada. Fatia 2 iniciada em `backend/` (Go, presence-service) e `frontend/` (Flutter, `social_proximity_domain/data/ui`), seguida de revisão dedicada de segurança e depois auditoria geral contra os padrões de Spotify/YouTube Music e o critério de parada (100% cobertura ajustada + zero vulnerabilidade crítica).

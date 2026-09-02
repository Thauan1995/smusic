# Arquitetura de Segurança — smusic

Status: **Fase de planejamento.** Este documento define a arquitetura de segurança alvo do produto (autenticação, proteção de dados, infraestrutura, modelagem de ameaças e estratégia de auditoria), com foco especial na decisão de privacidade da feature de descoberta social por proximidade. Não é um guia de implementação — é o contrato de segurança que a implementação (backend Go, apps Dart/Flutter) deve satisfazer.

Escopo do sistema: backend em Go (API, autenticação, streaming, presença), clientes Dart/Flutter (web e mobile), dados de usuários (conta, hábitos de escuta, localização em tempo real).

---

## 1. Decisão central: modelo de privacidade da descoberta por proximidade

Esta é a feature de maior risco do produto: cruza **localização em tempo real** com **hábitos de escuta de terceiros**, duas categorias de dado que, combinadas, permitem inferir rotina, paradeiro e comportamento de uma pessoa identificável — dado pessoal sensível na prática, mesmo que a LGPD não a liste em "dado sensível" do Art. 5º, X (que é uma lista fechada). O risco de abuso concreto é stalking. A proposta abaixo é a decisão de design, não uma lista de opções.

### 1.1 Modelo de consentimento — opt-in explícito, separado, e renovável

- A feature nasce **desligada** para todo usuário, inclusive contas novas. Não há dark pattern nem "ativado por padrão com opção de desligar depois".
- O consentimento é coletado em uma tela dedicada, **fora** do fluxo de aceite de Termos de Uso/Política de Privacidade geral — não pode ser um checkbox genérico enterrado em um bloco de texto. A tela explica em linguagem simples: (a) que a localização aproximada do usuário será processada continuamente enquanto a feature estiver ativa; (b) que a música que ele está ouvindo pode ficar visível a pessoas próximas; (c) o raio de descoberta atual; (d) como pausar/desativar a qualquer momento.
- Base legal: consentimento (LGPD Art. 7º, I e Art. 8º), não legítimo interesse — dado o risco de dano ao titular (stalking), legítimo interesse não é defensável aqui.
- **Re-confirmação periódica**: como é processamento contínuo de dado de localização, o consentimento expira a cada 6 meses e o app exige reconfirmação explícita para continuar ativo. Consentimento silenciosamente "sempre válido" para este tipo de dado é um risco de auditoria da ANPD.
- Revogação: um único toggle "Descoberta por proximidade: desligada" interrompe o processamento imediatamente e remove o usuário do índice de presença ativo (ver 1.5). LGPD Art. 8º §5º — revogação deve ser tão fácil quanto a concessão.
- Consentimento é granular e independente de: (i) compartilhar a música que está ouvindo com seguidores/amigos (feature social separada, pode existir sem geolocalização) e (ii) permissão de localização do SO. As três coisas podem estar ativas em combinações diferentes.

### 1.2 Granularidade de localização — bucket de distância relativo, nunca coordenada ou geohash exposto ao cliente

**Decisão**: o cliente nunca recebe coordenadas, geohash, endereço ou qualquer dado que permita reconstruir a posição absoluta de outro usuário. O servidor recebe as coordenadas precisas (necessário para calcular proximidade corretamente), mas devolve apenas uma **categoria de distância relativa ao usuário que está consultando**:

| Bucket | Distância real | Rótulo exibido |
|---|---|---|
| 1 | < 150 m | "Bem pertinho" |
| 2 | 150 m – 1 km | "No seu bairro" |
| 3 | 1 km – 5 km | "Na sua região" |
| 4 | 5 km – 15 km | "Na sua cidade" |

Sem pino em mapa, sem direção/bearing, sem distância exata em metros. Motivos:

1. **Geohash truncado tem um problema conhecido**: mesmo em precisão baixa (ex. geohash-5, células de ~4,9 km), se o usuário estiver perto da borda de uma célula, consultas repetidas de posições próprias diferentes por um atacante permitem inferir de que lado da borda ele está, reduzindo a incerteza real abaixo do que a célula sugere. Bucket de distância relativa ao consultante não sofre desse problema da mesma forma, porque não ancora a nenhuma grade fixa no mapa.
2. **Distância relativa impede triangulação trivial**: se o atacante só sabe "a vítima está a 150m–1km de mim", ele precisaria de 3+ pontos de observação com controle fino da própria posição para tentar multilateração. Para dificultar isso ainda mais:
   - o servidor aplica **jitter espacial aleatório de ±75 m** às coordenadas antes de calcular o bucket, renovado a cada heartbeat de presença (não é fixo por usuário, então não dá para "calibrar" o erro por observação repetida);
   - **rate limiting por par de usuários**: no máximo 1 consulta de proximidade por par a cada 30 segundos, e no máximo 200 consultas totais por usuário por dia. Isso encarece ataques de triangulação por consulta repetida.
3. Buckets são úteis o suficiente para o produto (saber que "tem gente perto ouvindo isso" é o valor central) sem exigir precisão de metro.

Uso interno de geohash/índice geoespacial (ex. Redis `GEOADD`) para performance de busca é permitido — a restrição é sobre o que **sai para o cliente**, nunca sobre a estrutura de indexação interna.

### 1.3 Raio de descoberta configurável — controle do usuário sobre sua própria visibilidade, não sobre o alcance de quem o observa

- Cada usuário define, em configurações, um **raio de visibilidade** — a distância máxima na qual ele aceita ser descoberto por outros. Slider com passos: 150 m / 1 km / 5 km / 15 km (mesmos limiares dos buckets). Padrão ao ativar a feature: **1 km**.
- Importante: esse raio é interpretado como limite de **quem pode me ver**, não como "até onde eu posso ver os outros". Isso evita que um usuário mal-intencionado configure um raio de 15 km para "varrer" uma cidade inteira à procura de uma pessoa específica: a visibilidade mútua entre A e B só existe se a distância real estiver dentro do raio de visibilidade **de ambos** (interseção, não união).
- Não existe raio "ilimitado" nem opção de desativar o teto de 15 km — é o limite superior do produto, por design.
- Piso mínimo de 150 m (não dá para configurar "só quem está a 5 metros", o que teoricamente ajudaria a identificar alguém em um cômodo específico).

### 1.4 Modo invisível / pausa e bloqueio

- Toggle único e de acesso rápido (tela inicial, não enterrado em configurações): **"Pausar descoberta"**. Efeito imediato: o usuário some do índice de presença ativo (equivalente a nunca ter sido inserido), mas continua ouvindo música normalmente — pausar descoberta não afeta reprodução.
- Pausa automática por inatividade: se o app fica em background por mais de 5 minutos, o heartbeat de presença para de ser enviado e o registro expira naturalmente pelo TTL (ver 1.5) — não é preciso um mecanismo de pausa separado para isso, é consequência do design de TTL curto.
- **Lista de bloqueio**: usuário pode bloquear outro usuário especificamente. Efeito: o bloqueado nunca recebe presença do bloqueador, independentemente de raio/reciprocidade, e o bloqueio é silencioso (o bloqueado não é notificado). Bloqueio é avaliado no momento da consulta (server-side), então tem efeito imediato mesmo sobre presença já "em cache" do lado do bloqueado.

### 1.5 Retenção e TTL — presença é dado efêmero por design, nunca durável

Esta é a decisão de maior impacto em superfície de risco: **o par (usuário, localização, faixa tocando, timestamp) nunca é persistido em armazenamento durável.**

- Estado de presença vive **somente em store efêmero em memória** (ex. Redis), com **TTL de 90 segundos**, renovado a cada heartbeat do cliente (heartbeat recomendado a cada 30–45s). Se o app fecha, perde conexão ou o usuário pausa, o registro simplesmente expira — não há passo de "exclusão" a fazer porque nunca houve escrita em disco.
- **Nenhuma tabela em banco relacional/analítico armazena a tupla bruta "usuário X ouviu música Y no local Z às HH:MM".** Isso deve valer também para pipelines de analytics/BI: o evento de presença não pode ser espelhado para um data warehouse em forma identificável e georreferenciada.
- Se o produto quiser métricas agregadas (ex. "1.500 pessoas ouviram esta faixa em São Paulo hoje"), só podem ser calculadas como contadores agregados, e só publicadas quando o agregado atingir um **limiar de k-anonimato de k ≥ 20** usuários distintos na célula geográfica/tempo — abaixo disso, a área é pequena o suficiente para reidentificar alguém por eliminação. Esse agregado não guarda identidade nem localização fina, só o contador.
- Exceção com retenção mais longa: **logs de auditoria de acesso** (quem consultou a presença de quem — não a presença em si), tratados separadamente em 1.7, porque servem à investigação de abuso.

### 1.6 Anonimização e níveis de revelação

Revelação por padrão é a mínima possível; a identidade completa é a exceção, liberada por proximidade social, não geográfica:

- **Nível 0 (padrão para desconhecidos)**: "Alguém por perto está ouvindo *[Faixa]*" — sem nome, sem avatar, só existência + faixa + bucket de distância.
- **Nível 1 (conexões mútuas / amigos)**: nome de exibição (pode ser um apelido distinto da identidade real usada em outras partes do app) + avatar + bucket de distância.
- **Nível 2 (opt-in explícito "descoberta aberta")**: mostra o Nível 1 também para não-conexões dentro do raio. Precisa de segundo consentimento explícito, separado do consentimento de ativar a feature — ativar a feature não implica aceitar Nível 2.
- Foto/nome real nunca é obrigatório para a camada de proximidade: o app permite um "nome de escuta" pseudônimo diferente do nome usado no perfil social, especificamente para essa feature.

### 1.7 Direitos do titular sob a LGPD aplicados a este dado

- **Acesso** (Art. 18, I e II): usuário pode solicitar, via painel de privacidade self-service, um relatório de: (a) configuração atual de consentimento/raio/nível de revelação e histórico de mudanças; (b) quantas vezes seus dados de presença foram consultados por terceiros e por quem (o próprio log de auditoria, ver 1.8), respeitando que a exposição desse log a ele não pode, por sua vez, virar ferramenta de retaliação contra quem o consultou legitimamente — por isso mostra-se contagem/período, e identidade do consultante só é revelada em investigação formal de abuso.
- **Eliminação** (Art. 18, VI): como a presença em si é efêmera (TTL de 90s, nunca persistida), não há "o que apagar" na maior parte dos casos — isso é um resultado desejado do design (privacy by design elimina o pedido de exclusão pela raiz). O que precisa de rota de exclusão: registros de consentimento, configurações salvas, e agregados aos quais o usuário contribuiu (removê-lo do agregado futuro; agregados já publicados não são individualmente atribuíveis, então não há o que reidentificar/apagar neles).
- **Portabilidade** (Art. 18, V): configurações de privacidade e histórico de escuta "normal" (fora do escopo de presença efêmera) exportáveis em JSON via API self-service.
- **Revogação de consentimento** (Art. 8º §5º): 1 toque, efeito imediato, sem necessidade de justificar, sem fricção (nada de "tem certeza?" repetido três vezes).
- SLA de resposta a solicitações: compromisso interno de **15 dias úteis** (a LGPD não fixa prazo rígido como o GDPR, mas a ANPD exige tempestividade — 15 dias úteis é o SLA que o produto assume publicamente na política de privacidade).

### 1.8 Auditoria de acesso — investigar abuso sem criar uma nova superfície de vigilância

- Toda consulta de presença de um usuário por outro gera um registro de auditoria **append-only, imutável** (armazenamento WORM ou equivalente, separado do banco operacional): `consultante_id, alvo_id (ou área/raio consultado), timestamp, bucket retornado, endpoint`.
- Este log **não é acessível ao usuário consultante nem ao alvo** em uso normal — só à equipe de Trust & Safety/segurança, com acesso privilegiado auditado por sua vez (quem acessou o log de auditoria também é logado — "quis custodiet ipsos custodes").
- Retenção do log de auditoria: **180 dias**. Suficiente para investigar denúncias de stalking que costumam surgir semanas depois do fato; limitado para não virar um dossiê permanente de quem observou quem.
- **Detecção automática de padrão de abuso**: alertas quando (a) um único consultante consulta repetidamente o mesmo alvo em frequência muito acima do padrão normal de uso; (b) múltiplas contas (possivelmente fake/descartáveis) convergem consultas sobre o mesmo alvo de posições diferentes em curto intervalo — assinatura de tentativa de triangulação. Esses alertas alimentam throttling automático e revisão por Trust & Safety.
- Ferramenta de denúncia: usuário reporta abuso → Trust & Safety recebe acesso ao trecho relevante do log de auditoria + pode aplicar bloqueio e suspensão preventiva da conta denunciada enquanto investiga.
- O rate limiting descrito em 1.2 (1 consulta/par/30s, 200/dia) serve tanto como mitigação técnica de triangulação quanto como controle de custo de scraping em massa.

---

## 2. Autenticação e autorização

- **Protocolo**: OAuth2 + OIDC como base, mesmo para login primário first-party (não só para "login social"). Suporte a login social (Google/Apple, exigido de fato para apps mobile na App Store) via OIDC federado, mais login com email/senha próprio.
- **Gestão de sessão/token — decisão: JWT de vida curta + refresh token opaco revogável**, não JWT stateless de vida longa. Justificativa: JWT puro sem estado no servidor não pode ser revogado antes de expirar — inaceitável para os cenários de segurança deste produto (token roubado, logout remoto após denúncia de stalking, banimento de conta durante investigação de abuso exigem revogação imediata, não "esperar expirar"). Modelo adotado:
  - Access token JWT, TTL curto (10–15 min), assinado (RS256/EdDSA), usado nas chamadas de API.
  - Refresh token **opaco**, armazenado hasheado no backend, TTL longo (ex. 30 dias, deslizante), revogável individualmente (logout de um dispositivo) ou em massa (logout de todos os dispositivos, usado em resposta a comprometimento de conta).
  - Toda revogação de refresh token invalida a próxima tentativa de renovação de access token — o pior caso de exposição é a janela do access token já emitido (10–15 min), aceitável.
- **MFA**: TOTP (RFC 6238) obrigatório para habilitar a feature de proximidade e para ações sensíveis (troca de senha/email, exportação de dados, gestão de sessões ativas); opcional, mas fortemente incentivado, para login geral. Sem SMS OTP como segundo fator principal (SIM swap).
- **Hashing de senha**: **Argon2id**, parâmetros mínimos alinhados à recomendação OWASP: memória 19 MiB, iterações = 2, paralelismo = 1, salt único de 16 bytes por usuário; onde a capacidade do servidor permitir, subir para memória 64 MiB / iterações 3 para maior resistência a hardware dedicado. Pepper adicional (segredo de aplicação, fora do banco, gerenciado via Vault/KMS) somado ao hash.
- **Rate limiting e bloqueio de força bruta** em login: backoff progressivo por IP+conta, CAPTCHA após N tentativas, alerta de login em novo dispositivo/localização incomum.

## 3. Proteção de dados

- **Em trânsito**: TLS 1.2 mínimo, TLS 1.3 preferencial, em toda comunicação externa (cliente↔API) e interna entre serviços (mTLS entre serviços do backend Go). HSTS habilitado. Certificate pinning nos apps mobile para o domínio da API.
- **Em repouso**: disco/volume criptografado com chaves gerenciadas por KMS do provedor de nuvem; adicionalmente, **criptografia em nível de campo** para os dados de maior sensibilidade (credenciais, tokens, e qualquer coordenada bruta que trafegue por processamento intermediário antes do bucket ser calculado) usando chaves de envelope via Vault/KMS, rotacionadas periodicamente.
- **Gestão de segredos**: HashiCorp Vault (ou KMS nativo do provedor) para credenciais de banco, chaves de assinatura JWT, chaves de API de terceiros. Nenhum segredo em variável de ambiente estática em produção sem passar por injeção via Vault Agent/sidecar; nenhum segredo em repositório de código (gate de CI com gitleaks/trufflehog, ver seção 5).
- **Tratamento de PII e minimização de dados**: classificação de dados em camadas (público / interno / pessoal / pessoal sensível-de-fato — presença + localização + hábito de escuta entram na última camada). Regra de minimização: cada dado coletado precisa de um consumidor de produto identificado; localização precisa é descartada assim que o bucket é calculado (nunca persistida — ver 1.5); dados de terceiros (contatos, etc.) não são coletados a menos que estritamente necessário e com consentimento próprio.

## 4. Segurança de infraestrutura

- **Segmentação de rede**: VPC com subnets separadas — pública/borda (load balancer, WAF), aplicação (serviços Go), dados (Postgres, Redis) sem rota direta à internet, e uma zona restrita para Vault/KMS. Tráfego entre zonas mediado por regras de firewall/security group explícitas, negar-por-padrão.
- **Menor privilégio**: IAM por serviço (cada serviço Go tem sua própria identidade/role, sem credenciais compartilhadas), acesso a banco de dados por role de aplicação com permissões mínimas (o serviço de presença, por exemplo, não tem permissão de leitura sobre tabelas de dados financeiros/pagamento, e vice-versa). Acesso humano a produção via bastion/SSO com MFA, nunca credencial estática, com sessão temporária e logada.
- **Scanning de dependências/supply chain**: scan automático de módulos Go (`govulncheck`, Snyk/Dependabot) e pacotes Dart/Flutter (`dart pub outdated`, auditoria de licenças/CVE), scan de imagens de contêiner (Trivy) antes de deploy, geração de SBOM por build.
- **Gates de segurança em CI/CD**: pipeline bloqueia merge/deploy se: SAST (gosec/semgrep) encontrar achado Alto/Crítico; secret scanning (gitleaks) encontrar segredo commitado; dependency scan encontrar CVE Crítico sem exceção aprovada; testes de política de infraestrutura (ex. Terraform/OPA) falharem.
- **Gestão de patches**: SLA por severidade — Crítico (CVSS ≥ 9.0): patch em produção em até 24h; Alto (7.0–8.9): até 7 dias; Médio: até 30 dias; janela recorrente de atualização de dependências mesmo sem CVE conhecido (higiene).

## 5. Modelagem de ameaças (STRIDE)

| Risco | S | T | R | I | D | E | Mitigação principal |
|---|---|---|---|---|---|---|---|
| **Account takeover** | Credencial roubada, session fixation | Alteração de token/sessão | Falta de log de login | Exposição de dados da conta comprometida | — | Escalada via conta admin comprometida | MFA, Argon2id, refresh token revogável, alerta de login incomum, rate limit de login |
| **Vazamento/abuso de presença (stalking)** | Falsificar identidade para se passar por "próximo" | Adulterar coordenadas enviadas para forçar bucket | Consulta sem rastro de quem acessou | Exposição de localização/hábito além do consentido | Consulta massiva para exaurir índice de presença | Usuário eleva reciprocidade de raio sem consentimento do alvo | Buckets + jitter (1.2), raio como interseção (1.3), TTL efêmero (1.5), auditoria imutável (1.8), rate limit por par |
| **Scraping/abuso de API** | Conta fake em massa | — | Requisições sem atribuição clara | Extração em massa de perfis/presença | Sobrecarga de endpoints via scraping agressivo | — | Rate limiting por conta/IP, CAPTCHA em padrões automatizados, detecção de anomalia (1.8), paginação com limites |
| **DDoS em endpoints de streaming** | — | — | — | — | Exaustão de banda/CPU do serviço de streaming | — | CDN/edge caching de conteúdo de mídia, WAF com proteção anti-DDoS na borda, autoscaling com limites, rate limiting por sessão |
| **Ataques ao pipeline de upload de conteúdo** | Upload com identidade forjada de direitos autorais | Upload de arquivo malicioso disfarçado de mídia | Falta de trilha de quem enviou o quê | Exposição de metadados sensíveis embutidos no arquivo | Upload de arquivos gigantes para exaurir storage/processamento | Bypass de verificação de direitos autorais | Validação de tipo real de arquivo (magic bytes, não extensão), scanning antivírus/sandboxing na ingestão, limite de tamanho e quota por usuário, fila assíncrona com backpressure, log de autoria do upload |

## 6. Estratégia de teste e auditoria de segurança contínua

- **SAST**: gosec (Go) e semgrep (regras customizadas, incluindo regra específica para bloquear qualquer código que persista coordenadas brutas fora do módulo de presença efêmero) rodando em todo PR.
- **DAST**: OWASP ZAP baseline scan automatizado contra ambiente de staging a cada deploy; scan autenticado completo mensal.
- **Dependency/supply-chain scanning**: contínuo (Dependabot/Snyk + `govulncheck`), não só no CI — alerta assíncrono quando CVE novo afeta dependência já em produção.
- **Pentest externo**: antes do lançamento público, depois anualmente, e sempre que houver mudança arquitetural relevante na feature de proximidade ou no fluxo de autenticação. Escopo obrigatório do pentest inclui explicitamente tentativa de triangulação/deanonimização da feature de presença, não só OWASP Top 10 genérico.
- **Bug bounty**: programa privado (convite) antes do lançamento, evoluindo para público após maturidade do produto; safe harbor explícito para pesquisadores; recompensa maior para achados que deanonimizem localização de terceiro (reflete o risco de negócio real, não só severidade técnica).
- **Definição objetiva de "crítico"** (para a meta de aceite "zero vulnerabilidades críticas identificadas"):
  - Um achado é **Crítico** se: CVSS v3.1 base score ≥ 9.0, **OU** — independente do score CVSS calculado — o achado permite (a) account takeover sem interação da vítima, (b) exposição de localização/identidade de um usuário para outro sem consentimento (viola diretamente o modelo da seção 1), ou (c) execução remota de código/acesso a dados de outros usuários via API. Essa cláusula de override por contexto de negócio existe porque CVSS genérico às vezes subpontua um bypass específico do modelo de privacidade de proximidade (ex. um bypass do jitter/bucket pode pontuar CVSS médio mas é crítico para este produto).
  - **Alto**: CVSS 7.0–8.9 sem se enquadrar nos overrides acima.
- **Verificação objetiva por Auditor externo à equipe de construção**: o critério de aceite "zero vulnerabilidades críticas" só é considerado atendido quando um auditor independente (não envolvido na implementação da feature) confirma, com evidência reproduzível:
  1. Relatório de pentest externo mais recente sem achado aberto classificado como Crítico (pelos critérios acima) há mais de 24h além do SLA de patch;
  2. Logs do pipeline de CI mostrando que os gates de SAST/dependency scan/secret scan estão ativos e bloqueando (não apenas configurados, mas com histórico de execução real nos últimos builds);
  3. Reexecução independente de ao menos um cenário de triangulação/deanonimização da feature de proximidade contra o ambiente de staging, documentando que os buckets + jitter + rate limit se comportam conforme a seção 1.2;
  4. Assinatura formal do auditor em um checklist de release gate, anexado ao registro de deploy — sem essa assinatura, o deploy de produção não é liberado para a feature de proximidade.

---

## 7. Perguntas em aberto para outros especialistas

- **Schema de presença**: confirmar com o dono do schema/backend Go que a tabela/estrutura de presença não terá coluna de latitude/longitude persistida — só campos ephemeral-only em Redis (ou equivalente) com TTL, e um campo explícito `distance_bucket` (enum) como o único dado geográfico que efetivamente trafega para fora do serviço de presença.
- **Tecnologia de indexação geoespacial**: confirmar se o backend usará Redis `GEO*` (TTL nativo, favorece o design de 1.5) ou PostGIS (exigiria job de purga própria) — recomendação deste documento é Redis pela aderência natural ao TTL de 90s.
- **Implementação do jitter espacial**: definir com o time de backend onde exatamente o jitter de ±75m é aplicado (antes ou depois de qualquer cache intermediário) para garantir que nunca exista, em nenhum ponto do sistema, uma coordenada "limpa" acessível a um serviço que não seja o de cálculo de bucket.
- **Schema de consentimento/configuração**: campos mínimos necessários — `proximity_consent_enabled`, `proximity_consent_ts`, `proximity_consent_renew_due`, `visibility_radius_m` (enum 150/1000/5000/15000), `reveal_level` (0/1/2), `paused_bool`, tabela de bloqueios (`blocker_id`, `blocked_id`). Precisa de validação do time de dados sobre onde essa tabela vive e como se relaciona ao restante do perfil.
- **Armazenamento do log de auditoria de acesso (1.8)**: decisão de infraestrutura pendente — store WORM dedicado vs. partição append-only em banco existente com controle de acesso reforçado; também definir quem no time (além de segurança) tem acesso de leitura em investigações.
- **Pipeline de eventos "tocando agora"**: confirmar com quem desenha o pipeline de analytics/BI que o evento de presença não é replicado para o data warehouse em forma identificável — só o agregado k-anonimizado (k≥20) descrito em 1.5 deve chegar lá.
- **Permissão de localização no Flutter (mobile)**: confirmar que a feature usa apenas localização em **foreground** (nunca background/always), tanto por minimização de dado quanto porque simplifica a revisão de permissões sensíveis nas lojas de app — decisão a validar com quem implementa o cliente mobile.
- **Infraestrutura de rate limiting**: confirmar se o gateway de API atual suporta rate limit por par de usuários (não só por IP/conta isolada), necessário para o controle anti-triangulação da seção 1.2.

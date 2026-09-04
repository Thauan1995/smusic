# Deploy do smusic num VPS gratuito (Oracle Cloud Always Free)

Guia passo a passo para colocar o smusic (backend + web) de pé numa
instância `VM.Standard.A1.Flex` (ARM) do tier **Always Free** da Oracle
Cloud — grátis para sempre, sem prazo de expiração — com HTTPS real via
[sslip.io](https://sslip.io) (DNS público gratuito, sem cadastro) + Caddy
(certificado Let's Encrypt automático).

Tudo neste diretório (`Dockerfiles`, `docker-compose.prod.yml`, `Caddyfile`)
já está pronto no repo; os passos abaixo são as ações manuais no console da
Oracle e via SSH que só você pode fazer.

## 1. Criar a VM na Oracle Cloud

1. Crie uma conta em <https://www.oracle.com/cloud/free/> (pede cartão só
   para verificação — o tier Always Free não cobra).
2. Console → **Compute → Instances → Create Instance**.
3. Shape: troque para **`VM.Standard.A1.Flex`** (Ampere/ARM), com **2 OCPUs
   / 12 GB RAM** (a cota Always Free total é 4 OCPU/24GB — usar metade
   deixa margem para outra VM ou para escalar essa mesma depois).
4. Imagem: **Ubuntu 24.04** (ou 22.04).
5. Em "Add SSH keys", cole sua chave pública (ou gere uma nova) — é como
   você vai entrar na VM depois.
6. Crie a instância e aguarde o estado "Running".

## 2. Reservar um IP público fixo

Por padrão a Oracle atribui um IP público efêmero. Reserve um fixo (também
Always Free) para o domínio sslip.io não mudar:

Console → **Networking → IP Management → Reserved Public IPs → Create
Reserved Public IP**, depois associe-o à VNIC da instância (Instance
details → attached VNIC → Edit → troque o IP efêmero pelo reservado).

Anote esse IP — vai virar `SMUSIC_DOMAIN`.

## 3. Abrir as portas 80/443 (duas camadas de firewall na Oracle)

**a) Security List / NSG da VCN** (nível de rede, console):
Console → **Networking → Virtual Cloud Networks → (sua VCN) → Security
Lists → Default Security List → Add Ingress Rules**:
- Source CIDR `0.0.0.0/0`, protocolo TCP, porta destino `80`
- Source CIDR `0.0.0.0/0`, protocolo TCP, porta destino `443`

**b) Firewall do próprio SO** (as imagens Ubuntu da Oracle vêm com
`iptables`/`netfilter-persistent` bloqueando tudo além da porta 22 por
padrão — a Security List sozinha não é suficiente). Via SSH na VM:

```bash
sudo iptables -I INPUT 6 -m state --state NEW -p tcp --dport 80 -j ACCEPT
sudo iptables -I INPUT 6 -m state --state NEW -p tcp --dport 443 -j ACCEPT
sudo netfilter-persistent save
```

## 4. Instalar Docker

Via SSH na VM:

```bash
curl -fsSL https://get.docker.com | sudo sh
sudo usermod -aG docker $USER
newgrp docker   # ou desconecte/reconecte o SSH
docker compose version   # confirma que o plugin compose está disponível
```

## 5. Clonar o repo e configurar o `.env.prod`

```bash
git clone <url-do-seu-repo> smusic_v1
cd smusic_v1
cp deploy/.env.prod.example deploy/.env.prod
```

Edite `deploy/.env.prod`:

- `SMUSIC_DOMAIN` / `SMUSIC_API_BASE_URL` / `CORS_ALLOWED_ORIGINS` /
  `MEDIA_BASE_URL`: troque `xxx-xxx-xxx-xxx` pelo IP reservado do passo 2,
  com `-` no lugar de `.` (ex. IP `152.67.12.34` →
  `152-67-12-34.sslip.io`).
- `POSTGRES_PASSWORD` e a senha dentro de `DATABASE_URL`: uma senha real,
  as duas devem bater.
- `JWT_ED25519_SEED_HEX`, `PASSWORD_PEPPER_HEX`, `MEDIA_SIGNING_KEY`:
  **gere valores reais**, não deixe em branco:
  ```bash
  openssl rand -hex 32   # rode 3x, um valor pra cada uma
  ```
  Isso é crítico: sem um `JWT_ED25519_SEED_HEX` fixo, cada restart do
  container invalida todos os tokens de sessão (mesmo aviso já documentado
  em `backend/README.md`).

## 6. Subir a stack

```bash
docker compose -f deploy/docker-compose.prod.yml --env-file deploy/.env.prod up -d --build postgres redis
```

Espere ficarem saudáveis (`docker compose -f deploy/docker-compose.prod.yml ps`
mostrando `healthy`), depois rode as migrations:

```bash
docker compose -f deploy/docker-compose.prod.yml --env-file deploy/.env.prod run --rm migrate
```

E suba o resto (server, presence-server, caddy — o build do Caddy demora
alguns minutos na primeira vez, ele compila o app Flutter web):

```bash
docker compose -f deploy/docker-compose.prod.yml --env-file deploy/.env.prod up -d --build
```

## 7. Verificar

```bash
curl https://$SMUSIC_DOMAIN/healthz   # deve responder "ok"
```

Abra `https://<seu-dominio>.sslip.io` no navegador: o app web deve carregar
com cadeado válido (certificado Let's Encrypt real, emitido automaticamente
pelo Caddy). Crie uma conta pela própria tela de signup do app para
confirmar que o backend está respondendo de verdade.

## Redeploys futuros

```bash
./deploy/deploy.sh
```

Faz `git pull`, rebuilda as imagens, roda o `migrate` automaticamente só se
o pull trouxe migration nova, sobe os serviços e confere o `/healthz` no
final.

Ou manualmente, os mesmos passos:

```bash
docker compose -f deploy/docker-compose.prod.yml --env-file deploy/.env.prod up -d --build
```

Se o pull trouxe uma migration nova, rode o passo do `migrate` de novo
antes (mesmo comando do passo 6).

## Por que isso "fica de pé" sozinho

Todo serviço tem `restart: unless-stopped`, e o Docker Engine já habilita o
próprio serviço no systemd na instalação — então os containers voltam
sozinhos depois de um reboot da VM, sem depender de nenhum terminal SSH
aberto.

## Fora de escopo deste guia (próximos passos, se quiser)

- **App mobile apontando pra produção**: já aceita
  `--dart-define=SMUSIC_API_BASE_URL=...` igual o web — é só rebuildar o
  APK apontando pro seu domínio sslip.io quando quiser, sem mudar código.
- **Backup do Postgres**: o volume Docker (`smusic_postgres_data`) persiste
  entre restarts/redeploys, mas não há backup fora da VM. Um cron com
  `pg_dump` é o próximo passo natural se os dados importarem de verdade.
- **CI/CD automatizado**: hoje o redeploy é manual via SSH (seção acima).

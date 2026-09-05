# Light Fatura

Portal Light preservado em HTML/CSS/JavaScript, com backend Go, Supabase/PostgreSQL, painel administrativo, PIX, roteamento de gateways e importação CSV.

## Arquitetura

- Frontend público: arquivos originais em `Light/`
- Backend: Go (`cmd/server`), sem dependências externas no `go.mod`
- Banco/Auth: Supabase
- Admin: `/admin` e `/auth`
- Proxy de produção: Nginx
- Processo: systemd
- Deploy: releases atômicas em `/opt/light-fatura/releases`

A base é criada sem clientes e sem faturas. Os registros entram pelo painel administrativo via CSV.

## Supabase

Em um projeto Supabase novo, execute no SQL Editor, nesta ordem:

1. `supabase/schema.sql`
2. `supabase/001_document_unique.sql`
3. `supabase/seed_gateways.sql`

Depois crie o usuário administrativo em Authentication. Pegue o UUID desse usuário e registre a role:

```sql
insert into public.user_roles(user_id, role)
values ('UUID-DO-USUARIO', 'admin')
on conflict (user_id, role) do nothing;
```

Não é necessário expor acesso `anon` às tabelas. O backend consulta o banco com `SUPABASE_SERVICE_ROLE_KEY`; o login usa `SUPABASE_PUBLISHABLE_KEY` e a autorização administrativa é confirmada em `user_roles`.

## Ambiente local

```bash
cp .env.example .env
# preencha SUPABASE_URL, SUPABASE_SERVICE_ROLE_KEY e SUPABASE_PUBLISHABLE_KEY
set -a
source .env
set +a
go run ./cmd/server
```

Abra:

- Portal: `http://localhost:8080/`
- Admin: `http://localhost:8080/admin`
- Health: `http://localhost:8080/healthz`

## CSV

A importação fica em **Admin → Clientes e Faturas → Importar CSV**.

O backend detecta separador `;`, `,` ou TAB. Cabeçalhos aceitos incluem:

```text
cpf;nome;telefone;email;valor_original;valor_desconto;vencimento;status;cte;instalacao;endereco;rua;bairro;cidade;estado;cep;contrato;conta_contrato;mes_ref;parcneg
```

`valor_desconto` é opcional. Quando estiver vazio, o backend grava 70% do `valor_original`, equivalente ao desconto de 30% usado pelo portal Light.

Datas podem ser `YYYY-MM-DD` ou `DD/MM/YYYY`. A importação faz upsert do cliente pelo CPF/documento e atualiza a fatura pendente mais recente ou cria uma nova.

## Gateways

O seed cria os quatro cards previstos no painel:

- CashinPay — adapter nativo
- BlackCat — adapter genérico configurável
- Pixzy — adapter genérico configurável
- Umbrella — adapter genérico configurável

Também existe o adapter `pix-estatico`, disponível ao cadastrar uma gateway com esse adapter e configurar `PIX_CHAVE`.

BlackCat, Pixzy e Umbrella só devem ser considerados totalmente integrados depois que seus contratos oficiais de criação, consulta de status e webhook forem implementados. O adapter genérico permite configurar API URL e secrets, mas não inventa contratos específicos desses provedores.

## Build e deploy

O build gera um bundle contendo **binário + frontend Light**:

```bash
bash scripts/build.sh /tmp/light-fatura-release
```

Em uma VPS Linux com Nginx/systemd:

```bash
sudo bash deploy/bootstrap-vps.sh seu-dominio.com
sudo nano /etc/light-fatura/light-fatura.env
sudo bash deploy/release.sh /tmp/light-fatura-release
```

Para rollback:

```bash
sudo bash deploy/rollback.sh
```

Depois configure HTTPS com o Certbot para o domínio escolhido. O serviço Go escuta em `127.0.0.1:8080` em produção e o Nginx fica na frente.

## Verificação

```bash
go test ./...
go test -race ./internal/payment ./internal/gateway/... ./internal/httpapi ./internal/invoice ./internal/supabase
go vet ./...
bash scripts/build.sh /tmp/light-fatura-release
```

O GitHub Actions executa essas verificações e também confirma a presença dos assets visuais e a ausência do antigo fixture de cliente fake no bundle de produção.

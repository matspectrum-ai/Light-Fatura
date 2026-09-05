create extension if not exists pgcrypto;

create table if not exists public.clientes (
  id uuid primary key default gen_random_uuid(),
  documento text,
  email text,
  nome text not null,
  observacoes text,
  telefone text unique,
  created_at timestamptz not null default now(),
  updated_at timestamptz not null default now()
);
create unique index if not exists clientes_documento_unique on public.clientes(documento) where documento is not null and documento <> '';

create table if not exists public.faturas (
  id uuid primary key default gen_random_uuid(),
  cliente_id uuid not null references public.clientes(id) on delete cascade,
  descricao text,
  referencia text,
  boleto_codigo text,
  boleto_url text,
  pix_copia_cola text,
  pix_txid text,
  pix_valor_centavos integer,
  valor_original numeric(10,2) not null default 0,
  valor_desconto numeric(10,2) not null default 0,
  vencimento date,
  status text not null default 'em_aberto' check (status in ('em_aberto','vencida','em_processamento','paga','expirada','falhou','cancelada')),
  data_pagamento timestamptz,
  created_at timestamptz not null default now(),
  updated_at timestamptz not null default now()
);
create index if not exists faturas_cliente_idx on public.faturas(cliente_id);
create index if not exists faturas_status_idx on public.faturas(status);

create table if not exists public.gateways_config (
  id uuid primary key default gen_random_uuid(),
  slug text not null unique,
  rotulo text not null,
  adapter text not null,
  ativo boolean not null default true,
  prioridade integer not null default 1,
  api_url text,
  ambiente text not null default 'producao',
  limite_diario integer,
  webhook_url text,
  secret_names text[] not null default '{}',
  observacoes text,
  created_at timestamptz not null default now(),
  updated_at timestamptz not null default now()
);

create table if not exists public.roteamento_config (
  id boolean primary key default true check (id = true),
  estrategia text not null default 'prioridade' check (estrategia in ('rodizio','prioridade','fixa')),
  gateway_fixa uuid references public.gateways_config(id) on delete set null,
  ponteiro integer not null default 0,
  novo_pix_por_acesso boolean not null default false,
  created_at timestamptz not null default now(),
  updated_at timestamptz not null default now()
);
insert into public.roteamento_config(id) values(true) on conflict(id) do nothing;

create table if not exists public.transacoes_pix (
  id uuid primary key default gen_random_uuid(),
  fatura_id uuid not null references public.faturas(id) on delete cascade,
  cliente_id uuid not null references public.clientes(id) on delete cascade,
  gateway_slug text not null,
  gateway_id uuid references public.gateways_config(id) on delete set null,
  transacao_gateway_id text,
  valor_centavos integer not null,
  valor_pago_centavos integer,
  copia_cola text,
  qrcode text,
  status text not null default 'pendente' check (status in ('pendente','pago','expirada','substituida','falhou')),
  idempotency_key text not null unique,
  expira_em timestamptz,
  substituida_em timestamptz,
  pago_em timestamptz,
  webhook_id text,
  created_at timestamptz not null default now(),
  updated_at timestamptz not null default now()
);
create index if not exists transacoes_pix_fatura_idx on public.transacoes_pix(fatura_id, created_at desc);

create table if not exists public.pix_generation_requests (
  id uuid primary key default gen_random_uuid(),
  request_key text not null unique,
  fatura_id uuid not null references public.faturas(id) on delete cascade,
  status text not null default 'processando' check (status in ('processando','concluida','falhou')),
  transacao_id uuid references public.transacoes_pix(id) on delete set null,
  erro text,
  created_at timestamptz not null default now(),
  updated_at timestamptz not null default now()
);

create table if not exists public.pagamentos (
  id uuid primary key default gen_random_uuid(),
  fatura_id uuid not null references public.faturas(id) on delete cascade,
  cliente_id uuid not null references public.clientes(id) on delete cascade,
  valor numeric(10,2) not null,
  metodo text not null default 'pix' check (metodo in ('pix','manual')),
  status text not null default 'pendente' check (status in ('pendente','confirmado')),
  gateway text,
  gateway_payment_id text,
  pago_em timestamptz,
  created_at timestamptz not null default now(),
  updated_at timestamptz not null default now()
);

create table if not exists public.pagamentos_log (
  id uuid primary key default gen_random_uuid(),
  gateway_slug text,
  fatura_id uuid references public.faturas(id) on delete set null,
  nivel text not null default 'info' check (nivel in ('info','aviso','erro')),
  http_status integer,
  mensagem text,
  created_at timestamptz not null default now()
);

create table if not exists public.acessos (
  id uuid primary key default gen_random_uuid(),
  data_hora timestamptz not null default now(),
  pagina text,
  telefone_consultado text,
  sucesso boolean not null default false,
  valor_original numeric(10,2),
  valor_desconto numeric(10,2),
  created_at timestamptz not null default now()
);

create table if not exists public.webhooks_log (
  id uuid primary key default gen_random_uuid(),
  gateway_slug text,
  payload jsonb,
  headers jsonb,
  valido boolean,
  transacao_gateway_id text,
  status text,
  created_at timestamptz not null default now()
);

create table if not exists public.user_roles (
  id uuid primary key default gen_random_uuid(),
  user_id uuid not null,
  role text not null check (role in ('admin','user')),
  created_at timestamptz not null default now(),
  unique(user_id, role)
);

create or replace function public.set_updated_at() returns trigger language plpgsql as $$
begin new.updated_at = now(); return new; end $$;

do $$ declare t text; begin
  foreach t in array array['clientes','faturas','gateways_config','roteamento_config','transacoes_pix','pix_generation_requests','pagamentos'] loop
    execute format('drop trigger if exists set_updated_at on public.%I', t);
    execute format('create trigger set_updated_at before update on public.%I for each row execute function public.set_updated_at()', t);
  end loop;
end $$;

create or replace function public.avancar_ponteiro_gateway(p_total int) returns int as $$
declare n int;
begin
  if p_total <= 0 then return 0; end if;
  update public.roteamento_config
     set ponteiro = (ponteiro + 1) % p_total
   where id = true
   returning ponteiro into n;
  return coalesce(n,0);
end;
$$ language plpgsql security definer;

alter table public.clientes enable row level security;
alter table public.faturas enable row level security;
alter table public.gateways_config enable row level security;
alter table public.roteamento_config enable row level security;
alter table public.transacoes_pix enable row level security;
alter table public.pix_generation_requests enable row level security;
alter table public.pagamentos enable row level security;
alter table public.pagamentos_log enable row level security;
alter table public.acessos enable row level security;
alter table public.webhooks_log enable row level security;
alter table public.user_roles enable row level security;

-- O backend usa service_role, que ignora RLS. Nenhum acesso público direto às tabelas é necessário.

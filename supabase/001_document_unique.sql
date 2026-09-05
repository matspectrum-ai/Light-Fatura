drop index if exists public.clientes_documento_unique;
alter table public.clientes drop constraint if exists clientes_documento_key;
alter table public.clientes add constraint clientes_documento_key unique(documento);

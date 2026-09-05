insert into public.gateways_config(slug,rotulo,adapter,ativo,prioridade,ambiente,secret_names,observacoes)
values
  ('cashinpay','CashinPay','cashinpay',true,1,'producao',array['CASHINPAY_SECRET_KEY'],'Adapter CashinPay nativo'),
  ('blackcat','BlackCat','generico',true,2,'producao',array['BLACKCAT_API_KEY','BLACKCAT_PK_LIVE'],'Configure api_url no painel'),
  ('pixzy','Pixzy','generico',true,3,'producao',array['PIXZY_TOKEN'],'Configure api_url no painel'),
  ('umbrella','Umbrella','generico',true,4,'producao',array['UMBRELLA_API_KEY'],'Configure api_url no painel')
on conflict(slug) do update set
  rotulo=excluded.rotulo,
  adapter=excluded.adapter,
  prioridade=excluded.prioridade,
  ambiente=excluded.ambiente,
  secret_names=excluded.secret_names;

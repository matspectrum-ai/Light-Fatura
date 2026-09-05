(() => {
  'use strict';
  const $ = s => document.querySelector(s);
  const cards = ['login','forgot','reset'];
  const show = name => cards.forEach(v => { const el=$('#'+v+'-card'); if(el) el.hidden=v!==name; });
  const message = (id,text,ok=false) => { const el=$(id); if(!el)return; el.textContent=text; el.hidden=false; el.classList.toggle('ok',ok); };
  const post = async (url,body) => { const r=await fetch(url,{method:'POST',headers:{'Content-Type':'application/json'},body:JSON.stringify(body)}); let data={};try{data=await r.json()}catch{};if(!r.ok)throw new Error(data.erro||'Não foi possível concluir.');return data; };
  const path=location.pathname;
  if(path==='/forgot-password') show('forgot');
  else if(path==='/reset-password') show('reset');
  else show('login');

  $('#forgot-link')?.addEventListener('click',()=>show('forgot'));
  document.querySelectorAll('.back-login').forEach(b=>b.addEventListener('click',()=>show('login')));
  $('#login-form')?.addEventListener('submit',async e=>{e.preventDefault();const f=new FormData(e.currentTarget);const b=e.currentTarget.querySelector('button');b.disabled=true;try{await post('/api/auth/login',{email:f.get('email'),senha:f.get('senha')});location.href='/admin';}catch(err){message('#login-message',err.message)}finally{b.disabled=false}});
  $('#forgot-form')?.addEventListener('submit',async e=>{e.preventDefault();const f=new FormData(e.currentTarget);const b=e.currentTarget.querySelector('button');b.disabled=true;try{await post('/api/auth/recover',{email:f.get('email')});message('#forgot-message','Link de recuperação enviado.',true)}catch(err){message('#forgot-message',err.message)}finally{b.disabled=false}});

  async function establishRecoverySession(){if(path!=='/reset-password')return;const p=new URLSearchParams(location.hash.replace(/^#/,''));const access=p.get('access_token'),refresh=p.get('refresh_token');if(!access||!refresh){message('#reset-message','Link de recuperação inválido ou expirado.');return}try{await post('/api/auth/recovery-session',{access_token:access,refresh_token:refresh,expires_in:Number(p.get('expires_in')||3600)});history.replaceState(null,'',location.pathname);}catch(err){message('#reset-message',err.message)}}
  establishRecoverySession();
  $('#reset-form')?.addEventListener('submit',async e=>{e.preventDefault();const f=new FormData(e.currentTarget);const b=e.currentTarget.querySelector('button');b.disabled=true;try{await post('/api/auth/password',{senha:f.get('senha')});message('#reset-message','Senha atualizada. Redirecionando...',true);setTimeout(()=>location.href='/admin',900)}catch(err){message('#reset-message',err.message)}finally{b.disabled=false}});
})();

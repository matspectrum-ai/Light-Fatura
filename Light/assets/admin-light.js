(() => {
  'use strict';

  const $ = selector => document.querySelector(selector);
  const $$ = selector => [...document.querySelectorAll(selector)];
  const state = { view: 'dashboard', page: 0, total: 0, invoices: [], gateways: [] };
  const titles = {
    dashboard: 'Dashboard',
    faturas: 'Clientes e Faturas',
    pagamentos: 'Pagamentos',
    transacoes: 'Transações PIX',
    gateways: 'Gateways',
    logs: 'Logs',
  };
  const money = value => Number(value || 0).toLocaleString('pt-BR', { style: 'currency', currency: 'BRL' });
  const date = value => value ? new Date(value.length === 10 ? value + 'T12:00:00' : value).toLocaleString('pt-BR') : '—';
  const esc = value => String(value ?? '').replace(/[&<>"']/g, c => ({ '&': '&amp;', '<': '&lt;', '>': '&gt;', '"': '&quot;', "'": '&#39;' }[c]));

  function toast(text, error = false) {
    const el = $('#toast');
    el.textContent = text;
    el.classList.toggle('error', error);
    el.classList.add('show');
    setTimeout(() => el.classList.remove('show'), 2800);
  }

  async function api(url, options = {}) {
    const response = await fetch(url, { cache: 'no-store', ...options });
    if (response.status === 401) {
      location.href = '/auth';
      throw new Error('Sessão expirada');
    }
    let data = {};
    try { data = await response.json(); } catch (_) {}
    if (!response.ok) throw new Error(data.erro || 'Falha na operação');
    return data;
  }

  async function init() {
    try {
      const me = await api('/api/auth/me');
      $('#admin-email').textContent = me.email || '';
    } catch (_) {
      return;
    }
    bind();
    routeFromPath();
    await loadView(state.view);
  }

  function bind() {
    $$('#nav button').forEach(button => button.onclick = () => switchView(button.dataset.view));
    $('#logout').onclick = async () => { await api('/api/auth/logout', { method: 'POST' }); location.href = '/auth'; };
    $('#clear-metrics-btn').onclick = clearMetrics;
    $('#invoice-search-btn').onclick = () => { state.page = 0; loadInvoices(); };
    $('#invoice-search').onkeydown = event => {
      if (event.key === 'Enter') { event.preventDefault(); state.page = 0; loadInvoices(); }
    };
    $('#prev-page').onclick = () => { if (state.page > 0) { state.page--; loadInvoices(); } };
    $('#next-page').onclick = () => { if ((state.page + 1) * 50 < state.total) { state.page++; loadInvoices(); } };
    $('#import-btn').onclick = () => $('#import-dialog').showModal();
    $('#delete-all-btn').onclick = deleteAll;
    $('#new-gateway-btn').onclick = openNewGateway;
    $('#delete-gateway-btn').onclick = deleteCurrentGateway;
    $$('[data-close]').forEach(button => button.onclick = () => button.closest('dialog').close());
    $('#edit-form').onsubmit = saveInvoice;
    $('#import-form').onsubmit = importCSV;
    $('#gateway-form').onsubmit = saveGateway;
    $('#save-routing').onclick = saveRouting;
    $$('[data-refresh]').forEach(button => button.onclick = () => loadView(button.dataset.refresh));
    window.addEventListener('popstate', () => { routeFromPath(); loadView(state.view); });
  }

  function routeFromPath() {
    const parts = location.pathname.split('/').filter(Boolean);
    const view = parts[1] || 'dashboard';
    state.view = titles[view] ? view : 'dashboard';
    paintView();
  }

  function switchView(view) {
    state.view = view;
    const url = view === 'dashboard' ? '/admin' : '/admin/' + view;
    history.pushState({}, '', url);
    paintView();
    loadView(view);
  }

  function paintView() {
    $$('.view').forEach(view => view.hidden = view.id !== 'view-' + state.view);
    $$('#nav button').forEach(button => button.classList.toggle('active', button.dataset.view === state.view));
    $('#page-title').textContent = titles[state.view] || 'Admin';
  }

  async function loadView(view) {
    try {
      if (view === 'dashboard') await loadMetrics();
      if (view === 'faturas') await loadInvoices();
      if (view === 'pagamentos') await loadPayments();
      if (view === 'transacoes') await loadTransactions();
      if (view === 'gateways') await loadGateways();
      if (view === 'logs') await loadLogs();
    } catch (error) {
      toast(error.message, true);
    }
  }

  async function loadMetrics() {
    const metrics = await api('/api/admin/metricas');
    $('#m-clients').textContent = metrics.clientes_total ?? 0;
    $('#m-invoices').textContent = metrics.faturas_total ?? 0;
    $('#m-payments').textContent = metrics.pagamentos_total ?? 0;
    $('#m-views').textContent = metrics.faturas_visualizadas_total ?? 0;
    $('#m-value').textContent = money(metrics.valor_visualizado_total);
  }

  async function clearMetrics() {
    if (!confirm('Limpar somente o histórico de consultas? Clientes, faturas e pagamentos serão preservados.')) return;
    try {
      await api('/api/admin/metricas', { method: 'DELETE' });
      toast('Histórico de consultas limpo');
      loadMetrics();
    } catch (error) {
      toast(error.message, true);
    }
  }

  async function loadInvoices() {
    const query = new URLSearchParams({ pagina: String(state.page), busca: $('#invoice-search')?.value || '' });
    const data = await api('/api/admin/faturas?' + query);
    state.invoices = data.linhas || [];
    state.total = data.total || 0;
    $('#invoice-total').textContent = state.total + ' registro(s)';
    $('#page-label').textContent = 'Página ' + (state.page + 1);
    $('#invoice-body').innerHTML = state.invoices.length ? state.invoices.map((row, index) => `
      <tr>
        <td><strong>${esc(row.cliente?.documento || '')}</strong></td>
        <td>${esc(row.cliente?.nome || '')}<div class="muted">${esc(row.cliente?.telefone || '')}</div></td>
        <td>${esc(row.vencimento || '—')}</td>
        <td>${money(row.valor_original)}</td>
        <td><strong>${money(row.valor_desconto)}</strong></td>
        <td><select class="field status-select" data-i="${index}">${['em_aberto','vencida','em_processamento','paga','expirada','falhou','cancelada'].map(status => `<option ${status === row.status ? 'selected' : ''}>${status}</option>`).join('')}</select></td>
        <td><button class="button small edit-invoice" data-i="${index}">Editar</button></td>
      </tr>`).join('') : '<tr><td colspan="7" class="empty">Nenhum cliente/fatura cadastrado.</td></tr>';
    $$('.edit-invoice').forEach(button => button.onclick = () => openInvoice(Number(button.dataset.i)));
    $$('.status-select').forEach(select => select.onchange = () => changeStatus(Number(select.dataset.i), select.value));
  }

  async function changeStatus(index, status) {
    const row = state.invoices[index];
    try {
      await api('/api/admin/faturas/' + row.id + '/status', {
        method: 'PATCH', headers: { 'Content-Type': 'application/json' }, body: JSON.stringify({ status }),
      });
      row.status = status;
      toast('Status atualizado');
    } catch (error) {
      toast(error.message, true);
      loadInvoices();
    }
  }

  function openInvoice(index) {
    const row = state.invoices[index];
    const form = $('#edit-form');
    form.id.value = row.id;
    form.cliente_id.value = row.cliente_id;
    form.nome.value = row.cliente?.nome || '';
    form.documento.value = row.cliente?.documento || '';
    form.telefone.value = row.cliente?.telefone || '';
    form.email.value = row.cliente?.email || '';
    form.valor_original.value = row.valor_original;
    form.valor_desconto.value = row.valor_desconto;
    form.vencimento.value = row.vencimento || '';
    form.status.value = row.status;
    form.descricao.value = row.descricao || '';
    form.referencia.value = row.referencia || '';
    $('#edit-dialog').showModal();
  }

  async function saveInvoice(event) {
    event.preventDefault();
    const form = new FormData(event.currentTarget);
    const body = {
      id: form.get('id'), cliente_id: form.get('cliente_id'), descricao: form.get('descricao'), referencia: form.get('referencia'),
      valor_original: Number(form.get('valor_original')), valor_desconto: Number(form.get('valor_desconto')),
      vencimento: form.get('vencimento'), status: form.get('status'),
      cliente: { id: form.get('cliente_id'), nome: form.get('nome'), documento: form.get('documento'), telefone: form.get('telefone') || null, email: form.get('email') || null },
    };
    try {
      await api('/api/admin/faturas/' + body.id, { method: 'PATCH', headers: { 'Content-Type': 'application/json' }, body: JSON.stringify(body) });
      $('#edit-dialog').close();
      toast('Fatura atualizada');
      loadInvoices();
    } catch (error) {
      toast(error.message, true);
    }
  }

  async function importCSV(event) {
    event.preventDefault();
    const form = event.currentTarget;
    const button = form.querySelector('button[type=submit]');
    const message = $('#import-result');
    button.disabled = true;
    message.hidden = true;
    try {
      const result = await api('/api/admin/importar-csv', { method: 'POST', body: new FormData(form) });
      message.textContent = `Importados: ${result.importados || 0} · Faturas criadas: ${result.faturasCriadas || 0} · Atualizadas: ${result.faturasAtualizadas || 0} · Rejeitados: ${(result.rejeitados || []).length}`;
      message.classList.add('ok');
      message.hidden = false;
      toast('CSV importado');
      state.page = 0;
      loadInvoices();
      loadMetrics();
    } catch (error) {
      message.textContent = error.message;
      message.classList.remove('ok');
      message.hidden = false;
    } finally {
      button.disabled = false;
    }
  }

  async function deleteAll() {
    const value = prompt('Esta ação apaga clientes, faturas, pagamentos e histórico. Digite APAGAR para confirmar.');
    if (value !== 'APAGAR') return;
    try {
      await api('/api/admin/base', { method: 'DELETE', headers: { 'Content-Type': 'application/json' }, body: JSON.stringify({ confirmacao: 'APAGAR' }) });
      toast('Base limpa');
      state.page = 0;
      loadInvoices();
      loadMetrics();
    } catch (error) {
      toast(error.message, true);
    }
  }

  async function loadPayments() {
    const rows = await api('/api/admin/pagamentos');
    $('#payments-body').innerHTML = rows.length ? rows.map(row => `<tr><td>${date(row.created_at)}</td><td>${esc((row.fatura_id || '').slice(0,8))}</td><td><strong>${money(row.valor)}</strong></td><td>${esc(row.metodo)}</td><td>${esc(row.gateway || '—')}</td><td><span class="badge ${esc(row.status)}">${esc(row.status)}</span></td></tr>`).join('') : '<tr><td colspan="6" class="empty">Nenhum pagamento.</td></tr>';
  }

  async function loadTransactions() {
    const rows = await api('/api/admin/transacoes');
    $('#transactions-body').innerHTML = rows.length ? rows.map(row => `<tr><td>${date(row.created_at)}</td><td>${esc(row.gateway_slug)}</td><td>${esc(row.transacao_gateway_id || '—')}</td><td>${money((row.valor_centavos || 0) / 100)}</td><td>${row.valor_pago_centavos == null ? '—' : money(row.valor_pago_centavos / 100)}</td><td><span class="badge ${esc(row.status)}">${esc(row.status)}</span></td></tr>`).join('') : '<tr><td colspan="6" class="empty">Nenhuma transação PIX.</td></tr>';
  }

  async function loadLogs() {
    const rows = await api('/api/admin/logs');
    $('#logs-body').innerHTML = rows.length ? rows.map(row => `<tr><td>${date(row.created_at)}</td><td>${esc(row.gateway_slug || '—')}</td><td><span class="badge">${esc(row.nivel)}</span></td><td>${row.http_status ?? '—'}</td><td>${esc(row.mensagem || '')}</td></tr>`).join('') : '<tr><td colspan="5" class="empty">Nenhum log.</td></tr>';
  }

  async function loadGateways() {
    const [gateways, routing, webhookRows] = await Promise.all([
      api('/api/admin/gateways'),
      api('/api/admin/roteamento'),
      api('/api/admin/gateways/webhooks-resumo'),
    ]);
    state.gateways = gateways;
    const webhookMap = Object.fromEntries((webhookRows || []).map(row => [row.gateway_slug, row]));
    $('#routing-strategy').value = routing.estrategia || 'prioridade';
    $('#routing-new').value = String(!!routing.novo_pix_por_acesso);
    $('#routing-fixed').innerHTML = '<option value="">Selecione</option>' + gateways.map(gateway => `<option value="${gateway.id}" ${routing.gateway_fixa === gateway.id ? 'selected' : ''}>${esc(gateway.rotulo)}</option>`).join('');
    $('#gateway-grid').innerHTML = gateways.length ? gateways.map((gateway, index) => {
      const summary = webhookMap[gateway.slug] || { total: 0, validos: 0, invalidos: 0, ultimo_em: null };
      const webhookURL = gateway.webhook_url || (location.origin + '/api/public/webhooks/' + gateway.slug);
      return `<article class="gateway-card">
        <div class="gateway-title"><div><h3>${esc(gateway.rotulo)}</h3><span class="muted">${esc(gateway.slug)}</span></div><span class="badge ${gateway.ativo ? 'confirmado' : ''}">${gateway.ativo ? 'Ativa' : 'Inativa'}</span></div>
        <div class="gateway-meta">
          <div>Prioridade: <strong>${gateway.prioridade}</strong></div>
          <div>Adapter: <strong>${esc(gateway.adapter)}</strong></div>
          <div>Ambiente: <strong>${esc(gateway.ambiente || 'producao')}</strong></div>
          <div>Credenciais: <strong>${gateway.configurado ? 'configuradas' : 'pendentes'}</strong></div>
          <div>API: <span>${esc(gateway.api_url || 'não configurada')}</span></div>
          <div>Webhook: <span class="webhook-url">${esc(webhookURL)}</span></div>
          <div>Webhooks recebidos: <strong>${summary.total || 0}</strong> · válidos ${summary.validos || 0} · inválidos ${summary.invalidos || 0}</div>
          <div>Último webhook: <span>${summary.ultimo_em ? date(summary.ultimo_em) : '—'}</span></div>
        </div>
        <div class="gateway-actions">
          <button class="button small gateway-edit" data-i="${index}">Editar</button>
          <button class="button small gateway-copy" data-url="${esc(webhookURL)}">Copiar webhook</button>
          <button class="button small gateway-only" data-id="${gateway.id}">Usar somente esta</button>
        </div>
      </article>`;
    }).join('') : '<div class="panel empty">Nenhuma gateway cadastrada.</div>';
    $$('.gateway-edit').forEach(button => button.onclick = () => openGateway(Number(button.dataset.i)));
    $$('.gateway-only').forEach(button => button.onclick = () => useOnly(button.dataset.id));
    $$('.gateway-copy').forEach(button => button.onclick = async () => {
      await navigator.clipboard.writeText(button.dataset.url || '');
      toast('Webhook copiado');
    });
  }

  function openGateway(index) {
    const gateway = state.gateways[index];
    const form = $('#gateway-form');
    form.reset();
    $('#gateway-dialog-title').textContent = 'Editar gateway';
    form.id.value = gateway.id;
    form.rotulo.value = gateway.rotulo;
    form.slug.value = gateway.slug;
    form.slug.readOnly = true;
    form.prioridade.value = gateway.prioridade;
    form.adapter.value = gateway.adapter;
    form.ambiente.value = gateway.ambiente || 'producao';
    form.api_url.value = gateway.api_url || '';
    form.webhook_url.value = gateway.webhook_url || '';
    form.limite_diario.value = gateway.limite_diario ?? '';
    form.secret_names.value = (gateway.secret_names || []).join(',');
    form.observacoes.value = gateway.observacoes || '';
    form.ativo.checked = !!gateway.ativo;
    $('#delete-gateway-btn').hidden = false;
    $('#gateway-dialog').showModal();
  }

  function openNewGateway() {
    const form = $('#gateway-form');
    form.reset();
    $('#gateway-dialog-title').textContent = 'Adicionar gateway';
    form.id.value = '';
    form.slug.readOnly = false;
    form.adapter.value = 'generico';
    form.ambiente.value = 'producao';
    form.prioridade.value = String(Math.max(1, state.gateways.length + 1));
    form.ativo.checked = false;
    $('#delete-gateway-btn').hidden = true;
    $('#gateway-dialog').showModal();
  }

  async function saveGateway(event) {
    event.preventDefault();
    const form = event.currentTarget;
    const data = new FormData(form);
    const id = data.get('id');
    const body = {
      id,
      slug: data.get('slug'),
      rotulo: data.get('rotulo'),
      adapter: data.get('adapter'),
      ativo: form.ativo.checked,
      prioridade: Number(data.get('prioridade')),
      api_url: data.get('api_url') || null,
      ambiente: data.get('ambiente') || 'producao',
      limite_diario: data.get('limite_diario') ? Number(data.get('limite_diario')) : null,
      webhook_url: data.get('webhook_url') || null,
      secret_names: String(data.get('secret_names') || '').split(',').map(value => value.trim()).filter(Boolean),
      observacoes: data.get('observacoes') || null,
    };
    const url = id ? '/api/admin/gateways/' + id : '/api/admin/gateways';
    const method = id ? 'PATCH' : 'POST';
    try {
      await api(url, { method, headers: { 'Content-Type': 'application/json' }, body: JSON.stringify(body) });
      $('#gateway-dialog').close();
      toast(id ? 'Gateway atualizada' : 'Gateway criada');
      loadGateways();
    } catch (error) {
      toast(error.message, true);
    }
  }

  async function deleteCurrentGateway() {
    const id = $('#gateway-form').id.value;
    if (!id || !confirm('Excluir esta gateway?')) return;
    try {
      await api('/api/admin/gateways/' + id, { method: 'DELETE' });
      $('#gateway-dialog').close();
      toast('Gateway excluída');
      loadGateways();
    } catch (error) {
      toast(error.message, true);
    }
  }

  async function useOnly(id) {
    try {
      await api('/api/admin/gateways/' + id + '/somente', { method: 'POST' });
      toast('Roteamento atualizado');
      loadGateways();
    } catch (error) {
      toast(error.message, true);
    }
  }

  async function saveRouting() {
    const fixed = $('#routing-fixed').value || null;
    const body = {
      estrategia: $('#routing-strategy').value,
      gateway_fixa: fixed,
      novo_pix_por_acesso: $('#routing-new').value === 'true',
    };
    try {
      await api('/api/admin/roteamento', { method: 'POST', headers: { 'Content-Type': 'application/json' }, body: JSON.stringify(body) });
      toast('Roteamento salvo');
      loadGateways();
    } catch (error) {
      toast(error.message, true);
    }
  }

  init();
})();

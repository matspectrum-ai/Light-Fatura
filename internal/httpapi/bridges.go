package httpapi

const lightAPIBridge = `(() => {
  'use strict';
  window.buscarUsuarioPorCodigo = async function(codigo) {
    const documento = String(codigo || '').replace(/\D/g, '');
    if (documento.length !== 11) return null;
    const response = await fetch('/api/v1/faturas?documento=' + encodeURIComponent(documento), {
      headers: { 'Accept': 'application/json' }, cache: 'no-store'
    });
    if (!response.ok) return null;
    const result = await response.json();
    if (!result.encontrado || !result.usuario || !result.fatura_id) return null;
    const user = result.usuario;
    localStorage.setItem('light_fatura_id', result.fatura_id);
    localStorage.setItem('light_fatura_status', result.status || 'em_aberto');
    localStorage.setItem('light_valor_desconto', String(result.valor_desconto || ''));
    localStorage.setItem('light_vencimento', result.vencimento || '');
    return user;
  };
})();`

const lightPaymentBridge = `(() => {
  'use strict';
  function uuid() {
    if (crypto && crypto.randomUUID) return crypto.randomUUID();
    return 'xxxxxxxx-xxxx-4xxx-yxxx-xxxxxxxxxxxx'.replace(/[xy]/g, c => {
      const r = Math.random() * 16 | 0, v = c === 'x' ? r : (r & 3 | 8); return v.toString(16);
    });
  }
  function escapeHTML(value) {
    return String(value || '').replace(/[&<>"']/g, c => ({'&':'&amp;','<':'&lt;','>':'&gt;','"':'&quot;',"'":'&#39;'}[c]));
  }
  function pixBox(result) {
    const card = document.querySelector('.card');
    let box = document.getElementById('pix-real');
    if (!box) {
      box = document.createElement('div'); box.id = 'pix-real';
      box.style.cssText = 'margin-top:16px;border:1px solid #d1fae5;background:#ecfdf5;border-radius:12px;padding:14px;text-align:center';
      card.appendChild(box);
    }
    box.innerHTML = '<div style="font-weight:800;color:#057a75;margin-bottom:8px">PIX gerado</div>' +
      '<div style="font-size:12px;color:#475569;margin-bottom:10px">Copie o código abaixo e pague no aplicativo do seu banco.</div>' +
      '<div id="pix-code" style="font-size:11px;line-height:1.45;word-break:break-all;background:white;border-radius:8px;padding:10px;border:1px solid #d1d5db">' + escapeHTML(result.copia_cola) + '</div>' +
      '<button id="copy-pix" style="margin-top:10px;width:100%;padding:12px;border:0;border-radius:10px;background:#06a09a;color:white;font-weight:800;cursor:pointer">Copiar código PIX</button>' +
      '<div id="pix-status" style="margin-top:10px;font-size:12px;color:#64748b">Aguardando pagamento...</div>';
    document.getElementById('copy-pix').onclick = async () => {
      await navigator.clipboard.writeText(result.copia_cola);
      document.getElementById('copy-pix').textContent = 'Código copiado';
    };
    return box;
  }
  async function poll(invoiceID) {
    const status = document.getElementById('pix-status');
    if (!status) return;
    try {
      const response = await fetch('/api/v1/faturas/' + encodeURIComponent(invoiceID) + '/status', {method:'POST',headers:{'Content-Type':'application/json'},body:'{}',cache:'no-store'});
      if (response.ok) {
        const data = await response.json();
        if (data.status === 'paga') { status.textContent = 'Pagamento confirmado.'; status.style.color = '#15803d'; status.style.fontWeight = '800'; return; }
      }
    } catch (_) {}
    setTimeout(() => poll(invoiceID), 5000);
  }
  window.addEventListener('DOMContentLoaded', () => {
    const button = document.querySelector('.btn');
    if (!button) return;
    button.onclick = null;
    button.addEventListener('click', async event => {
      event.preventDefault();
      const invoiceID = localStorage.getItem('light_fatura_id');
      if (!invoiceID) { alert('Fatura não localizada. Volte e consulte o CPF novamente.'); return; }
      button.disabled = true; button.textContent = 'Gerando PIX...';
      try {
        const response = await fetch('/api/v1/faturas/' + encodeURIComponent(invoiceID) + '/pix', {
          method:'POST',headers:{'Content-Type':'application/json'},body:JSON.stringify({request_key:uuid()})
        });
        const result = await response.json();
        if (!response.ok || !result.disponivel || !result.copia_cola) throw new Error(result.erro || result.mensagem || 'PIX indisponível');
        pixBox(result); poll(invoiceID); button.style.display = 'none';
      } catch (error) {
        alert(error.message || 'Não foi possível gerar o PIX.'); button.disabled = false; button.textContent = 'Pagar com PIX';
      }
    });
  });
})();`

package supabase

import (
	"context"
	"encoding/json"
	"fmt"
	"net/url"
	"os"
	"strings"
	"time"

	"github.com/matspectrum-ai/Claro-Fatura/internal/admin"
)

func (c *Client) Dashboard(ctx context.Context) (admin.Dashboard, error) {
	clients, err := c.Count(ctx, "clientes", nil)
	if err != nil {
		return admin.Dashboard{}, err
	}
	invoices, err := c.Count(ctx, "faturas", nil)
	if err != nil {
		return admin.Dashboard{}, err
	}
	payments, err := c.Count(ctx, "pagamentos", url.Values{"status": {"eq.confirmado"}})
	if err != nil {
		return admin.Dashboard{}, err
	}
	views, err := c.Count(ctx, "acessos", url.Values{"sucesso": {"eq.true"}})
	if err != nil {
		return admin.Dashboard{}, err
	}
	var accesses []struct {
		Value *float64 `json:"valor_desconto"`
	}
	_ = c.Select(ctx, "acessos", url.Values{"select": {"valor_desconto"}, "sucesso": {"eq.true"}, "limit": {"50000"}}, &accesses)
	total := 0.0
	for _, a := range accesses {
		if a.Value != nil {
			total += *a.Value
		}
	}
	return admin.Dashboard{Clients: clients, Invoices: invoices, Payments: payments, Views: views, ViewedAmount: total}, nil
}

func (c *Client) ListInvoices(ctx context.Context, search string, page, pageSize int) (admin.InvoicePage, error) {
	var rows []struct {
		ID          string  `json:"id"`
		ClientID    string  `json:"cliente_id"`
		Description *string `json:"descricao"`
		Reference   *string `json:"referencia"`
		Original    float64 `json:"valor_original"`
		Discount    float64 `json:"valor_desconto"`
		DueDate     string  `json:"vencimento"`
		Status      string  `json:"status"`
		Client      struct {
			ID       string  `json:"id"`
			Name     string  `json:"nome"`
			Document *string `json:"documento"`
			Phone    *string `json:"telefone"`
			Email    *string `json:"email"`
		} `json:"clientes"`
	}
	q := url.Values{
		"select": {"id,cliente_id,descricao,referencia,valor_original,valor_desconto,vencimento,status,clientes!inner(id,nome,documento,telefone,email)"},
		"order":  {"created_at.desc"},
	}
	if search != "" {
		term := strings.ReplaceAll(search, ",", "")
		q.Set("clientes.or", "(documento.ilike.*"+term+"*,nome.ilike.*"+term+"*,telefone.ilike.*"+term+"*)")
	}
	from := page * pageSize
	total, err := c.SelectRange(ctx, "faturas", q, from, from+pageSize-1, &rows)
	if err != nil {
		return admin.InvoicePage{}, err
	}
	out := make([]admin.Invoice, 0, len(rows))
	for _, r := range rows {
		desc, ref := "", ""
		if r.Description != nil {
			desc = *r.Description
		}
		if r.Reference != nil {
			ref = *r.Reference
		}
		doc := ""
		if r.Client.Document != nil {
			doc = *r.Client.Document
		}
		out = append(out, admin.Invoice{
			ID: r.ID, ClientID: r.ClientID, Description: desc, Reference: ref,
			Original: r.Original, Discount: r.Discount, DueDate: r.DueDate, Status: r.Status,
			Client: admin.Client{ID: r.Client.ID, Name: r.Client.Name, Document: doc, Phone: r.Client.Phone, Email: r.Client.Email},
		})
	}
	return admin.InvoicePage{Rows: out, Total: total}, nil
}

func (c *Client) SaveInvoice(ctx context.Context, in admin.Invoice) error {
	clientBody := map[string]any{"nome": in.Client.Name, "documento": in.Client.Document, "telefone": in.Client.Phone, "email": in.Client.Email}
	if err := c.Update(ctx, "clientes", url.Values{"id": {"eq." + in.ClientID}}, clientBody); err != nil {
		return err
	}
	return c.Update(ctx, "faturas", url.Values{"id": {"eq." + in.ID}}, map[string]any{
		"descricao": in.Description, "referencia": in.Reference, "valor_original": in.Original,
		"valor_desconto": in.Discount, "vencimento": in.DueDate, "status": in.Status,
	})
}

func (c *Client) SetInvoiceStatus(ctx context.Context, id, status string) error {
	body := map[string]any{"status": status}
	if status == "paga" {
		body["data_pagamento"] = time.Now().UTC().Format(time.RFC3339Nano)
	}
	if err := c.Update(ctx, "faturas", url.Values{"id": {"eq." + id}}, body); err != nil {
		return err
	}
	if status != "paga" {
		return nil
	}
	var rows []struct {
		ClientID string  `json:"cliente_id"`
		Value    float64 `json:"valor_desconto"`
	}
	if err := c.Select(ctx, "faturas", url.Values{"select": {"cliente_id,valor_desconto"}, "id": {"eq." + id}, "limit": {"1"}}, &rows); err != nil || len(rows) == 0 {
		return err
	}
	var existing []struct {
		ID string `json:"id"`
	}
	_ = c.Select(ctx, "pagamentos", url.Values{"select": {"id"}, "fatura_id": {"eq." + id}, "status": {"eq.confirmado"}, "limit": {"1"}}, &existing)
	if len(existing) > 0 {
		return nil
	}
	return c.Insert(ctx, "pagamentos", map[string]any{
		"fatura_id": id, "cliente_id": rows[0].ClientID, "valor": rows[0].Value,
		"metodo": "manual", "status": "confirmado", "pago_em": time.Now().UTC().Format(time.RFC3339Nano),
	})
}

func (c *Client) DeleteAll(ctx context.Context) error {
	tables := []string{"webhooks_log", "pagamentos_log", "pix_generation_requests", "transacoes_pix", "pagamentos", "faturas", "acessos", "clientes"}
	for _, table := range tables {
		if err := c.DeleteReturning(ctx, table, url.Values{"id": {"not.is.null"}}, nil); err != nil {
			return fmt.Errorf("delete %s: %w", table, err)
		}
	}
	return nil
}

func (c *Client) Import(ctx context.Context, rows []admin.ImportRow) (admin.ImportResult, error) {
	result := admin.ImportResult{Rejected: []string{}}
	clients := make([]map[string]any, 0, len(rows))
	for _, r := range rows {
		metadata, _ := json.Marshal(r.Metadata)
		var phone any
		if r.Phone != "" {
			phone = r.Phone
		}
		var email any
		if r.Email != "" {
			email = r.Email
		}
		clients = append(clients, map[string]any{
			"documento": r.Document, "nome": r.Name, "telefone": phone,
			"email": email, "observacoes": string(metadata),
		})
	}
	var saved []struct {
		ID       string `json:"id"`
		Document string `json:"documento"`
	}
	q := url.Values{"on_conflict": {"documento"}, "select": {"id,documento"}}
	if err := c.UpsertReturning(ctx, "clientes", q, clients, &saved); err != nil {
		return result, err
	}
	result.Imported = len(saved)
	byDoc := map[string]string{}
	ids := make([]string, 0, len(saved))
	for _, r := range saved {
		byDoc[r.Document] = r.ID
		ids = append(ids, r.ID)
	}
	pending := map[string]string{}
	for start := 0; start < len(ids); start += 200 {
		end := start + 200
		if end > len(ids) {
			end = len(ids)
		}
		var found []struct {
			ID       string `json:"id"`
			ClientID string `json:"cliente_id"`
		}
		inq := "(" + strings.Join(ids[start:end], ",") + ")"
		if err := c.Select(ctx, "faturas", url.Values{
			"select": {"id,cliente_id"}, "cliente_id": {"in." + inq},
			"status": {"in.(em_aberto,vencida,em_processamento,expirada,falhou)"}, "order": {"created_at.desc"},
		}, &found); err != nil {
			return result, err
		}
		for _, f := range found {
			if _, ok := pending[f.ClientID]; !ok {
				pending[f.ClientID] = f.ID
			}
		}
	}
	existing := []map[string]any{}
	fresh := []map[string]any{}
	for _, r := range rows {
		id, ok := byDoc[r.Document]
		if !ok {
			result.Rejected = append(result.Rejected, r.Document)
			continue
		}
		discount := r.Original * .70
		if r.Discount != nil {
			discount = *r.Discount
		}
		reference := r.Metadata["mes_ref"]
		body := map[string]any{
			"cliente_id": id, "descricao": "Fatura Light importada", "referencia": reference,
			"valor_original": r.Original, "valor_desconto": discount, "vencimento": r.DueDate, "status": r.Status,
		}
		if invoiceID, ok := pending[id]; ok {
			body["id"] = invoiceID
			existing = append(existing, body)
		} else {
			fresh = append(fresh, body)
		}
	}
	if len(existing) > 0 {
		var returned []map[string]any
		if err := c.UpsertReturning(ctx, "faturas", url.Values{"on_conflict": {"id"}, "select": {"id"}}, existing, &returned); err != nil {
			return result, err
		}
		result.InvoicesUpdated = len(returned)
	}
	if len(fresh) > 0 {
		var returned []map[string]any
		if err := c.InsertReturning(ctx, "faturas", url.Values{"select": {"id"}}, fresh, &returned); err != nil {
			return result, err
		}
		result.InvoicesCreated = len(returned)
	}
	return result, nil
}

func (c *Client) Payments(ctx context.Context) ([]admin.Payment, error) {
	var rows []admin.Payment
	err := c.Select(ctx, "pagamentos", url.Values{
		"select": {"id,fatura_id,cliente_id,valor,metodo,status,gateway,gateway_payment_id,pago_em,created_at"},
		"order": {"created_at.desc"}, "limit": {"500"},
	}, &rows)
	return rows, err
}
func (c *Client) Transactions(ctx context.Context) ([]admin.Transaction, error) {
	var rows []admin.Transaction
	err := c.Select(ctx, "transacoes_pix", url.Values{
		"select": {"id,fatura_id,cliente_id,gateway_slug,transacao_gateway_id,valor_centavos,valor_pago_centavos,status,created_at"},
		"order": {"created_at.desc"}, "limit": {"500"},
	}, &rows)
	return rows, err
}
func (c *Client) Logs(ctx context.Context) ([]admin.Log, error) {
	var rows []admin.Log
	err := c.Select(ctx, "pagamentos_log", url.Values{
		"select": {"id,gateway_slug,fatura_id,nivel,http_status,mensagem,created_at"},
		"order": {"created_at.desc"}, "limit": {"1000"},
	}, &rows)
	return rows, err
}
func (c *Client) Gateways(ctx context.Context) ([]admin.Gateway, error) {
	var rows []admin.Gateway
	if err := c.Select(ctx, "gateways_config", url.Values{
		"select": {"id,slug,rotulo,adapter,ativo,prioridade,api_url,ambiente,limite_diario,webhook_url,secret_names,observacoes"},
		"order": {"prioridade.asc"},
	}, &rows); err != nil {
		return nil, err
	}
	for i := range rows {
		configured := len(rows[i].SecretNames) > 0
		for _, name := range rows[i].SecretNames {
			if strings.TrimSpace(os.Getenv(name)) == "" {
				configured = false
				break
			}
		}
		if rows[i].Adapter == "generico" && (rows[i].APIURL == nil || strings.TrimSpace(*rows[i].APIURL) == "") {
			configured = false
		}
		rows[i].Configured = configured
	}
	return rows, nil
}
func (c *Client) SaveGateway(ctx context.Context, g admin.Gateway) error {
	return c.Update(ctx, "gateways_config", url.Values{"id": {"eq." + g.ID}}, map[string]any{
		"rotulo": g.Label, "adapter": g.Adapter, "ativo": g.Active, "prioridade": g.Priority,
		"api_url": g.APIURL, "ambiente": g.Environment, "limite_diario": g.DailyLimit,
		"webhook_url": g.WebhookURL, "secret_names": g.SecretNames, "observacoes": g.Observations,
	})
}
func (c *Client) DeleteGateway(ctx context.Context, id string) error {
	return c.DeleteReturning(ctx, "gateways_config", url.Values{"id": {"eq." + id}}, nil)
}
func (c *Client) UseOnlyGateway(ctx context.Context, id string) error {
	if err := c.Update(ctx, "gateways_config", url.Values{"id": {"not.is.null"}}, map[string]any{"ativo": false}); err != nil {
		return err
	}
	if err := c.Update(ctx, "gateways_config", url.Values{"id": {"eq." + id}}, map[string]any{"ativo": true}); err != nil {
		return err
	}
	return c.Update(ctx, "roteamento_config", url.Values{"id": {"eq.true"}}, map[string]any{"estrategia": "fixa", "gateway_fixa": id})
}
func (c *Client) AdminRouting(ctx context.Context) (admin.Routing, error) {
	var rows []struct {
		Strategy string  `json:"estrategia"`
		Fixed    *string `json:"gateway_fixa"`
		NewPIX   bool    `json:"novo_pix_por_acesso"`
	}
	if err := c.Select(ctx, "roteamento_config", url.Values{
		"select": {"estrategia,gateway_fixa,novo_pix_por_acesso"}, "id": {"eq.true"}, "limit": {"1"},
	}, &rows); err != nil {
		return admin.Routing{}, err
	}
	if len(rows) == 0 {
		return admin.Routing{Strategy: "prioridade"}, nil
	}
	return admin.Routing{Strategy: rows[0].Strategy, FixedGateway: rows[0].Fixed, NewPIXPerAccess: rows[0].NewPIX}, nil
}
func (c *Client) SaveRouting(ctx context.Context, r admin.Routing) error {
	return c.Update(ctx, "roteamento_config", url.Values{"id": {"eq.true"}}, map[string]any{
		"estrategia": r.Strategy, "gateway_fixa": r.FixedGateway, "novo_pix_por_acesso": r.NewPIXPerAccess,
	})
}

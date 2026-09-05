package supabase

import (
	"context"
	"errors"
	"fmt"
	"net/url"
	"strconv"
	"time"

	"github.com/matspectrum-ai/Light-Fatura/internal/gateway"
	"github.com/matspectrum-ai/Light-Fatura/internal/payment"
)

type requestRow struct {
	ID            string  `json:"id"`
	Status        string  `json:"status"`
	TransactionID *string `json:"transacao_id"`
}
type transactionRow struct {
	ID                   string  `json:"id"`
	InvoiceID            string  `json:"fatura_id"`
	CustomerID           *string `json:"cliente_id"`
	GatewaySlug          string  `json:"gateway_slug"`
	GatewayID            string  `json:"gateway_id"`
	GatewayTransactionID *string `json:"transacao_gateway_id"`
	AmountCents          int     `json:"valor_centavos"`
	CopyPaste            *string `json:"copia_cola"`
	QRCode               *string `json:"qrcode"`
	Status               string  `json:"status"`
	ExpiresAt            *string `json:"expira_em"`
}

func (c *Client) ReserveRequest(ctx context.Context, key, invoiceID string) (payment.RequestReservation, bool, error) {
	q := url.Values{"select": {"id,status,transacao_id"}}
	var rows []requestRow
	err := c.InsertReturning(ctx, "pix_generation_requests", q, map[string]any{"request_key": key, "fatura_id": invoiceID}, &rows)
	if err == nil && len(rows) > 0 {
		return reservation(rows[0]), true, nil
	}
	var httpErr *HTTPError
	if !errors.As(err, &httpErr) || httpErr.Code != "23505" {
		return payment.RequestReservation{}, false, err
	}
	rows = nil
	q = url.Values{"select": {"id,status,transacao_id"}, "request_key": {"eq." + key}, "limit": {"1"}}
	if err := c.Select(ctx, "pix_generation_requests", q, &rows); err != nil {
		return payment.RequestReservation{}, false, err
	}
	if len(rows) == 0 {
		return payment.RequestReservation{}, false, fmt.Errorf("solicitação PIX não encontrada após conflito")
	}
	return reservation(rows[0]), false, nil
}
func (c *Client) RequestTransaction(ctx context.Context, res payment.RequestReservation) (*payment.Transaction, error) {
	if res.Status != "concluida" || res.TransactionID == nil {
		return nil, nil
	}
	q := url.Values{"select": {"id,fatura_id,cliente_id,gateway_slug,gateway_id,transacao_gateway_id,valor_centavos,copia_cola,qrcode,status,expira_em"}, "id": {"eq." + *res.TransactionID}, "limit": {"1"}}
	var rows []transactionRow
	if err := c.Select(ctx, "transacoes_pix", q, &rows); err != nil {
		return nil, err
	}
	if len(rows) == 0 {
		return nil, nil
	}
	tx := transaction(rows[0])
	return &tx, nil
}
func (c *Client) CompleteRequest(ctx context.Context, id, transactionID string) error {
	return c.Update(ctx, "pix_generation_requests", url.Values{"id": {"eq." + id}}, map[string]any{"status": "concluida", "transacao_id": transactionID, "erro": nil})
}
func (c *Client) FailRequest(ctx context.Context, id, message string) error {
	if len(message) > 500 {
		message = message[:500]
	}
	return c.Update(ctx, "pix_generation_requests", url.Values{"id": {"eq." + id}}, map[string]any{"status": "falhou", "erro": message})
}
func (c *Client) InvoiceStatus(ctx context.Context, id string) (string, bool, error) {
	var rows []struct {
		Status string `json:"status"`
	}
	q := url.Values{"select": {"status"}, "id": {"eq." + id}, "limit": {"1"}}
	if err := c.Select(ctx, "faturas", q, &rows); err != nil {
		return "", false, err
	}
	if len(rows) == 0 {
		return "", false, nil
	}
	return rows[0].Status, true, nil
}
func (c *Client) ActiveGateways(ctx context.Context) ([]gateway.Record, error) {
	var rows []gateway.Record
	q := url.Values{"select": {"id,slug,rotulo,adapter,ativo,prioridade,api_url,ambiente,limite_diario,webhook_url,secret_names,observacoes"}, "ativo": {"eq.true"}, "order": {"prioridade.asc"}}
	if err := c.Select(ctx, "gateways_config", q, &rows); err != nil {
		return nil, err
	}
	return rows, nil
}
func (c *Client) Routing(ctx context.Context) (payment.RoutingConfig, error) {
	var rows []struct {
		Strategy     string  `json:"estrategia"`
		FixedGateway *string `json:"gateway_fixa"`
		Pointer      int     `json:"ponteiro"`
	}
	q := url.Values{"select": {"estrategia,gateway_fixa,ponteiro"}, "id": {"eq.true"}, "limit": {"1"}}
	if err := c.Select(ctx, "roteamento_config", q, &rows); err != nil {
		return payment.RoutingConfig{}, err
	}
	if len(rows) == 0 {
		return payment.RoutingConfig{Strategy: "prioridade"}, nil
	}
	return payment.RoutingConfig{Strategy: rows[0].Strategy, FixedGateway: rows[0].FixedGateway, Pointer: rows[0].Pointer}, nil
}
func (c *Client) AdvanceGatewayPointer(ctx context.Context, total int) (int, error) {
	var value any
	if err := c.RPC(ctx, "avancar_ponteiro_gateway", map[string]any{"p_total": total}, &value); err != nil {
		return 0, err
	}
	switch v := value.(type) {
	case float64:
		return int(v), nil
	case string:
		return strconv.Atoi(v)
	default:
		return 0, fmt.Errorf("ponteiro inválido: %T", value)
	}
}
func (c *Client) GatewayTransactionsSince(ctx context.Context, slug string, since time.Time) (int, error) {
	q := url.Values{"gateway_slug": {"eq." + slug}, "created_at": {"gte." + since.UTC().Format(time.RFC3339Nano)}}
	return c.Count(ctx, "transacoes_pix", q)
}
func (c *Client) InsertTransaction(ctx context.Context, in payment.TransactionCreate) (payment.Transaction, error) {
	body := map[string]any{"fatura_id": in.InvoiceID, "cliente_id": in.CustomerID, "gateway_slug": in.GatewaySlug, "gateway_id": in.GatewayID, "transacao_gateway_id": in.GatewayTransactionID, "valor_centavos": in.AmountCents, "copia_cola": in.CopyPaste, "qrcode": in.QRCode, "status": in.Status, "idempotency_key": in.IdempotencyKey, "expira_em": in.ExpiresAt}
	q := url.Values{"select": {"id,fatura_id,cliente_id,gateway_slug,gateway_id,transacao_gateway_id,valor_centavos,copia_cola,qrcode,status,expira_em"}}
	var rows []transactionRow
	if err := c.InsertReturning(ctx, "transacoes_pix", q, body, &rows); err != nil {
		return payment.Transaction{}, err
	}
	if len(rows) == 0 {
		return payment.Transaction{}, fmt.Errorf("transação PIX não retornada pelo banco")
	}
	return transaction(rows[0]), nil
}
func (c *Client) ReplaceOtherPending(ctx context.Context, invoiceID, exceptID string, at time.Time) error {
	q := url.Values{"fatura_id": {"eq." + invoiceID}, "status": {"eq.pendente"}, "id": {"neq." + exceptID}}
	return c.Update(ctx, "transacoes_pix", q, map[string]any{"status": "substituida", "substituida_em": at.UTC().Format(time.RFC3339Nano)})
}
func (c *Client) Log(ctx context.Context, entry payment.LogEntry) error {
	return c.Insert(ctx, "pagamentos_log", map[string]any{"gateway_slug": entry.GatewaySlug, "fatura_id": entry.InvoiceID, "nivel": entry.Level, "http_status": entry.HTTPStatus, "mensagem": entry.Message})
}
func reservation(row requestRow) payment.RequestReservation {
	return payment.RequestReservation{ID: row.ID, Status: row.Status, TransactionID: row.TransactionID}
}
func transaction(row transactionRow) payment.Transaction {
	return payment.Transaction{ID: row.ID, InvoiceID: row.InvoiceID, CustomerID: row.CustomerID, GatewaySlug: row.GatewaySlug, GatewayID: row.GatewayID, GatewayTransactionID: row.GatewayTransactionID, AmountCents: row.AmountCents, CopyPaste: row.CopyPaste, QRCode: row.QRCode, Status: row.Status, ExpiresAt: row.ExpiresAt}
}

package supabase

import (
	"context"
	"encoding/json"
	"net/url"
	"strings"
	"time"

	"github.com/matspectrum-ai/Claro-Fatura/internal/invoice"
)

func (c *Client) CustomerByDocument(ctx context.Context, document string) (invoice.Customer, bool, error) {
	var rows []struct {
		ID string `json:"id"`; Documento string `json:"documento"`; Nome string `json:"nome"`; Telefone string `json:"telefone"`; Email string `json:"email"`; Observacoes string `json:"observacoes"`
	}
	q := url.Values{"select":{"id,documento,nome,telefone,email,observacoes"},"documento":{"eq."+document},"limit":{"1"}}
	if err := c.Select(ctx, "clientes", q, &rows); err != nil { return invoice.Customer{}, false, err }
	if len(rows) == 0 { return invoice.Customer{}, false, nil }
	r := rows[0]
	return invoice.Customer{ID:r.ID, Document:r.Documento, Name:r.Nome, Phone:r.Telefone, Email:r.Email, Observacoes:r.Observacoes}, true, nil
}

func (c *Client) LatestOpenInvoice(ctx context.Context, customerID string) (invoice.Invoice, bool, error) {
	var rows []struct {
		ID string `json:"id"`; ClienteID string `json:"cliente_id"`; Descricao string `json:"descricao"`; Referencia string `json:"referencia"`; ValorOriginal float64 `json:"valor_original"`; ValorDesconto float64 `json:"valor_desconto"`; Vencimento string `json:"vencimento"`; Status string `json:"status"`
	}
	q := url.Values{"select":{"id,cliente_id,descricao,referencia,valor_original,valor_desconto,vencimento,status"},"cliente_id":{"eq."+customerID},"status":{"in.(em_aberto,vencida,em_processamento)"},"order":{"created_at.desc"},"limit":{"1"}}
	if err := c.Select(ctx, "faturas", q, &rows); err != nil { return invoice.Invoice{}, false, err }
	if len(rows) == 0 { return invoice.Invoice{}, false, nil }
	r := rows[0]
	return invoice.Invoice{ID:r.ID, CustomerID:r.ClienteID, Description:r.Descricao, Reference:r.Referencia, OriginalValue:r.ValorOriginal, DiscountedValue:r.ValorDesconto, DueDate:r.Vencimento, Status:r.Status}, true, nil
}

func (c *Client) LogAccess(ctx context.Context, document string, success bool, original, discounted float64) error {
	return c.Insert(ctx, "acessos", map[string]any{"data_hora":time.Now().UTC(), "pagina":"/", "telefone_consultado":document, "sucesso":success, "valor_original":original, "valor_desconto":discounted})
}

func (c *Client) IsAdmin(ctx context.Context, userID string) (bool, error) {
	var rows []struct{ Role string `json:"role"` }
	q := url.Values{"select":{"role"},"user_id":{"eq."+userID},"role":{"eq.admin"},"limit":{"1"}}
	if err := c.Select(ctx, "user_roles", q, &rows); err != nil { return false, err }
	return len(rows) > 0, nil
}

func EncodeLightMetadata(values map[string]string) string {
	clean := map[string]string{}
	for key, value := range values { if strings.TrimSpace(value) != "" { clean[key] = strings.TrimSpace(value) } }
	raw, _ := json.Marshal(clean)
	return string(raw)
}

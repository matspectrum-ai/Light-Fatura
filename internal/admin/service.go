package admin

import (
	"context"
	"errors"
	"math"
	"strings"
)

var ErrInvalid = errors.New("dados administrativos inválidos")

type Dashboard struct {
	Clients      int     `json:"clientes_total"`
	Invoices     int     `json:"faturas_total"`
	Payments     int     `json:"pagamentos_total"`
	Views        int     `json:"faturas_visualizadas_total"`
	ViewedAmount float64 `json:"valor_visualizado_total"`
}

type Client struct {
	ID       string  `json:"id"`
	Name     string  `json:"nome"`
	Document string  `json:"documento"`
	Phone    *string `json:"telefone"`
	Email    *string `json:"email"`
}

type Invoice struct {
	ID          string  `json:"id"`
	ClientID    string  `json:"cliente_id"`
	Description string  `json:"descricao"`
	Reference   string  `json:"referencia"`
	Original    float64 `json:"valor_original"`
	Discount    float64 `json:"valor_desconto"`
	DueDate     string  `json:"vencimento"`
	Status      string  `json:"status"`
	Client      Client  `json:"cliente"`
}
type InvoicePage struct {
	Rows  []Invoice `json:"linhas"`
	Total int       `json:"total"`
}

type ImportRow struct {
	Document string
	Name     string
	Phone    string
	Email    string
	Original float64
	Discount *float64
	DueDate  string
	Status   string
	Metadata map[string]string
}
type ImportResult struct {
	Imported        int      `json:"importados"`
	InvoicesCreated int      `json:"faturasCriadas"`
	InvoicesUpdated int      `json:"faturasAtualizadas"`
	Rejected        []string `json:"rejeitados"`
}

type Payment struct {
	ID               string  `json:"id"`
	InvoiceID        string  `json:"fatura_id"`
	ClientID         string  `json:"cliente_id"`
	Value            float64 `json:"valor"`
	Method           string  `json:"metodo"`
	Status           string  `json:"status"`
	Gateway          string  `json:"gateway"`
	GatewayPaymentID *string `json:"gateway_payment_id"`
	PaidAt           *string `json:"pago_em"`
	CreatedAt        string  `json:"created_at"`
}
type Transaction struct {
	ID                   string  `json:"id"`
	InvoiceID            string  `json:"fatura_id"`
	ClientID             string  `json:"cliente_id"`
	GatewaySlug          string  `json:"gateway_slug"`
	GatewayTransactionID *string `json:"transacao_gateway_id"`
	AmountCents          int     `json:"valor_centavos"`
	PaidCents            *int    `json:"valor_pago_centavos"`
	Status               string  `json:"status"`
	CreatedAt            string  `json:"created_at"`
}
type Log struct {
	ID          string  `json:"id"`
	GatewaySlug string  `json:"gateway_slug"`
	InvoiceID   *string `json:"fatura_id"`
	Level       string  `json:"nivel"`
	HTTPStatus  *int    `json:"http_status"`
	Message     string  `json:"mensagem"`
	CreatedAt   string  `json:"created_at"`
}

type Gateway struct {
	ID           string   `json:"id"`
	Slug         string   `json:"slug"`
	Label        string   `json:"rotulo"`
	Adapter      string   `json:"adapter"`
	Active       bool     `json:"ativo"`
	Priority     int      `json:"prioridade"`
	APIURL       *string  `json:"api_url"`
	Environment  string   `json:"ambiente"`
	DailyLimit   *int     `json:"limite_diario"`
	WebhookURL   *string  `json:"webhook_url"`
	SecretNames  []string `json:"secret_names"`
	Observations *string  `json:"observacoes"`
	Configured   bool     `json:"configurado"`
}
type Routing struct {
	Strategy        string  `json:"estrategia"`
	FixedGateway    *string `json:"gateway_fixa"`
	NewPIXPerAccess bool    `json:"novo_pix_por_acesso"`
}

type Store interface {
	Dashboard(context.Context) (Dashboard, error)
	ListInvoices(context.Context, string, int, int) (InvoicePage, error)
	SaveInvoice(context.Context, Invoice) error
	SetInvoiceStatus(context.Context, string, string) error
	DeleteAll(context.Context) error
	Import(context.Context, []ImportRow) (ImportResult, error)
	Payments(context.Context) ([]Payment, error)
	Transactions(context.Context) ([]Transaction, error)
	Logs(context.Context) ([]Log, error)
	Gateways(context.Context) ([]Gateway, error)
	SaveGateway(context.Context, Gateway) error
	DeleteGateway(context.Context, string) error
	UseOnlyGateway(context.Context, string) error
	AdminRouting(context.Context) (Routing, error)
	SaveRouting(context.Context, Routing) error
}

type Service struct{ store Store }

func New(store Store) *Service { return &Service{store: store} }
func (s *Service) Dashboard(ctx context.Context) (Dashboard, error) {
	return s.store.Dashboard(ctx)
}
func (s *Service) ListInvoices(ctx context.Context, search string, page int) (InvoicePage, error) {
	if page < 0 {
		page = 0
	}
	return s.store.ListInvoices(ctx, strings.TrimSpace(search), page, 50)
}
func (s *Service) SaveInvoice(ctx context.Context, in Invoice) error {
	in.DocumentDigits()
	if in.ID == "" || in.ClientID == "" || len(in.Client.Document) != 11 || strings.TrimSpace(in.Client.Name) == "" || !dateOnly(in.DueDate) || in.Original < 0 || in.Discount < 0 || !statusAllowed(in.Status) {
		return ErrInvalid
	}
	return s.store.SaveInvoice(ctx, in)
}
func (s *Service) SetInvoiceStatus(ctx context.Context, id, status string) error {
	if id == "" || !statusAllowed(status) {
		return ErrInvalid
	}
	return s.store.SetInvoiceStatus(ctx, id, status)
}
func (s *Service) DeleteAll(ctx context.Context, confirmation string) error {
	if confirmation != "APAGAR" {
		return ErrInvalid
	}
	return s.store.DeleteAll(ctx)
}
func (s *Service) Import(ctx context.Context, rows []ImportRow) (ImportResult, error) {
	if len(rows) == 0 || len(rows) > 5000 {
		return ImportResult{}, ErrInvalid
	}
	seen := map[string]struct{}{}
	clean := make([]ImportRow, 0, len(rows))
	result := ImportResult{Rejected: []string{}}
	for _, r := range rows {
		r.Document = digits(r.Document)
		r.Phone = digits(r.Phone)
		r.Name = strings.TrimSpace(r.Name)
		r.Email = strings.TrimSpace(r.Email)
		r.Status = strings.TrimSpace(r.Status)
		if r.Status == "" {
			r.Status = "em_aberto"
		}
		if len(r.Document) != 11 || r.Name == "" || r.Original < 0 || !dateOnly(r.DueDate) || !statusAllowed(r.Status) {
			result.Rejected = append(result.Rejected, r.Document)
			continue
		}
		if _, ok := seen[r.Document]; ok {
			result.Rejected = append(result.Rejected, r.Document)
			continue
		}
		seen[r.Document] = struct{}{}
		if r.Discount == nil {
			v := math.Round(r.Original*70) / 100
			r.Discount = &v
		}
		if *r.Discount < 0 || *r.Discount > r.Original {
			result.Rejected = append(result.Rejected, r.Document)
			continue
		}
		clean = append(clean, r)
	}
	if len(clean) == 0 {
		return result, nil
	}
	saved, err := s.store.Import(ctx, clean)
	if err != nil {
		return ImportResult{}, err
	}
	saved.Rejected = append(result.Rejected, saved.Rejected...)
	return saved, nil
}
func (s *Service) Payments(ctx context.Context) ([]Payment, error) {
	return s.store.Payments(ctx)
}
func (s *Service) Transactions(ctx context.Context) ([]Transaction, error) {
	return s.store.Transactions(ctx)
}
func (s *Service) Logs(ctx context.Context) ([]Log, error) { return s.store.Logs(ctx) }
func (s *Service) Gateways(ctx context.Context) ([]Gateway, error) {
	return s.store.Gateways(ctx)
}
func (s *Service) SaveGateway(ctx context.Context, g Gateway) error {
	if g.ID == "" || g.Slug == "" || g.Label == "" || g.Priority < 1 {
		return ErrInvalid
	}
	return s.store.SaveGateway(ctx, g)
}
func (s *Service) DeleteGateway(ctx context.Context, id string) error {
	if id == "" {
		return ErrInvalid
	}
	return s.store.DeleteGateway(ctx, id)
}
func (s *Service) UseOnlyGateway(ctx context.Context, id string) error {
	if id == "" {
		return ErrInvalid
	}
	return s.store.UseOnlyGateway(ctx, id)
}
func (s *Service) Routing(ctx context.Context) (Routing, error) {
	return s.store.AdminRouting(ctx)
}
func (s *Service) SaveRouting(ctx context.Context, r Routing) error {
	if r.Strategy != "prioridade" && r.Strategy != "rodizio" && r.Strategy != "fixa" {
		return ErrInvalid
	}
	if r.Strategy == "fixa" && r.FixedGateway == nil {
		return ErrInvalid
	}
	return s.store.SaveRouting(ctx, r)
}
func (i *Invoice) DocumentDigits() {
	i.Client.Document = digits(i.Client.Document)
	if i.Client.Phone != nil {
		v := digits(*i.Client.Phone)
		i.Client.Phone = &v
	}
}
func digits(v string) string {
	var b strings.Builder
	for _, r := range v {
		if r >= '0' && r <= '9' {
			b.WriteRune(r)
		}
	}
	return b.String()
}
func dateOnly(v string) bool {
	if len(v) != 10 || v[4] != '-' || v[7] != '-' {
		return false
	}
	for i, r := range v {
		if i == 4 || i == 7 {
			continue
		}
		if r < '0' || r > '9' {
			return false
		}
	}
	return true
}
func statusAllowed(v string) bool {
	switch v {
	case "em_aberto", "vencida", "em_processamento", "paga", "expirada", "falhou", "cancelada":
		return true
	}
	return false
}

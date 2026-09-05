package gateway

import (
	"context"
	"net/http"
)

type Record struct {
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
}

type CreateInput struct {
	Gateway     Record
	AmountCents int
	Name        string
	Phone       string
	Email       *string
	Document    *string
	Description string
	Reference   string
	WebhookURL  string
}

type CreatedPIX struct {
	TransactionID string
	CopyPaste     string
	QRCode        *string
	Status        string
	ExpiresAt     *string
}

type WebhookRead struct {
	Valid         bool
	TransactionID *string
	Status        *string
	Event         *string
}

type Adapter interface {
	Name() string
	Configured(Record) bool
	CreatePIX(context.Context, CreateInput) (CreatedPIX, error)
	Status(context.Context, string, Record) (*string, error)
	Paid(*string) bool
	ReadWebhook(*http.Request, []byte, Record) (WebhookRead, error)
}

type Registry interface {
	AdapterFor(Record) Adapter
}

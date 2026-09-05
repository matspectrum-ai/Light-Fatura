package invoice

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"
)

type fakeStore struct {
	customer      Customer
	customerFound bool
	invoice       Invoice
	invoiceFound  bool
	customerErr   error
	invoiceErr    error
	logErr        error
	logs          []accessLog
}

type accessLog struct {
	document   string
	success    bool
	original   float64
	discounted float64
}

func (s *fakeStore) CustomerByDocument(_ context.Context, document string) (Customer, bool, error) {
	return s.customer, s.customerFound, s.customerErr
}
func (s *fakeStore) LatestOpenInvoice(_ context.Context, _ string) (Invoice, bool, error) {
	return s.invoice, s.invoiceFound, s.invoiceErr
}
func (s *fakeStore) LogAccess(_ context.Context, document string, success bool, original, discounted float64) error {
	s.logs = append(s.logs, accessLog{document: document, success: success, original: original, discounted: discounted})
	return s.logErr
}

func TestQueryByDocumentRejectsInvalidCPFShape(t *testing.T) {
	service := New(&fakeStore{})
	_, err := service.QueryByDocument(context.Background(), "123.456")
	if !errors.Is(err, ErrInvalidDocument) {
		t.Fatalf("expected ErrInvalidDocument, got %v", err)
	}
}

func TestQueryByDocumentMissingCustomerLogsFailedAccess(t *testing.T) {
	store := &fakeStore{}
	service := New(store)
	result, err := service.QueryByDocument(context.Background(), "123.456.789-01")
	if err != nil {
		t.Fatal(err)
	}
	if result.Found {
		t.Fatal("expected customer not found")
	}
	if len(store.logs) != 1 || store.logs[0].success || store.logs[0].document != "12345678901" {
		t.Fatalf("unexpected access log: %#v", store.logs)
	}
}

func TestQueryByDocumentCustomerWithoutOpenInvoice(t *testing.T) {
	store := &fakeStore{
		customerFound: true,
		customer: Customer{ID: "internal-customer-id", Document: "12345678901", Name: "Cliente Light"},
	}
	result, err := New(store).QueryByDocument(context.Background(), "12345678901")
	if err != nil {
		t.Fatal(err)
	}
	if !result.Found || result.InvoiceID != "" {
		t.Fatalf("unexpected result: %#v", result)
	}
	if len(store.logs) != 1 || !store.logs[0].success {
		t.Fatalf("unexpected access log: %#v", store.logs)
	}
}

func TestQueryByDocumentMapsLightMetadataAndHidesInternalCustomerID(t *testing.T) {
	store := &fakeStore{
		customerFound: true,
		invoiceFound:  true,
		customer: Customer{
			ID:          "internal-customer-id",
			Document:    "12345678901",
			Name:        "Cliente Light",
			Phone:       "5521999999999",
			Email:       "cliente@example.com",
			Observacoes: `{"cte":"CTE123","instalacao":"9988","endereco":"Rua A, 10","endereco_rua":"Rua A","bairro":"Centro","cidade":"Rio de Janeiro","estado":"RJ","cep":"20000-000","contrato":"CTR1","conta_contrato":"CC1","mes_ref":"09/2026","parcneg":"1/3"}`,
		},
		invoice: Invoice{ID: "invoice-id", CustomerID: "internal-customer-id", OriginalValue: 100, DiscountedValue: 70, DueDate: "2026-09-10", Status: "em_aberto"},
		logErr: errors.New("log failure must not break lookup"),
	}
	result, err := New(store).QueryByDocument(context.Background(), "12345678901")
	if err != nil {
		t.Fatal(err)
	}
	if result.InvoiceID != "invoice-id" || result.DiscountedValue != 70 || result.User.Original != 100 {
		t.Fatalf("unexpected invoice result: %#v", result)
	}
	if result.User.CTE != "CTE123" || result.User.Installation != "9988" || result.User.Light["contrato"] != "CTR1" || result.User.Light["mes_ref"] != "09/2026" {
		t.Fatalf("Light metadata not mapped: %#v", result.User)
	}
	raw, err := json.Marshal(result)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(raw), "internal-customer-id") {
		t.Fatalf("public JSON leaked internal customer id: %s", raw)
	}
	if len(store.logs) != 1 || store.logs[0].original != 100 || store.logs[0].discounted != 70 {
		t.Fatalf("unexpected access log: %#v", store.logs)
	}
}

package payment

import (
	"context"
	"testing"
)

type generatorStoreFake struct {
	invoice      InvoiceForPayment
	invoiceFound bool
	customer     CustomerForPayment
	newPerAccess bool
	pending      *Transaction
	expiredID    string
	syncedCents  int
	paymentValue float64
}

func (s *generatorStoreFake) PaymentInvoice(context.Context, string) (InvoiceForPayment, bool, error) {
	return s.invoice, s.invoiceFound, nil
}
func (s *generatorStoreFake) NewPIXPerAccess(context.Context) (bool, error) {
	return s.newPerAccess, nil
}
func (s *generatorStoreFake) LatestPendingTransaction(context.Context, string) (*Transaction, error) {
	return s.pending, nil
}
func (s *generatorStoreFake) ExpireTransaction(_ context.Context, id string) error {
	s.expiredID = id
	return nil
}
func (s *generatorStoreFake) PaymentCustomer(context.Context, string) (CustomerForPayment, error) {
	return s.customer, nil
}
func (s *generatorStoreFake) SyncInvoicePIX(_ context.Context, _ string, _ Transaction, cents int) error {
	s.syncedCents = cents
	return nil
}
func (s *generatorStoreFake) UpsertPendingPayment(_ context.Context, _ InvoiceForPayment, _ Transaction, value float64) error {
	s.paymentValue = value
	return nil
}

type creatorFake struct {
	request CreateRequest
	tx      *Transaction
}

func (c *creatorFake) CreatePIX(_ context.Context, request CreateRequest) (*Transaction, error) {
	c.request = request
	return c.tx, nil
}

func TestGeneratorChargesDatabaseDiscountValueExactly(t *testing.T) {
	copyPaste := "000201PIXTEST"
	gatewayID := "gw-123"
	store := &generatorStoreFake{
		invoiceFound: true,
		invoice:      InvoiceForPayment{ID: "invoice-1", CustomerID: "customer-1", Description: "Fatura Light", DiscountAmount: 70.01, Status: "em_aberto"},
		customer:     CustomerForPayment{Name: "Cliente", Phone: "21999999999"},
		newPerAccess: true,
	}
	creator := &creatorFake{tx: &Transaction{ID: "tx-1", GatewaySlug: "cashinpay", GatewayTransactionID: &gatewayID, AmountCents: 7001, CopyPaste: &copyPaste, Status: "pendente"}}
	result, err := NewGenerator(store, creator).Generate(context.Background(), GenerateInput{InvoiceID: "invoice-1", RequestKey: "request-1", BaseURL: "https://light.example"})
	if err != nil {
		t.Fatal(err)
	}
	if creator.request.AmountCents != 7001 {
		t.Fatalf("gateway amount = %d cents, want 7001", creator.request.AmountCents)
	}
	if result.Value != 70.01 || result.CopyPaste != copyPaste || !result.Available {
		t.Fatalf("unexpected generated PIX: %#v", result)
	}
	if store.syncedCents != 7001 || store.paymentValue != 70.01 {
		t.Fatalf("persistence values mismatch: cents=%d value=%v", store.syncedCents, store.paymentValue)
	}
}

func TestGeneratorDoesNotCreatePIXForPaidInvoice(t *testing.T) {
	store := &generatorStoreFake{invoiceFound: true, invoice: InvoiceForPayment{ID: "invoice-1", Status: "paga"}}
	creator := &creatorFake{}
	result, err := NewGenerator(store, creator).Generate(context.Background(), GenerateInput{InvoiceID: "invoice-1"})
	if err != nil {
		t.Fatal(err)
	}
	if result.Status != "paga" || result.Available {
		t.Fatalf("unexpected result: %#v", result)
	}
	if creator.request.InvoiceID != "" {
		t.Fatal("creator must not be called for paid invoice")
	}
}

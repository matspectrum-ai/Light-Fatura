package payment

import (
	"context"
	"math"
	"time"
)

type InvoiceForPayment struct { ID string; CustomerID string; Description string; DiscountAmount float64; Status string }
type CustomerForPayment struct { Name string; Phone string; Email *string; Document *string }

type GeneratorStore interface {
	PaymentInvoice(context.Context, string) (InvoiceForPayment, bool, error)
	NewPIXPerAccess(context.Context) (bool, error)
	LatestPendingTransaction(context.Context, string) (*Transaction, error)
	ExpireTransaction(context.Context, string) error
	PaymentCustomer(context.Context, string) (CustomerForPayment, error)
	SyncInvoicePIX(context.Context, string, Transaction, int) error
	UpsertPendingPayment(context.Context, InvoiceForPayment, Transaction, float64) error
}
type PIXCreator interface { CreatePIX(context.Context, CreateRequest) (*Transaction, error) }
type GenerateInput struct { InvoiceID string; RequestKey string; Force bool; BaseURL string }
type GeneratedPIX struct { Value float64 `json:"valor"`; CopyPaste string `json:"copia_cola"`; TXID string `json:"txid"`; Status string `json:"status"`; Available bool `json:"disponivel"`; TransactionID string `json:"transacao_id,omitempty"`; Gateway string `json:"gateway,omitempty"`; ExpiresAt *string `json:"expira_em,omitempty"`; Message string `json:"mensagem,omitempty"` }
type Generator struct { store GeneratorStore; creator PIXCreator; now func() time.Time }
func NewGenerator(store GeneratorStore, creator PIXCreator)*Generator{return &Generator{store:store,creator:creator,now:time.Now}}
func(g *Generator)Generate(ctx context.Context,in GenerateInput)(GeneratedPIX,error){invoice,found,err:=g.store.PaymentInvoice(ctx,in.InvoiceID);if err!=nil{return GeneratedPIX{},err};if !found{return GeneratedPIX{},ErrInvoiceNotFound};if invoice.Status=="paga"{return GeneratedPIX{Status:"paga",Available:false},nil};amountCents:=int(math.Round(invoice.DiscountAmount*100));if amountCents<=0||math.IsNaN(invoice.DiscountAmount)||math.IsInf(invoice.DiscountAmount,0){return GeneratedPIX{Status:invoice.Status,Available:false,Message:"Esta fatura não possui um valor com desconto válido para pagamento."},nil};value:=float64(amountCents)/100;var tx *Transaction;if !in.Force{newEachAccess,e:=g.store.NewPIXPerAccess(ctx);if e!=nil{return GeneratedPIX{},e};if !newEachAccess{tx,e=g.store.LatestPendingTransaction(ctx,invoice.ID);if e!=nil{return GeneratedPIX{},e};if tx!=nil{if tx.CopyPaste==nil||*tx.CopyPaste==""||tx.AmountCents!=amountCents{tx=nil}else if tx.ExpiresAt!=nil{expires,parseErr:=time.Parse(time.RFC3339Nano,*tx.ExpiresAt);if parseErr==nil&&!expires.After(g.now()){_ = g.store.ExpireTransaction(ctx,tx.ID);tx=nil}}}}};customer,err:=g.store.PaymentCustomer(ctx,invoice.CustomerID);if err!=nil{return GeneratedPIX{},err};if tx==nil{tx,err=g.creator.CreatePIX(ctx,CreateRequest{InvoiceID:invoice.ID,CustomerID:&invoice.CustomerID,AmountCents:amountCents,Name:customer.Name,Phone:customer.Phone,Email:customer.Email,Document:customer.Document,Description:invoice.Description,BaseURL:in.BaseURL,RequestKey:in.RequestKey});if err!=nil{return GeneratedPIX{},err}};if tx==nil||tx.CopyPaste==nil||*tx.CopyPaste==""{return GeneratedPIX{Value:value,Status:invoice.Status,Available:false,Message:"Pagamento indisponível no momento. Tente novamente em alguns minutos."},nil};if err:=g.store.SyncInvoicePIX(ctx,invoice.ID,*tx,amountCents);err!=nil{return GeneratedPIX{},err};if err:=g.store.UpsertPendingPayment(ctx,invoice,*tx,value);err!=nil{return GeneratedPIX{},err};gatewayID:="";if tx.GatewayTransactionID!=nil{gatewayID=*tx.GatewayTransactionID};status:=invoice.Status;if tx.Status=="pago"{status="paga"};return GeneratedPIX{Value:value,CopyPaste:*tx.CopyPaste,TXID:gatewayID,Status:status,Available:true,TransactionID:tx.ID,Gateway:tx.GatewaySlug,ExpiresAt:tx.ExpiresAt},nil}

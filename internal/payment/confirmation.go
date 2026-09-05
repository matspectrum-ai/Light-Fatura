package payment

import (
	"context"
	"strings"
	"time"

	"github.com/matspectrum-ai/Claro-Fatura/internal/gateway"
)

type ConfirmationTransaction struct { ID string; InvoiceID string; CustomerID *string; GatewaySlug string; GatewayTransactionID *string; AmountCents int; Status string }
type ConfirmationStore interface {
	GatewayBySlug(context.Context,string)(gateway.Record,bool,error)
	ConfirmationTransaction(context.Context,string)(ConfirmationTransaction,bool,error)
	TransactionByGatewayReference(context.Context,string,string)(ConfirmationTransaction,bool,error)
	MarkTransactionPaid(context.Context,ConfirmationTransaction,time.Time)error
	CancelOtherPendingTransactions(context.Context,string,string,time.Time)error
	MarkInvoicePaid(context.Context,string,time.Time)error
	ConfirmOrInsertPayment(context.Context,ConfirmationTransaction,time.Time)error
	UpdateTransactionStatus(context.Context,string,string)error
	InsertWebhookLog(context.Context,WebhookLog)error
}
type WebhookLog struct{GatewaySlug string;Event *string;GatewayTransaction *string;SignatureValid bool;Summary string}
type Confirmer struct{store ConfirmationStore;registry gateway.Registry;now func()time.Time}
func NewConfirmer(store ConfirmationStore,registry gateway.Registry)*Confirmer{return &Confirmer{store:store,registry:registry,now:time.Now}}
func(c *Confirmer)StatusAtGateway(ctx context.Context,tx ConfirmationTransaction)bool{if tx.GatewayTransactionID==nil||strings.TrimSpace(*tx.GatewayTransactionID)==""{return false};record,found,err:=c.store.GatewayBySlug(ctx,tx.GatewaySlug);if err!=nil||!found{return false};adapter:=c.registry.AdapterFor(record);if adapter==nil{return false};status,err:=adapter.Status(ctx,*tx.GatewayTransactionID,record);if err!=nil{return false};return adapter.Paid(status)}
func(c *Confirmer)Confirm(ctx context.Context,transactionID string)error{tx,found,err:=c.store.ConfirmationTransaction(ctx,transactionID);if err!=nil||!found{return err};if tx.Status=="pago"{return nil};now:=c.now().UTC();if err:=c.store.MarkTransactionPaid(ctx,tx,now);err!=nil{return err};if err:=c.store.CancelOtherPendingTransactions(ctx,tx.InvoiceID,tx.ID,now);err!=nil{return err};if err:=c.store.MarkInvoicePaid(ctx,tx.InvoiceID,now);err!=nil{return err};return c.store.ConfirmOrInsertPayment(ctx,tx,now)}

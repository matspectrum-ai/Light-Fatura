package payment

import "context"

type InvoiceStatusStore interface {
	InvoiceStatus(context.Context,string)(string,bool,error)
	LatestInvoiceTransaction(context.Context,string)(*ConfirmationTransaction,error)
}
type StatusService struct{store InvoiceStatusStore;confirmer *Confirmer}
func NewStatusService(store InvoiceStatusStore,confirmer *Confirmer)*StatusService{return &StatusService{store:store,confirmer:confirmer}}
func(s *StatusService)Invoice(ctx context.Context,invoiceID string)(string,error){status,found,err:=s.store.InvoiceStatus(ctx,invoiceID);if err!=nil{return "",err};if !found{return "em_aberto",nil};if status=="paga"{return "paga",nil};tx,err:=s.store.LatestInvoiceTransaction(ctx,invoiceID);if err!=nil{return "",err};if tx==nil{return statusOrOpen(status),nil};if tx.Status=="pago"{return "paga",nil};if s.confirmer.StatusAtGateway(ctx,*tx){if err:=s.confirmer.Confirm(ctx,tx.ID);err!=nil{return "",err};return "paga",nil};return statusOrOpen(status),nil}
func statusOrOpen(status string)string{if status==""{return "em_aberto"};return status}

package supabase

import (
	"context"
	"net/url"
	"time"

	"github.com/matspectrum-ai/Claro-Fatura/internal/gateway"
	"github.com/matspectrum-ai/Claro-Fatura/internal/payment"
)

func (c *Client) GatewayBySlug(ctx context.Context, slug string) (gateway.Record, bool, error) {
	var rows []gateway.Record
	q:=url.Values{"select":{"id,slug,rotulo,adapter,ativo,prioridade,api_url,ambiente,limite_diario,webhook_url,secret_names,observacoes"},"slug":{"eq."+slug},"limit":{"1"}}
	if err:=c.Select(ctx,"gateways_config",q,&rows);err!=nil{return gateway.Record{},false,err};if len(rows)==0{return gateway.Record{},false,nil};return rows[0],true,nil
}
func (c *Client) ConfirmationTransaction(ctx context.Context,id string)(payment.ConfirmationTransaction,bool,error){return c.confirmationTransaction(ctx,url.Values{"id":{"eq."+id},"limit":{"1"}})}
func (c *Client) TransactionByGatewayReference(ctx context.Context,slug,ref string)(payment.ConfirmationTransaction,bool,error){return c.confirmationTransaction(ctx,url.Values{"gateway_slug":{"eq."+slug},"transacao_gateway_id":{"eq."+ref},"limit":{"1"}})}
func (c *Client) confirmationTransaction(ctx context.Context,q url.Values)(payment.ConfirmationTransaction,bool,error){q.Set("select","id,fatura_id,status,transacao_gateway_id,valor_centavos,cliente_id,gateway_slug");var rows []struct{ID string `json:"id"`;InvoiceID string `json:"fatura_id"`;Status string `json:"status"`;GatewayTransactionID *string `json:"transacao_gateway_id"`;AmountCents int `json:"valor_centavos"`;CustomerID *string `json:"cliente_id"`;GatewaySlug string `json:"gateway_slug"`};if err:=c.Select(ctx,"transacoes_pix",q,&rows);err!=nil{return payment.ConfirmationTransaction{},false,err};if len(rows)==0{return payment.ConfirmationTransaction{},false,nil};r:=rows[0];return payment.ConfirmationTransaction{ID:r.ID,InvoiceID:r.InvoiceID,CustomerID:r.CustomerID,GatewaySlug:r.GatewaySlug,GatewayTransactionID:r.GatewayTransactionID,AmountCents:r.AmountCents,Status:r.Status},true,nil}
func (c *Client) MarkTransactionPaid(ctx context.Context,tx payment.ConfirmationTransaction,at time.Time)error{return c.Update(ctx,"transacoes_pix",url.Values{"id":{"eq."+tx.ID}},map[string]any{"status":"pago","pago_em":at.Format(time.RFC3339Nano),"valor_pago_centavos":tx.AmountCents})}
func (c *Client) CancelOtherPendingTransactions(ctx context.Context,invoiceID,exceptID string,at time.Time)error{return c.Update(ctx,"transacoes_pix",url.Values{"fatura_id":{"eq."+invoiceID},"status":{"eq.pendente"},"id":{"neq."+exceptID}},map[string]any{"status":"substituida","substituida_em":at.Format(time.RFC3339Nano)})}
func (c *Client) MarkInvoicePaid(ctx context.Context,invoiceID string,at time.Time)error{return c.Update(ctx,"faturas",url.Values{"id":{"eq."+invoiceID}},map[string]any{"status":"paga","data_pagamento":at.Format(time.RFC3339Nano)})}
func (c *Client) ConfirmOrInsertPayment(ctx context.Context,tx payment.ConfirmationTransaction,at time.Time)error{var rows []struct{ID string `json:"id"`};q:=url.Values{"select":{"id"},"fatura_id":{"eq."+tx.InvoiceID},"status":{"eq.pendente"},"limit":{"1"}};if err:=c.Select(ctx,"pagamentos",q,&rows);err!=nil{return err};if len(rows)>0{return c.Update(ctx,"pagamentos",url.Values{"id":{"eq."+rows[0].ID}},map[string]any{"status":"confirmado","pago_em":at.Format(time.RFC3339Nano)})};return c.Insert(ctx,"pagamentos",map[string]any{"fatura_id":tx.InvoiceID,"cliente_id":tx.CustomerID,"valor":float64(tx.AmountCents)/100,"metodo":"pix","status":"confirmado","gateway":tx.GatewaySlug,"gateway_payment_id":tx.GatewayTransactionID,"pago_em":at.Format(time.RFC3339Nano)})}
func (c *Client) UpdateTransactionStatus(ctx context.Context,id,status string)error{switch status{case "paid","approved","completed","success","succeeded","done","confirmado","realizado","finalizado":status="pago";case "expired":status="expirada";case "failed","error","canceled","cancelled":status="falhou";default:status="pendente"};return c.Update(ctx,"transacoes_pix",url.Values{"id":{"eq."+id}},map[string]any{"status":status})}
func (c *Client) InsertWebhookLog(ctx context.Context,log payment.WebhookLog)error{return c.Insert(ctx,"webhooks_log",map[string]any{"gateway_slug":log.GatewaySlug,"payload":map[string]any{"event":log.Event,"summary":log.Summary},"valido":log.SignatureValid,"transacao_gateway_id":log.GatewayTransaction,"status":log.Summary})}

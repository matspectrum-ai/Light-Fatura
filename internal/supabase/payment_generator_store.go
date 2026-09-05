package supabase

import (
	"context"
	"net/url"

	"github.com/matspectrum-ai/Claro-Fatura/internal/payment"
)

func (c *Client) PaymentInvoice(ctx context.Context, id string) (payment.InvoiceForPayment, bool, error) {
	var rows []struct { ID string `json:"id"`; CustomerID string `json:"cliente_id"`; Description *string `json:"descricao"`; DiscountAmount float64 `json:"valor_desconto"`; Status string `json:"status"` }
	q:=url.Values{"select":{"id,cliente_id,descricao,valor_desconto,status"},"id":{"eq."+id},"limit":{"1"}}
	if err:=c.Select(ctx,"faturas",q,&rows);err!=nil{return payment.InvoiceForPayment{},false,err};if len(rows)==0{return payment.InvoiceForPayment{},false,nil};description:="";if rows[0].Description!=nil{description=*rows[0].Description};return payment.InvoiceForPayment{ID:rows[0].ID,CustomerID:rows[0].CustomerID,Description:description,DiscountAmount:rows[0].DiscountAmount,Status:rows[0].Status},true,nil
}
func (c *Client) NewPIXPerAccess(ctx context.Context)(bool,error){var rows []struct{Value *bool `json:"novo_pix_por_acesso"`};q:=url.Values{"select":{"novo_pix_por_acesso"},"id":{"eq.true"},"limit":{"1"}};if err:=c.Select(ctx,"roteamento_config",q,&rows);err!=nil{return true,err};if len(rows)==0||rows[0].Value==nil{return true,nil};return *rows[0].Value,nil}
func (c *Client) LatestPendingTransaction(ctx context.Context,invoiceID string)(*payment.Transaction,error){var rows []transactionRow;q:=url.Values{"select":{"id,fatura_id,cliente_id,gateway_slug,gateway_id,transacao_gateway_id,valor_centavos,copia_cola,qrcode,status,expira_em"},"fatura_id":{"eq."+invoiceID},"status":{"eq.pendente"},"substituida_em":{"is.null"},"order":{"created_at.desc"},"limit":{"1"}};if err:=c.Select(ctx,"transacoes_pix",q,&rows);err!=nil{return nil,err};if len(rows)==0{return nil,nil};tx:=transaction(rows[0]);return &tx,nil}
func (c *Client) ExpireTransaction(ctx context.Context,id string)error{return c.Update(ctx,"transacoes_pix",url.Values{"id":{"eq."+id}},map[string]any{"status":"expirada"})}
func (c *Client) PaymentCustomer(ctx context.Context,id string)(payment.CustomerForPayment,error){var rows []struct{Name string `json:"nome"`;Phone *string `json:"telefone"`;Email *string `json:"email"`;Document *string `json:"documento"`};q:=url.Values{"select":{"nome,telefone,email,documento"},"id":{"eq."+id},"limit":{"1"}};if err:=c.Select(ctx,"clientes",q,&rows);err!=nil{return payment.CustomerForPayment{},err};if len(rows)==0{return payment.CustomerForPayment{Name:"Cliente"},nil};phone:="";if rows[0].Phone!=nil{phone=*rows[0].Phone};name:=rows[0].Name;if name==""{name="Cliente"};return payment.CustomerForPayment{Name:name,Phone:phone,Email:rows[0].Email,Document:rows[0].Document},nil}
func (c *Client) SyncInvoicePIX(ctx context.Context,invoiceID string,tx payment.Transaction,amountCents int)error{return c.Update(ctx,"faturas",url.Values{"id":{"eq."+invoiceID}},map[string]any{"pix_txid":tx.GatewayTransactionID,"pix_copia_cola":tx.CopyPaste,"pix_valor_centavos":amountCents})}
func (c *Client) UpsertPendingPayment(ctx context.Context,invoice payment.InvoiceForPayment,tx payment.Transaction,value float64)error{var rows []struct{ID string `json:"id"`};q:=url.Values{"select":{"id"},"fatura_id":{"eq."+invoice.ID},"status":{"eq.pendente"},"limit":{"1"}};if err:=c.Select(ctx,"pagamentos",q,&rows);err!=nil{return err};body:=map[string]any{"valor":value,"gateway":tx.GatewaySlug,"gateway_payment_id":tx.GatewayTransactionID};if len(rows)>0{return c.Update(ctx,"pagamentos",url.Values{"id":{"eq."+rows[0].ID}},body)};body["fatura_id"]=invoice.ID;body["cliente_id"]=invoice.CustomerID;body["metodo"]="pix";body["status"]="pendente";return c.Insert(ctx,"pagamentos",body)}

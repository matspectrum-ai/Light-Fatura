package payment

import (
	"context"
	"fmt"
	"net/http"
	"strings"
)

type WebhookResult struct{Status int;Message string;OK bool}
type WebhookService struct{confirmer *Confirmer}
func NewWebhookService(confirmer *Confirmer)*WebhookService{return &WebhookService{confirmer:confirmer}}
func(s *WebhookService)Handle(ctx context.Context,req *http.Request,slug string,raw []byte)WebhookResult{record,found,err:=s.confirmer.store.GatewayBySlug(ctx,slug);if err!=nil||!found{return WebhookResult{Status:http.StatusNotFound,Message:"Gateway desconhecida"}};adapter:=s.confirmer.registry.AdapterFor(record);if adapter==nil{return WebhookResult{Status:http.StatusNotFound,Message:"Gateway desconhecida"}};read,err:=adapter.ReadWebhook(req,raw,record);if err!=nil{read.Valid=false};_ = s.confirmer.store.InsertWebhookLog(ctx,WebhookLog{GatewaySlug:slug,Event:read.Event,GatewayTransaction:read.TransactionID,SignatureValid:read.Valid,Summary:fmt.Sprintf("status=%s",derefStatus(read.Status))});if !read.Valid{return WebhookResult{Status:http.StatusUnauthorized,Message:"Assinatura inválida"}};if read.TransactionID==nil||strings.TrimSpace(*read.TransactionID)==""{return WebhookResult{Status:http.StatusBadRequest,Message:"Transação ausente"}};tx,found,err:=s.confirmer.store.TransactionByGatewayReference(ctx,slug,*read.TransactionID);if err!=nil||!found{return WebhookResult{Status:http.StatusNotFound,Message:"Transação não encontrada"}};if adapter.Paid(read.Status){if !s.confirmer.StatusAtGateway(ctx,tx){return WebhookResult{Status:http.StatusAccepted,Message:"Pagamento ainda não confirmado pela gateway"}};if err:=s.confirmer.Confirm(ctx,tx.ID);err!=nil{return WebhookResult{Status:http.StatusInternalServerError,Message:"Falha ao confirmar pagamento"}}}else if read.Status!=nil{_ = s.confirmer.store.UpdateTransactionStatus(ctx,tx.ID,strings.ToLower(*read.Status))};return WebhookResult{Status:http.StatusOK,OK:true}}
func derefStatus(v *string)string{if v==nil{return "-"};return *v}

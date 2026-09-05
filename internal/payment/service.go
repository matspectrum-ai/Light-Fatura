package payment

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/matspectrum-ai/Claro-Fatura/internal/gateway"
)

var (
	ErrAlreadyProcessing = errors.New("esta solicitação PIX já está sendo processada")
	ErrRequestUsed       = errors.New("esta solicitação PIX já foi utilizada")
	ErrInvoicePaid       = errors.New("a fatura já está paga")
	ErrInvoiceNotFound   = errors.New("fatura não encontrada")
)

type Transaction struct {
	ID                   string  `json:"id"`
	InvoiceID            string  `json:"fatura_id,omitempty"`
	CustomerID           *string `json:"cliente_id,omitempty"`
	GatewaySlug          string  `json:"gateway_slug"`
	GatewayID            string  `json:"gateway_id,omitempty"`
	GatewayTransactionID *string `json:"transacao_gateway_id"`
	AmountCents          int     `json:"valor_centavos"`
	CopyPaste            *string `json:"copia_cola"`
	QRCode               *string `json:"qrcode"`
	Status               string  `json:"status"`
	ExpiresAt            *string `json:"expira_em"`
}

type RequestReservation struct { ID string; Status string; TransactionID *string }
type RoutingConfig struct { Strategy string; FixedGateway *string; Pointer int }

type Store interface {
	ReserveRequest(context.Context, string, string) (RequestReservation, bool, error)
	RequestTransaction(context.Context, RequestReservation) (*Transaction, error)
	CompleteRequest(context.Context, string, string) error
	FailRequest(context.Context, string, string) error
	InvoiceStatus(context.Context, string) (string, bool, error)
	ActiveGateways(context.Context) ([]gateway.Record, error)
	Routing(context.Context) (RoutingConfig, error)
	AdvanceGatewayPointer(context.Context, int) (int, error)
	GatewayTransactionsSince(context.Context, string, time.Time) (int, error)
	InsertTransaction(context.Context, TransactionCreate) (Transaction, error)
	ReplaceOtherPending(context.Context, string, string, time.Time) error
	Log(context.Context, LogEntry) error
}

type TransactionCreate struct {
	InvoiceID string; CustomerID *string; GatewaySlug string; GatewayID string; GatewayTransactionID *string
	AmountCents int; CopyPaste string; QRCode *string; Status string; IdempotencyKey string; ExpiresAt string
}

type LogEntry struct { GatewaySlug string; InvoiceID *string; Level string; HTTPStatus *int; Message string }

type CreateRequest struct {
	InvoiceID string; CustomerID *string; AmountCents int; Name string; Phone string; Email *string; Document *string
	Description string; BaseURL string; RequestKey string
}

type Service struct { store Store; gateways gateway.Registry; now func() time.Time; expiration time.Duration; productName string }

func New(store Store, gateways gateway.Registry, productName string) *Service {
	if strings.TrimSpace(productName)=="" { productName="Fatura" }
	return &Service{store:store,gateways:gateways,now:time.Now,expiration:30*time.Minute,productName:productName}
}
func NewWithExpiration(store Store, gateways gateway.Registry, productName string, expiration time.Duration) *Service {
	s:=New(store,gateways,productName); if expiration>0{s.expiration=expiration}; return s
}

func (s *Service) CreatePIX(ctx context.Context, request CreateRequest) (*Transaction,error) {
	reservation,created,err:=s.store.ReserveRequest(ctx,request.RequestKey,request.InvoiceID)
	if err!=nil{return nil,fmt.Errorf("não foi possível iniciar a geração do PIX: %w",err)}
	if !created {
		existing,err:=s.store.RequestTransaction(ctx,reservation); if err!=nil{return nil,fmt.Errorf("não foi possível recuperar a solicitação do PIX: %w",err)}
		if existing!=nil{return existing,nil}; if reservation.Status=="processando"{return nil,ErrAlreadyProcessing}; return nil,ErrRequestUsed
	}
	transaction,err:=s.createReserved(ctx,reservation,request); if err!=nil{_ = s.store.FailRequest(ctx,reservation.ID,err.Error()); return nil,err}
	if transaction==nil{_ = s.store.FailRequest(ctx,reservation.ID,"Nenhum gateway conseguiu gerar o PIX."); return nil,nil}
	if err:=s.store.CompleteRequest(ctx,reservation.ID,transaction.ID);err!=nil{return nil,fmt.Errorf("PIX criado, mas não foi possível concluir a solicitação: %w",err)}
	return transaction,nil
}

func (s *Service) createReserved(ctx context.Context,reservation RequestReservation,request CreateRequest)(*Transaction,error){
	status,found,err:=s.store.InvoiceStatus(ctx,request.InvoiceID); if err!=nil{return nil,err}; if !found{return nil,ErrInvoiceNotFound}; if status=="paga"{return nil,ErrInvoicePaid}
	order,err:=s.attemptOrder(ctx); if err!=nil{return nil,err}; if len(order)==0{s.log(ctx,LogEntry{GatewaySlug:"-",InvoiceID:&request.InvoiceID,Level:"erro",Message:"Nenhuma gateway ativa disponível."});return nil,nil}
	for _,record:=range order{
		adapter:=s.gateways.AdapterFor(record); if adapter==nil||!adapter.Configured(record){s.log(ctx,LogEntry{GatewaySlug:record.Slug,InvoiceID:&request.InvoiceID,Level:"aviso",Message:"Credenciais ausentes — gateway ignorada."});continue}
		if record.DailyLimit!=nil&&*record.DailyLimit>0{start:=s.now().UTC().Truncate(24*time.Hour);count,e:=s.store.GatewayTransactionsSince(ctx,record.Slug,start);if e!=nil{s.log(ctx,LogEntry{GatewaySlug:record.Slug,InvoiceID:&request.InvoiceID,Level:"erro",Message:e.Error()});continue};if count>=*record.DailyLimit{s.log(ctx,LogEntry{GatewaySlug:record.Slug,InvoiceID:&request.InvoiceID,Level:"aviso",Message:"Limite diário atingido — gateway ignorada."});continue}}
		webhookURL:=strings.TrimSpace(request.BaseURL)+"/api/public/webhooks/"+record.Slug;if record.WebhookURL!=nil&&strings.TrimSpace(*record.WebhookURL)!=""{webhookURL=*record.WebhookURL}
		created,e:=adapter.CreatePIX(ctx,gateway.CreateInput{Gateway:record,AmountCents:request.AmountCents,Name:request.Name,Phone:request.Phone,Email:request.Email,Document:request.Document,Description:s.productName,Reference:request.RequestKey,WebhookURL:webhookURL});if e!=nil{s.log(ctx,LogEntry{GatewaySlug:record.Slug,InvoiceID:&request.InvoiceID,Level:"erro",Message:e.Error()});continue};if strings.TrimSpace(created.CopyPaste)==""{s.log(ctx,LogEntry{GatewaySlug:record.Slug,InvoiceID:&request.InvoiceID,Level:"erro",Message:"Gateway não devolveu o código PIX."});continue}
		expires:=created.ExpiresAt;if expires==nil{value:=s.now().Add(s.expiration).UTC().Format(time.RFC3339Nano);expires=&value};var gatewayTx *string;if created.TransactionID!=""{gatewayTx=&created.TransactionID}
		inserted,e:=s.store.InsertTransaction(ctx,TransactionCreate{InvoiceID:request.InvoiceID,CustomerID:request.CustomerID,GatewaySlug:record.Slug,GatewayID:record.ID,GatewayTransactionID:gatewayTx,AmountCents:request.AmountCents,CopyPaste:created.CopyPaste,QRCode:created.QRCode,Status:"pendente",IdempotencyKey:request.RequestKey,ExpiresAt:*expires});if e!=nil{s.log(ctx,LogEntry{GatewaySlug:record.Slug,InvoiceID:&request.InvoiceID,Level:"erro",Message:e.Error()});continue};_ = s.store.ReplaceOtherPending(ctx,request.InvoiceID,inserted.ID,s.now());s.log(ctx,LogEntry{GatewaySlug:record.Slug,InvoiceID:&request.InvoiceID,Level:"info",Message:fmt.Sprintf("PIX criado (%d centavos).",request.AmountCents)});return &inserted,nil
	};return nil,nil
}

func (s *Service) attemptOrder(ctx context.Context)([]gateway.Record,error){
	active,err:=s.store.ActiveGateways(ctx);if err!=nil{return nil,err};sort.SliceStable(active,func(i,j int)bool{return active[i].Priority<active[j].Priority});if len(active)==0{return nil,nil};cfg,err:=s.store.Routing(ctx);if err!=nil{return nil,err};switch cfg.Strategy{case "fixa":if cfg.FixedGateway==nil{return active,nil};for _,record:=range active{if record.ID==*cfg.FixedGateway{return []gateway.Record{record},nil}};return nil,nil;case "rodizio":position,e:=s.store.AdvanceGatewayPointer(ctx,len(active));if e!=nil{position=cfg.Pointer};if position<0{position=-position};start:=position%len(active);return append(append([]gateway.Record{},active[start:]...),active[:start]...),nil;default:return active,nil}
}
func (s *Service) log(ctx context.Context,entry LogEntry){if len(entry.Message)>500{entry.Message=entry.Message[:500]};_ = s.store.Log(ctx,entry)}

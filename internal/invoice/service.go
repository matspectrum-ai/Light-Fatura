package invoice

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"unicode"
)

var ErrInvalidDocument = errors.New("documento inválido")

type Customer struct {
	ID          string
	Document    string
	Name        string
	Phone       string
	Email       string
	Observacoes string
}

type Invoice struct {
	ID              string  `json:"id"`
	CustomerID      string  `json:"cliente_id"`
	Description     string  `json:"descricao"`
	Reference       string  `json:"referencia"`
	OriginalValue   float64 `json:"valor_original"`
	DiscountedValue float64 `json:"valor_desconto"`
	DueDate         string  `json:"vencimento"`
	Status          string  `json:"status"`
}

type Store interface {
	CustomerByDocument(context.Context, string) (Customer, bool, error)
	LatestOpenInvoice(context.Context, string) (Invoice, bool, error)
	LogAccess(context.Context, string, bool, float64, float64) error
}

type LightMeta struct {
	CTE          string `json:"cte,omitempty"`
	Installation string `json:"instalacao,omitempty"`
	Address      string `json:"endereco,omitempty"`
	Street       string `json:"endereco_rua,omitempty"`
	District     string `json:"bairro,omitempty"`
	City         string `json:"cidade,omitempty"`
	State        string `json:"estado,omitempty"`
	PostalCode   string `json:"cep,omitempty"`
	Contract     string `json:"contrato,omitempty"`
	Account      string `json:"conta_contrato,omitempty"`
	MonthRef     string `json:"mes_ref,omitempty"`
	Installment  string `json:"parcneg,omitempty"`
}

type PublicUser struct {
	Code         string         `json:"codigo"`
	CPF          string         `json:"cpf"`
	Name         string         `json:"nome"`
	Original     float64        `json:"valor_taxa"`
	Phone        string         `json:"telefone"`
	Email        string         `json:"email,omitempty"`
	CTE          string         `json:"cte,omitempty"`
	Installation string         `json:"instalacao,omitempty"`
	Address      string         `json:"endereco,omitempty"`
	Street       string         `json:"endereco_rua,omitempty"`
	District     string         `json:"bairro,omitempty"`
	City         string         `json:"cidade,omitempty"`
	State        string         `json:"estado,omitempty"`
	PostalCode   string         `json:"cep,omitempty"`
	Light        map[string]string `json:"light,omitempty"`
}

type QueryResult struct {
	Found           bool       `json:"encontrado"`
	User            PublicUser `json:"usuario,omitempty"`
	InvoiceID       string     `json:"fatura_id,omitempty"`
	DiscountedValue float64    `json:"valor_desconto,omitempty"`
	DueDate         string     `json:"vencimento,omitempty"`
	Status          string     `json:"status,omitempty"`
}

type Service struct{ store Store }

func New(store Store) *Service { return &Service{store: store} }

func (s *Service) QueryByDocument(ctx context.Context, document string) (QueryResult, error) {
	document = digits(document)
	if len(document) != 11 { return QueryResult{}, ErrInvalidDocument }
	customer, ok, err := s.store.CustomerByDocument(ctx, document)
	if err != nil { return QueryResult{}, err }
	if !ok {
		_ = s.store.LogAccess(ctx, document, false, 0, 0)
		return QueryResult{Found:false}, nil
	}
	inv, hasInvoice, err := s.store.LatestOpenInvoice(ctx, customer.ID)
	if err != nil { return QueryResult{}, err }
	if !hasInvoice {
		_ = s.store.LogAccess(ctx, document, true, 0, 0)
		return QueryResult{Found:true, User: publicUser(customer, Invoice{})}, nil
	}
	_ = s.store.LogAccess(ctx, document, true, inv.OriginalValue, inv.DiscountedValue)
	return QueryResult{
		Found:true, User:publicUser(customer, inv), InvoiceID:inv.ID,
		DiscountedValue:inv.DiscountedValue, DueDate:inv.DueDate, Status:inv.Status,
	}, nil
}

func publicUser(customer Customer, inv Invoice) PublicUser {
	meta := LightMeta{}
	if strings.TrimSpace(customer.Observacoes) != "" { _ = json.Unmarshal([]byte(customer.Observacoes), &meta) }
	light := map[string]string{}
	if meta.Contract != "" { light["contrato"] = meta.Contract }
	if meta.Account != "" { light["conta_contrato"] = meta.Account }
	if meta.MonthRef != "" { light["mes_ref"] = meta.MonthRef }
	if meta.Installment != "" { light["parcneg"] = meta.Installment }
	return PublicUser{
		Code:customer.Document, CPF:customer.Document, Name:customer.Name, Original:inv.OriginalValue,
		Phone:customer.Phone, Email:customer.Email, CTE:meta.CTE, Installation:meta.Installation,
		Address:meta.Address, Street:meta.Street, District:meta.District, City:meta.City, State:meta.State,
		PostalCode:meta.PostalCode, Light:light,
	}
}

func digits(value string) string {
	var b strings.Builder
	for _, r := range value { if unicode.IsDigit(r) { b.WriteRune(r) } }
	return b.String()
}

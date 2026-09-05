package httpapi

import (
	"log/slog"
	"net/http"

	"github.com/matspectrum-ai/Claro-Fatura/internal/admin"
)

type FullDependencies struct {
	Public Dependencies
	Auth   AdminAuth
	Admin  *admin.Service
}

type FullServer struct {
	base    http.Handler
	auth    AdminAuth
	admin   *admin.Service
	siteURL string
	logger  *slog.Logger
}

func NewFull(deps FullDependencies, logger *slog.Logger) http.Handler {
	s:=&FullServer{base:New(deps.Public,logger),auth:deps.Auth,admin:deps.Admin,siteURL:deps.Public.SiteURL,logger:logger}
	mux:=http.NewServeMux()
	mux.HandleFunc("GET /auth",s.authPage)
	mux.HandleFunc("GET /forgot-password",s.forgotPage)
	mux.HandleFunc("GET /reset-password",s.resetPage)
	mux.HandleFunc("GET /admin",s.adminPage)
	mux.HandleFunc("GET /admin/{$}",s.adminPage)
	mux.HandleFunc("GET /admin/faturas",s.adminPage)
	mux.HandleFunc("GET /admin/pagamentos",s.adminPage)
	mux.HandleFunc("GET /admin/transacoes",s.adminPage)
	mux.HandleFunc("GET /admin/gateways",s.adminPage)
	mux.HandleFunc("GET /admin/logs",s.adminPage)
	mux.HandleFunc("POST /api/auth/login",s.login)
	mux.HandleFunc("GET /api/auth/me",s.authMe)
	mux.HandleFunc("POST /api/auth/logout",s.logout)
	mux.HandleFunc("POST /api/auth/recover",s.recoverPassword)
	mux.HandleFunc("POST /api/auth/password",s.updatePassword)
	mux.HandleFunc("GET /api/admin/metricas",s.adminMetrics)
	mux.HandleFunc("GET /api/admin/faturas",s.adminInvoices)
	mux.HandleFunc("PATCH /api/admin/faturas/{id}",s.adminSaveInvoice)
	mux.HandleFunc("PATCH /api/admin/faturas/{id}/status",s.adminInvoiceStatus)
	mux.HandleFunc("DELETE /api/admin/base",s.adminDeleteAll)
	mux.HandleFunc("POST /api/admin/importar-csv",s.adminImportCSV)
	mux.HandleFunc("GET /api/admin/pagamentos",s.adminPayments)
	mux.HandleFunc("GET /api/admin/transacoes",s.adminTransactions)
	mux.HandleFunc("GET /api/admin/logs",s.adminLogs)
	mux.HandleFunc("GET /api/admin/gateways",s.adminGateways)
	mux.HandleFunc("PATCH /api/admin/gateways/{id}",s.adminSaveGateway)
	mux.HandleFunc("DELETE /api/admin/gateways/{id}",s.adminDeleteGateway)
	mux.HandleFunc("POST /api/admin/gateways/{id}/somente",s.adminUseOnlyGateway)
	mux.HandleFunc("GET /api/admin/roteamento",s.adminRouting)
	mux.HandleFunc("POST /api/admin/roteamento",s.adminSaveRouting)
	mux.Handle("/",s.base)
	return mux
}

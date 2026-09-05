package httpapi

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"os"
	"path"
	"strings"
	"time"

	"github.com/matspectrum-ai/Light-Fatura/internal/invoice"
	"github.com/matspectrum-ai/Light-Fatura/internal/payment"
)

type DocumentLookup interface {
	QueryByDocument(context.Context, string) (invoice.QueryResult, error)
}
type PIXGenerator interface {
	Generate(context.Context, payment.GenerateInput) (payment.GeneratedPIX, error)
}
type InvoiceStatus interface {
	Invoice(context.Context, string) (string, error)
}
type WebhookHandler interface {
	Handle(context.Context, *http.Request, string, []byte) payment.WebhookResult
}

type Dependencies struct {
	Invoices DocumentLookup
	PIX      PIXGenerator
	Status   InvoiceStatus
	Webhooks WebhookHandler
	SiteURL  string
	LightDir string
}
type Server struct {
	deps   Dependencies
	logger *slog.Logger
}

func New(deps Dependencies, logger *slog.Logger) http.Handler {
	if strings.TrimSpace(deps.LightDir) == "" {
		deps.LightDir = "Light"
	}
	s := &Server{deps: deps, logger: logger}
	mux := http.NewServeMux()
	mux.HandleFunc("GET /", s.home)
	mux.HandleFunc("GET /index.html", s.home)
	mux.HandleFunc("GET /pagamento.html", s.paymentPage)
	mux.HandleFunc("GET /assets/{path...}", s.asset)
	mux.HandleFunc("GET /healthz", s.health)
	mux.HandleFunc("GET /api/v1/faturas", s.queryInvoices)
	mux.HandleFunc("POST /api/v1/faturas/{id}/pix", s.generatePIX)
	mux.HandleFunc("POST /api/v1/faturas/{id}/status", s.invoiceStatus)
	mux.HandleFunc("POST /api/public/webhooks/{slug}", s.webhook)
	return securityHeaders(accessLog(logger, mux))
}

func (s *Server) home(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path != "/" && r.URL.Path != "/index.html" {
		http.NotFound(w, r)
		return
	}
	s.serveHTMLWithBridge(w, path.Join(s.deps.LightDir, "index.html"), "/assets/light-api.js")
}
func (s *Server) paymentPage(w http.ResponseWriter, r *http.Request) {
	s.serveHTMLWithBridge(w, path.Join(s.deps.LightDir, "pagamento.html"), "/assets/light-payment.js")
}
func (s *Server) asset(w http.ResponseWriter, r *http.Request) {
	name := strings.TrimSpace(r.PathValue("path"))
	if name == "" || strings.Contains(name, "..") || strings.HasPrefix(name, "/") {
		http.NotFound(w, r)
		return
	}
	if name == "light-api.js" {
		writeJS(w, lightAPIBridge)
		return
	}
	if name == "light-payment.js" {
		writeJS(w, lightPaymentBridge)
		return
	}
	http.ServeFile(w, r, path.Join(s.deps.LightDir, "assets", name))
}
func (s *Server) serveHTMLWithBridge(w http.ResponseWriter, file, script string) {
	raw, err := os.ReadFile(file)
	if err != nil {
		http.NotFound(w, nil)
		return
	}
	html := string(raw)
	tag := "<script defer src=\"" + script + "\"></script>"
	if i := strings.LastIndex(strings.ToLower(html), "</body>"); i >= 0 {
		html = html[:i] + tag + html[i:]
	} else {
		html += tag
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Header().Set("Cache-Control", "no-cache")
	_, _ = io.WriteString(w, html)
}
func (s *Server) health(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, 200, map[string]string{"status": "ok"})
}
func (s *Server) queryInvoices(w http.ResponseWriter, r *http.Request) {
	document := strings.TrimSpace(r.URL.Query().Get("documento"))
	result, err := s.deps.Invoices.QueryByDocument(r.Context(), document)
	if err != nil {
		status := 500
		if errors.Is(err, invoice.ErrInvalidDocument) {
			status = 400
		}
		writeJSON(w, status, map[string]string{"erro": publicError(status)})
		return
	}
	writeJSON(w, 200, result)
}
func (s *Server) generatePIX(w http.ResponseWriter, r *http.Request) {
	invoiceID := strings.TrimSpace(r.PathValue("id"))
	if !isUUID(invoiceID) {
		writeJSON(w, 400, map[string]string{"erro": "Fatura inválida."})
		return
	}
	var body struct {
		RequestKey string `json:"request_key"`
		Force      bool   `json:"forcar"`
	}
	if err := decodeJSON(r, &body); err != nil || !isUUID(body.RequestKey) {
		writeJSON(w, 400, map[string]string{"erro": "Dados inválidos."})
		return
	}
	result, err := s.deps.PIX.Generate(r.Context(), payment.GenerateInput{InvoiceID: invoiceID, RequestKey: body.RequestKey, Force: body.Force, BaseURL: requestBaseURL(r, s.deps.SiteURL)})
	if err != nil {
		switch {
		case errors.Is(err, payment.ErrInvoiceNotFound):
			writeJSON(w, 404, map[string]string{"erro": "Fatura não encontrada."})
		case errors.Is(err, payment.ErrAlreadyProcessing), errors.Is(err, payment.ErrRequestUsed):
			writeJSON(w, 409, map[string]string{"erro": err.Error()})
		default:
			s.logger.Error("pix generation failed", "invoice_id", invoiceID, "error", err)
			writeJSON(w, 500, map[string]string{"erro": "Não foi possível gerar o PIX agora."})
		}
		return
	}
	writeJSON(w, 200, result)
}
func (s *Server) invoiceStatus(w http.ResponseWriter, r *http.Request) {
	invoiceID := strings.TrimSpace(r.PathValue("id"))
	if !isUUID(invoiceID) {
		writeJSON(w, 400, map[string]string{"erro": "Fatura inválida."})
		return
	}
	status, err := s.deps.Status.Invoice(r.Context(), invoiceID)
	if err != nil {
		writeJSON(w, 500, map[string]string{"erro": "Não foi possível consultar no momento."})
		return
	}
	writeJSON(w, 200, map[string]string{"status": status})
}
func (s *Server) webhook(w http.ResponseWriter, r *http.Request) {
	slug := strings.TrimSpace(r.PathValue("slug"))
	raw, err := io.ReadAll(io.LimitReader(r.Body, 1<<20))
	if err != nil {
		writeText(w, 400, "Corpo inválido")
		return
	}
	result := s.deps.Webhooks.Handle(r.Context(), r, slug, raw)
	if result.OK {
		writeJSON(w, result.Status, map[string]bool{"ok": true})
		return
	}
	writeText(w, result.Status, result.Message)
}
func decodeJSON(r *http.Request, dst any) error {
	decoder := json.NewDecoder(io.LimitReader(r.Body, 64<<10))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(dst); err != nil {
		return err
	}
	var trailing any
	if err := decoder.Decode(&trailing); err != io.EOF {
		return errors.New("multiple JSON values")
	}
	return nil
}
func requestBaseURL(r *http.Request, fallback string) string {
	if strings.TrimSpace(fallback) != "" {
		return strings.TrimRight(fallback, "/")
	}
	scheme := "http"
	if r.TLS != nil {
		scheme = "https"
	}
	if forwarded := strings.TrimSpace(strings.Split(r.Header.Get("X-Forwarded-Proto"), ",")[0]); forwarded == "http" || forwarded == "https" {
		scheme = forwarded
	}
	return scheme + "://" + r.Host
}
func isUUID(value string) bool {
	if len(value) != 36 || value[8] != '-' || value[13] != '-' || value[18] != '-' || value[23] != '-' {
		return false
	}
	for i, ch := range value {
		if i == 8 || i == 13 || i == 18 || i == 23 {
			continue
		}
		if !((ch >= '0' && ch <= '9') || (ch >= 'a' && ch <= 'f') || (ch >= 'A' && ch <= 'F')) {
			return false
		}
	}
	return true
}
func publicError(status int) string {
	if status == 400 {
		return "CPF inválido"
	}
	return "Não foi possível consultar no momento."
}
func writeJSON(w http.ResponseWriter, status int, value any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.Header().Set("Cache-Control", "no-store")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(value)
}
func writeText(w http.ResponseWriter, status int, value string) {
	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	w.Header().Set("Cache-Control", "no-store")
	w.WriteHeader(status)
	_, _ = io.WriteString(w, value)
}
func writeJS(w http.ResponseWriter, value string) {
	w.Header().Set("Content-Type", "text/javascript; charset=utf-8")
	w.Header().Set("Cache-Control", "no-cache")
	_, _ = io.WriteString(w, value)
}
func securityHeaders(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("X-Content-Type-Options", "nosniff")
		w.Header().Set("X-Frame-Options", "DENY")
		w.Header().Set("Referrer-Policy", "strict-origin-when-cross-origin")
		next.ServeHTTP(w, r)
	})
}
func accessLog(logger *slog.Logger, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		started := time.Now()
		next.ServeHTTP(w, r)
		logger.Info("http request", "method", r.Method, "path", r.URL.Path, "duration_ms", time.Since(started).Milliseconds())
	})
}

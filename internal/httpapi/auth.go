package httpapi

import (
	"context"
	"errors"
	"net/http"
	"strings"
	"time"

	"github.com/matspectrum-ai/Light-Fatura/internal/auth"
)

type AdminAuth interface {
	Configured() bool
	SignIn(context.Context, string, string) (auth.Session, error)
	User(context.Context, string) (auth.User, error)
	Refresh(context.Context, string) (auth.Session, error)
	RequireAdmin(context.Context, string) (auth.User, error)
	SendRecovery(context.Context, string, string) error
	UpdatePassword(context.Context, string, string) error
	Logout(context.Context, string) error
}

const accessCookie = "lf_admin_access"
const refreshCookie = "lf_admin_refresh"

func (s *FullServer) login(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Email    string `json:"email"`
		Password string `json:"senha"`
	}
	if decodeJSON(r, &body) != nil {
		writeJSON(w, 400, map[string]string{"erro": "Dados inválidos."})
		return
	}
	session, err := s.auth.SignIn(r.Context(), body.Email, body.Password)
	if err != nil {
		status := 500
		if errors.Is(err, auth.ErrInvalidCredentials) {
			status = 401
		} else if errors.Is(err, auth.ErrNotAdmin) {
			status = 403
		}
		writeJSON(w, status, map[string]string{"erro": authMessage(status)})
		return
	}
	s.setSession(w, r, session)
	writeJSON(w, 200, map[string]bool{"ok": true})
}
func (s *FullServer) authMe(w http.ResponseWriter, r *http.Request) {
	user, ok := s.adminUser(w, r)
	if !ok {
		return
	}
	writeJSON(w, 200, user)
}
func (s *FullServer) logout(w http.ResponseWriter, r *http.Request) {
	if c, err := r.Cookie(accessCookie); err == nil {
		_ = s.auth.Logout(r.Context(), c.Value)
	}
	clearCookie(w, accessCookie)
	clearCookie(w, refreshCookie)
	writeJSON(w, 200, map[string]bool{"ok": true})
}
func (s *FullServer) recoverPassword(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Email string `json:"email"`
	}
	if decodeJSON(r, &body) != nil || !strings.Contains(body.Email, "@") {
		writeJSON(w, 400, map[string]string{"erro": "E-mail inválido."})
		return
	}
	redirect := strings.TrimRight(requestBaseURL(r, s.siteURL), "/") + "/reset-password"
	if err := s.auth.SendRecovery(r.Context(), body.Email, redirect); err != nil {
		writeJSON(w, 500, map[string]string{"erro": "Não foi possível enviar a recuperação."})
		return
	}
	writeJSON(w, 200, map[string]bool{"ok": true})
}
func (s *FullServer) updatePassword(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Password string `json:"senha"`
	}
	if decodeJSON(r, &body) != nil || len(body.Password) < 6 {
		writeJSON(w, 400, map[string]string{"erro": "Senha inválida."})
		return
	}
	c, err := r.Cookie(accessCookie)
	if err != nil {
		writeJSON(w, 401, map[string]string{"erro": "Sessão ausente."})
		return
	}
	if err := s.auth.UpdatePassword(r.Context(), c.Value, body.Password); err != nil {
		writeJSON(w, 500, map[string]string{"erro": "Não foi possível atualizar a senha."})
		return
	}
	writeJSON(w, 200, map[string]bool{"ok": true})
}
func (s *FullServer) adminUser(w http.ResponseWriter, r *http.Request) (auth.User, bool) {
	if s.auth == nil || !s.auth.Configured() {
		writeJSON(w, 503, map[string]string{"erro": "Autenticação não configurada."})
		return auth.User{}, false
	}
	if c, err := r.Cookie(accessCookie); err == nil && c.Value != "" {
		if user, err := s.auth.RequireAdmin(r.Context(), c.Value); err == nil {
			return user, true
		}
	}
	refresh, err := r.Cookie(refreshCookie)
	if err != nil || refresh.Value == "" {
		writeJSON(w, 401, map[string]string{"erro": "Sessão expirada."})
		return auth.User{}, false
	}
	session, err := s.auth.Refresh(r.Context(), refresh.Value)
	if err != nil {
		clearCookie(w, accessCookie)
		clearCookie(w, refreshCookie)
		writeJSON(w, 401, map[string]string{"erro": "Sessão expirada."})
		return auth.User{}, false
	}
	user, err := s.auth.RequireAdmin(r.Context(), session.AccessToken)
	if err != nil {
		writeJSON(w, 403, map[string]string{"erro": "Acesso restrito."})
		return auth.User{}, false
	}
	s.setSession(w, r, session)
	return user, true
}
func (s *FullServer) setSession(w http.ResponseWriter, r *http.Request, session auth.Session) {
	secure := r.TLS != nil || strings.EqualFold(strings.TrimSpace(strings.Split(r.Header.Get("X-Forwarded-Proto"), ",")[0]), "https")
	maxAge := session.ExpiresIn
	if maxAge <= 0 {
		maxAge = 3600
	}
	http.SetCookie(w, &http.Cookie{Name: accessCookie, Value: session.AccessToken, Path: "/", HttpOnly: true, Secure: secure, SameSite: http.SameSiteLaxMode, MaxAge: maxAge})
	http.SetCookie(w, &http.Cookie{Name: refreshCookie, Value: session.RefreshToken, Path: "/", HttpOnly: true, Secure: secure, SameSite: http.SameSiteLaxMode, MaxAge: int((30 * 24 * time.Hour).Seconds())})
}
func clearCookie(w http.ResponseWriter, name string) {
	http.SetCookie(w, &http.Cookie{Name: name, Value: "", Path: "/", HttpOnly: true, MaxAge: -1, Expires: time.Unix(1, 0), SameSite: http.SameSiteLaxMode})
}
func authMessage(status int) string {
	if status == 401 {
		return "E-mail ou senha inválidos."
	}
	if status == 403 {
		return "Usuário sem permissão de administrador."
	}
	return "Não foi possível autenticar."
}

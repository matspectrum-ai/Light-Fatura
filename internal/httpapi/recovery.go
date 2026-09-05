package httpapi

import (
	"net/http"

	"github.com/matspectrum-ai/Claro-Fatura/internal/auth"
)

func (s *FullServer) recoverySession(w http.ResponseWriter,r *http.Request){var body struct{AccessToken string `json:"access_token"`;RefreshToken string `json:"refresh_token"`;ExpiresIn int `json:"expires_in"`};if decodeJSON(r,&body)!=nil||body.AccessToken==""||body.RefreshToken==""{writeJSON(w,400,map[string]string{"erro":"Link de recuperação inválido."});return};user,err:=s.auth.RequireAdmin(r.Context(),body.AccessToken);if err!=nil||user.ID==""{writeJSON(w,403,map[string]string{"erro":"Acesso restrito."});return};s.setSession(w,r,auth.Session{AccessToken:body.AccessToken,RefreshToken:body.RefreshToken,ExpiresIn:body.ExpiresIn,User:user});writeJSON(w,200,map[string]bool{"ok":true})}

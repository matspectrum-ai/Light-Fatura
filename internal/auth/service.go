package auth

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

var (
	ErrInvalidCredentials = errors.New("credenciais inválidas")
	ErrNotAdmin           = errors.New("acesso restrito")
	ErrNoSession          = errors.New("sessão ausente")
)

type RoleStore interface {
	IsAdmin(context.Context, string) (bool, error)
}

type User struct {
	ID    string `json:"id"`
	Email string `json:"email"`
}

type Session struct {
	AccessToken  string `json:"access_token"`
	RefreshToken string `json:"refresh_token"`
	ExpiresIn    int    `json:"expires_in"`
	TokenType    string `json:"token_type"`
	User         User   `json:"user"`
}

type Service struct {
	baseURL        string
	publishableKey string
	store          RoleStore
	http           *http.Client
}

func New(baseURL, publishableKey string, store RoleStore) *Service {
	return &Service{
		baseURL:        strings.TrimRight(baseURL, "/"),
		publishableKey: publishableKey,
		store:          store,
		http: &http.Client{Timeout: 10 * time.Second, Transport: &http.Transport{
			MaxIdleConns: 64, MaxIdleConnsPerHost: 32, IdleConnTimeout: 90 * time.Second,
		}},
	}
}

func (s *Service) Configured() bool { return s.baseURL != "" && s.publishableKey != "" }

func (s *Service) SignIn(ctx context.Context, email, password string) (Session, error) {
	var session Session
	err := s.doJSON(ctx, http.MethodPost, "/auth/v1/token?grant_type=password", "", map[string]any{
		"email": strings.TrimSpace(email), "password": password,
	}, &session)
	if err != nil {
		var api *APIError
		if errors.As(err, &api) && (api.Status == http.StatusBadRequest || api.Status == http.StatusUnauthorized) {
			return Session{}, ErrInvalidCredentials
		}
		return Session{}, err
	}
	if session.User.ID == "" || session.AccessToken == "" {
		return Session{}, ErrInvalidCredentials
	}
	admin, err := s.store.IsAdmin(ctx, session.User.ID)
	if err != nil {
		return Session{}, err
	}
	if !admin {
		return Session{}, ErrNotAdmin
	}
	return session, nil
}

func (s *Service) User(ctx context.Context, accessToken string) (User, error) {
	if accessToken == "" {
		return User{}, ErrNoSession
	}
	var user User
	if err := s.doJSON(ctx, http.MethodGet, "/auth/v1/user", accessToken, nil, &user); err != nil {
		return User{}, err
	}
	return user, nil
}

func (s *Service) Refresh(ctx context.Context, refreshToken string) (Session, error) {
	if refreshToken == "" {
		return Session{}, ErrNoSession
	}
	var session Session
	if err := s.doJSON(ctx, http.MethodPost, "/auth/v1/token?grant_type=refresh_token", "", map[string]any{"refresh_token": refreshToken}, &session); err != nil {
		return Session{}, err
	}
	if session.AccessToken == "" {
		return Session{}, ErrNoSession
	}
	return session, nil
}

func (s *Service) RequireAdmin(ctx context.Context, accessToken string) (User, error) {
	user, err := s.User(ctx, accessToken)
	if err != nil {
		return User{}, err
	}
	admin, err := s.store.IsAdmin(ctx, user.ID)
	if err != nil {
		return User{}, err
	}
	if !admin {
		return User{}, ErrNotAdmin
	}
	return user, nil
}

func (s *Service) SendRecovery(ctx context.Context, email, redirectTo string) error {
	endpoint := "/auth/v1/recover"
	if redirectTo != "" {
		endpoint += "?redirect_to=" + url.QueryEscape(redirectTo)
	}
	return s.doJSON(ctx, http.MethodPost, endpoint, "", map[string]any{"email": strings.TrimSpace(email)}, nil)
}

func (s *Service) UpdatePassword(ctx context.Context, accessToken, password string) error {
	if accessToken == "" {
		return ErrNoSession
	}
	return s.doJSON(ctx, http.MethodPut, "/auth/v1/user", accessToken, map[string]any{"password": password}, nil)
}

func (s *Service) Logout(ctx context.Context, accessToken string) error {
	if accessToken == "" {
		return nil
	}
	err := s.doJSON(ctx, http.MethodPost, "/auth/v1/logout", accessToken, nil, nil)
	var api *APIError
	if errors.As(err, &api) && api.Status == http.StatusUnauthorized {
		return nil
	}
	return err
}

type APIError struct {
	Status  int
	Message string
}

func (e *APIError) Error() string {
	return fmt.Sprintf("supabase auth HTTP %d: %s", e.Status, e.Message)
}

func (s *Service) doJSON(ctx context.Context, method, endpoint, accessToken string, body any, dst any) error {
	var reader io.Reader
	if body != nil {
		encoded, err := json.Marshal(body)
		if err != nil {
			return err
		}
		reader = bytes.NewReader(encoded)
	}
	req, err := http.NewRequestWithContext(ctx, method, s.baseURL+endpoint, reader)
	if err != nil {
		return err
	}
	req.Header.Set("apikey", s.publishableKey)
	req.Header.Set("Accept", "application/json")
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	if accessToken != "" {
		req.Header.Set("Authorization", "Bearer "+accessToken)
	}
	resp, err := s.http.Do(req)
	if err != nil {
		return fmt.Errorf("supabase auth: %w", err)
	}
	defer resp.Body.Close()
	raw, _ := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		var message struct {
			Message          string `json:"message"`
			ErrorDescription string `json:"error_description"`
			Msg              string `json:"msg"`
		}
		_ = json.Unmarshal(raw, &message)
		text := first(message.Message, message.ErrorDescription, message.Msg, strings.TrimSpace(string(raw)))
		return &APIError{Status: resp.StatusCode, Message: text}
	}
	if dst != nil && len(raw) > 0 {
		if err := json.Unmarshal(raw, dst); err != nil {
			return err
		}
	}
	return nil
}

func first(values ...string) string {
	for _, v := range values {
		if strings.TrimSpace(v) != "" {
			return v
		}
	}
	return "erro de autenticação"
}

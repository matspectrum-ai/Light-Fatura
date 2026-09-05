package config

import (
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"
)

type Config struct {
	Addr                     string
	SiteURL                  string
	SupabaseURL               string
	SupabaseServiceRoleKey    string
	SupabasePublishableKey    string
	ProductName              string
	PIXExpiration            time.Duration
	CashinPaySecretKey       string
	CashinPayWebhookSecret   string
	BlackCatAPIKey           string
	BlackCatPKLive           string
	PixzyToken               string
	UmbrellaAPIKey           string
	PIXKey                   string
	PIXReceiver              string
	PIXCity                  string
}

func Load() (Config, error) {
	minutes := 30
	if raw := strings.TrimSpace(os.Getenv("PIX_EXPIRACAO_MINUTOS")); raw != "" {
		value, err := strconv.Atoi(raw)
		if err != nil || value <= 0 {
			return Config{}, fmt.Errorf("PIX_EXPIRACAO_MINUTOS inválido")
		}
		minutes = value
	}
	cfg := Config{
		Addr:                   valueOr(os.Getenv("ADDR"), ":8080"),
		SiteURL:                strings.TrimRight(strings.TrimSpace(os.Getenv("SITE_URL")), "/"),
		SupabaseURL:             strings.TrimRight(strings.TrimSpace(os.Getenv("SUPABASE_URL")), "/"),
		SupabaseServiceRoleKey:  strings.TrimSpace(os.Getenv("SUPABASE_SERVICE_ROLE_KEY")),
		SupabasePublishableKey:  strings.TrimSpace(os.Getenv("SUPABASE_PUBLISHABLE_KEY")),
		ProductName:             valueOr(os.Getenv("PRODUTO_NOME"), "Fatura Light"),
		PIXExpiration:          time.Duration(minutes) * time.Minute,
		CashinPaySecretKey:     strings.TrimSpace(os.Getenv("CASHINPAY_SECRET_KEY")),
		CashinPayWebhookSecret: strings.TrimSpace(os.Getenv("CASHINPAY_WEBHOOK_SECRET")),
		BlackCatAPIKey:         strings.TrimSpace(os.Getenv("BLACKCAT_API_KEY")),
		BlackCatPKLive:         strings.TrimSpace(os.Getenv("BLACKCAT_PK_LIVE")),
		PixzyToken:             strings.TrimSpace(os.Getenv("PIXZY_TOKEN")),
		UmbrellaAPIKey:         strings.TrimSpace(os.Getenv("UMBRELLA_API_KEY")),
		PIXKey:                 strings.TrimSpace(os.Getenv("PIX_CHAVE")),
		PIXReceiver:            valueOr(os.Getenv("PIX_RECEBEDOR"), "LIGHT SERVICOS"),
		PIXCity:                valueOr(os.Getenv("PIX_CIDADE"), "RIO DE JANEIRO"),
	}
	if cfg.SupabaseURL == "" || cfg.SupabaseServiceRoleKey == "" {
		return Config{}, fmt.Errorf("SUPABASE_URL e SUPABASE_SERVICE_ROLE_KEY são obrigatórios")
	}
	return cfg, nil
}

func valueOr(value, fallback string) string {
	if strings.TrimSpace(value) != "" {
		return strings.TrimSpace(value)
	}
	return fallback
}

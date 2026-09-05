package supabase

import (
	"context"
	"net/url"

	"github.com/matspectrum-ai/Claro-Fatura/internal/admin"
)

func (c *Client) ClearMetrics(ctx context.Context) error {
	return c.DeleteReturning(ctx, "acessos", url.Values{"id": {"not.is.null"}}, nil)
}

func (c *Client) CreateGateway(ctx context.Context, g admin.Gateway) error {
	return c.Insert(ctx, "gateways_config", map[string]any{
		"slug":          g.Slug,
		"rotulo":        g.Label,
		"adapter":       g.Adapter,
		"ativo":         g.Active,
		"prioridade":    g.Priority,
		"api_url":       g.APIURL,
		"ambiente":      g.Environment,
		"limite_diario": g.DailyLimit,
		"webhook_url":   g.WebhookURL,
		"secret_names":  g.SecretNames,
		"observacoes":   g.Observations,
	})
}

func (c *Client) WebhookSummaries(ctx context.Context) ([]admin.WebhookSummary, error) {
	gateways, err := c.Gateways(ctx)
	if err != nil {
		return nil, err
	}

	var rows []struct {
		GatewaySlug string `json:"gateway_slug"`
		Valid       *bool  `json:"valido"`
		CreatedAt   string `json:"created_at"`
	}
	if err := c.Select(ctx, "webhooks_log", url.Values{
		"select": {"gateway_slug,valido,created_at"},
		"order":  {"created_at.desc"},
		"limit":  {"5000"},
	}, &rows); err != nil {
		return nil, err
	}

	bySlug := make(map[string]*admin.WebhookSummary, len(gateways))
	out := make([]admin.WebhookSummary, 0, len(gateways))
	for _, g := range gateways {
		out = append(out, admin.WebhookSummary{GatewaySlug: g.Slug})
		bySlug[g.Slug] = &out[len(out)-1]
	}
	for _, row := range rows {
		summary := bySlug[row.GatewaySlug]
		if summary == nil {
			out = append(out, admin.WebhookSummary{GatewaySlug: row.GatewaySlug})
			summary = &out[len(out)-1]
			bySlug[row.GatewaySlug] = summary
		}
		summary.Total++
		if row.Valid != nil && *row.Valid {
			summary.Valid++
		} else {
			summary.Invalid++
		}
		if summary.LastAt == nil && row.CreatedAt != "" {
			value := row.CreatedAt
			summary.LastAt = &value
		}
	}
	return out, nil
}

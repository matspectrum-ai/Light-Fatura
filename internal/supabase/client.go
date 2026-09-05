package supabase

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

type Client struct {
	baseURL string
	key     string
	http    *http.Client
}

type HTTPError struct {
	Status  int
	Code    string
	Message string
	Body    string
}

func (e *HTTPError) Error() string {
	return fmt.Sprintf("supabase HTTP %d: %s", e.Status, e.Message)
}

func New(baseURL, serviceRoleKey string) *Client {
	return &Client{
		baseURL: strings.TrimRight(baseURL, "/"),
		key:     serviceRoleKey,
		http: &http.Client{Timeout: 20 * time.Second, Transport: &http.Transport{
			MaxIdleConns: 128, MaxIdleConnsPerHost: 64, IdleConnTimeout: 90 * time.Second,
		}},
	}
}

func (c *Client) Select(ctx context.Context, table, query string, dst any) error {
	return c.do(ctx, http.MethodGet, "/rest/v1/"+table+withQuery(query), nil, "", dst)
}

func (c *Client) Insert(ctx context.Context, table string, body any) error {
	return c.do(ctx, http.MethodPost, "/rest/v1/"+table, body, "return=minimal", nil)
}

func (c *Client) InsertReturning(ctx context.Context, table string, body any, dst any) error {
	return c.do(ctx, http.MethodPost, "/rest/v1/"+table, body, "return=representation", dst)
}

func (c *Client) Upsert(ctx context.Context, table, onConflict string, body any, dst any) error {
	q := ""
	if onConflict != "" {
		q = "on_conflict=" + url.QueryEscape(onConflict)
	}
	return c.do(ctx, http.MethodPost, "/rest/v1/"+table+withQuery(q), body, "resolution=merge-duplicates,return=representation", dst)
}

func (c *Client) Update(ctx context.Context, table, query string, body any, dst any) error {
	return c.do(ctx, http.MethodPatch, "/rest/v1/"+table+withQuery(query), body, "return=representation", dst)
}

func (c *Client) Delete(ctx context.Context, table, query string, dst any) error {
	return c.do(ctx, http.MethodDelete, "/rest/v1/"+table+withQuery(query), nil, "return=representation", dst)
}

func (c *Client) RPC(ctx context.Context, name string, body any, dst any) error {
	return c.do(ctx, http.MethodPost, "/rest/v1/rpc/"+name, body, "", dst)
}

func (c *Client) do(ctx context.Context, method, endpoint string, body any, prefer string, dst any) error {
	var reader io.Reader
	if body != nil {
		raw, err := json.Marshal(body)
		if err != nil { return err }
		reader = bytes.NewReader(raw)
	}
	req, err := http.NewRequestWithContext(ctx, method, c.baseURL+endpoint, reader)
	if err != nil { return err }
	req.Header.Set("apikey", c.key)
	req.Header.Set("Authorization", "Bearer "+c.key)
	req.Header.Set("Accept", "application/json")
	if body != nil { req.Header.Set("Content-Type", "application/json") }
	if prefer != "" { req.Header.Set("Prefer", prefer) }
	resp, err := c.http.Do(req)
	if err != nil { return fmt.Errorf("supabase: %w", err) }
	defer resp.Body.Close()
	raw, _ := io.ReadAll(io.LimitReader(resp.Body, 8<<20))
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		var detail struct { Code string `json:"code"`; Message string `json:"message"` }
		_ = json.Unmarshal(raw, &detail)
		if detail.Message == "" { detail.Message = strings.TrimSpace(string(raw)) }
		return &HTTPError{Status: resp.StatusCode, Code: detail.Code, Message: detail.Message, Body: string(raw)}
	}
	if dst != nil && len(raw) > 0 {
		if err := json.Unmarshal(raw, dst); err != nil { return err }
	}
	return nil
}

func withQuery(query string) string {
	query = strings.TrimPrefix(strings.TrimSpace(query), "?")
	if query == "" { return "" }
	return "?" + query
}

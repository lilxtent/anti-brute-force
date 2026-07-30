package apiclient

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/netip"
	"net/url"
	"strings"
	"time"
)

type Client struct {
	baseURL string
	httpc   *http.Client
}

func New(baseURL string, timeout time.Duration) (*Client, error) {
	u, err := url.Parse(baseURL)
	if err != nil {
		return nil, fmt.Errorf("parse address %q: %w", baseURL, err)
	}
	if u.Scheme != "http" && u.Scheme != "https" {
		return nil, fmt.Errorf("address %q: want an http:// or https:// URL", baseURL)
	}
	if u.Host == "" {
		return nil, fmt.Errorf("address %q: missing host", baseURL)
	}

	return &Client{
		baseURL: strings.TrimRight(baseURL, "/"),
		httpc:   &http.Client{Timeout: timeout},
	}, nil
}

type authRequest struct {
	Login    string `json:"login"`
	Password string `json:"password"`
	IP       string `json:"ip"`
}

type authResponse struct {
	OK bool `json:"ok"`
}

type resetRequest struct {
	Login string `json:"login"`
	IP    string `json:"ip"`
}

type subnetRequest struct {
	Subnet string `json:"subnet"`
}

type errorResponse struct {
	Error string `json:"error"`
}

func (c *Client) Check(ctx context.Context, login, password, ip string) (bool, error) {
	var resp authResponse
	req := authRequest{Login: login, Password: password, IP: ip}
	if err := c.do(ctx, http.MethodPost, "/auth", req, &resp); err != nil {
		return false, err
	}
	return resp.OK, nil
}

func (c *Client) Reset(ctx context.Context, login, ip string) error {
	return c.do(ctx, http.MethodPost, "/reset", resetRequest{Login: login, IP: ip}, nil)
}

func (c *Client) AddWhitelist(ctx context.Context, p netip.Prefix) error {
	return c.subnet(ctx, http.MethodPost, "/whitelist", p)
}

func (c *Client) RemoveWhitelist(ctx context.Context, p netip.Prefix) error {
	return c.subnet(ctx, http.MethodDelete, "/whitelist", p)
}

func (c *Client) AddBlacklist(ctx context.Context, p netip.Prefix) error {
	return c.subnet(ctx, http.MethodPost, "/blacklist", p)
}

func (c *Client) RemoveBlacklist(ctx context.Context, p netip.Prefix) error {
	return c.subnet(ctx, http.MethodDelete, "/blacklist", p)
}

func (c *Client) Health(ctx context.Context) error {
	return c.do(ctx, http.MethodGet, "/healthz", nil, nil)
}

func (c *Client) subnet(ctx context.Context, method, path string, p netip.Prefix) error {
	return c.do(ctx, method, path, subnetRequest{Subnet: p.String()}, nil)
}

func (c *Client) do(ctx context.Context, method, path string, reqBody, respBody any) error {
	var body io.Reader
	if reqBody != nil {
		raw, err := json.Marshal(reqBody)
		if err != nil {
			return fmt.Errorf("%s %s: encode request: %w", method, path, err)
		}
		body = bytes.NewReader(raw)
	}

	req, err := http.NewRequestWithContext(ctx, method, c.baseURL+path, body)
	if err != nil {
		return fmt.Errorf("%s %s: build request: %w", method, path, err)
	}
	if reqBody != nil {
		req.Header.Set("Content-Type", "application/json")
	}

	resp, err := c.httpc.Do(req)
	if err != nil {
		return fmt.Errorf("%s %s: %w", method, path, err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode < 200 || resp.StatusCode > 299 {
		return fmt.Errorf("%s %s: %w", method, path, serverError(resp))
	}

	if respBody == nil || resp.StatusCode == http.StatusNoContent {
		return nil
	}
	if err := json.NewDecoder(resp.Body).Decode(respBody); err != nil {
		return fmt.Errorf("%s %s: decode response: %w", method, path, err)
	}
	return nil
}

func serverError(resp *http.Response) error {
	raw, err := io.ReadAll(io.LimitReader(resp.Body, 4<<10))
	if err == nil {
		var payload errorResponse
		if jerr := json.Unmarshal(raw, &payload); jerr == nil && payload.Error != "" {
			return fmt.Errorf("server: %s (%s)", payload.Error, resp.Status)
		}
	}
	return fmt.Errorf("server: %s", resp.Status)
}

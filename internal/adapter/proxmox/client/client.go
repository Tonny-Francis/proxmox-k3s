package client

import (
	"bytes"
	"context"
	"crypto/tls"
	"crypto/x509"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"time"
)

type Client struct {
	endpoint   string
	tokenID    string
	tokenSecret string
	http       *http.Client
}

type Options struct {
	Endpoint        string
	TokenID         string
	TokenSecret     string
	InsecureSkipTLS bool
	CAFile          string
}

func New(opts Options) (*Client, error) {
	tlsCfg := &tls.Config{
		InsecureSkipVerify: opts.InsecureSkipTLS,
	}

	if opts.CAFile != "" {
		pem, err := os.ReadFile(opts.CAFile)
		if err != nil {
			return nil, fmt.Errorf("reading CA file: %w", err)
		}
		pool := x509.NewCertPool()
		if !pool.AppendCertsFromPEM(pem) {
			return nil, fmt.Errorf("parsing CA file: no valid certificates found")
		}
		tlsCfg.RootCAs = pool
	}

	transport := &http.Transport{
		TLSClientConfig: tlsCfg,
	}

	return &Client{
		endpoint:    opts.Endpoint,
		tokenID:     opts.TokenID,
		tokenSecret: opts.TokenSecret,
		http: &http.Client{
			Transport: transport,
			Timeout:   30 * time.Second,
		},
	}, nil
}

type apiResponse[T any] struct {
	Data T `json:"data"`
}

func GetJSON[T any](ctx context.Context, c *Client, path string) (T, error) {
	return get[T](ctx, c, path)
}

func PostJSON[T any](ctx context.Context, c *Client, path string, body any) (T, error) {
	return post[T](ctx, c, path, body)
}

func PutJSON[T any](ctx context.Context, c *Client, path string, body any) (T, error) {
	return put[T](ctx, c, path, body)
}

func DeleteJSON(ctx context.Context, c *Client, path string) (string, error) {
	return delete(ctx, c, path)
}

func get[T any](ctx context.Context, c *Client, path string) (T, error) {
	var zero T
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.endpoint+path, nil)
	if err != nil {
		return zero, err
	}
	c.addAuth(req)

	resp, err := c.http.Do(req)
	if err != nil {
		return zero, fmt.Errorf("GET %s: %w", path, err)
	}
	defer resp.Body.Close()

	if err := checkStatus(resp); err != nil {
		return zero, err
	}

	var out apiResponse[T]
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		return zero, fmt.Errorf("decoding response from %s: %w", path, err)
	}
	return out.Data, nil
}

func post[T any](ctx context.Context, c *Client, path string, body any) (T, error) {
	var zero T
	data, err := json.Marshal(body)
	if err != nil {
		return zero, err
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.endpoint+path, bytes.NewReader(data))
	if err != nil {
		return zero, err
	}
	req.Header.Set("Content-Type", "application/json")
	c.addAuth(req)

	resp, err := c.http.Do(req)
	if err != nil {
		return zero, fmt.Errorf("POST %s: %w", path, err)
	}
	defer resp.Body.Close()

	if err := checkStatus(resp); err != nil {
		return zero, err
	}

	var out apiResponse[T]
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		return zero, fmt.Errorf("decoding response from %s: %w", path, err)
	}
	return out.Data, nil
}

func put[T any](ctx context.Context, c *Client, path string, body any) (T, error) {
	var zero T
	data, err := json.Marshal(body)
	if err != nil {
		return zero, err
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPut, c.endpoint+path, bytes.NewReader(data))
	if err != nil {
		return zero, err
	}
	req.Header.Set("Content-Type", "application/json")
	c.addAuth(req)

	resp, err := c.http.Do(req)
	if err != nil {
		return zero, fmt.Errorf("PUT %s: %w", path, err)
	}
	defer resp.Body.Close()

	if err := checkStatus(resp); err != nil {
		return zero, err
	}

	var out apiResponse[T]
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		return zero, fmt.Errorf("decoding response from %s: %w", path, err)
	}
	return out.Data, nil
}

func delete(ctx context.Context, c *Client, path string) (string, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodDelete, c.endpoint+path, nil)
	if err != nil {
		return "", err
	}
	c.addAuth(req)

	resp, err := c.http.Do(req)
	if err != nil {
		return "", fmt.Errorf("DELETE %s: %w", path, err)
	}
	defer resp.Body.Close()

	if err := checkStatus(resp); err != nil {
		return "", err
	}

	var out apiResponse[string]
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		return "", fmt.Errorf("decoding response from %s: %w", path, err)
	}
	return out.Data, nil
}

func (c *Client) addAuth(req *http.Request) {
	req.Header.Set("Authorization", fmt.Sprintf("PVEAPIToken=%s=%s", c.tokenID, c.tokenSecret))
}

func checkStatus(resp *http.Response) error {
	if resp.StatusCode >= 200 && resp.StatusCode < 300 {
		return nil
	}

	body, _ := io.ReadAll(resp.Body)

	switch resp.StatusCode {
	case 401, 403:
		return fmt.Errorf("permission denied (HTTP %d): check API token privileges", resp.StatusCode)
	case 404:
		return fmt.Errorf("not found (HTTP 404): %s", string(body))
	case 500:
		return fmt.Errorf("proxmox internal error (HTTP 500): %s", string(body))
	default:
		return fmt.Errorf("unexpected status %d: %s", resp.StatusCode, string(body))
	}
}

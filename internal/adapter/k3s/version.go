package k3s

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"
)

const releaseAPI = "https://api.github.com/repos/k3s-io/k3s/releases"

type Resolver struct {
	http *http.Client
}

func NewResolver() *Resolver {
	return &Resolver{
		http: &http.Client{Timeout: 15 * time.Second},
	}
}

func (r *Resolver) Resolve(ctx context.Context, versionOrChannel string) (string, error) {
	if isExactVersion(versionOrChannel) {
		return versionOrChannel, nil
	}

	channel := versionOrChannel
	if channel == "" {
		channel = "stable"
	}

	url := fmt.Sprintf("https://update.k3s.io/v1-release/channels/%s", channel)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return "", err
	}

	resp, err := r.http.Do(req)
	if err != nil {
		return "", fmt.Errorf("resolving K3s channel %q: %w", channel, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		return "", fmt.Errorf("resolving K3s channel %q: HTTP %d", channel, resp.StatusCode)
	}

	var result struct {
		Data []struct {
			ID      string `json:"id"`
			Latest  string `json:"latest"`
		} `json:"data"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return "", fmt.Errorf("parsing K3s channel response: %w", err)
	}

	for _, ch := range result.Data {
		if ch.ID == channel {
			return ch.Latest, nil
		}
	}

	return "", fmt.Errorf("channel %q not found in K3s release channels", channel)
}

func isExactVersion(v string) bool {
	return strings.HasPrefix(v, "v") && strings.Contains(v, "+k3s")
}

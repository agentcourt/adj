package mcpbridge

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

type APIClient struct {
	baseURL     string
	bearerToken string
	httpClient  *http.Client
}

func NewAPIClient(baseURL, bearerToken string, timeout time.Duration) (*APIClient, error) {
	baseURL, err := NormalizeAPIBase(baseURL)
	if err != nil {
		return nil, err
	}
	if baseURL == "" {
		return nil, fmt.Errorf("case API base URL is required")
	}
	if timeout <= 0 {
		return nil, fmt.Errorf("case API HTTP timeout must be positive")
	}
	return &APIClient{
		baseURL:     baseURL,
		bearerToken: strings.TrimSpace(bearerToken),
		httpClient:  &http.Client{Timeout: timeout},
	}, nil
}

func NormalizeAPIBase(value string) (string, error) {
	baseURL := strings.TrimRight(strings.TrimSpace(value), "/")
	if baseURL == "" {
		return "", nil
	}
	parsed, err := url.ParseRequestURI(baseURL)
	if err != nil {
		return "", err
	}
	if parsed.Host == "" || (parsed.Scheme != "http" && parsed.Scheme != "https") {
		return "", fmt.Errorf("absolute HTTP or HTTPS URL with a host required")
	}
	if parsed.RawQuery != "" || parsed.Fragment != "" {
		return "", fmt.Errorf("case API base URL must not contain a query or fragment")
	}
	return baseURL, nil
}

func (c *APIClient) Get(ctx context.Context, path string, query url.Values) (value map[string]any, err error) {
	target, err := c.requestURL(path, query)
	if err != nil {
		return nil, err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, target, nil)
	if err != nil {
		return nil, err
	}
	return c.do(req)
}

func (c *APIClient) Post(ctx context.Context, path string, body map[string]any) (value map[string]any, err error) {
	raw, err := json.Marshal(body)
	if err != nil {
		return nil, err
	}
	target, err := c.requestURL(path, nil)
	if err != nil {
		return nil, err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, target, bytes.NewReader(raw))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	return c.do(req)
}

func (c *APIClient) requestURL(path string, query url.Values) (string, error) {
	path = strings.TrimSpace(path)
	if !strings.HasPrefix(path, "/") {
		return "", fmt.Errorf("case API path must begin with /")
	}
	target, err := url.Parse(c.baseURL + path)
	if err != nil {
		return "", err
	}
	if query != nil {
		target.RawQuery = query.Encode()
	}
	return target.String(), nil
}

func (c *APIClient) do(req *http.Request) (value map[string]any, err error) {
	if c.bearerToken != "" {
		req.Header.Set("Authorization", "Bearer "+c.bearerToken)
	}
	response, err := c.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer func() {
		if closeErr := response.Body.Close(); closeErr != nil {
			err = errors.Join(err, fmt.Errorf("close case API response: %w", closeErr))
		}
	}()
	decoder := json.NewDecoder(response.Body)
	decoder.UseNumber()
	if err := decoder.Decode(&value); err != nil {
		if errors.Is(err, io.EOF) {
			return nil, fmt.Errorf("case API returned an empty response")
		}
		return nil, err
	}
	if value == nil {
		return nil, fmt.Errorf("case API response must be a JSON object")
	}
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		value["ok"] = false
		value["http_status"] = response.StatusCode
	}
	return value, nil
}

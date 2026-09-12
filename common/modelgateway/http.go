package modelgateway

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/agentcourt/adj/common/modelapi"
)

const maxProviderResponseBytes = 32 * 1024 * 1024

var retryDelays = []time.Duration{0, 5 * time.Second, 30 * time.Second}

type rawHTTPClient struct {
	client      *http.Client
	maxAttempts int
}

func newRawHTTPClient(timeout time.Duration, maxAttempts int) rawHTTPClient {
	return rawHTTPClient{
		client:      &http.Client{Timeout: timeout},
		maxAttempts: maxAttempts,
	}
}

func (c rawHTTPClient) postJSON(ctx context.Context, endpoint string, headers http.Header, body any) ([]byte, error) {
	wire, err := json.Marshal(body)
	if err != nil {
		return nil, &modelapi.ProviderError{Class: modelapi.ProviderErrorRequest, Err: fmt.Errorf("encode provider request: %w", err)}
	}
	var lastErr error
	for attempt := 0; attempt < c.maxAttempts; attempt++ {
		if attempt > 0 {
			delay := retryDelays[attempt-1]
			timer := time.NewTimer(delay)
			select {
			case <-ctx.Done():
				timer.Stop()
				return nil, ctx.Err()
			case <-timer.C:
			}
		}
		req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewReader(wire))
		if err != nil {
			return nil, &modelapi.ProviderError{Class: modelapi.ProviderErrorRequest, Err: fmt.Errorf("create provider request: %w", err)}
		}
		req.Header = headers.Clone()
		req.Header.Set("content-type", "application/json")
		resp, err := c.client.Do(req)
		if err != nil {
			if errors.Is(ctx.Err(), context.Canceled) || errors.Is(ctx.Err(), context.DeadlineExceeded) {
				return nil, ctx.Err()
			}
			lastErr = &modelapi.ProviderError{Class: networkErrorClass(err), Err: fmt.Errorf("provider request: %w", err)}
			if attempt+1 < c.maxAttempts && modelapi.ErrorClass(lastErr) == modelapi.ProviderErrorTransient {
				continue
			}
			return nil, lastErr
		}
		responseBody, readErr := io.ReadAll(io.LimitReader(resp.Body, maxProviderResponseBytes+1))
		closeErr := resp.Body.Close()
		if readErr != nil || closeErr != nil {
			return nil, &modelapi.ProviderError{Class: modelapi.ProviderErrorTransient, Err: errors.Join(readErr, closeErr)}
		}
		if len(responseBody) > maxProviderResponseBytes {
			return nil, &modelapi.ProviderError{Class: modelapi.ProviderErrorProtocol, Err: fmt.Errorf("provider response exceeds %d bytes", maxProviderResponseBytes)}
		}
		if resp.StatusCode >= 200 && resp.StatusCode < 300 {
			return responseBody, nil
		}
		class := statusErrorClass(resp.StatusCode)
		lastErr = &modelapi.ProviderError{
			Class: class,
			Err:   fmt.Errorf("provider HTTP %d: %s", resp.StatusCode, strings.TrimSpace(string(responseBody))),
		}
		if attempt+1 < c.maxAttempts && class == modelapi.ProviderErrorTransient {
			continue
		}
		return nil, lastErr
	}
	return nil, lastErr
}

func statusErrorClass(status int) modelapi.ProviderErrorClass {
	switch status {
	case http.StatusUnauthorized, http.StatusForbidden:
		return modelapi.ProviderErrorAuthentication
	case http.StatusRequestTimeout, http.StatusConflict, http.StatusTooManyRequests:
		return modelapi.ProviderErrorTransient
	}
	if status >= 500 && status <= 599 {
		return modelapi.ProviderErrorTransient
	}
	return modelapi.ProviderErrorRequest
}

func networkErrorClass(err error) modelapi.ProviderErrorClass {
	var netErr net.Error
	if errors.As(err, &netErr) && (netErr.Timeout() || netErr.Temporary()) {
		return modelapi.ProviderErrorTransient
	}
	return modelapi.ProviderErrorTransient
}

func int64Value(value any) (int64, bool) {
	switch value := value.(type) {
	case float64:
		return int64(value), value >= 0
	case json.Number:
		parsed, err := value.Int64()
		return parsed, err == nil && parsed >= 0
	case int64:
		return value, value >= 0
	case int:
		return int64(value), value >= 0
	case string:
		parsed, err := strconv.ParseInt(strings.TrimSpace(value), 10, 64)
		return parsed, err == nil && parsed >= 0
	default:
		return 0, false
	}
}

// Package worker provides the delivery dispatcher and HTTP deliverer for the webhook relay service.
package worker

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"strings"

	"github.com/devaloi/hookrelay/internal/config"
	"github.com/devaloi/hookrelay/internal/domain"
	"github.com/devaloi/hookrelay/internal/signature"
)

// DeliveryResult contains the outcome of a delivery attempt.
type DeliveryResult struct {
	Success    bool
	StatusCode int
	Body       string
	Error      error
}

// Deliverer handles HTTP delivery of webhooks to target endpoints.
type Deliverer struct {
	client *http.Client
	cfg    *config.Config
	logger *slog.Logger
}

// NewDeliverer creates a new deliverer with the given configuration.
func NewDeliverer(cfg *config.Config, logger *slog.Logger) *Deliverer {
	return &Deliverer{
		client: &http.Client{
			Timeout: cfg.DeliveryTimeout,
			CheckRedirect: func(_ *http.Request, via []*http.Request) error {
				if len(via) >= domain.MaxRedirects {
					return fmt.Errorf("too many redirects")
				}
				return nil
			},
		},
		cfg:    cfg,
		logger: logger,
	}
}

// Deliver attempts to deliver a webhook to its target endpoint.
func (d *Deliverer) Deliver(item *domain.DeliveryWithWebhook) *DeliveryResult {
	// SSRF protection: only allow http and https schemes
	targetURL := item.Endpoint.TargetURL
	lower := strings.ToLower(targetURL)
	if !strings.HasPrefix(lower, "http://") && !strings.HasPrefix(lower, "https://") {
		return &DeliveryResult{Success: false, Error: fmt.Errorf("disallowed URL scheme in %q: only http and https are allowed", targetURL)}
	}

	ctx, cancel := context.WithTimeout(context.Background(), d.cfg.DeliveryTimeout)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, targetURL, bytes.NewReader(item.Webhook.Payload))
	if err != nil {
		return &DeliveryResult{Success: false, Error: fmt.Errorf("creating request: %w", err)}
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Hook-ID", item.Webhook.ID)
	req.Header.Set("X-Hook-Delivery", item.Delivery.ID)
	req.Header.Set("User-Agent", "hookrelay/1.0")

	if item.Endpoint.Secret != "" {
		sig := signature.Sign(item.Webhook.Payload, item.Endpoint.Secret)
		req.Header.Set(signature.SignatureHeader, sig)
	}

	resp, err := d.client.Do(req)
	if err != nil {
		return &DeliveryResult{Success: false, Error: fmt.Errorf("executing request: %w", err)}
	}
	defer func() { _ = resp.Body.Close() }()

	body, err := io.ReadAll(io.LimitReader(resp.Body, domain.MaxResponseBodySize))
	if err != nil {
		body = []byte(fmt.Sprintf("error reading body: %v", err))
	}

	if resp.StatusCode >= 200 && resp.StatusCode < 300 {
		return &DeliveryResult{Success: true, StatusCode: resp.StatusCode, Body: string(body)}
	}

	return &DeliveryResult{
		Success: false, StatusCode: resp.StatusCode, Body: string(body),
		Error: fmt.Errorf("non-2xx status code: %d", resp.StatusCode),
	}
}

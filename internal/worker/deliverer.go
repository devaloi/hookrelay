package worker

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"log/slog"
	"net/http"

	"github.com/devaloi/hookrelay/internal/config"
	"github.com/devaloi/hookrelay/internal/domain"
	"github.com/devaloi/hookrelay/internal/signature"
)

type DeliveryResult struct {
	Success    bool
	StatusCode int
	Body       string
	Error      error
}

type Deliverer struct {
	client *http.Client
	cfg    *config.Config
	logger *slog.Logger
}

func NewDeliverer(cfg *config.Config, logger *slog.Logger) *Deliverer {
	return &Deliverer{
		client: &http.Client{
			Timeout: cfg.DeliveryTimeout,
			CheckRedirect: func(req *http.Request, via []*http.Request) error {
				if len(via) >= 3 {
					return fmt.Errorf("too many redirects")
				}
				return nil
			},
		},
		cfg:    cfg,
		logger: logger,
	}
}

func (d *Deliverer) Deliver(item *domain.DeliveryWithWebhook) *DeliveryResult {
	ctx, cancel := context.WithTimeout(context.Background(), d.cfg.DeliveryTimeout)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, item.Endpoint.TargetURL, bytes.NewReader(item.Webhook.Payload))
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
	defer resp.Body.Close()

	body, err := io.ReadAll(io.LimitReader(resp.Body, 10*1024))
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

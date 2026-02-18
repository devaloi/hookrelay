package worker

import (
	"context"
	"log/slog"
	"sync"
	"time"

	"github.com/devaloi/hookrelay/internal/config"
	"github.com/devaloi/hookrelay/internal/domain"
	"github.com/devaloi/hookrelay/internal/queue"
)

type Dispatcher struct {
	store     queue.Store
	deliverer *Deliverer
	cfg       *config.Config
	logger    *slog.Logger
	stopCh    chan struct{}
	wg        sync.WaitGroup
}

func NewDispatcher(store queue.Store, cfg *config.Config, logger *slog.Logger) *Dispatcher {
	return &Dispatcher{
		store:     store,
		deliverer: NewDeliverer(cfg, logger),
		cfg:       cfg,
		logger:    logger,
		stopCh:    make(chan struct{}),
	}
}

func (d *Dispatcher) Start(ctx context.Context) {
	d.wg.Add(1)
	go d.run(ctx)
	d.logger.Info("dispatcher started", "interval", d.cfg.WorkerInterval)
}

func (d *Dispatcher) Stop() {
	close(d.stopCh)
	d.wg.Wait()
	d.logger.Info("dispatcher stopped")
}

func (d *Dispatcher) run(ctx context.Context) {
	defer d.wg.Done()
	ticker := time.NewTicker(d.cfg.WorkerInterval)
	defer ticker.Stop()
	d.process()
	for {
		select {
		case <-ctx.Done():
			return
		case <-d.stopCh:
			return
		case <-ticker.C:
			d.process()
		}
	}
}

func (d *Dispatcher) process() {
	items, err := d.store.Dequeue(10)
	if err != nil {
		d.logger.Error("dequeue failed", "error", err)
		return
	}
	if len(items) == 0 {
		return
	}
	d.logger.Debug("processing deliveries", "count", len(items))
	for _, item := range items {
		d.processDelivery(item)
	}
}

func (d *Dispatcher) processDelivery(item *domain.DeliveryWithWebhook) {
	result := d.deliverer.Deliver(item)
	if result.Success {
		err := d.store.MarkDelivered(item.Delivery.ID, result.StatusCode, result.Body)
		if err != nil {
			d.logger.Error("marking delivery as delivered", "delivery_id", item.Delivery.ID, "error", err)
		}
		d.logger.Info("delivery successful", "delivery_id", item.Delivery.ID, "status_code", result.StatusCode)
	} else {
		attempts := item.Delivery.Attempts + 1
		if attempts >= d.cfg.MaxRetries {
			err := d.store.MarkDead(item.Delivery.ID, result.Error.Error())
			if err != nil {
				d.logger.Error("marking delivery as dead", "delivery_id", item.Delivery.ID, "error", err)
			}
			d.logger.Warn("delivery moved to DLQ", "delivery_id", item.Delivery.ID, "attempts", attempts, "error", result.Error)
		} else {
			nextRetry := d.calculateNextRetry(attempts)
			err := d.store.MarkFailed(item.Delivery.ID, result.Error, &result.StatusCode, &nextRetry)
			if err != nil {
				d.logger.Error("marking delivery as failed", "delivery_id", item.Delivery.ID, "error", err)
			}
			d.logger.Warn("delivery failed, will retry", "delivery_id", item.Delivery.ID, "attempt", attempts, "next_retry", nextRetry, "error", result.Error)
		}
	}
}

func (d *Dispatcher) calculateNextRetry(attempt int) time.Time {
	base := time.Duration(d.cfg.RetryBackoffBase) * time.Second
	delay := base * (1 << attempt)
	jitter := time.Duration(float64(delay) * 0.1)
	delay += jitter
	return time.Now().Add(delay)
}

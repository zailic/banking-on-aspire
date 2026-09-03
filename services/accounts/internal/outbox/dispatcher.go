package outbox

import (
	"bytes"
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"net/url"
	"strings"
	"time"

	"dev.local/banking-on-aspire/platform/observability"
	"dev.local/banking-on-aspire/services/accounts/internal/accountrepo"
	"go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp"
)

const (
	pubSubName              = "pubsub"
	paymentSentEventType    = "banking.events.v1.PaymentSentEvent"
	fundsDepositedEventType = "banking.events.v1.FundsDepositedEvent"
	paymentSentTopic        = "payments.sent"
	fundsDepositedTopic     = "funds.deposited"
)

type Store interface {
	PendingOutboxEvents(context.Context, int) ([]accountrepo.OutboxEvent, error)
	MarkOutboxEventPublished(context.Context, string) error
}

type Dispatcher struct {
	store    Store
	endpoint string
	client   *http.Client
	logger   *slog.Logger
}

func New(store Store, daprHTTPPort string, logger *slog.Logger) *Dispatcher {
	return &Dispatcher{
		store: store,
		endpoint: "http://127.0.0.1:" + strings.TrimSpace(daprHTTPPort) +
			"/v1.0/publish/" + url.PathEscape(pubSubName) + "/",
		client: &http.Client{Transport: otelhttp.NewTransport(http.DefaultTransport), Timeout: 5 * time.Second}, logger: logger,
	}
}

func (d *Dispatcher) Run(ctx context.Context) {
	ticker := time.NewTicker(500 * time.Millisecond)
	defer ticker.Stop()
	for {
		if err := d.dispatch(ctx); err != nil && ctx.Err() == nil {
			d.logger.Warn("outbox dispatch failed", "error", err)
		}
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
		}
	}
}

func (d *Dispatcher) dispatch(ctx context.Context) error {
	events, err := d.store.PendingOutboxEvents(ctx, 50)
	if err != nil {
		return err
	}
	for _, event := range events {
		eventCtx := observability.ExtractTraceContext(ctx, event.TraceParent, event.TraceState)
		topic, err := topicFor(event.Type)
		if err != nil {
			return err
		}
		req, err := http.NewRequestWithContext(eventCtx, http.MethodPost, d.endpoint+url.PathEscape(topic), bytes.NewReader(event.Payload))
		if err != nil {
			return fmt.Errorf("create publish request: %w", err)
		}
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("ce-id", event.ID)
		req.Header.Set("ce-type", event.Type)
		resp, err := d.client.Do(req)
		if err != nil {
			return fmt.Errorf("publish event %s: %w", event.ID, err)
		}
		_ = resp.Body.Close()
		if resp.StatusCode < 200 || resp.StatusCode >= 300 {
			return fmt.Errorf("publish event %s: Dapr returned %s", event.ID, resp.Status)
		}
		if err := d.store.MarkOutboxEventPublished(ctx, event.ID); err != nil {
			return err
		}
		d.logger.Info("published account event", "event_id", event.ID, "topic", topic)
	}
	return nil
}

func topicFor(eventType string) (string, error) {
	switch eventType {
	case paymentSentEventType:
		return paymentSentTopic, nil
	case fundsDepositedEventType:
		return fundsDepositedTopic, nil
	default:
		return "", fmt.Errorf("unsupported outbox event type %q", eventType)
	}
}

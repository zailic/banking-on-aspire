package outbox

import (
	"context"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"dev.local/banking-on-aspire/services/accounts/internal/accountrepo"
	"go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/propagation"
)

type retryStore struct {
	event  accountrepo.OutboxEvent
	marked int
}

func (s *retryStore) PendingOutboxEvents(context.Context, int) ([]accountrepo.OutboxEvent, error) {
	if s.marked > 0 {
		return nil, nil
	}
	return []accountrepo.OutboxEvent{s.event}, nil
}

func (s *retryStore) MarkOutboxEventPublished(context.Context, string) error {
	s.marked++
	return nil
}

func TestDispatchRetriesUnpublishedEventAndPropagatesTraceContext(t *testing.T) {
	otel.SetTextMapPropagator(propagation.TraceContext{})
	const traceParent = "00-01020300000000000000000000000000-0405060000000000-01"
	store := &retryStore{event: accountrepo.OutboxEvent{
		ID:          "event-1",
		Type:        fundsDepositedEventType,
		Payload:     []byte(`{"data":"test"}`),
		TraceParent: traceParent,
	}}
	requests := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requests++
		if got := r.Header.Get("traceparent"); got != traceParent {
			t.Errorf("traceparent = %q, want %q", got, traceParent)
		}
		if requests == 1 {
			http.Error(w, "temporarily unavailable", http.StatusServiceUnavailable)
			return
		}
		w.WriteHeader(http.StatusNoContent)
	}))
	defer server.Close()

	dispatcher := &Dispatcher{
		store:    store,
		endpoint: server.URL + "/",
		logger:   slog.New(slog.NewTextHandler(io.Discard, nil)),
		client: &http.Client{
			Transport: otelhttp.NewTransport(http.DefaultTransport),
			Timeout:   time.Second,
		},
	}
	if err := dispatcher.dispatch(context.Background()); err == nil {
		t.Fatal("first dispatch succeeded, want transient failure")
	}
	if store.marked != 0 {
		t.Fatalf("event marked after failed publish: %d", store.marked)
	}
	if err := dispatcher.dispatch(context.Background()); err != nil {
		t.Fatalf("second dispatch: %v", err)
	}
	if requests != 2 || store.marked != 1 {
		t.Fatalf("requests = %d, marked = %d; want 2, 1", requests, store.marked)
	}
}

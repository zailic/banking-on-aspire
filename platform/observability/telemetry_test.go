package observability

import (
	"context"
	"testing"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/propagation"
	"go.opentelemetry.io/otel/trace"
)

func TestDurableTraceContextRoundTrip(t *testing.T) {
	otel.SetTextMapPropagator(propagation.NewCompositeTextMapPropagator(propagation.TraceContext{}, propagation.Baggage{}))
	spanContext := trace.NewSpanContext(trace.SpanContextConfig{
		TraceID:    trace.TraceID{1, 2, 3},
		SpanID:     trace.SpanID{4, 5, 6},
		TraceFlags: trace.FlagsSampled,
		TraceState: mustTraceState(t, "vendor=value"),
	})

	traceParent, traceState := InjectTraceContext(trace.ContextWithSpanContext(context.Background(), spanContext))
	restored := trace.SpanContextFromContext(ExtractTraceContext(context.Background(), traceParent, traceState))

	if restored.TraceID() != spanContext.TraceID() || restored.SpanID() != spanContext.SpanID() {
		t.Fatalf("restored span context = %v, want %v", restored, spanContext)
	}
	if !restored.IsRemote() {
		t.Fatal("restored span context must be remote")
	}
	if restored.TraceState().String() != "vendor=value" {
		t.Fatalf("restored tracestate = %q", restored.TraceState().String())
	}
}

func mustTraceState(t *testing.T, value string) trace.TraceState {
	t.Helper()
	state, err := trace.ParseTraceState(value)
	if err != nil {
		t.Fatal(err)
	}
	return state
}

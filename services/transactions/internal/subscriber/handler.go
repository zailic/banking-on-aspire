package subscriber

import (
	"encoding/json"
	"net/http"
	"strings"

	eventsv1 "dev.local/banking-on-aspire/platform/gen/go/banking/events/v1"
	"dev.local/banking-on-aspire/services/transactions/internal/transactionrepo"
	"google.golang.org/protobuf/encoding/protojson"
)

type Handler struct{ repository transactionrepo.Repository }

func New(repository transactionrepo.Repository) http.Handler {
	h := &Handler{repository: repository}
	mux := http.NewServeMux()
	mux.HandleFunc("GET /dapr/subscribe", h.subscribe)
	mux.HandleFunc("POST /events/payment-sent", h.paymentSent)
	mux.HandleFunc("POST /events/funds-deposited", h.fundsDeposited)
	mux.HandleFunc(
		"GET /health",
		func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusNoContent) },
	)
	return mux
}

func (h *Handler) subscribe(w http.ResponseWriter, _ *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode([]map[string]string{
		{"pubsubname": "pubsub", "topic": "payments.sent", "route": "events/payment-sent"},
		{"pubsubname": "pubsub", "topic": "funds.deposited", "route": "events/funds-deposited"},
	})
}

func (h *Handler) fundsDeposited(w http.ResponseWriter, r *http.Request) {
	data, ok := cloudEventData(w, r)
	if !ok {
		return
	}
	var event eventsv1.FundsDepositedEvent
	if err := protojson.Unmarshal(data, &event); err != nil || !validFundsDeposited(&event) {
		http.Error(w, "invalid funds deposited event", http.StatusBadRequest)
		return
	}
	if err := h.repository.ApplyFundsDeposited(r.Context(), &event); err != nil {
		http.Error(w, "funds deposited event could not be applied", http.StatusInternalServerError)
		return
	}
	success(w)
}

func (h *Handler) paymentSent(w http.ResponseWriter, r *http.Request) {
	data, ok := cloudEventData(w, r)
	if !ok {
		return
	}
	var event eventsv1.PaymentSentEvent
	if err := protojson.Unmarshal(data, &event); err != nil || !validPaymentSent(&event) {
		http.Error(w, "invalid payment event", http.StatusBadRequest)
		return
	}
	if err := h.repository.ApplyPaymentSent(r.Context(), &event); err != nil {
		http.Error(w, "payment event could not be applied", http.StatusInternalServerError)
		return
	}
	success(w)
}

func cloudEventData(w http.ResponseWriter, r *http.Request) (json.RawMessage, bool) {
	var envelope struct {
		Data json.RawMessage `json:"data"`
	}
	if err := json.NewDecoder(r.Body).Decode(&envelope); err != nil || len(envelope.Data) == 0 {
		http.Error(w, "invalid CloudEvent", http.StatusBadRequest)
		return nil, false
	}
	return envelope.Data, true
}

func success(w http.ResponseWriter) {
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]string{"status": "SUCCESS"})
}

func validPaymentSent(event *eventsv1.PaymentSentEvent) bool {
	return event.GetSchemaVersion() == 1 && strings.TrimSpace(event.GetEventId()) != "" &&
		strings.TrimSpace(
			event.GetPayment(),
		) != "" && strings.TrimSpace(event.GetSourceAccount()) != "" &&
		strings.TrimSpace(
			event.GetSourceOwner(),
		) != "" && strings.TrimSpace(event.GetBeneficiary()) != "" &&
		event.GetAmount() != nil && strings.TrimSpace(event.GetAmount().GetCurrencyCode()) != "" &&
		event.GetOccurredTime() != nil && event.GetOccurredTime().IsValid()
}

func validFundsDeposited(event *eventsv1.FundsDepositedEvent) bool {
	return event.GetSchemaVersion() == 1 && strings.TrimSpace(event.GetEventId()) != "" &&
		strings.TrimSpace(
			event.GetDeposit(),
		) != "" && strings.TrimSpace(event.GetAccount()) != "" &&
		strings.TrimSpace(event.GetOwner()) != "" && event.GetAmount() != nil &&
		strings.TrimSpace(
			event.GetAmount().GetCurrencyCode(),
		) != "" && event.GetOccurredTime() != nil &&
		event.GetOccurredTime().IsValid()
}

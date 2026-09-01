package subscriber

import (
	"bytes"
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	eventsv1 "dev.local/banking-on-aspire/platform/gen/go/banking/events/v1"
	transactionsv1 "dev.local/banking-on-aspire/platform/gen/go/banking/transactions/v1"
	"google.golang.org/genproto/googleapis/type/money"
	"google.golang.org/protobuf/encoding/protojson"
	"google.golang.org/protobuf/types/known/timestamppb"
)

type repoStub struct {
	payments int
	deposits int
}

func (r *repoStub) ApplyPaymentSent(context.Context, *eventsv1.PaymentSentEvent) error {
	r.payments++
	return nil
}
func (r *repoStub) ApplyFundsDeposited(context.Context, *eventsv1.FundsDepositedEvent) error {
	r.deposits++
	return nil
}
func (r *repoStub) List(context.Context, string, int, int) ([]*transactionsv1.Transaction, error) {
	return nil, nil
}
func (r *repoStub) ResolveUserName(context.Context, string) (string, error) { return "", nil }

func TestPaymentSentAppliesCloudEventData(t *testing.T) {
	eventJSON, err := protojson.Marshal(&eventsv1.PaymentSentEvent{EventId: "evt-1", SchemaVersion: 1, Payment: "payments/pay-1", SourceAccount: "accounts/a", SourceOwner: "users/u", Beneficiary: "users/u/contacts/c", Amount: &money.Money{CurrencyCode: "RON", Units: 1}, OccurredTime: timestamppb.Now()})
	if err != nil {
		t.Fatal(err)
	}
	body := append([]byte(`{"data":`), eventJSON...)
	body = append(body, '}')
	repo := &repoStub{}
	recorder := httptest.NewRecorder()
	New(repo).ServeHTTP(recorder, httptest.NewRequest(http.MethodPost, "/events/payment-sent", bytes.NewReader(body)))
	if recorder.Code != http.StatusOK || repo.payments != 1 {
		t.Fatalf("status = %d, applied = %d, body = %s", recorder.Code, repo.payments, recorder.Body.String())
	}
}

func TestFundsDepositedAppliesCloudEventData(t *testing.T) {
	eventJSON, err := protojson.Marshal(&eventsv1.FundsDepositedEvent{
		EventId: "evt-deposit-1", SchemaVersion: 1, Deposit: "deposits/dep-1",
		Account: "accounts/a", Owner: "users/u", Amount: &money.Money{CurrencyCode: "RON", Units: 100},
		OccurredTime: timestamppb.Now(),
	})
	if err != nil {
		t.Fatal(err)
	}
	body := append([]byte(`{"data":`), eventJSON...)
	body = append(body, '}')
	repo := &repoStub{}
	recorder := httptest.NewRecorder()
	New(repo).ServeHTTP(recorder, httptest.NewRequest(http.MethodPost, "/events/funds-deposited", bytes.NewReader(body)))
	if recorder.Code != http.StatusOK || repo.deposits != 1 {
		t.Fatalf("status = %d, deposits = %d, body = %s", recorder.Code, repo.deposits, recorder.Body.String())
	}
}

func TestPaymentSentRejectsUnsupportedSchema(t *testing.T) {
	recorder := httptest.NewRecorder()
	New(&repoStub{}).ServeHTTP(recorder, httptest.NewRequest(http.MethodPost, "/events/payment-sent", bytes.NewBufferString(`{"data":{"eventId":"evt","schemaVersion":2}}`)))
	if recorder.Code != http.StatusBadRequest {
		t.Fatalf("status = %d", recorder.Code)
	}
}

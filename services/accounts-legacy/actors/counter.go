// Package actors contains the preserved legacy Dapr actor implementation.
package actors

import (
	"context"
	"fmt"
	"log"
	"math/rand"

	"time"

	"dev.local/banking-on-aspire/services/accounts-legacy/api"
	"github.com/dapr/go-sdk/actor"
	dapr "github.com/dapr/go-sdk/client"
)

const CounterActorType = "CounterActor"

// CounterActor is the implementation created for every actor ID.
//
// ServerImplBaseCtx provides:
//   - ID()
//   - GetStateManager()
//   - actor lifecycle integration
type CounterActor struct {
	actor.ServerImplBaseCtx
	daprClient dapr.Client
}

// Type is the actor type registered with Dapr.
func (a *CounterActor) Type() string {
	return CounterActorType
}

func (a *CounterActor) StopTimer(ctx context.Context, req *api.TimerRequest) error {
	return a.daprClient.UnregisterActorTimer(ctx, &dapr.UnregisterActorTimerRequest{
		ActorType: CounterActorType,
		ActorID:   a.ID(),
		Name:      req.TimerName,
	})
}

func (a *CounterActor) StartTimer(ctx context.Context, req *api.TimerRequest) error {
	return a.daprClient.RegisterActorTimer(ctx, &dapr.RegisterActorTimerRequest{
		ActorType: CounterActorType,
		ActorID:   a.ID(),
		Name:      req.TimerName,
		DueTime:   req.Duration,
		Period:    req.Period,
		Data:      []byte(req.Data),
		CallBack:  req.Callback,
	})
}

func (a *CounterActor) StopReminder(ctx context.Context, req *api.ReminderRequest) error {
	return a.daprClient.UnregisterActorReminder(ctx, &dapr.UnregisterActorReminderRequest{
		ActorType: CounterActorType,
		ActorID:   a.ID(),
		Name:      req.ReminderName,
	})
}

func (a *CounterActor) StartReminder(ctx context.Context, req *api.ReminderRequest) error {
	return a.daprClient.RegisterActorReminder(ctx, &dapr.RegisterActorReminderRequest{
		ActorType: CounterActorType,
		ActorID:   a.ID(),
		Name:      req.ReminderName,
		DueTime:   req.Duration,
		Period:    req.Period,
		Data:      []byte(req.Data),
		FailurePolicy: &dapr.JobFailurePolicyConstant{
			MaxRetries: new(uint32(3)),
			Interval:   new(time.Second * 1),
		},
	})
}

func (a *CounterActor) ReminderCall(
	reminderName string,
	state []byte,
	_ string,
	_ string,
) {
	sign := []int{1, -1}[rand.Intn(2)]
	randomAmount := rand.Intn(100) + 1

	log.Printf("reminder callback: %s, state: %s, amount: %d", reminderName, string(state), sign*randomAmount)
	opResponse, err := a.Add(context.Background(), &api.AddRequest{
		Amount: sign * randomAmount,
	})
	if err != nil {
		log.Printf("failed to update counter in reminder callback: %v", err)
	} else {
		log.Printf("counter updated in reminder callback. New value: %d", opResponse.Value)
	}

}

// Get returns the current counter value.
func (a *CounterActor) Get(ctx context.Context) (*api.CounterResponse, error) {
	value, err := a.readCounter(ctx)
	if err != nil {
		return nil, err
	}

	return &api.CounterResponse{
		ActorID: a.ID(),
		Value:   value,
	}, nil
}

// Add increments the counter and stores the new value.
func (a *CounterActor) Add(
	ctx context.Context,
	request *api.AddRequest,
) (*api.CounterResponse, error) {
	if request == nil {
		return nil, fmt.Errorf("request cannot be nil")
	}

	value, err := a.readCounter(ctx)
	if err != nil {
		return nil, err
	}

	value += request.Amount

	if err := a.GetStateManager().Set(ctx, "counter", value); err != nil {
		return nil, fmt.Errorf("save actor state: %w", err)
	}

	return &api.CounterResponse{
		ActorID: a.ID(),
		Value:   value,
	}, nil
}

func (a *CounterActor) Subtract(
	ctx context.Context,
	request *api.AddRequest,
) (*api.CounterResponse, error) {
	if request == nil {
		return nil, fmt.Errorf("request cannot be nil")
	}

	value, err := a.readCounter(ctx)
	if err != nil {
		return nil, err
	}

	value -= request.Amount

	if err := a.GetStateManager().Set(ctx, "counter", value); err != nil {
		return nil, fmt.Errorf("save actor state: %w", err)
	}

	return &api.CounterResponse{
		ActorID: a.ID(),
		Value:   value,
	}, nil
}

// Reset removes the counter state.
func (a *CounterActor) Reset(ctx context.Context) error {
	if err := a.GetStateManager().Remove(ctx, "counter"); err != nil {
		return fmt.Errorf("remove actor state: %w", err)
	}

	return nil
}

func (a *CounterActor) readCounter(ctx context.Context) (int, error) {
	state := a.GetStateManager()

	exists, err := state.Contains(ctx, "counter")
	if err != nil {
		return 0, fmt.Errorf("check actor state: %w", err)
	}

	if !exists {
		return 0, nil
	}

	var value int

	if err := state.Get(ctx, "counter", &value); err != nil {
		return 0, fmt.Errorf("read actor state: %w", err)
	}

	return value, nil
}

// The factory is called when Dapr activates a new actor instance.
func CounterActorFactory() actor.ServerContext {
	client, err := dapr.NewClient()
	if err != nil {
		log.Fatalf("failed to create Dapr client: %v", err)
	}

	return &CounterActor{
		daprClient: client,
	}
}

// Package api contains the preserved legacy Dapr actor implementation.
package api

import "context"

type AddRequest struct {
	Amount int `json:"amount"`
}

type CounterResponse struct {
	ActorID string `json:"actorId"`
	Value   int    `json:"value"`
}

type TimerRequest struct {
	TimerName string `json:"timer_name"`
	Callback  string `json:"callback"`
	Duration  string `json:"duration"`
	Period    string `json:"period"`
	Data      string `json:"data"`
}

type ReminderRequest struct {
	ReminderName string `json:"reminder_name"`
	Duration     string `json:"duration"`
	Period       string `json:"period"`
	Data         string `json:"data"`
}

type ClientStub struct {
	Add           func(ctx context.Context, req *AddRequest) (*CounterResponse, error)
	Subtract      func(ctx context.Context, req *AddRequest) (*CounterResponse, error)
	Get           func(ctx context.Context) (*CounterResponse, error)
	Reset         func(ctx context.Context) error
	StartTimer    func(ctx context.Context, req *TimerRequest) error
	StopTimer     func(ctx context.Context, timerName string) error
	StartReminder func(ctx context.Context, req *ReminderRequest) error
	StopReminder  func(ctx context.Context, reminderName string) error
}

// Package domain contains the preserved legacy Dapr actor implementation.
package domain

import "fmt"

type Money struct {
	Amount   int64  `json:"amount"`
	Currency string `json:"currency"`
}

func (m Money) Add(other Money) Money {
	if m.Currency != other.Currency {
		panic("cannot add money with different currencies")
	}
	return Money{
		Amount:   m.Amount + other.Amount,
		Currency: m.Currency,
	}
}

func (m Money) Subtract(other Money) Money {
	if m.Currency != other.Currency {
		panic("cannot subtract money with different currencies")
	}
	return Money{
		Amount:   m.Amount - other.Amount,
		Currency: m.Currency,
	}
}

func (m Money) IsNegative() bool {
	return m.Amount < 0
}

func (m Money) IsZero() bool {
	return m.Amount == 0
}

func (m Money) Format() string {
	units := m.Amount / 100
	cents := m.Amount % 100

	return fmt.Sprintf("%s %d.%02d", m.Currency, units, cents)
}

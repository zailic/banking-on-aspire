package domain

import "testing"

func TestMoneyAdd(t *testing.T) {
	tests := []struct {
		name string
		a    Money
		b    Money
		want Money
	}{
		{
			name: "same currency adds amounts",
			a:    Money{Amount: 250, Currency: "USD"},
			b:    Money{Amount: 125, Currency: "USD"},
			want: Money{Amount: 375, Currency: "USD"},
		},
		{
			name: "handles negative amounts",
			a:    Money{Amount: 100, Currency: "USD"},
			b:    Money{Amount: -25, Currency: "USD"},
			want: Money{Amount: 75, Currency: "USD"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.a.Add(tt.b)
			if got != tt.want {
				t.Fatalf("Add() = %+v, want %+v", got, tt.want)
			}
		})
	}
}

func TestMoneyAddPanicsOnCurrencyMismatch(t *testing.T) {
	defer func() {
		if recover() == nil {
			t.Fatal("expected panic for currency mismatch")
		}
	}()

	_ = Money{Amount: 100, Currency: "USD"}.Add(Money{Amount: 100, Currency: "EUR"})
}

func TestMoneySubtract(t *testing.T) {
	tests := []struct {
		name string
		a    Money
		b    Money
		want Money
	}{
		{
			name: "same currency subtracts amounts",
			a:    Money{Amount: 500, Currency: "USD"},
			b:    Money{Amount: 125, Currency: "USD"},
			want: Money{Amount: 375, Currency: "USD"},
		},
		{
			name: "can go negative",
			a:    Money{Amount: 50, Currency: "USD"},
			b:    Money{Amount: 100, Currency: "USD"},
			want: Money{Amount: -50, Currency: "USD"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.a.Subtract(tt.b)
			if got != tt.want {
				t.Fatalf("Subtract() = %+v, want %+v", got, tt.want)
			}
		})
	}
}

func TestMoneySubtractPanicsOnCurrencyMismatch(t *testing.T) {
	defer func() {
		if recover() == nil {
			t.Fatal("expected panic for currency mismatch")
		}
	}()

	_ = Money{Amount: 100, Currency: "USD"}.Subtract(Money{Amount: 100, Currency: "EUR"})
}

func TestMoneyIsNegativeAndIsZero(t *testing.T) {
	if !(Money{Amount: -1, Currency: "USD"}.IsNegative()) {
		t.Fatal("expected negative amount to be negative")
	}
	if (Money{Amount: 0, Currency: "USD"}.IsNegative()) {
		t.Fatal("expected zero amount to not be negative")
	}

	if !(Money{Amount: 0, Currency: "USD"}.IsZero()) {
		t.Fatal("expected zero amount to be zero")
	}
	if (Money{Amount: 1, Currency: "USD"}.IsZero()) {
		t.Fatal("expected non-zero amount to not be zero")
	}
}

func TestMoneyFormat(t *testing.T) {
	tests := []struct {
		name string
		m    Money
		want string
	}{
		{
			name: "formats cents",
			m:    Money{Amount: 12345, Currency: "USD"},
			want: "USD 123.45",
		},
		{
			name: "formats leading zero cents",
			m:    Money{Amount: 12005, Currency: "USD"},
			want: "USD 120.05",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.m.Format()
			if got != tt.want {
				t.Fatalf("Format() = %q, want %q", got, tt.want)
			}
		})
	}
}

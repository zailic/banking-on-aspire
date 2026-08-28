// Package bank contains the preserved legacy Dapr actor implementation.
package bank

import "errors"

var (
	// ErrAccountClosed is returned when an operation is attempted on a closed account.
	ErrAccountClosed = errors.New("account is closed")
)

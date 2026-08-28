// Package server contains the preserved legacy Dapr actor implementation.
package server

import "errors"

var (
	// ErrUnauthorized is returned when the request is unauthorized.
	ErrUnauthorized = errors.New("unauthorized")

	// ErrForbidden is returned when the request is forbidden.
	ErrForbidden = errors.New("forbidden")

	// ErrInvalidAuthHeader is returned when the authorization header is invalid.
	ErrInvalidAuthHeader = errors.New("invalid authorization header")

	// ErrEmptyToken is returned when the token is empty.
	ErrEmptyToken = errors.New("empty token")
)

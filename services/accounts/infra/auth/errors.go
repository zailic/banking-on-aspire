package auth

import "errors"

var (
	// ErrUnauthorized is returned when the request is unauthorized.
	ErrUnauthorized = errors.New("unauthorized")

	// ErrForbidden is returned when the request is forbidden.
	ErrForbidden = errors.New("forbidden")
)

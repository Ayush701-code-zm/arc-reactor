package auth

import "errors"

var (
	ErrUnauthorized = errors.New("unauthorized")
	ErrForbidden    = errors.New("forbidden")
	ErrConflict     = errors.New("conflict")
	ErrInvalidCreds = errors.New("invalid username or password")
	ErrNotFound     = errors.New("not found")
)

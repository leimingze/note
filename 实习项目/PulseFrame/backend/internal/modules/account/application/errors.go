package application

import "errors"

var (
	ErrInvalidCredentials   = errors.New("invalid credentials")
	ErrUnauthorized         = errors.New("unauthorized")
	ErrRateLimited          = errors.New("rate limited")
	ErrUnavailable          = errors.New("authentication dependency unavailable")
	ErrPasswordHashCapacity = errors.New("password hash capacity exhausted")
)

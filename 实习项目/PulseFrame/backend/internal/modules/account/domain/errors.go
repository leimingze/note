package domain

import "errors"

var (
	ErrInvalidUsername = errors.New("invalid username")
	ErrInvalidPassword = errors.New("invalid password")
	ErrUsernameTaken   = errors.New("username already exists")
	ErrUserNotFound    = errors.New("user not found")
	ErrAccountDisabled = errors.New("account is disabled")
)

package store

import "errors"

var (
	ErrUsernameTaken               = errors.New("username already taken")
	ErrEmailTaken                  = errors.New("email already taken")
	ErrInvalidCredentials          = errors.New("invalid username or password")
	ErrInvalidCharactersInUsername = errors.New("invalid characters in username")
	ErrSessionExpired              = errors.New("session expired")
	ErrUsernameTooShort            = errors.New("username too short")
	ErrPasswordTooShort            = errors.New("password too short")
)

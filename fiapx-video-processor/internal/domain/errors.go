package domain

import "errors"

var (
	ErrInvalidTransition  = errors.New("invalid job status transition")
	ErrInvalidEmail       = errors.New("invalid email")
	ErrInvalidPassword    = errors.New("password must be at least 8 characters")
	ErrForbidden          = errors.New("job does not belong to user")
	ErrJobNotFound        = errors.New("job not found")
	ErrUserExists         = errors.New("user already exists")
	ErrInvalidCredentials = errors.New("invalid credentials")
	ErrJobNotReady        = errors.New("job is not ready for download")
	ErrJobNotDeletable    = errors.New("job can only be deleted after processing finishes")
	ErrUnsupportedMedia   = errors.New("unsupported video format")
	ErrUnknownProcessor   = errors.New("unknown video processor")
)

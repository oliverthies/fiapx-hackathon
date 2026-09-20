package domain

import (
	"strings"
	"time"

	"github.com/google/uuid"
)

type User struct {
	ID           uuid.UUID
	Email        string
	PasswordHash string
	CreatedAt    time.Time
}

func NewUser(email, passwordHash string) (*User, error) {
	email = strings.TrimSpace(strings.ToLower(email))
	if !strings.Contains(email, "@") || len(email) < 5 {
		return nil, ErrInvalidEmail
	}
	if passwordHash == "" {
		return nil, ErrInvalidPassword
	}
	return &User{
		ID:           uuid.New(),
		Email:        email,
		PasswordHash: passwordHash,
		CreatedAt:    time.Now().UTC(),
	}, nil
}

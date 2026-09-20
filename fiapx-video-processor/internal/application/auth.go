package application

import (
	"context"
	"strings"

	"github.com/oliverthies/fiapx-video-processor/internal/domain"
)

type AuthService struct {
	users  UserRepository
	hasher PasswordHasher
	tokens TokenIssuer
}

func NewAuthService(users UserRepository, hasher PasswordHasher, tokens TokenIssuer) *AuthService {
	return &AuthService{users: users, hasher: hasher, tokens: tokens}
}

func (s *AuthService) Register(ctx context.Context, email, password string) (*domain.User, error) {
	if len(password) < 8 {
		return nil, domain.ErrInvalidPassword
	}
	email = strings.TrimSpace(strings.ToLower(email))
	if existing, _ := s.users.FindByEmail(ctx, email); existing != nil {
		return nil, domain.ErrUserExists
	}
	hash, err := s.hasher.Hash(password)
	if err != nil {
		return nil, err
	}
	user, err := domain.NewUser(email, hash)
	if err != nil {
		return nil, err
	}
	if err := s.users.Create(ctx, user); err != nil {
		return nil, err
	}
	return user, nil
}

func (s *AuthService) Login(ctx context.Context, email, password string) (string, error) {
	user, err := s.users.FindByEmail(ctx, strings.TrimSpace(strings.ToLower(email)))
	if err != nil || user == nil {
		return "", domain.ErrInvalidCredentials
	}
	if !s.hasher.Match(user.PasswordHash, password) {
		return "", domain.ErrInvalidCredentials
	}
	return s.tokens.Issue(user.ID, user.Email)
}

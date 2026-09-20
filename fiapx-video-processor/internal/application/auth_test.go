package application

import (
	"context"
	"testing"

	"github.com/google/uuid"
	"github.com/oliverthies/fiapx-video-processor/internal/domain"
)

type memUsers struct{ byEmail map[string]*domain.User }

func (m *memUsers) Create(_ context.Context, u *domain.User) error {
	m.byEmail[u.Email] = u
	return nil
}
func (m *memUsers) FindByEmail(_ context.Context, email string) (*domain.User, error) {
	return m.byEmail[email], nil
}
func (m *memUsers) FindByID(_ context.Context, id uuid.UUID) (*domain.User, error) { return nil, nil }

type fakeHash struct{}

func (fakeHash) Hash(p string) (string, error) { return "hash:" + p, nil }
func (fakeHash) Match(hash, p string) bool     { return hash == "hash:"+p }

type fakeTok struct{}

func (fakeTok) Issue(id uuid.UUID, email string) (string, error) { return "tok-" + email, nil }

func TestRegisterRejectsShortPassword(t *testing.T) {
	s := NewAuthService(&memUsers{byEmail: map[string]*domain.User{}}, fakeHash{}, fakeTok{})
	_, err := s.Register(context.Background(), "a@b.com", "short")
	if err != domain.ErrInvalidPassword {
		t.Fatalf("got %v", err)
	}
}

func TestRegisterAndLogin(t *testing.T) {
	s := NewAuthService(&memUsers{byEmail: map[string]*domain.User{}}, fakeHash{}, fakeTok{})
	u, err := s.Register(context.Background(), "Ana@FIAPX.local", "secret123")
	if err != nil {
		t.Fatal(err)
	}
	if u.Email != "ana@fiapx.local" {
		t.Fatalf("email normalized: %s", u.Email)
	}
	tok, err := s.Login(context.Background(), "ana@fiapx.local", "secret123")
	if err != nil || tok != "tok-ana@fiapx.local" {
		t.Fatalf("login %s %v", tok, err)
	}
}

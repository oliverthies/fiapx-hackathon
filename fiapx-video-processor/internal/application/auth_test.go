package application

import (
	"context"
	"errors"
	"testing"

	"github.com/google/uuid"
	"github.com/oliverthies/fiapx-video-processor/internal/domain"
)

type memUsers struct {
	byEmail   map[string]*domain.User
	byID      map[uuid.UUID]*domain.User
	findErr   error
	createErr error
}

func (m *memUsers) Create(_ context.Context, u *domain.User) error {
	if m.createErr != nil {
		return m.createErr
	}
	m.byEmail[u.Email] = u
	if m.byID != nil {
		m.byID[u.ID] = u
	}
	return nil
}
func (m *memUsers) FindByEmail(_ context.Context, email string) (*domain.User, error) {
	if m.findErr != nil {
		return nil, m.findErr
	}
	return m.byEmail[email], nil
}
func (m *memUsers) FindByID(_ context.Context, id uuid.UUID) (*domain.User, error) {
	if m.findErr != nil {
		return nil, m.findErr
	}
	if m.byID == nil {
		return nil, nil
	}
	return m.byID[id], nil
}

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
	tok, err := s.Login(context.Background(), "  ANA@fiapx.local ", "secret123")
	if err != nil || tok != "tok-ana@fiapx.local" {
		t.Fatalf("login %s %v", tok, err)
	}
	if _, err := s.Login(context.Background(), "ana@fiapx.local", "wrong"); err != domain.ErrInvalidCredentials {
		t.Fatalf("bad password: %v", err)
	}
}

func TestRegisterRejectsDuplicateAndInvalidEmail(t *testing.T) {
	users := &memUsers{byEmail: map[string]*domain.User{
		"ana@fiapx.local": {Email: "ana@fiapx.local"},
	}}
	s := NewAuthService(users, fakeHash{}, fakeTok{})
	if _, err := s.Register(context.Background(), "ana@fiapx.local", "secret123"); err != domain.ErrUserExists {
		t.Fatalf("duplicate: %v", err)
	}
	s = NewAuthService(&memUsers{byEmail: map[string]*domain.User{}}, fakeHash{}, fakeTok{})
	if _, err := s.Register(context.Background(), "not-an-email", "secret123"); err != domain.ErrInvalidEmail {
		t.Fatalf("email: %v", err)
	}
}

func TestRegisterSurfacesHasherAndStoreErrors(t *testing.T) {
	users := &memUsers{byEmail: map[string]*domain.User{}}
	s := NewAuthService(users, errHash{err: errors.New("hash")}, fakeTok{})
	if _, err := s.Register(context.Background(), "ana@fiapx.local", "secret123"); err == nil {
		t.Fatal("expected hasher error")
	}
	s = NewAuthService(users, errHash{hash: ""}, fakeTok{})
	if _, err := s.Register(context.Background(), "ana@fiapx.local", "secret123"); err != domain.ErrInvalidPassword {
		t.Fatalf("empty hash: %v", err)
	}
	users.createErr = errors.New("db")
	s = NewAuthService(users, fakeHash{}, fakeTok{})
	if _, err := s.Register(context.Background(), "ana@fiapx.local", "secret123"); err == nil {
		t.Fatal("expected create error")
	}
}

func TestLoginRejectsUnknownUserAndTokenFailure(t *testing.T) {
	s := NewAuthService(&memUsers{byEmail: map[string]*domain.User{}}, fakeHash{}, fakeTok{})
	if _, err := s.Login(context.Background(), "missing@fiapx.local", "secret123"); err != domain.ErrInvalidCredentials {
		t.Fatalf("missing: %v", err)
	}
	s = NewAuthService(&memUsers{byEmail: map[string]*domain.User{}, findErr: errors.New("db")}, fakeHash{}, fakeTok{})
	if _, err := s.Login(context.Background(), "ana@fiapx.local", "secret123"); err != domain.ErrInvalidCredentials {
		t.Fatalf("lookup: %v", err)
	}
	users := &memUsers{byEmail: map[string]*domain.User{}}
	s = NewAuthService(users, fakeHash{}, fakeTok{})
	if _, err := s.Register(context.Background(), "ana@fiapx.local", "secret123"); err != nil {
		t.Fatal(err)
	}
	s = NewAuthService(users, fakeHash{}, errTok{})
	if _, err := s.Login(context.Background(), "ana@fiapx.local", "secret123"); err == nil {
		t.Fatal("expected token error")
	}
}

type errHash struct {
	err  error
	hash string
}

func (h errHash) Hash(string) (string, error) {
	if h.err != nil {
		return "", h.err
	}
	return h.hash, nil
}
func (errHash) Match(string, string) bool { return false }

type errTok struct{}

func (errTok) Issue(uuid.UUID, string) (string, error) { return "", errors.New("issue") }

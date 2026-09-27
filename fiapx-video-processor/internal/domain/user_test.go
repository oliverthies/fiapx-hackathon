package domain

import (
	"testing"

	"github.com/google/uuid"
)

func TestNewUser(t *testing.T) {
	u, err := NewUser("  Ana@FIAPX.local ", "hash")
	if err != nil {
		t.Fatal(err)
	}
	if u.Email != "ana@fiapx.local" || u.PasswordHash != "hash" || u.ID == uuid.Nil || u.CreatedAt.IsZero() {
		t.Fatalf("user: %+v", u)
	}

	if _, err := NewUser("bad", "hash"); err != ErrInvalidEmail {
		t.Fatalf("email: %v", err)
	}
	if _, err := NewUser("a@b", "hash"); err != ErrInvalidEmail {
		t.Fatalf("short email: %v", err)
	}
	if _, err := NewUser("ana@fiapx.local", ""); err != ErrInvalidPassword {
		t.Fatalf("hash: %v", err)
	}
}

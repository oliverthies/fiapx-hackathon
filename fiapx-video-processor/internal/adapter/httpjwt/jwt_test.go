package httpjwt

import (
	"testing"
	"time"

	"github.com/google/uuid"
)

func TestIssueAndParse(t *testing.T) {
	iss := New("test-secret", time.Hour)
	id := uuid.New()
	tok, err := iss.Issue(id, "ana@fiapx.local")
	if err != nil || tok == "" {
		t.Fatal(err)
	}
	got, err := iss.Parse(tok)
	if err != nil || got != id {
		t.Fatalf("parse: %s %v", got, err)
	}
}

func TestParseRejectsInvalidTokens(t *testing.T) {
	iss := New("test-secret", 0)
	if iss.ttl != 24*time.Hour {
		t.Fatalf("default ttl: %s", iss.ttl)
	}
	if _, err := iss.Parse("not-a-token"); err == nil {
		t.Fatal("expected invalid token")
	}
	other := New("other-secret", time.Minute)
	tok, err := other.Issue(uuid.New(), "ana@fiapx.local")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := iss.Parse(tok); err == nil {
		t.Fatal("expected signature mismatch")
	}
}

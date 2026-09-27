package crypto

import "testing"

func TestHashAndMatch(t *testing.T) {
	var h Hasher
	hash, err := h.Hash("secret123")
	if err != nil || hash == "" {
		t.Fatal(err)
	}
	if !h.Match(hash, "secret123") {
		t.Fatal("expected match")
	}
	if h.Match(hash, "other-pass") {
		t.Fatal("expected mismatch")
	}
}

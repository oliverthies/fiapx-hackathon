package platform

import "testing"

func TestEnv(t *testing.T) {
	if got := Env("FIAPX_UNIT_MISSING", "fallback"); got != "fallback" {
		t.Fatalf("missing: %s", got)
	}
	t.Setenv("FIAPX_UNIT_PRESENT", "set")
	if got := Env("FIAPX_UNIT_PRESENT", "fallback"); got != "set" {
		t.Fatalf("present: %s", got)
	}
}

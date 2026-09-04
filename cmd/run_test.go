package cmd

import "testing"

func TestFirstNonEmpty(t *testing.T) {
	if got := firstNonEmpty("", "DATABASE_URL", "fallback"); got != "DATABASE_URL" {
		t.Fatalf("firstNonEmpty() = %q", got)
	}
	if got := firstNonEmpty("", ""); got != "" {
		t.Fatalf("firstNonEmpty() = %q", got)
	}
}

package passive

import (
	"testing"
	"time"
)

func TestAmassTimeoutDefault(t *testing.T) {
	t.Setenv("AMASS_TIMEOUT_SECONDS", "")

	got := amassTimeout()
	want := time.Duration(defaultAmassTimeoutSeconds) * time.Second
	if got != want {
		t.Fatalf("amassTimeout() = %s, want %s", got, want)
	}
}

func TestAmassTimeoutDisabled(t *testing.T) {
	t.Setenv("AMASS_TIMEOUT_SECONDS", "0")

	if got := amassTimeout(); got != 0 {
		t.Fatalf("amassTimeout() = %s, want 0 when disabled", got)
	}
}

func TestAmassTimeoutCustom(t *testing.T) {
	t.Setenv("AMASS_TIMEOUT_SECONDS", "42")

	if got := amassTimeout(); got != 42*time.Second {
		t.Fatalf("amassTimeout() = %s, want 42s", got)
	}
}

func TestAmassTimeoutInvalidFallsBackToDefault(t *testing.T) {
	t.Setenv("AMASS_TIMEOUT_SECONDS", "not-a-number")

	got := amassTimeout()
	want := time.Duration(defaultAmassTimeoutSeconds) * time.Second
	if got != want {
		t.Fatalf("amassTimeout() = %s, want %s", got, want)
	}
}

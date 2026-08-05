package agent

import (
	"context"
	"errors"
	"testing"
)

func TestNewRunnerReturnsNativeForNative(t *testing.T) {
	if got := NewRunner(Native).Name(); got != Native {
		t.Errorf("NewRunner(native).Name() = %q, want native", got)
	}
	if got := NewRunner("claude").Name(); got != "claude" {
		t.Errorf("NewRunner(claude).Name() = %q, want claude", got)
	}
}

func TestNativeRunnerInteractiveCommand(t *testing.T) {
	cmd, ok := NewRunner(Native).InteractiveCommand()
	if !ok {
		t.Fatal("native InteractiveCommand ok = false")
	}
	if cmd == "" || cmd[len(cmd)-len(" agent native"):] != " agent native" {
		t.Errorf("native InteractiveCommand = %q, want it to end in ' agent native'", cmd)
	}
}

func TestNativeAvailabilityFollowsAPIKey(t *testing.T) {
	t.Setenv("ANTHROPIC_API_KEY", "")
	if NativeAvailable() {
		t.Error("native should be unavailable with no API key")
	}
	if Available(Native) {
		t.Error("Available(native) should be false with no API key")
	}
	// Headless without a key behaves like an agent with no one-shot mode.
	_, err := NewRunner(Native).Headless(context.Background(), t.TempDir(), "hi")
	if !errors.Is(err, ErrNoHeadless) {
		t.Errorf("native Headless with no key = %v, want ErrNoHeadless", err)
	}

	t.Setenv("ANTHROPIC_API_KEY", "test-key")
	if !NativeAvailable() {
		t.Error("native should be available once a key is set")
	}
	// Detect lists native first when configured.
	if d := Detect(); len(d) == 0 || d[0] != Native {
		t.Errorf("Detect() = %v, want native first", d)
	}
	if !SupportsHeadless(Native) {
		t.Error("native should support headless when a key is set")
	}
}

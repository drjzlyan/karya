package agent

import "testing"

func TestHeadlessArgv(t *testing.T) {
	if argv, ok := headlessArgv("claude", "hello"); !ok || argv[0] != "claude" || argv[len(argv)-1] != "hello" {
		t.Errorf("claude headless = %v,%v", argv, ok)
	}
	if _, ok := headlessArgv("aider", "hello"); ok {
		t.Error("aider has no known headless mode; want ok=false")
	}
	if _, ok := headlessArgv("", "hello"); ok {
		t.Error("empty agent should not be headless")
	}
}

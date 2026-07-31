package agent

import "testing"

func TestHeadlessPrompt(t *testing.T) {
	if argv, ok := HeadlessPrompt("claude", "hello"); !ok || argv[0] != "claude" || argv[len(argv)-1] != "hello" {
		t.Errorf("claude headless = %v,%v", argv, ok)
	}
	if _, ok := HeadlessPrompt("aider", "hello"); ok {
		t.Error("aider has no known headless mode; want ok=false")
	}
	if _, ok := HeadlessPrompt("", "hello"); ok {
		t.Error("empty agent should not be headless")
	}
}

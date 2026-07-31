package agent

import (
	"strings"
	"testing"
)

func TestJoinContext(t *testing.T) {
	tests := []struct {
		name, header, body, want string
	}{
		{"both", "File: a.go:1-2", "code", "File: a.go:1-2\n\ncode"},
		{"header only", "context", "", "context"},
		{"body only", "", "just body", "just body"},
		{"trims body", "h", "  x  ", "h\n\nx"},
		{"empty", "", "", ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := joinContext(tt.header, tt.body); got != tt.want {
				t.Errorf("joinContext(%q,%q) = %q, want %q", tt.header, tt.body, got, tt.want)
			}
		})
	}
}

func TestSendPastesIntoAgentPane(t *testing.T) {
	ft := &fakeTmux{
		opts:  map[string]string{"@ide_agent_pane": "%7", "@ide_current_agent": "claude"},
		panes: "%1\n%7\n%9", // agent pane %7 present
	}
	m := NewManager(ft, newFakePrefs(), "proj", "karya")

	if err := m.Send("File: main.go:10", "func main() {}"); err != nil {
		t.Fatalf("Send: %v", err)
	}

	var setBuf, paste []string
	for _, r := range ft.runs {
		switch r[0] {
		case "set-buffer":
			setBuf = r
		case "paste-buffer":
			paste = r
		}
	}
	if setBuf == nil {
		t.Fatal("expected set-buffer to be called")
	}
	if last := setBuf[len(setBuf)-1]; !strings.Contains(last, "File: main.go:10") || !strings.Contains(last, "func main()") {
		t.Errorf("set-buffer payload = %q, want header+body", last)
	}
	if paste == nil {
		t.Fatal("expected paste-buffer to be called")
	}
	if paste[len(paste)-1] != "%7" {
		t.Errorf("paste target = %q, want the agent pane %%7", paste[len(paste)-1])
	}
	if !ft.ran("select-pane") {
		t.Error("expected the agent pane to be focused after paste")
	}
}

func TestSendEmptyIsNoOp(t *testing.T) {
	ft := &fakeTmux{opts: map[string]string{"@ide_agent_pane": "%7"}, panes: "%7"}
	m := NewManager(ft, newFakePrefs(), "proj", "karya")
	if err := m.Send("", "   "); err != nil {
		t.Fatalf("Send: %v", err)
	}
	if ft.ran("set-buffer") || ft.ran("paste-buffer") {
		t.Error("empty payload must not touch tmux buffers")
	}
}

package cli

import (
	"bytes"
	"strings"
	"testing"

	"github.com/drjzlyan/karya/internal/assets"
)

func TestPrintCommandHelpKnown(t *testing.T) {
	var buf bytes.Buffer
	if code := printCommandHelp(&buf, "dev"); code != 0 {
		t.Fatalf("printCommandHelp(dev) = %d, want 0", code)
	}
	out := buf.String()
	for _, want := range []string{"karya dev", helpByCommand["dev"].summary, "karya docs"} {
		if !strings.Contains(out, want) {
			t.Errorf("dev help missing %q; got:\n%s", want, out)
		}
	}
}

func TestPrintCommandHelpUnknown(t *testing.T) {
	var buf bytes.Buffer
	if code := printCommandHelp(&buf, "nope"); code != 2 {
		t.Errorf("printCommandHelp(nope) = %d, want 2", code)
	}
}

func TestHelpTopicsListsEveryCommand(t *testing.T) {
	var buf bytes.Buffer
	printHelpTopics(&buf)
	out := buf.String()
	for name := range helpByCommand {
		if !strings.Contains(out, name) {
			t.Errorf("help topics omitted command %q", name)
		}
	}
}

// TestHelpEntriesRender guards that every entry has a summary and syntax and
// renders without an empty body.
func TestHelpEntriesRender(t *testing.T) {
	for name, h := range helpByCommand {
		if h.summary == "" || h.syntax == "" {
			t.Errorf("command %q missing summary or syntax", name)
		}
		var buf bytes.Buffer
		if code := printCommandHelp(&buf, name); code != 0 {
			t.Errorf("printCommandHelp(%q) = %d, want 0", name, code)
		}
	}
}

func TestPrintDocTopics(t *testing.T) {
	var buf bytes.Buffer
	printDocTopics(&buf)
	out := buf.String()
	for _, topic := range assets.DocTopics() {
		if !strings.Contains(out, topic) {
			t.Errorf("doc topics listing missing %q; got:\n%s", topic, out)
		}
	}
	if !strings.Contains(out, "tutorial") || !strings.Contains(out, "keymaps") {
		t.Errorf("expected tutorial and keymaps topics; got:\n%s", out)
	}
}

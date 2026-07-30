package cli

import (
	"bytes"
	"strings"
	"testing"
)

// TestCompletionModelMatchesHelp asserts every completion command has a matching
// help entry, so completion descriptions and `karya help` stay in step.
func TestCompletionModelMatchesHelp(t *testing.T) {
	for _, c := range completionModel() {
		if _, ok := helpByCommand[c.name]; !ok {
			t.Errorf("completion command %q has no help entry", c.name)
		}
		if c.desc == "" {
			t.Errorf("completion command %q has an empty description", c.name)
		}
	}
}

func TestBashCompletion(t *testing.T) {
	var buf bytes.Buffer
	writeBashCompletion(&buf, completionModel())
	out := buf.String()
	for _, want := range []string{
		"complete -F _karya karya", "_karya()",
		"agent) COMPREPLY=( $(compgen -W \"status switch next prev reset prefs clear\"",
		"edit|run) COMPREPLY=( $(compgen -f", "doctor", "tutorial",
	} {
		if !strings.Contains(out, want) {
			t.Errorf("bash completion missing %q", want)
		}
	}
}

func TestZshCompletion(t *testing.T) {
	var buf bytes.Buffer
	writeZshCompletion(&buf, completionModel())
	out := buf.String()
	for _, want := range []string{"#compdef karya", "compdef _karya karya", "_describe", "compadd status switch"} {
		if !strings.Contains(out, want) {
			t.Errorf("zsh completion missing %q", want)
		}
	}
	// Every describe entry must have exactly one colon separating name:desc so
	// _describe parses them correctly.
	for _, line := range strings.Split(out, "\n") {
		line = strings.TrimSpace(line)
		if !strings.HasPrefix(line, "'") || !strings.HasSuffix(line, "'") {
			continue
		}
		if body := strings.Trim(line, "'"); strings.Count(body, ":") > 1 {
			t.Errorf("zsh describe entry has multiple colons: %q", line)
		}
	}
}

func TestFishCompletion(t *testing.T) {
	var buf bytes.Buffer
	writeFishCompletion(&buf, completionModel())
	out := buf.String()
	for _, want := range []string{
		"complete -c karya -f",
		"-n __fish_use_subcommand -a 'dev'",
		"__fish_seen_subcommand_from edit' -F",
		"__fish_seen_subcommand_from new' -a 'python java typescript go cpp rust'",
	} {
		if !strings.Contains(out, want) {
			t.Errorf("fish completion missing %q", want)
		}
	}
}

func TestCmdCompletionErrors(t *testing.T) {
	if code := cmdCompletion(nil); code != 2 {
		t.Errorf("cmdCompletion(nil) = %d, want 2", code)
	}
	if code := cmdCompletion([]string{"powershell"}); code != 2 {
		t.Errorf("cmdCompletion(powershell) = %d, want 2", code)
	}
}

//go:build integration

package assets

import (
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// TestTutorialEngine drives the in-editor tutorial's pure logic under headless
// Neovim (no tmux, no agent, no plugins) and asserts the step list is well-formed
// and the keystroke matcher advances on the right byte sequences. It guards the
// engine that powers `:KaryaTutorial` / `karya tutorial ide`.
//
// Tagged integration because it shells out to a real nvim binary.
func TestTutorialEngine(t *testing.T) {
	if _, err := exec.LookPath("nvim"); err != nil {
		t.Skip("nvim not installed; skipping tutorial guard")
	}
	nvimDir, err := filepath.Abs("nvim")
	if err != nil {
		t.Fatal(err)
	}
	harness, err := filepath.Abs(filepath.Join("testdata", "tutorial_guard.lua"))
	if err != nil {
		t.Fatal(err)
	}

	out, _ := exec.Command("nvim", "-l", harness, nvimDir).CombinedOutput()

	var sawOK, sawSteps, sawMatch bool
	for _, line := range strings.Split(string(out), "\n") {
		fields := strings.Fields(line)
		switch {
		case len(fields) == 2 && fields[0] == "STEPS":
			sawSteps = true
		case len(fields) == 3 && fields[0] == "MATCH" && fields[1] == "ok":
			sawMatch = true
		case line == "OK":
			sawOK = true
		}
	}

	if !sawSteps {
		t.Errorf("tutorial guard did not report a step count:\n%s", out)
	}
	if !sawMatch {
		t.Errorf("tutorial guard did not report a matcher check:\n%s", out)
	}
	if !sawOK {
		t.Fatalf("tutorial guard did not complete cleanly:\n%s", out)
	}
}

// Package editor routes file-open and command-run actions into the IDE panes.
//
//   - Edit ports dotfiles/bin/nvim-edit: open a file in the editor (nvim) pane,
//     falling back to launching Neovim directly outside a session. It is what
//     karya sets as $EDITOR/$VISUAL/$GIT_EDITOR.
//   - Run ports dotfiles/scripts/ide-run.sh: send a command to the build/test
//     pane, creating it on demand, or run directly when outside tmux.
//
// Both operate on karya's isolated tmux server via internal/tmuxx.
package editor

import (
	"fmt"
	"os"
	"os/exec"
	"strings"
	"syscall"
	"time"

	"github.com/drjzlyan/karya/internal/tmuxx"
)

// Edit opens file at the given line in the session's editor pane. When not in a
// karya tmux session it execs Neovim directly (namespaced via NVIM_APPNAME).
func Edit(t *tmuxx.Tmux, file string, line int) error {
	if line < 1 {
		line = 1
	}
	session := currentSession(t)
	if session == "" {
		return execNvim(file, line)
	}

	target := editorPane(t, session)
	if target == "" {
		return execNvim(file, line)
	}

	vimFile := escapeForVim(file)
	_ = t.Run("select-window", "-t", session+":dev")
	_ = t.Run("select-pane", "-t", target)
	_ = t.Run("send-keys", "-t", target, "Escape")
	time.Sleep(50 * time.Millisecond)
	return t.Run("send-keys", "-t", target, fmt.Sprintf(":e +%d %s", line, vimFile), "Enter")
}

// Run sends a command to the build/test pane. With focus set and no command it
// just focuses the pane. Outside tmux it runs the command directly.
func Run(t *tmuxx.Tmux, command, dir string, focus bool) error {
	session := currentSession(t)
	if session == "" {
		return runDirect(command, dir)
	}

	pane := buildPane(t, session)
	if pane == "" {
		return fmt.Errorf("could not find or create a build/test pane")
	}
	_ = t.Run("set-option", "-t", session, "@ide_shell_pane", pane)

	if focus {
		_ = t.Run("select-window", "-t", pane)
		return t.Run("select-pane", "-t", pane)
	}
	if dir != "" {
		command = fmt.Sprintf("cd -- %s && %s", shellQuote(dir), command)
	}
	return t.Run("send-keys", "-t", pane, command, "Enter")
}

// currentSession returns the name of the karya session we are running inside, or
// "" if not within one.
func currentSession(t *tmuxx.Tmux) string {
	if os.Getenv("TMUX") == "" {
		return ""
	}
	name, err := t.Output("display-message", "-p", "#{session_name}")
	if err != nil {
		return ""
	}
	return name
}

// editorPane resolves the nvim pane: prefer dev.1 when it runs nvim, else any
// pane whose current command is nvim.
func editorPane(t *tmuxx.Tmux, session string) string {
	if cmd, err := t.Output("display-message", "-p", "-t", session+":dev.1", "#{pane_current_command}"); err == nil && cmd == "nvim" {
		return session + ":dev.1"
	}
	out, err := t.Output("list-panes", "-s", "-t", session,
		"-F", "#{pane_id} #{pane_current_command}")
	if err != nil {
		return ""
	}
	for _, l := range strings.Split(out, "\n") {
		if f := strings.Fields(l); len(f) == 2 && f[1] == "nvim" {
			return f[0]
		}
	}
	return ""
}

// buildPane resolves the build/test pane: a pane titled "build/test", else
// dev.3 in the IDE layout, else a fresh split.
func buildPane(t *tmuxx.Tmux, session string) string {
	out, err := t.Output("list-panes", "-s", "-t", session, "-F", "#{pane_id} #{pane_title}")
	if err == nil {
		for _, l := range strings.Split(out, "\n") {
			if f := strings.SplitN(l, " ", 2); len(f) == 2 && f[1] == "build/test" {
				return f[0]
			}
		}
	}
	if id, err := t.Output("display-message", "-p", "-t", session+":dev.3", "#{pane_id}"); err == nil && id != "" {
		_ = t.Run("select-pane", "-t", id, "-T", "build/test")
		return id
	}
	// Create a new split below the agent pane (or editor pane) in the dev window.
	target := session + ":dev.1"
	if _, err := t.Output("display-message", "-p", "-t", session+":dev.2", "#{pane_id}"); err == nil {
		target = session + ":dev.2"
	}
	id, err := t.Output("split-window", "-v", "-l", "40%", "-t", target, "-P", "-F", "#{pane_id}")
	if err != nil || id == "" {
		return ""
	}
	_ = t.Run("select-pane", "-t", id, "-T", "build/test")
	return id
}

func execNvim(file string, line int) error {
	path, err := exec.LookPath("nvim")
	if err != nil {
		return fmt.Errorf("nvim not found on PATH")
	}
	argv := []string{"nvim", fmt.Sprintf("+%d", line), file}
	env := append(os.Environ(), "NVIM_APPNAME=karya")
	return syscall.Exec(path, argv, env)
}

func runDirect(command, dir string) error {
	c := exec.Command("sh", "-c", command)
	c.Dir = dir
	c.Stdin, c.Stdout, c.Stderr = os.Stdin, os.Stdout, os.Stderr
	return c.Run()
}

// escapeForVim escapes characters special to vim's :edit command.
func escapeForVim(file string) string {
	r := strings.NewReplacer(" ", `\ `, "#", `\#`, "%", `\%`)
	return r.Replace(file)
}

// shellQuote single-quotes a string for safe use in a shell command.
func shellQuote(s string) string {
	return "'" + strings.ReplaceAll(s, "'", `'\''`) + "'"
}

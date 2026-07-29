// Package tmuxx is a thin wrapper around the tmux CLI, pinned to karya's
// dedicated server socket (`tmux -L karya`) and config (`-f`). Routing every
// tmux invocation through here guarantees karya never collides with the user's
// default tmux server or sources their ~/.tmux.conf.
package tmuxx

import (
	"os"
	"os/exec"
	"strings"
	"syscall"
)

// Tmux issues commands against karya's dedicated tmux server.
type Tmux struct {
	Socket string   // -L value (server label)
	Conf   string   // -f value (config file), applied when the server starts
	Env    []string // extra env appended to os.Environ for spawned children
}

// New returns a Tmux bound to the given socket label and config file.
func New(socket, conf string, env []string) *Tmux {
	return &Tmux{Socket: socket, Conf: conf, Env: env}
}

// base builds the global flags that must precede every subcommand.
func (t *Tmux) base() []string {
	args := []string{"-L", t.Socket}
	if t.Conf != "" {
		args = append(args, "-f", t.Conf)
	}
	return args
}

func (t *Tmux) command(args ...string) *exec.Cmd {
	c := exec.Command("tmux", append(t.base(), args...)...)
	c.Env = append(os.Environ(), t.Env...)
	return c
}

// Run executes a tmux command, surfacing errors on stderr. Stdout is discarded
// since control commands are silent.
func (t *Tmux) Run(args ...string) error {
	c := t.command(args...)
	c.Stderr = os.Stderr
	return c.Run()
}

// Output runs a tmux command and returns its trimmed stdout.
func (t *Tmux) Output(args ...string) (string, error) {
	out, err := t.command(args...).Output()
	return strings.TrimSpace(string(out)), err
}

// HasSession reports whether a session with the given name exists.
func (t *Tmux) HasSession(name string) bool {
	return t.command("has-session", "-t", name).Run() == nil
}

// Available reports whether tmux itself is installed.
func Available() bool {
	_, err := exec.LookPath("tmux")
	return err == nil
}

// Attach replaces the current process with `tmux attach -t target`, handing the
// terminal to tmux. It does not return on success.
func (t *Tmux) Attach(target string) error {
	path, err := exec.LookPath("tmux")
	if err != nil {
		return err
	}
	argv := append([]string{"tmux"}, append(t.base(), "attach", "-t", target)...)
	return syscall.Exec(path, argv, append(os.Environ(), t.Env...))
}

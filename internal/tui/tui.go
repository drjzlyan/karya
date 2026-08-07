// Package tui is karya's minimal, stdlib-only Elm-style TUI runtime: the
// Model/Update/View contract and the Program loop that drives it against the
// terminal (DESIGN.md §6.1, §8.1).
//
// The contract is what makes the whole IDE testable: a Model's Update is a pure
// (Model, Msg) -> (Model, Cmd) transition and its View is a pure render into a
// cellbuf.Buffer, so components are tested by feeding messages and asserting the
// model or a golden snapshot — no terminal required. All side effects are Cmds
// executed off the render path by the Program.
package tui

import "github.com/drjzlyan/karya/internal/cellbuf"

// Msg is any message delivered to a Model's Update. Terminal events
// (term.KeyEvent, term.ResizeEvent, …) are Msgs, as are Cmd results.
type Msg interface{}

// Cmd is a side effect run off the render path; its result (if any) is delivered
// back as a Msg. A nil Cmd does nothing.
type Cmd func() Msg

// Model is a TUI component: pure state with an update/view split.
type Model interface {
	// Init returns an optional command to run when the program starts.
	Init() Cmd
	// Update handles a message, returning the next model and an optional command.
	Update(Msg) (Model, Cmd)
	// View renders the current state into buf (sized to the screen).
	View(buf *cellbuf.Buffer)
}

// quitMsg signals the program loop to stop.
type quitMsg struct{}

// Quit is a Cmd that stops the program.
func Quit() Msg { return quitMsg{} }

// batchMsg carries several commands to be run concurrently.
type batchMsg struct{ cmds []Cmd }

// Batch groups commands so a single Update can trigger several side effects.
// nil commands are dropped; an empty batch is a no-op.
func Batch(cmds ...Cmd) Cmd {
	filtered := make([]Cmd, 0, len(cmds))
	for _, c := range cmds {
		if c != nil {
			filtered = append(filtered, c)
		}
	}
	if len(filtered) == 0 {
		return nil
	}
	return func() Msg { return batchMsg{cmds: filtered} }
}

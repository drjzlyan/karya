// Package keymap is karya's single, unified keymap engine. Every keystroke in
// the IDE flows through it first: it resolves leader chords into karya Actions,
// tracks a pending chord for which-key discovery, and reports which keys should
// be forwarded to the focused pane (the embedded editor or a PTY). There is one
// leader (Ctrl+Space) and one binding table for the whole IDE — no per-tool
// keymaps (DESIGN.md §6.2).
//
// The engine is pure and data-driven: Feed is a function of (key, context, and
// the pending chord), with no terminal involved, so keybinding behavior is
// exhaustively table-testable.
package keymap

import "github.com/drjzlyan/karya/internal/term"

// ActionID names a karya IDE action dispatched by a binding.
type ActionID string

// Focus is what currently has keyboard focus; bindings may be scoped to it.
type Focus uint8

// Focus kinds.
const (
	FocusAny Focus = iota
	FocusEditor
	FocusTerminal
	FocusView
)

// Context is the state the engine resolves keys against.
type Context struct {
	Focus Focus
}

// Binding maps a chord (a sequence of keys after nothing — the leader is just
// the first key of the sequence) to an Action. Desc and Group drive the
// which-key popup.
type Binding struct {
	Keys   []term.Key
	Action ActionID
	Desc   string
	Group  string // label shown for the group this binding lives under
	// When, if non-nil, restricts the binding to contexts it accepts.
	When func(Context) bool
}

// ResKind is the category of a Resolution.
type ResKind uint8

// Resolution kinds.
const (
	// ResForward: the key is not claimed; send it to the focused pane.
	ResForward ResKind = iota
	// ResDispatch: a complete binding matched; Action should be run.
	ResDispatch
	// ResPending: the key extends a chord; more input is expected. Pending
	// lists the possible continuations for a which-key popup.
	ResPending
	// ResCancel: a pending chord was cancelled (Esc).
	ResCancel
	// ResNoMatch: a pending chord was completed by an unbound key; nothing runs.
	ResNoMatch
)

// Resolution is the outcome of feeding one key.
type Resolution struct {
	Kind    ResKind
	Action  ActionID    // for ResDispatch
	Key     term.Key    // for ResForward (the key to forward)
	Pending []Candidate // for ResPending
}

// Candidate is one possible continuation of a pending chord, shown in which-key.
type Candidate struct {
	Key     term.Key
	Desc    string
	Group   string
	IsGroup bool // true if this key leads to further chords rather than an action
}

// Engine resolves keys against a binding table, tracking the pending chord.
type Engine struct {
	bindings []Binding
	leader   term.Key
	pending  []term.Key
}

// New returns an Engine over the given bindings, using the default leader
// (Ctrl+Space).
func New(bindings []Binding) *Engine {
	return &Engine{bindings: bindings, leader: Leader}
}

// NewWithLeader is like New but with an explicit leader key.
func NewWithLeader(bindings []Binding, leader term.Key) *Engine {
	return &Engine{bindings: bindings, leader: leader}
}

// Pending reports the chord accumulated so far (nil when idle). Callers use its
// length to show the leader/which-key state.
func (e *Engine) Pending() []term.Key { return e.pending }

// Reset clears any pending chord.
func (e *Engine) Reset() { e.pending = nil }

var escKey = term.Named(term.SymEsc)

// Feed resolves a single key in the given context.
func (e *Engine) Feed(key term.Key, ctx Context) Resolution {
	// Esc cancels an in-progress chord.
	if len(e.pending) > 0 && key == escKey {
		e.pending = nil
		return Resolution{Kind: ResCancel}
	}

	trial := make([]term.Key, len(e.pending)+1)
	copy(trial, e.pending)
	trial[len(e.pending)] = key

	exact, hasCont := e.lookup(trial, ctx)
	switch {
	case exact != nil:
		e.pending = nil
		return Resolution{Kind: ResDispatch, Action: exact.Action}
	case hasCont:
		e.pending = trial
		return Resolution{Kind: ResPending, Pending: e.candidates(trial, ctx)}
	case len(e.pending) == 0 && key == e.leader:
		// The leader is always intercepted so it never leaks to the pane, even
		// when no binding currently continues it (e.g. all are context-filtered).
		e.pending = trial
		return Resolution{Kind: ResPending, Pending: e.candidates(trial, ctx)}
	default:
		if len(e.pending) == 0 {
			// The first key started no binding: forward it to the pane.
			return Resolution{Kind: ResForward, Key: key}
		}
		// Mid-chord dead end.
		e.pending = nil
		return Resolution{Kind: ResNoMatch}
	}
}

// lookup reports the exact binding for seq (if any) and whether seq is a strict
// prefix of at least one binding, both filtered by context.
func (e *Engine) lookup(seq []term.Key, ctx Context) (exact *Binding, hasCont bool) {
	for i := range e.bindings {
		b := &e.bindings[i]
		if !b.applies(ctx) {
			continue
		}
		if keysEqual(b.Keys, seq) {
			exact = b
			continue
		}
		if hasPrefix(b.Keys, seq) {
			hasCont = true
		}
	}
	return exact, hasCont
}

// candidates returns the deduped continuations of prefix, in table order.
func (e *Engine) candidates(prefix []term.Key, ctx Context) []Candidate {
	var out []Candidate
	seen := make(map[term.Key]int) // key -> index in out
	for i := range e.bindings {
		b := &e.bindings[i]
		if !b.applies(ctx) || !hasPrefix(b.Keys, prefix) {
			continue
		}
		next := b.Keys[len(prefix)]
		isGroup := len(b.Keys) > len(prefix)+1
		if idx, ok := seen[next]; ok {
			// Prefer a concrete leaf description over a group placeholder.
			if !isGroup {
				out[idx] = Candidate{Key: next, Desc: b.Desc, Group: b.Group}
			}
			continue
		}
		c := Candidate{Key: next, Desc: b.Desc, Group: b.Group, IsGroup: isGroup}
		if isGroup && c.Desc == "" {
			c.Desc = b.Group
		}
		seen[next] = len(out)
		out = append(out, c)
	}
	return out
}

func (b *Binding) applies(ctx Context) bool {
	if b.When == nil {
		return true
	}
	return b.When(ctx)
}

func keysEqual(a, b []term.Key) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

// hasPrefix reports whether pre is a strict prefix of seq.
func hasPrefix(seq, pre []term.Key) bool {
	if len(pre) >= len(seq) {
		return false
	}
	for i := range pre {
		if seq[i] != pre[i] {
			return false
		}
	}
	return true
}

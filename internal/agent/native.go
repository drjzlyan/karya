package agent

import (
	"context"
	"os"

	"github.com/drjzlyan/karya/internal/native"
)

// Native is the identifier for karya's built-in Claude-API agent engine — the
// second Runner implementation that Phase 9's interface was designed to admit
// (ROADMAP Phase 13). BYO-CLI agents stay the default; native is offered only
// when an API key is configured.
const Native = "native"

// NativeAvailable reports whether the native engine can run (an API key is set).
func NativeAvailable() bool { return native.Available() }

// nativeRunner drives karya's built-in engine behind the Runner interface. Its
// interactive form is karya running its own agent loop in the pane; its headless
// form is a one-shot Claude-API chat.
type nativeRunner struct {
	bin string // absolute karya binary, so the pane launch works regardless of PATH
}

// newNativeRunner builds a nativeRunner bound to the running karya binary.
func newNativeRunner() Runner {
	bin, err := os.Executable()
	if err != nil || bin == "" {
		bin = "karya"
	}
	return &nativeRunner{bin: bin}
}

// NewRunner returns the Runner for name: the native engine for Native, otherwise
// a BYO-CLI runner. It is the single construction point so every consumer
// (session launch, agent management, ship/task authoring) is engine-agnostic.
func NewRunner(name string) Runner {
	if name == Native {
		return newNativeRunner()
	}
	return NewCLIRunner(name)
}

// Name implements Runner.
func (r *nativeRunner) Name() string { return Native }

// InteractiveCommand implements Runner: the pane runs karya's own agent loop.
func (r *nativeRunner) InteractiveCommand() (string, bool) {
	return r.bin + " agent native", true
}

// Headless implements Runner using a one-shot Claude-API chat. It returns
// ErrNoHeadless when no key is configured, so callers fall back exactly as they
// would for a CLI without a one-shot mode.
func (r *nativeRunner) Headless(ctx context.Context, dir, prompt string) (string, error) {
	c, ok := native.New()
	if !ok {
		return "", ErrNoHeadless
	}
	return c.Chat(ctx, dir, prompt)
}

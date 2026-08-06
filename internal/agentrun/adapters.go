package agentrun

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"strings"
)

// execFunc is the seam every adapter runs its argv through: it executes
// argv[0] in dir with extraEnv appended to the process environment and returns
// combined-captured stdout. Tests substitute it to assert the exact argv each
// adapter emits without spawning real CLIs.
type execFunc func(ctx context.Context, dir string, argv, extraEnv []string) (string, error)

// execAgent is the production execFunc. It verifies the binary resolves, runs
// it in dir, and folds stderr into the error on failure so the CLI can explain
// what the agent said before dying.
func execAgent(ctx context.Context, dir string, argv, extraEnv []string) (string, error) {
	if len(argv) == 0 {
		return "", fmt.Errorf("agentrun: empty argv")
	}
	if _, err := exec.LookPath(argv[0]); err != nil {
		return "", fmt.Errorf("agent %q not on PATH: %w", argv[0], err)
	}
	cmd := exec.CommandContext(ctx, argv[0], argv[1:]...)
	cmd.Dir = dir
	if len(extraEnv) > 0 {
		cmd.Env = append(os.Environ(), extraEnv...)
	}
	var stderr strings.Builder
	cmd.Stderr = &stderr
	out, err := cmd.Output()
	if err != nil {
		if msg := strings.TrimSpace(stderr.String()); msg != "" {
			return "", fmt.Errorf("agent %q failed: %s: %w", argv[0], msg, err)
		}
		return "", fmt.Errorf("agent %q failed: %w", argv[0], err)
	}
	return string(out), nil
}

// cliAdapter normalizes one coding-agent CLI behind the Agent interface. Only
// well-documented non-interactive entry points are wired (per AGENTS.md's
// no-invented-flags rule); planArgv is nil when the CLI has no native
// read-only plan mode, in which case the plan step runs the same one-shot with
// the read-only prompt scaffold (plan emulation — Caps.PlanMode reports it).
type cliAdapter struct {
	name     string
	caps     Caps
	argv     func(prompt string) []string
	planArgv func(prompt string) []string
	exec     execFunc
}

// Name implements Agent.
func (a *cliAdapter) Name() string { return a.name }

// Caps implements Agent.
func (a *cliAdapter) Caps() Caps { return a.caps }

// Plan implements Agent: native plan mode when the CLI has one, otherwise the
// regular one-shot — the prompt's planning rules are the emulation scaffold.
func (a *cliAdapter) Plan(ctx context.Context, dir, prompt string) (string, error) {
	argv := a.planArgv
	if argv == nil {
		argv = a.argv
	}
	return a.exec(ctx, dir, argv(prompt), nil)
}

// Implement implements Agent using the agent's one-shot mode in the task
// worktree.
func (a *cliAdapter) Implement(ctx context.Context, dir, prompt string) (string, error) {
	return a.exec(ctx, dir, a.argv(prompt), nil)
}

// claude adapts Anthropic's Claude Code CLI.
// Headless: `claude -p <prompt>`; native plan mode: `--permission-mode plan`
// (the CLI refuses edits). Resume (`-c`), skills (~/.claude/skills), MCP
// (.mcp.json), and streaming (`--output-format stream-json`) are documented
// capabilities of the CLI.
func claude() Agent {
	return &cliAdapter{
		name: "claude",
		caps: Caps{PlanMode: true, Headless: true, Resume: true, Skills: true, MCP: true, Streaming: true},
		argv: func(prompt string) []string {
			return []string{"claude", "-p", prompt}
		},
		planArgv: func(prompt string) []string {
			return []string{"claude", "-p", "--permission-mode", "plan", prompt}
		},
		exec: defaultExec,
	}
}

// codex adapts OpenAI's Codex CLI. Headless: `codex exec <prompt>`; the plan
// step adds `--sandbox read-only`, the CLI-enforced no-edits mode that serves
// as its plan mode.
func codex() Agent {
	return &cliAdapter{
		name: "codex",
		caps: Caps{PlanMode: true, Headless: true, Resume: true, MCP: true},
		argv: func(prompt string) []string {
			return []string{"codex", "exec", prompt}
		},
		planArgv: func(prompt string) []string {
			return []string{"codex", "exec", "--sandbox", "read-only", prompt}
		},
		exec: defaultExec,
	}
}

// crush adapts Charm's Crush CLI. Headless: `crush run <prompt>`. Crush has no
// documented read-only one-shot mode, so its plan step is prompt-scaffold
// emulation.
func crush() Agent {
	return &cliAdapter{
		name: "crush",
		caps: Caps{Headless: true, Skills: true, MCP: true},
		argv: func(prompt string) []string {
			return []string{"crush", "run", prompt}
		},
		exec: defaultExec,
	}
}

// gemini adapts Google's Gemini CLI. Headless: `gemini -p <prompt>`; no
// documented read-only one-shot mode (plan step is emulation).
func gemini() Agent {
	return &cliAdapter{
		name: "gemini",
		caps: Caps{Headless: true, MCP: true},
		argv: func(prompt string) []string {
			return []string{"gemini", "-p", prompt}
		},
		exec: defaultExec,
	}
}

// aider adapts the aider CLI. Headless (scripting mode): `aider -m <prompt>`
// processes one message and exits; `--yes-always` answers its confirmations
// and `--no-auto-commits` keeps git mutations karya's job (agents never
// commit). No read-only one-shot mode (plan step is emulation).
func aider() Agent {
	return &cliAdapter{
		name: "aider",
		caps: Caps{Headless: true},
		argv: func(prompt string) []string {
			return []string{"aider", "--yes-always", "--no-auto-commits", "-m", prompt}
		},
		exec: defaultExec,
	}
}

// copilot adapts the GitHub Copilot CLI. Headless: `copilot -p <prompt>` with
// `--allow-all-tools` so the non-interactive run is not stranded on tool
// approval prompts (the task worktree bounds the blast radius). No read-only
// one-shot mode (plan step is emulation).
func copilot() Agent {
	return &cliAdapter{
		name: "copilot",
		caps: Caps{Headless: true, MCP: true},
		argv: func(prompt string) []string {
			return []string{"copilot", "--allow-all-tools", "-p", prompt}
		},
		exec: defaultExec,
	}
}

// shellEnv is the environment variable naming the command behind the generic
// shell adapter, e.g. KARYA_AGENT_SHELL="my-agent --do-it".
const shellEnv = "KARYA_AGENT_SHELL"

// promptFileEnv is the environment variable that carries the prompt file path
// to the shell adapter's command.
const promptFileEnv = "KARYA_PROMPT_FILE"

// shellAdapter is the generic adapter (DESIGN.md §5): any CLI that reads a
// prompt file and edits its cwd, so unsupported agents still work in degraded
// mode. The command comes from KARYA_AGENT_SHELL; karya writes the assembled
// prompt to a temp file and exports its path as KARYA_PROMPT_FILE. Everything
// except a headless one-shot is absent — that absence is the point of the
// Caps matrix.
type shellAdapter struct {
	exec execFunc
}

// newShellAdapter returns the generic shell adapter.
func newShellAdapter() Agent { return &shellAdapter{exec: defaultExec} }

// Name implements Agent.
func (a *shellAdapter) Name() string { return "shell" }

// Caps implements Agent: the shell adapter only guarantees a headless run.
func (a *shellAdapter) Caps() Caps { return Caps{Headless: true} }

// Plan implements Agent (prompt-scaffold emulation; there is no native plan
// mode to map to).
func (a *shellAdapter) Plan(ctx context.Context, dir, prompt string) (string, error) {
	return a.run(ctx, dir, prompt)
}

// Implement implements Agent.
func (a *shellAdapter) Implement(ctx context.Context, dir, prompt string) (string, error) {
	return a.run(ctx, dir, prompt)
}

// run writes the prompt to a temp file and executes the configured shell
// command in dir with KARYA_PROMPT_FILE pointing at it.
func (a *shellAdapter) run(ctx context.Context, dir, prompt string) (string, error) {
	shellCmd := strings.TrimSpace(os.Getenv(shellEnv))
	if shellCmd == "" {
		return "", fmt.Errorf("agentrun: shell adapter needs %s set to the agent command (e.g. %s=\"my-agent --run\")", shellEnv, shellEnv)
	}
	f, err := os.CreateTemp("", "karya-prompt-*.md")
	if err != nil {
		return "", err
	}
	defer os.Remove(f.Name())
	if _, err := f.WriteString(prompt); err != nil {
		f.Close()
		return "", err
	}
	if err := f.Close(); err != nil {
		return "", err
	}
	return a.exec(ctx, dir, []string{"sh", "-c", shellCmd}, []string{promptFileEnv + "=" + f.Name()})
}

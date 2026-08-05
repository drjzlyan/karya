package agent

// SupportsHeadless reports whether the named agent has a documented
// non-interactive (one-shot) mode, without running it. Consumers use it to
// decide whether to attempt headless authoring before falling back to the pane.
func SupportsHeadless(name string) bool {
	_, ok := headlessArgv(name, "")
	return ok
}

// headlessArgv returns an exec-ready argv that runs the named agent
// non-interactively with prompt as input, printing its response to stdout, plus
// ok=false when the agent has no known one-shot mode. It is the lookup table
// behind cliRunner.Headless; callers that get ok=false fall back to a
// conversational request in the agent pane.
//
// Only well-documented non-interactive entry points are listed; an unknown agent
// falls back rather than risk an invented flag producing a bad commit message.
func headlessArgv(name, prompt string) (argv []string, ok bool) {
	switch name {
	case "claude":
		return []string{"claude", "-p", prompt}, true
	case "codex":
		return []string{"codex", "exec", prompt}, true
	case "gemini":
		return []string{"gemini", "-p", prompt}, true
	default:
		return nil, false
	}
}

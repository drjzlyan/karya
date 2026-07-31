package agent

import "strings"

// sendBuffer is the named tmux buffer karya reuses for agent-send payloads so it
// never disturbs the user's paste history for long.
const sendBuffer = "karya-agent-send"

// Send pastes editor context (an optional header followed by the body) into the
// agent pane and focuses it, so the coding agent receives the buffer, selection
// or diagnostic exactly as if the user had pasted it. It deliberately does NOT
// submit — the user reviews the prompt and presses Enter — which keeps a human
// in the loop and works uniformly across every agent CLI.
//
// If the agent pane has disappeared the layout is reset first, mirroring launch.
// Payloads travel through a named tmux buffer (set-buffer/paste-buffer) rather
// than send-keys, which is robust for multi-line text and shell metacharacters.
func (m *Manager) Send(header, body string) error {
	text := joinContext(header, body)
	if strings.TrimSpace(text) == "" {
		m.notify("Nothing to send to the agent")
		return nil
	}

	pane := m.getOpt("@ide_agent_pane")
	if !m.paneExists(pane) {
		if err := m.Reset(); err != nil {
			return err
		}
		pane = m.getOpt("@ide_agent_pane")
	}
	if pane == "" {
		m.notify("No agent pane to send to")
		return nil
	}

	if err := m.tmux.Run("set-buffer", "-b", sendBuffer, "--", text); err != nil {
		return err
	}
	// -d deletes the buffer once pasted so it does not linger in paste history.
	if err := m.tmux.Run("paste-buffer", "-d", "-b", sendBuffer, "-t", pane); err != nil {
		return err
	}
	_ = m.tmux.Run("select-window", "-t", m.session+":dev")
	_ = m.tmux.Run("select-pane", "-t", pane)
	m.notify("Sent to agent: " + m.Current())
	return nil
}

// Focus selects the session's agent pane (resetting the layout if it is gone),
// so the editor can jump to the agent with a single keystroke.
func (m *Manager) Focus() error {
	pane := m.getOpt("@ide_agent_pane")
	if !m.paneExists(pane) {
		if err := m.Reset(); err != nil {
			return err
		}
		pane = m.getOpt("@ide_agent_pane")
	}
	if pane == "" {
		m.notify("No agent pane")
		return nil
	}
	_ = m.tmux.Run("select-window", "-t", m.session+":dev")
	return m.tmux.Run("select-pane", "-t", pane)
}

// joinContext combines a context header and body with a blank separator, keeping
// either side optional.
func joinContext(header, body string) string {
	header, body = strings.TrimRight(header, "\n"), strings.TrimSpace(body)
	switch {
	case header == "":
		return body
	case body == "":
		return header
	default:
		return header + "\n\n" + body
	}
}

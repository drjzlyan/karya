package native

import "encoding/json"

// The Messages API wire types karya needs — a minimal subset of POST
// /v1/messages, hand-written so the package depends only on the standard library.

type apiRequest struct {
	Model     string       `json:"model"`
	MaxTokens int          `json:"max_tokens"`
	System    string       `json:"system,omitempty"`
	Messages  []message    `json:"messages"`
	Tools     []tool       `json:"tools,omitempty"`
	Thinking  *thinkingCfg `json:"thinking,omitempty"`
}

type thinkingCfg struct {
	Type string `json:"type"`
}

type apiResponse struct {
	StopReason string  `json:"stop_reason"`
	Content    []block `json:"content"`
}

type message struct {
	Role    string  `json:"role"`
	Content []block `json:"content"`
}

// block is a content block in either direction. Fields are omitempty so one
// struct serializes correctly as a text block, a tool_use block (echoed back
// unchanged), or a tool_result block.
type block struct {
	Type      string          `json:"type"`
	Text      string          `json:"text,omitempty"`
	ID        string          `json:"id,omitempty"`
	Name      string          `json:"name,omitempty"`
	Input     json.RawMessage `json:"input,omitempty"`
	ToolUseID string          `json:"tool_use_id,omitempty"`
	Content   []block         `json:"content,omitempty"`
	IsError   bool            `json:"is_error,omitempty"`
}

// input returns the string value of key in a tool_use block's input object, or
// "" when absent.
func (b block) input(key string) string {
	if len(b.Input) == 0 {
		return ""
	}
	var m map[string]any
	if json.Unmarshal(b.Input, &m) != nil {
		return ""
	}
	if v, ok := m[key].(string); ok {
		return v
	}
	return ""
}

type tool struct {
	Name        string          `json:"name"`
	Description string          `json:"description"`
	InputSchema json.RawMessage `json:"input_schema"`
}

// systemPrompt steers the native engine. Thinking is disabled (keeps block
// round-tripping simple), so it carries the documented mitigations for that mode
// on Claude Opus 5: allow a lead-in sentence before a tool call, and forbid
// internal XML tags leaking into the reply.
const systemPrompt = `You are karya's built-in coding agent, working inside a single project directory.
Use the read_file, write_file, and run_command tools to make the change the user asks for.
You may say a brief sentence before using a tool. Do not include internal or system XML tags in your response.
When the task is complete, reply with a short summary of what you did.`

// toolSchemas returns the three tools the native loop exposes. read_file is
// read-only (never gated); write_file and run_command are gated per call.
func toolSchemas() []tool {
	return []tool{
		{
			Name:        "read_file",
			Description: "Read a UTF-8 text file, relative to the project directory.",
			InputSchema: json.RawMessage(`{"type":"object","properties":{"path":{"type":"string","description":"File path relative to the project root"}},"required":["path"]}`),
		},
		{
			Name:        "write_file",
			Description: "Create or overwrite a text file, relative to the project directory. Requires the user's approval.",
			InputSchema: json.RawMessage(`{"type":"object","properties":{"path":{"type":"string","description":"File path relative to the project root"},"content":{"type":"string","description":"Full file contents to write"}},"required":["path","content"]}`),
		},
		{
			Name:        "run_command",
			Description: "Run a shell command in the project directory. Requires the user's approval.",
			InputSchema: json.RawMessage(`{"type":"object","properties":{"command":{"type":"string","description":"Shell command to run"}},"required":["command"]}`),
		},
	}
}

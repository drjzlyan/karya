// Package native is karya's built-in agent engine: a Claude-API tool-use loop
// that plugs in behind agent.Runner (ROADMAP Phase 13) alongside the BYO-CLI
// agents. It exists to make one thing first-class that karya cannot do with a
// BYO-CLI: gate every tool call. Each file write or shell command the model
// requests passes through a permission callback the human answers — the
// per-tool-call permission prompt the Phase 11 gates could only approximate for
// external CLIs.
//
// It talks to POST /v1/messages directly over net/http rather than pulling in an
// SDK, keeping karya's single-binary, minimal-dependency, no-CGO promise (see
// AGENTS.md). BYO-CLI stays the default engine; this is opt-in.
package native

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

// DefaultModel is karya's native-agent model. Claude Opus 5 is the current
// most-capable coding model; override with KARYA_AGENT_MODEL.
const DefaultModel = "claude-opus-5"

const (
	apiURL        = "https://api.anthropic.com/v1/messages"
	apiVersion    = "2023-06-01"
	maxTokens     = 16000
	maxIterations = 24 // safety bound on the agentic loop
)

// Permit decides whether a tool call the model requested may run. action is a
// short verb ("write", "run"); detail is the file path or command. Returning
// false surfaces a declined tool_result to the model so it can adjust. Read-only
// tools are never gated, so this is only called for side-effecting actions.
type Permit func(action, detail string) bool

// Client is a configured native-agent engine. Construct with New.
type Client struct {
	APIKey string
	Model  string
	Base   string       // API base URL; overridable in tests
	HTTP   *http.Client // overridable in tests
}

// Available reports whether the native engine can run — i.e. an API key is set.
func Available() bool {
	_, ok := New()
	return ok
}

// New returns a Client using ANTHROPIC_API_KEY and the configured model, or
// ok=false when no key is set (native agent unavailable).
func New() (*Client, bool) {
	key := strings.TrimSpace(os.Getenv("ANTHROPIC_API_KEY"))
	if key == "" {
		return nil, false
	}
	model := strings.TrimSpace(os.Getenv("KARYA_AGENT_MODEL"))
	if model == "" {
		model = DefaultModel
	}
	return &Client{
		APIKey: key,
		Model:  model,
		Base:   apiURL,
		HTTP:   &http.Client{Timeout: 5 * time.Minute},
	}, true
}

// Chat runs a single-prompt conversation with no tools and returns the model's
// text reply. It backs headless authoring (plans, commit messages) — the same
// role a BYO-CLI's one-shot mode plays, but through the native engine.
func (c *Client) Chat(ctx context.Context, dir, prompt string) (string, error) {
	msgs := []message{{Role: "user", Content: []block{{Type: "text", Text: prompt}}}}
	resp, err := c.send(ctx, "", msgs, nil)
	if err != nil {
		return "", err
	}
	return textOf(resp.Content), nil
}

// Run drives the agentic tool-use loop in dir: the model may read files, and —
// with the human's per-call permission — write files and run commands, until it
// finishes with a text response, which is returned. out receives the model's
// text and a one-line note per tool call so the human can follow along.
func (c *Client) Run(ctx context.Context, dir, prompt string, permit Permit, out io.Writer) (string, error) {
	msgs := []message{{Role: "user", Content: []block{{Type: "text", Text: prompt}}}}
	tools := toolSchemas()

	for i := 0; i < maxIterations; i++ {
		resp, err := c.send(ctx, systemPrompt, msgs, tools)
		if err != nil {
			return "", err
		}
		if t := textOf(resp.Content); t != "" {
			fmt.Fprintln(out, t)
		}
		if resp.StopReason != "tool_use" {
			return textOf(resp.Content), nil
		}
		// Append the assistant turn (with its tool_use blocks) before the results.
		msgs = append(msgs, message{Role: "assistant", Content: resp.Content})
		results := c.runTools(ctx, dir, resp.Content, permit, out)
		msgs = append(msgs, message{Role: "user", Content: results})
	}
	return "", fmt.Errorf("native: stopped after %d tool iterations without finishing", maxIterations)
}

// runTools executes every tool_use block in the assistant turn, gating
// side-effecting tools through permit, and returns the tool_result blocks.
func (c *Client) runTools(ctx context.Context, dir string, content []block, permit Permit, out io.Writer) []block {
	var results []block
	for _, b := range content {
		if b.Type != "tool_use" {
			continue
		}
		text, isErr := c.execTool(ctx, dir, b, permit, out)
		results = append(results, block{
			Type:      "tool_result",
			ToolUseID: b.ID,
			Content:   []block{{Type: "text", Text: text}},
			IsError:   isErr,
		})
	}
	return results
}

// commandTimeout bounds a single run_command so a hanging command (a server, a
// prompt-for-input) can't wedge the whole agent loop.
const commandTimeout = 2 * time.Minute

// execTool runs one tool call and returns its result text plus whether it errored.
func (c *Client) execTool(ctx context.Context, dir string, b block, permit Permit, out io.Writer) (string, bool) {
	switch b.Name {
	case "read_file":
		path, err := safePath(dir, b.input("path"))
		if err != nil {
			return err.Error(), true
		}
		fmt.Fprintf(out, "  · read %s\n", b.input("path"))
		data, err := os.ReadFile(path)
		if err != nil {
			return err.Error(), true
		}
		return string(data), false

	case "write_file":
		rel := b.input("path")
		if permit != nil && !permit("write", rel) {
			return "The user declined this file write.", true
		}
		path, err := safePath(dir, rel)
		if err != nil {
			return err.Error(), true
		}
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			return err.Error(), true
		}
		if err := os.WriteFile(path, []byte(b.input("content")), 0o644); err != nil {
			return err.Error(), true
		}
		fmt.Fprintf(out, "  · wrote %s\n", rel)
		return "wrote " + rel, false

	case "run_command":
		cmdStr := b.input("command")
		if permit != nil && !permit("run", cmdStr) {
			return "The user declined this command.", true
		}
		fmt.Fprintf(out, "  · run: %s\n", cmdStr)
		cctx, cancel := context.WithTimeout(ctx, commandTimeout)
		defer cancel()
		cmd := exec.CommandContext(cctx, "sh", "-c", cmdStr)
		cmd.Dir = dir
		combined, err := cmd.CombinedOutput()
		if err != nil {
			return string(combined) + "\n" + err.Error(), true
		}
		return string(combined), false

	default:
		return "unknown tool: " + b.Name, true
	}
}

// safePath resolves rel under dir and rejects paths that escape it — the native
// engine must never read or write outside the task's workspace.
func safePath(dir, rel string) (string, error) {
	if rel == "" {
		return "", fmt.Errorf("empty path")
	}
	abs := filepath.Join(dir, rel)
	clean := filepath.Clean(abs)
	root := filepath.Clean(dir)
	if clean != root && !strings.HasPrefix(clean, root+string(filepath.Separator)) {
		return "", fmt.Errorf("path %q escapes the workspace", rel)
	}
	return clean, nil
}

// send performs one Messages API request and returns the parsed response.
// Thinking is disabled so response blocks (text and tool_use only) round-trip
// through the typed wire structs without needing to preserve thinking blocks.
func (c *Client) send(ctx context.Context, system string, msgs []message, tools []tool) (*apiResponse, error) {
	body, err := json.Marshal(apiRequest{
		Model:     c.Model,
		MaxTokens: maxTokens,
		System:    system,
		Messages:  msgs,
		Tools:     tools,
		Thinking:  &thinkingCfg{Type: "disabled"},
	})
	if err != nil {
		return nil, err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.Base, bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	req.Header.Set("x-api-key", c.APIKey)
	req.Header.Set("anthropic-version", apiVersion)
	req.Header.Set("content-type", "application/json")

	resp, err := c.HTTP.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("claude API %d: %s", resp.StatusCode, strings.TrimSpace(string(data)))
	}
	var out apiResponse
	if err := json.Unmarshal(data, &out); err != nil {
		return nil, fmt.Errorf("native: decode response: %w", err)
	}
	return &out, nil
}

// textOf concatenates the text blocks of a response's content.
func textOf(content []block) string {
	var b strings.Builder
	for _, blk := range content {
		if blk.Type == "text" {
			b.WriteString(blk.Text)
		}
	}
	return strings.TrimSpace(b.String())
}

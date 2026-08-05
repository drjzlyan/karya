package native

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// stubServer scripts the Messages API: each call returns the next canned
// response, and it records the requests it received for assertions.
type stubServer struct {
	responses []apiResponse
	requests  []apiRequest
	calls     int
}

func (s *stubServer) handler(w http.ResponseWriter, r *http.Request) {
	body, _ := io.ReadAll(r.Body)
	var req apiRequest
	_ = json.Unmarshal(body, &req)
	s.requests = append(s.requests, req)
	idx := s.calls
	if idx >= len(s.responses) {
		idx = len(s.responses) - 1
	}
	resp := s.responses[idx]
	s.calls++
	_ = json.NewEncoder(w).Encode(resp)
}

func newClient(t *testing.T, s *stubServer) *Client {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(s.handler))
	t.Cleanup(srv.Close)
	return &Client{APIKey: "test-key", Model: "claude-opus-5", Base: srv.URL, HTTP: srv.Client()}
}

func toolUseBlock(id, name string, input map[string]string) block {
	raw, _ := json.Marshal(input)
	return block{Type: "tool_use", ID: id, Name: name, Input: raw}
}

func TestChatReturnsText(t *testing.T) {
	s := &stubServer{responses: []apiResponse{
		{StopReason: "end_turn", Content: []block{{Type: "text", Text: "feat: add thing"}}},
	}}
	c := newClient(t, s)
	got, err := c.Chat(context.Background(), t.TempDir(), "write a commit message")
	if err != nil {
		t.Fatalf("Chat: %v", err)
	}
	if got != "feat: add thing" {
		t.Errorf("Chat = %q", got)
	}
	// Sanity: the request carried our key path and model.
	if s.requests[0].Model != "claude-opus-5" {
		t.Errorf("model = %q", s.requests[0].Model)
	}
}

func TestRunToolRoundTripReadThenFinish(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "app.go"), []byte("package main\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	s := &stubServer{responses: []apiResponse{
		// 1) model asks to read a file
		{StopReason: "tool_use", Content: []block{toolUseBlock("t1", "read_file", map[string]string{"path": "app.go"})}},
		// 2) model finishes
		{StopReason: "end_turn", Content: []block{{Type: "text", Text: "It is a Go main package."}}},
	}}
	c := newClient(t, s)

	out, err := c.Run(context.Background(), dir, "what is this?", allowAll, io.Discard)
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if out != "It is a Go main package." {
		t.Errorf("Run final = %q", out)
	}
	// The second request must include the tool_result we produced for the read.
	if len(s.requests) < 2 {
		t.Fatalf("expected 2 requests, got %d", len(s.requests))
	}
	last := s.requests[1]
	if !containsToolResult(last.Messages, "t1", "package main") {
		t.Errorf("second request missing read_file tool_result with file contents: %+v", last.Messages)
	}
}

func TestRunGatesWriteAndCommandThroughPermit(t *testing.T) {
	dir := t.TempDir()
	s := &stubServer{responses: []apiResponse{
		{StopReason: "tool_use", Content: []block{
			toolUseBlock("w1", "write_file", map[string]string{"path": "new.txt", "content": "hi"}),
			toolUseBlock("r1", "run_command", map[string]string{"command": "echo hello"}),
		}},
		{StopReason: "end_turn", Content: []block{{Type: "text", Text: "done"}}},
	}}
	c := newClient(t, s)

	// Deny everything: neither the file nor the command may take effect.
	var asked []string
	deny := func(action, detail string) bool { asked = append(asked, action+":"+detail); return false }

	if _, err := c.Run(context.Background(), dir, "make changes", deny, io.Discard); err != nil {
		t.Fatalf("Run: %v", err)
	}
	if _, err := os.Stat(filepath.Join(dir, "new.txt")); !os.IsNotExist(err) {
		t.Error("write_file ran despite the permit denying it")
	}
	if len(asked) != 2 || asked[0] != "write:new.txt" || asked[1] != "run:echo hello" {
		t.Errorf("permit not consulted per tool call: %v", asked)
	}
	// The declined results must be reported back as errors so the model adapts.
	tr := findToolResult(s.requests[1].Messages, "w1")
	if tr == nil || !tr.IsError {
		t.Errorf("declined write should return an is_error tool_result, got %+v", tr)
	}
}

func TestRunPermitAllowsWrite(t *testing.T) {
	dir := t.TempDir()
	s := &stubServer{responses: []apiResponse{
		{StopReason: "tool_use", Content: []block{toolUseBlock("w1", "write_file", map[string]string{"path": "out.txt", "content": "hello"})}},
		{StopReason: "end_turn", Content: []block{{Type: "text", Text: "wrote it"}}},
	}}
	c := newClient(t, s)
	if _, err := c.Run(context.Background(), dir, "write out.txt", allowAll, io.Discard); err != nil {
		t.Fatalf("Run: %v", err)
	}
	data, err := os.ReadFile(filepath.Join(dir, "out.txt"))
	if err != nil || string(data) != "hello" {
		t.Errorf("write_file did not write approved content: %q, %v", data, err)
	}
}

func TestRunCommandRespectsContext(t *testing.T) {
	// A cancelled context must abort a run_command rather than let it execute —
	// the guard against a hanging command wedging the agent loop.
	c := &Client{}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	b := toolUseBlock("r1", "run_command", map[string]string{"command": "echo should-not-run"})
	_, isErr := c.execTool(ctx, t.TempDir(), b, allowAll, io.Discard)
	if !isErr {
		t.Error("run_command with a cancelled context should return an error result")
	}
}

func TestSafePathRejectsEscape(t *testing.T) {
	dir := t.TempDir()
	if _, err := safePath(dir, "../escape"); err == nil {
		t.Error("safePath allowed a path escaping the workspace")
	}
	if _, err := safePath(dir, "sub/ok.txt"); err != nil {
		t.Errorf("safePath rejected a valid nested path: %v", err)
	}
}

func allowAll(string, string) bool { return true }

func containsToolResult(msgs []message, id, wantSubstr string) bool {
	tr := findToolResult(msgs, id)
	if tr == nil {
		return false
	}
	for _, c := range tr.Content {
		if strings.Contains(c.Text, wantSubstr) {
			return true
		}
	}
	return false
}

func findToolResult(msgs []message, id string) *block {
	for _, m := range msgs {
		for i := range m.Content {
			if m.Content[i].Type == "tool_result" && m.Content[i].ToolUseID == id {
				return &m.Content[i]
			}
		}
	}
	return nil
}

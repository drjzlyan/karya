package tutorial

import (
	"bytes"
	"os"
	"strings"
	"testing"
)

func TestLessonsWellFormed(t *testing.T) {
	ls := Lessons()
	if len(ls) == 0 {
		t.Fatal("no lessons")
	}
	for i, l := range ls {
		if l.Num != i+1 {
			t.Errorf("lesson %d has Num %d, want %d", i, l.Num, i+1)
		}
		if strings.TrimSpace(l.Title) == "" || strings.TrimSpace(l.Body) == "" {
			t.Errorf("lesson %d missing title or body", l.Num)
		}
	}
}

// TestVerifiedLessonsPass runs every self-working lesson against a real sandbox
// and asserts each verification passes and prints a ✓ with detail. This is the
// heart of the "self-working" guarantee.
func TestVerifiedLessonsPass(t *testing.T) {
	sb, err := NewSandbox()
	if err != nil {
		t.Fatalf("NewSandbox: %v", err)
	}
	t.Cleanup(func() { _ = sb.Cleanup() })

	verified := 0
	for _, l := range Lessons() {
		var buf bytes.Buffer
		ok := Render(&buf, sb, l)
		out := buf.String()
		if !strings.Contains(out, l.Title) {
			t.Errorf("lesson %d output missing its title", l.Num)
		}
		if !ok {
			t.Errorf("lesson %d failed verification:\n%s", l.Num, out)
		}
		if l.Run != nil {
			verified++
			if !strings.Contains(out, "✓") {
				t.Errorf("lesson %d has a Run but printed no ✓:\n%s", l.Num, out)
			}
		}
	}
	if verified < 3 {
		t.Errorf("expected several self-working lessons, got %d", verified)
	}
}

func TestRenderReportsFailure(t *testing.T) {
	sb, err := NewSandbox()
	if err != nil {
		t.Fatalf("NewSandbox: %v", err)
	}
	t.Cleanup(func() { _ = sb.Cleanup() })

	boom := Lesson{Num: 1, Title: "boom", Body: "b", Run: func(*Sandbox) (string, error) {
		return "", os.ErrPermission
	}}
	var buf bytes.Buffer
	if Render(&buf, sb, boom) {
		t.Error("Render returned true for a failing lesson")
	}
	if !strings.Contains(buf.String(), "✗") {
		t.Errorf("failing lesson printed no ✗:\n%s", buf.String())
	}
}

func TestSandboxLifecycle(t *testing.T) {
	sb, err := NewSandbox()
	if err != nil {
		t.Fatalf("NewSandbox: %v", err)
	}
	if st, err := os.Stat(sb.Dir); err != nil || !st.IsDir() {
		t.Fatalf("sandbox dir not created: %v", err)
	}
	if err := sb.Cleanup(); err != nil {
		t.Fatalf("Cleanup: %v", err)
	}
	if _, err := os.Stat(sb.Dir); !os.IsNotExist(err) {
		t.Errorf("sandbox dir still present after Cleanup")
	}
	// Cleanup is idempotent.
	if err := sb.Cleanup(); err != nil {
		t.Errorf("second Cleanup: %v", err)
	}
}

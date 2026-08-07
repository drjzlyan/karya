package tui

import (
	"bytes"
	"fmt"
	"io"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/drjzlyan/karya/internal/cellbuf"
	"github.com/drjzlyan/karya/internal/term"
)

// syncBuffer is a goroutine-safe buffer for capturing program output in tests.
type syncBuffer struct {
	mu  sync.Mutex
	buf bytes.Buffer
}

func (b *syncBuffer) Write(p []byte) (int, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.buf.Write(p)
}

func (b *syncBuffer) String() string {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.buf.String()
}

// counterModel counts key presses and quits on 'q'.
type counterModel struct {
	keys    int
	custom  int
	lastKey term.Key
}

type incMsg struct{}

func (m counterModel) Init() Cmd { return nil }

func (m counterModel) Update(msg Msg) (Model, Cmd) {
	switch v := msg.(type) {
	case term.KeyEvent:
		if v.Key == term.RuneKey('q') {
			return m, Quit
		}
		m.keys++
		m.lastKey = v.Key
	case incMsg:
		m.custom++
	}
	return m, nil
}

func (m counterModel) View(buf *cellbuf.Buffer) {
	buf.SetString(0, 0, fmt.Sprintf("keys=%d custom=%d", m.keys, m.custom), cellbuf.Style{})
}

func runProgram(t *testing.T, m Model, feed func(pw io.Writer, prog *Program)) (Model, string) {
	t.Helper()
	pr, pw := io.Pipe()
	var out syncBuffer
	prog := NewProgram(m, WithInput(pr), WithOutput(&out), WithoutRawMode(), WithSize(30, 3))

	done := make(chan struct{})
	var final Model
	go func() {
		final, _ = prog.Run()
		close(done)
	}()

	feed(pw, prog)

	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("program did not quit in time")
	}
	_ = pw.Close()
	return final, out.String()
}

// rendered renders a model to a fresh buffer and returns its text (what the user
// would see), independent of the diff renderer's minimal on-wire updates.
func rendered(m Model, w, h int) string {
	buf := cellbuf.New(w, h)
	m.View(buf)
	return buf.String()
}

func TestProgramProcessesKeysAndQuits(t *testing.T) {
	final, out := runProgram(t, counterModel{}, func(pw io.Writer, _ *Program) {
		_, _ = pw.Write([]byte("ab"))
		_, _ = pw.Write([]byte("q"))
	})
	cm := final.(counterModel)
	if cm.keys != 2 {
		t.Fatalf("keys = %d want 2", cm.keys)
	}
	if cm.lastKey != term.RuneKey('b') {
		t.Fatalf("lastKey = %v want b", cm.lastKey)
	}
	if !strings.Contains(rendered(final, 30, 3), "keys=2") {
		t.Fatalf("final model view wrong: %q", rendered(final, 30, 3))
	}
	if out == "" {
		t.Fatalf("expected some rendered output")
	}
}

func TestProgramDeliversArrowKey(t *testing.T) {
	final, _ := runProgram(t, counterModel{}, func(pw io.Writer, _ *Program) {
		_, _ = pw.Write([]byte("\x1b[A")) // Up
		_, _ = pw.Write([]byte("q"))
	})
	cm := final.(counterModel)
	if cm.keys != 1 || cm.lastKey != term.Named(term.SymUp) {
		t.Fatalf("expected one Up key, got keys=%d last=%v", cm.keys, cm.lastKey)
	}
}

func TestProgramSendInjectsMessages(t *testing.T) {
	// All three go through the FIFO message channel, so they are processed in
	// order (mixing Send with pipe input would be nondeterministic).
	final, out := runProgram(t, counterModel{}, func(_ io.Writer, prog *Program) {
		prog.Send(incMsg{})
		prog.Send(incMsg{})
		prog.Send(quitMsg{})
	})
	cm := final.(counterModel)
	if cm.custom != 2 {
		t.Fatalf("custom = %d want 2", cm.custom)
	}
	if !strings.Contains(rendered(final, 30, 3), "custom=2") {
		t.Fatalf("final model view wrong: %q", rendered(final, 30, 3))
	}
	_ = out
}

func TestProgramLoneEscViaFlush(t *testing.T) {
	// A lone ESC with no follow-up should decode to Esc after the timeout.
	final, _ := runProgram(t, counterModel{}, func(pw io.Writer, _ *Program) {
		_, _ = pw.Write([]byte("\x1b")) // lone ESC
		time.Sleep(2 * escTimeout)      // let the flush timer fire
		_, _ = pw.Write([]byte("q"))
	})
	cm := final.(counterModel)
	// Esc is a key press (SymEsc), so keys should be 1.
	if cm.keys != 1 || cm.lastKey != term.Named(term.SymEsc) {
		t.Fatalf("expected lone Esc, got keys=%d last=%v", cm.keys, cm.lastKey)
	}
}

func TestBatchExpandsCommands(t *testing.T) {
	// startBatch fans out two bump commands; the model self-quits once both have
	// been applied, so the assertion is deterministic.
	final, _ := runProgram(t, batchModel{}, func(_ io.Writer, prog *Program) {
		prog.Send(startBatchMsg{})
	})
	bm := final.(batchModel)
	if bm.count != 2 {
		t.Fatalf("batch count = %d want 2", bm.count)
	}
}

type batchModel struct{ count int }
type startBatchMsg struct{}
type bumpMsg struct{}

func (m batchModel) Init() Cmd { return nil }
func (m batchModel) Update(msg Msg) (Model, Cmd) {
	switch msg.(type) {
	case startBatchMsg:
		bump := func() Msg { return bumpMsg{} }
		return m, Batch(bump, bump)
	case bumpMsg:
		m.count++
		if m.count == 2 {
			return m, Quit
		}
	}
	return m, nil
}
func (m batchModel) View(buf *cellbuf.Buffer) {}

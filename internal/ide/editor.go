package ide

import (
	"context"
	"fmt"
	"path/filepath"
	"strings"
	"sync"

	"github.com/drjzlyan/karya/internal/assets"
	"github.com/drjzlyan/karya/internal/cellbuf"
	"github.com/drjzlyan/karya/internal/config"
	"github.com/drjzlyan/karya/internal/layout"
	"github.com/drjzlyan/karya/internal/nvimrpc"
	"github.com/drjzlyan/karya/internal/term"
)

// editorPane hosts an embedded Neovim as the editing engine. karya renders
// Neovim's grid into its own cell buffer and forwards input; Neovim presents no
// UI or keymap surface of its own (DESIGN.md §6.3).
type editorPane struct {
	client *nvimrpc.Client
	cancel context.CancelFunc

	lang string // detected language of the opened file (for auto-provisioning)

	mu     sync.Mutex
	grid   *nvimrpc.Grid
	flush  chan struct{}
	closed chan struct{}
	dead   bool
}

// newEditorPane spawns an embedded Neovim sized cols×rows, attaches a UI, sets
// chrome-off options, and opens file (if non-empty).
func newEditorPane(dir, file string, cols, rows int) (*editorPane, error) {
	if cols < 1 {
		cols = 1
	}
	if rows < 1 {
		rows = 1
	}
	ctx, cancel := context.WithCancel(context.Background())
	argv, env := engineArgs()
	client, err := nvimrpc.Start(ctx, argv, env)
	if err != nil {
		cancel()
		return nil, err
	}
	ep := &editorPane{
		client: client,
		cancel: cancel,
		lang:   langForFile(file),
		grid:   nvimrpc.NewGrid(cols, rows),
		flush:  make(chan struct{}, 1),
		closed: make(chan struct{}),
	}
	client.SetNotificationHandler(ep.onNotify)
	if err := client.UIAttach(cols, rows, map[string]any{"ext_linegrid": true, "rgb": true}); err != nil {
		client.Close()
		cancel()
		return nil, err
	}
	// karya draws all chrome, so hide Neovim's own status/tab lines.
	_ = client.SetOption("laststatus", int64(0))
	_ = client.SetOption("showtabline", int64(0))
	_ = client.SetOption("ruler", false)
	_ = client.SetOption("number", true)
	if file != "" {
		if wd := dir; wd != "" {
			_ = client.Command("cd " + escapeEx(wd))
		}
		_ = client.Command("edit " + escapeEx(file))
	}
	return ep, nil
}

// onNotify applies redraw batches to the grid and signals a flush so the program
// repaints.
func (e *editorPane) onNotify(method string, params []any) {
	if method != "redraw" {
		return
	}
	e.mu.Lock()
	if e.dead {
		e.mu.Unlock()
		return
	}
	flushed := e.grid.HandleRedraw(params)
	e.mu.Unlock()
	if flushed {
		select {
		case e.flush <- struct{}{}:
		default: // coalesce: a repaint is already pending
		}
	}
}

func (e *editorPane) View(buf *cellbuf.Buffer, r cellbuf.Rect, focused bool) {
	e.mu.Lock()
	defer e.mu.Unlock()
	src := e.grid.Buffer()
	sw, sh := src.Size()
	for y := 0; y < r.H && y < sh; y++ {
		for x := 0; x < r.W && x < sw; x++ {
			buf.Set(r.X+x, r.Y+y, src.Cell(x, y))
		}
	}
}

// resize matches Neovim's UI to a new inner size.
func (e *editorPane) resize(cols, rows int) {
	if cols < 1 || rows < 1 || e.dead {
		return
	}
	_ = e.client.TryResize(cols, rows)
}

// write forwards a key (in Neovim notation) to the editor.
func (e *editorPane) write(k term.Key) {
	if e.dead {
		return
	}
	if s := encodeNvimKey(k); s != "" {
		_ = e.client.Input(s)
	}
}

// reattachLSP re-fires the FileType autocmd so the engine config starts a
// language server that has just become available on PATH (after a background
// install). vim.lsp.start de-dups, so this is safe if a server already runs.
func (e *editorPane) reattachLSP() {
	if e.dead {
		return
	}
	_ = e.client.Command("silent! doautocmd FileType")
}

func (e *editorPane) close() {
	e.mu.Lock()
	if e.dead {
		e.mu.Unlock()
		return
	}
	e.dead = true
	e.mu.Unlock()
	close(e.closed)
	e.client.Close()
	e.cancel()
}

// editorFlushMsg wakes the program to repaint after an editor redraw.
type editorFlushMsg struct{ pane *editorPane }

// engineArgs returns the argv and env to launch the embedded editor with karya's
// slim engine config (options + syntax + treesitter + native LSP). It extracts
// the config to an isolated app-name dir; if extraction fails it falls back to
// --clean (editing still works, without LSP/treesitter).
func engineArgs() (argv, env []string) {
	p := config.Resolve()
	if err := assets.ExtractNvimEngine(p.NvimEngineDir()); err != nil {
		return []string{"nvim", "--embed", "--clean", "-n"},
			[]string{"NVIM_APPNAME=" + config.NvimAppName}
	}
	return []string{"nvim", "--embed", "-n"},
		[]string{"NVIM_APPNAME=" + config.NvimEngineAppName}
}

// escapeEx escapes a path for use in an Ex command argument.
func escapeEx(s string) string {
	return strings.ReplaceAll(s, " ", `\ `)
}

// langForFile maps a file path to a catalog language name (matching toolreg's
// Lang() locations), or "" when karya has no per-language tooling for it. Used to
// drive lazy auto-provisioning of LSP servers.
func langForFile(path string) string {
	switch strings.ToLower(filepath.Ext(path)) {
	case ".go":
		return "go"
	case ".py", ".pyi":
		return "python"
	case ".rs":
		return "rust"
	case ".ts", ".tsx", ".js", ".jsx", ".mts", ".cts", ".mjs", ".cjs":
		return "typescript"
	case ".c", ".h", ".cc", ".cpp", ".cxx", ".hpp", ".hh", ".hxx":
		return "cpp"
	}
	return ""
}

// encodeNvimKey converts a Key into Neovim's input notation (for nvim_input).
func encodeNvimKey(k term.Key) string {
	if k.Sym == term.SymRune {
		r := k.Rune
		switch {
		case k.Mod.Has(term.ModCtrl):
			if r == ' ' {
				return "<C-Space>"
			}
			return fmt.Sprintf("<C-%c>", r)
		case k.Mod.Has(term.ModAlt):
			return fmt.Sprintf("<M-%c>", r)
		}
		switch r {
		case '<':
			return "<lt>"
		case ' ':
			return "<Space>"
		}
		return string(r)
	}
	switch k.Sym {
	case term.SymEnter:
		return "<CR>"
	case term.SymEsc:
		return "<Esc>"
	case term.SymBackspace:
		return "<BS>"
	case term.SymTab:
		if k.Mod.Has(term.ModShift) {
			return "<S-Tab>"
		}
		return "<Tab>"
	case term.SymUp:
		return "<Up>"
	case term.SymDown:
		return "<Down>"
	case term.SymLeft:
		return "<Left>"
	case term.SymRight:
		return "<Right>"
	case term.SymHome:
		return "<Home>"
	case term.SymEnd:
		return "<End>"
	case term.SymPageUp:
		return "<PageUp>"
	case term.SymPageDown:
		return "<PageDown>"
	case term.SymInsert:
		return "<Insert>"
	case term.SymDelete:
		return "<Del>"
	}
	return ""
}

var _ layout.PaneContent = (*editorPane)(nil)

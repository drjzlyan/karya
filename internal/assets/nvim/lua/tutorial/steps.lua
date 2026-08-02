-- Step definitions for the in-editor karya tutorial.
--
-- Each step is a table understood by tutorial.engine:
--   title  : short heading shown in the panel
--   body   : list of instruction lines
--   action : the exact thing to do (key sequence or command), shown prominently
--   keys   : an nvim key sequence to detect (e.g. "<leader>ff"). The real keymap
--            still fires — detection is observe-only (vim.on_key) — so the user
--            performs the actual operation, then the tutorial auto-advances.
--   poll   : function(step, ctx) -> bool. Used for tmux/lazygit/agent steps that
--            nvim can't observe as keystrokes; the engine polls it on a timer and
--            advances when it returns true (i.e. the real effect happened).
--   setup  : function(ctx) run once when the step becomes active.
--   confirm: true for informational steps that advance when the user presses <CR>.
--   needs_tmux / needs_agent / needs_lazygit: gate a step; skipped with a note
--            when the requirement is absent, so a bare `nvim` or a minimal machine
--            never gets stuck.
--
-- The whole developer workflow is covered — modes, motions, files/search, LSP,
-- the language-agnostic Code group, the agent bridge, tmux panes/windows, lazygit
-- and ship — closely mirroring docs/tutorial.md but as something you *do*.

local M = {}

-- Minimal, throwaway sample projects. Just enough real code + a root marker so
-- the LSP/Code-group/test steps have something to act on. This is deliberately
-- tiny teaching scaffolding, not karya's real `karya new` templates.
local samples = {
  go = {
    ext = "go",
    files = {
      ["go.mod"] = "module example.com/tutorial\n\ngo 1.21\n",
      ["main.go"] = table.concat({
        "package main",
        "",
        'import "fmt"',
        "",
        "// Add returns the sum of two integers.",
        "func Add(a, b int) int {",
        "\treturn a + b",
        "}",
        "",
        "func main() {",
        '\tfmt.Println(Add(2, 3))',
        "}",
        "",
      }, "\n"),
      ["main_test.go"] = table.concat({
        "package main",
        "",
        'import "testing"',
        "",
        "func TestAdd(t *testing.T) {",
        "\tif Add(2, 3) != 5 {",
        '\t\tt.Fatal("expected 5")',
        "\t}",
        "}",
        "",
      }, "\n"),
    },
    open = "main.go",
  },
  python = {
    ext = "py",
    files = {
      ["pyproject.toml"] = "[project]\nname = \"tutorial\"\nversion = \"0.0.0\"\n",
      ["main.py"] = "def add(a: int, b: int) -> int:\n    return a + b\n\n\nif __name__ == \"__main__\":\n    print(add(2, 3))\n",
      ["test_main.py"] = "from main import add\n\n\ndef test_add():\n    assert add(2, 3) == 5\n",
    },
    open = "main.py",
  },
  typescript = {
    ext = "ts",
    files = {
      ["package.json"] = "{\n  \"name\": \"tutorial\",\n  \"type\": \"module\",\n  \"version\": \"0.0.0\"\n}\n",
      ["index.ts"] = "export function add(a: number, b: number): number {\n  return a + b;\n}\n\nconsole.log(add(2, 3));\n",
    },
    open = "index.ts",
  },
  java = {
    ext = "java",
    files = {
      ["pom.xml"] = "<project>\n  <modelVersion>4.0.0</modelVersion>\n  <groupId>com.example</groupId>\n  <artifactId>tutorial</artifactId>\n  <version>0.0.0</version>\n</project>\n",
      ["src/main/java/com/example/App.java"] = "package com.example;\n\npublic class App {\n    public static int add(int a, int b) {\n        return a + b;\n    }\n\n    public static void main(String[] args) {\n        System.out.println(add(2, 3));\n    }\n}\n",
    },
    open = "src/main/java/com/example/App.java",
  },
  cpp = {
    ext = "cpp",
    files = {
      ["CMakeLists.txt"] = "cmake_minimum_required(VERSION 3.16)\nproject(tutorial CXX)\nset(CMAKE_CXX_STANDARD 20)\nadd_executable(tutorial main.cpp)\n",
      ["main.cpp"] = "#include <iostream>\n\nint add(int a, int b) { return a + b; }\n\nint main() {\n    std::cout << add(2, 3) << std::endl;\n}\n",
    },
    open = "main.cpp",
  },
  rust = {
    ext = "rs",
    files = {
      ["Cargo.toml"] = "[package]\nname = \"tutorial\"\nversion = \"0.0.0\"\nedition = \"2021\"\n",
      ["src/main.rs"] = "fn add(a: i32, b: i32) -> i32 {\n    a + b\n}\n\nfn main() {\n    println!(\"{}\", add(2, 3));\n}\n",
    },
    open = "src/main.rs",
  },
}

-- languages returns the list of languages a sample exists for, in a stable order.
function M.languages()
  return { "go", "python", "typescript", "java", "cpp", "rust" }
end

-- scaffold writes the sample for lang into a fresh temp dir, returns { dir, file }.
function M.scaffold(lang)
  local spec = samples[lang] or samples.go
  local dir = vim.fn.tempname() .. "-karya-tutorial"
  for rel, content in pairs(spec.files) do
    local path = dir .. "/" .. rel
    vim.fn.mkdir(vim.fn.fnamemodify(path, ":h"), "p")
    vim.fn.writefile(vim.split(content, "\n"), path)
  end
  return { dir = dir, file = dir .. "/" .. spec.open }
end

-- ── tmux poll helpers ───────────────────────────────────────────────────────
-- All queries are anchored to $TMUX_PANE (the tutorial's own pane) so they never
-- depend on guessing the session/client — robust inside the karya layout.

local function tmux_display(fmt)
  local pane = vim.env.TMUX_PANE
  if not pane then
    return nil
  end
  local ok, res = pcall(function()
    return vim.system({ "tmux", "display-message", "-p", "-t", pane, fmt }, { text = true }):wait(1000)
  end)
  if not ok or not res or res.code ~= 0 then
    return nil
  end
  return vim.trim(res.stdout or "")
end

-- roundtrip advances only after the watched flag leaves "1" and returns to "1",
-- so a step teaches "go there and come back" and always ends focused in the
-- editor. state is kept per-activation on step._phase.
local function roundtrip(step, fmt)
  local here = tmux_display(fmt)
  if here == nil then
    return false
  end
  if not step._phase then
    step._phase = (here == "1") and "await_leave" or "await_return"
  end
  if step._phase == "await_leave" and here ~= "1" then
    step._phase = "await_return"
  elseif step._phase == "await_return" and here == "1" then
    return true
  end
  return false
end

-- Exposed so the engine can reuse the same anchored query for step gating.
M.tmux_display = tmux_display

-- Steps builds the ordered step list. ctx carries { lang } for wording; the
-- Code-group keys are identical across languages, so only the sample differs.
function M.build(ctx)
  local lang = ctx.lang or "go"
  local L = "<leader>"

  return {
    {
      title = "Welcome to the karya IDE tutorial",
      body = {
        "You'll drive a real IDE session: nvim, tmux, lazygit, the agent bridge.",
        "Do each action yourself — karya detects it, runs the real operation, and",
        "advances automatically. Everything happens in a throwaway sample project.",
        "",
        "Stuck on a step? Jump into this panel with <C-w>w, then press s to skip,",
        "n for next, or q to quit.",
      },
      action = "Press <CR> to begin",
      confirm = true,
    },
    {
      title = "Modes: enter Insert",
      body = {
        "nvim is modal. Normal mode navigates; Insert mode types.",
        "You're editing the sample " .. lang .. " file now.",
      },
      action = "Press  i   (enter Insert mode)",
      keys = "i",
    },
    {
      title = "Modes: back to Normal",
      body = { "Esc always returns you to Normal mode — the home base for commands." },
      action = "Press  <Esc>",
      keys = "<Esc>",
    },
    {
      title = "Motions: jump to end of file",
      body = { "Motions move the cursor precisely. gg is top of file, G is the bottom." },
      action = "Press  G",
      keys = "G",
    },
    {
      title = "Motions: jump back to the top",
      body = { "…and gg returns to the first line." },
      action = "Type  gg",
      keys = "gg",
    },
    {
      title = "Save the file",
      body = { "karya's unified save map writes the buffer (and formats, in real projects)." },
      action = "Press  " .. L .. "S",
      keys = L .. "S",
    },
    {
      title = "Discover every key group",
      body = {
        "which-key shows what a prefix can do. Press the Code prefix and pause —",
        "the language-agnostic <leader>c menu appears.",
      },
      action = "Press  " .. L .. "c   (then look at the menu)",
      keys = L .. "c",
    },
    {
      title = "Dismiss the menu",
      body = { "Esc closes which-key without running anything." },
      action = "Press  <Esc>",
      keys = "<Esc>",
    },
    {
      title = "Find files",
      body = { "The finder opens files fast anywhere in the project." },
      action = "Press  " .. L .. "ff",
      keys = L .. "ff",
    },
    {
      title = "Close the finder",
      body = { "Esc dismisses the picker (or pick a file with <CR>)." },
      action = "Press  <Esc>",
      keys = "<Esc>",
    },
    {
      title = "Search the project",
      body = { "Live-grep searches file contents across the whole project." },
      action = "Press  " .. L .. "s/",
      keys = L .. "s/",
    },
    {
      title = "Close the search",
      body = { "Esc closes it." },
      action = "Press  <Esc>",
      keys = "<Esc>",
    },
    {
      title = "LSP: hover documentation",
      body = {
        "Put your cursor on a symbol (e.g. the add function) and hover.",
        "If a language server is installed for " .. lang .. ", docs pop up.",
      },
      action = "Press  K",
      keys = "K",
    },
    {
      title = "LSP: go to definition",
      body = { "gd jumps to where a symbol is defined; <C-o> jumps back." },
      action = "Press  gd",
      keys = "gd",
    },
    {
      title = "The Code group: build",
      body = {
        "<leader>c is identical in every language. <leader>cc builds the project;",
        "the command runs in the build/test pane. It's fine if the toolchain isn't",
        "installed here — you're learning the key.",
      },
      action = "Press  " .. L .. "cc",
      keys = L .. "cc",
    },
    {
      title = "The Code group: run the tests",
      body = { "<leader>cT runs the current file's tests; <leader>ct runs the nearest test." },
      action = "Press  " .. L .. "cT",
      keys = L .. "cT",
    },
    {
      title = "Debug: toggle a breakpoint",
      body = { "The <leader>d group drives DAP. <leader>db toggles a breakpoint on this line." },
      action = "Press  " .. L .. "db",
      keys = L .. "db",
    },
    {
      title = "Agent bridge: send the buffer",
      body = {
        "Instead of copy-pasting, push editor context into the coding-agent pane.",
        "<leader>ab sends the whole file (pasted unsubmitted, so you review first).",
      },
      action = "Press  " .. L .. "ab",
      keys = L .. "ab",
      needs_agent = true,
    },
    {
      title = "Agent bridge: focus the agent",
      body = { "<leader>aa jumps your cursor into the agent pane." },
      action = "Press  " .. L .. "aa",
      keys = L .. "aa",
      needs_agent = true,
    },
    {
      title = "Git: line blame",
      body = { "gitsigns lives under <leader>g. <leader>gb shows who last changed this line." },
      action = "Press  " .. L .. "gb",
      keys = L .. "gb",
    },
    {
      title = "Git: review all changes",
      body = { "<leader>gd opens diffview across every current change." },
      action = "Press  " .. L .. "gd",
      keys = L .. "gd",
    },
    {
      title = "Close the diff",
      body = { "Back to your file." },
      action = "Press  <Esc>  (or :DiffviewClose)",
      keys = "<Esc>",
    },
    -- ── tmux workflow ─────────────────────────────────────────────────────
    {
      title = "tmux: zoom the editor pane",
      body = {
        "The tmux prefix is Ctrl-a. Zoom makes the current pane fill the window.",
        "(Press Ctrl-a, release, then z.)",
      },
      action = "Press  Ctrl-a z",
      needs_tmux = true,
      poll = function(_, _)
        return tmux_display("#{window_zoomed_flag}") == "1"
      end,
    },
    {
      title = "tmux: unzoom",
      body = { "Press it again to restore the 3-pane layout." },
      action = "Press  Ctrl-a z",
      needs_tmux = true,
      poll = function(_, _)
        return tmux_display("#{window_zoomed_flag}") == "0"
      end,
    },
    {
      title = "tmux: visit another pane and return",
      body = {
        "Panes are vi-navigable from the prefix: h/j/k/l.",
        "Hop to the agent pane on the right, then come back to the editor.",
      },
      action = "Press  Ctrl-a l   then   Ctrl-a h",
      needs_tmux = true,
      poll = function(step, _)
        return roundtrip(step, "#{pane_active}")
      end,
    },
    {
      title = "tmux: cycle the coding agent",
      body = {
        "Ctrl-a N cycles to the next detected agent; Ctrl-a A is the picker.",
        "Your choice is remembered per project.",
      },
      action = "Press  Ctrl-a N",
      needs_tmux = true,
      needs_multi_agent = true,
      poll = function(step, _)
        local cur = tmux_display("#{@ide_current_agent}")
        if cur == nil then
          return false
        end
        if step._start == nil then
          step._start = cur
          return false
        end
        return cur ~= step._start
      end,
    },
    {
      title = "lazygit: open the git window and return",
      body = {
        "Ctrl-a g opens (or reuses) a dedicated git window running lazygit.",
        "Look around, then Ctrl-a p returns to the dev window.",
      },
      action = "Press  Ctrl-a g   then   Ctrl-a p",
      needs_tmux = true,
      needs_lazygit = true,
      poll = function(step, _)
        return roundtrip(step, "#{window_active}")
      end,
    },
    {
      title = "Ship it with the agent",
      body = {
        "The full loop finishes here: <leader>gc (or Ctrl-a G) stages the work,",
        "the agent writes a Conventional-Commit message, and karya commits.",
        "This runs against the throwaway sample, so it's safe to try.",
      },
      action = "Press  " .. L .. "gc   (optional — or n to skip)",
      keys = L .. "gc",
      optional = true,
    },
    {
      title = "You've done the whole loop",
      body = {
        "Scaffold → edit → navigate with LSP → build/test → debug → hand context",
        "to the agent → review → ship — all inside one terminal IDE.",
        "",
        "Next: run `karya lang` to add your languages, `karya docs keymaps` for the",
        "full reference, and `karya` in any project to start for real.",
      },
      action = "Press <CR> to finish",
      confirm = true,
    },
  }
end

return M

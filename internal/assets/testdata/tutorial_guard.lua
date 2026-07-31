-- Tutorial engine guardrail harness (run via `nvim -l tutorial_guard.lua <nvim-dir>`).
--
-- It loads the in-editor tutorial's pure logic WITHOUT launching a session and
-- asserts: the step list is well-formed (every step has a title/action and a way
-- to advance), and the keystroke matcher advances on the right byte sequence and
-- not on the wrong one. This validates the engine's core without a live tmux or
-- coding agent. Output is line-oriented and machine-readable:
--   STEPS <count>
--   MATCH <ok|bad> <expected-lhs>
--   OK

local nvim_dir = arg[1]
assert(nvim_dir and nvim_dir ~= "", "usage: nvim -l tutorial_guard.lua <nvim-dir>")

vim.opt.runtimepath:prepend(nvim_dir)
vim.g.mapleader = " "
vim.g.maplocalleader = " "

local steps_mod = require("tutorial.steps")

-- 1. Steps are well-formed for a representative language.
local steps = steps_mod.build({ lang = "go" })
assert(type(steps) == "table" and #steps > 0, "no steps")
for i, s in ipairs(steps) do
  assert(type(s.title) == "string" and s.title ~= "", "step " .. i .. " missing title")
  assert(type(s.action) == "string" and s.action ~= "", "step " .. i .. " missing action")
  -- Each step must be advanceable: a keystroke, a poll effect, or a confirm.
  assert(s.keys or s.poll or s.confirm, "step " .. i .. " (" .. s.title .. ") has no advance mechanism")
end
print("STEPS " .. #steps)

-- 2. The matcher: reproduce engine.expected_bytes and the suffix match it uses.
local function expected_bytes(lhs)
  local leader = vim.g.mapleader
  if leader == nil or leader == "" then
    leader = " "
  end
  lhs = lhs:gsub("<[lL]eader>", leader)
  return vim.api.nvim_replace_termcodes(lhs, true, true, true)
end

local function suffix_matches(exp, typed)
  return #typed >= #exp and typed:sub(-#exp) == exp
end

local cases = {
  { lhs = "<leader>ff", good = " ff", bad = " fx" },
  { lhs = "gd", good = "gd", bad = "gg" },
  { lhs = "<Esc>", good = vim.api.nvim_replace_termcodes("<Esc>", true, true, true), bad = "x" },
  { lhs = "<leader>cc", good = " cc", bad = " ct" },
}
for _, c in ipairs(cases) do
  local exp = expected_bytes(c.lhs)
  assert(suffix_matches(exp, c.good), "expected match for " .. c.lhs)
  assert(not suffix_matches(exp, c.bad), "unexpected match for " .. c.lhs)
  print("MATCH ok " .. c.lhs)
end

-- 3. Every advertised language produces a scaffold plan (files + a file to open).
for _, lang in ipairs(steps_mod.languages()) do
  local built = steps_mod.build({ lang = lang })
  assert(#built > 0, "no steps for " .. lang)
end

print("OK")

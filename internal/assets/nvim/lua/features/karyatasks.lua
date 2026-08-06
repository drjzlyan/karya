-- Karya task control from the editor (<leader>k — "Karya Tasks").
--
-- These maps drive `karya task`, karya's human-in-the-loop task workflow: each
-- task is a spec contract (.karya/tasks/<id>/SPEC.md) that runs in its own
-- isolated git worktree (branch task/<id>) and advances through human gates
-- (plan / diff / verification). The commands run in the build/test pane so
-- their output — the task board, a task's gate history — shows up there,
-- keeping a human in the loop.
--
-- Like features/agent.lua, this registers its (global) keymaps at import time
-- and returns an empty lazy spec; features are auto-imported by core/lazy.lua.

local terminal = require("features.terminal")
local karya = require("util.karya").bin

-- run sends a `karya task <sub>` invocation to the build/test pane.
local function run(sub)
  terminal.send(nil, karya() .. " task " .. sub, { dir = vim.fn.getcwd() })
end

-- with_id prompts for a task id and runs `karya task <sub> <id>`.
local function with_id(sub)
  vim.ui.input({ prompt = "Task id: " }, function(id)
    if id and id ~= "" then
      run(sub .. " " .. vim.fn.shellescape(id))
    end
  end)
end

local function map(lhs, fn, desc)
  vim.keymap.set("n", lhs, fn, { silent = true, desc = desc })
end

map("<leader>kn", function()
  vim.ui.input({ prompt = "New task slug: " }, function(slug)
    if slug and slug ~= "" then
      run("new " .. vim.fn.shellescape(slug))
    end
  end)
end, "New task")

map("<leader>kl", function()
  run("list")
end, "List tasks")

map("<leader>ks", function()
  with_id("show")
end, "Show task")

map("<leader>kt", function()
  with_id("start")
end, "Start task (worktree)")

map("<leader>ka", function()
  with_id("abandon")
end, "Abandon task")

return {}

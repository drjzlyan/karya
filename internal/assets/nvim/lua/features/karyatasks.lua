-- Karya task control from the editor (<leader>k — "Karya Tasks").
--
-- These maps drive `karya task`, the human-in-the-loop, agents-first workflow:
-- each task runs in its own isolated git worktree (branch karya/<id>) and the
-- human reviews the diff before it merges. Inside a task session (named
-- task-<id>) the CLI defaults to the *current* task, so you can review,
-- checkpoint, merge, and rewind without typing the id. The commands run in the
-- build/test pane so their output — the review diff, the task list, a merge's
-- confirmation prompt — shows up there, keeping a human in the loop.
--
-- Like features/agent.lua, this registers its (global) keymaps at import time
-- and returns an empty lazy spec; features are auto-imported by core/lazy.lua.

local terminal = require("features.terminal")
local karya = require("util.karya").bin

-- run sends a `karya task <sub>` invocation to the build/test pane.
local function run(sub)
  terminal.send(nil, karya() .. " task " .. sub, { dir = vim.fn.getcwd() })
end

local function map(lhs, fn, desc)
  vim.keymap.set("n", lhs, fn, { silent = true, desc = desc })
end

map("<leader>kn", function()
  vim.ui.input({ prompt = "New task: " }, function(desc)
    if desc and desc ~= "" then
      run("new " .. vim.fn.shellescape(desc))
    end
  end)
end, "New task")

map("<leader>kl", function()
  run("list")
end, "List tasks")

map("<leader>kr", function()
  run("review")
end, "Review current task diff")

map("<leader>km", function()
  run("merge")
end, "Merge current task")

map("<leader>kj", function()
  run("reject")
end, "Reject current task")

map("<leader>kc", function()
  run("checkpoint")
end, "Checkpoint current task")

map("<leader>kw", function()
  run("rewind")
end, "Rewind current task")

return {}

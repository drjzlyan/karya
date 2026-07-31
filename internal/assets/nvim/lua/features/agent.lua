-- Editor↔agent bridge.
--
-- These <leader>a maps pipe editor context (the buffer, a visual selection, a
-- diagnostic, a file reference) into `karya agent send`, which pastes it —
-- unsubmitted — into the session's coding-agent pane. The point is that the
-- agent feels native to the editor rather than a separate CLI in a pane.
--
-- Like the language modules, this file registers its (global) keymaps at import
-- time and returns an empty lazy spec.

local karya_bin = require("util.karya").bin

-- send pipes body to `karya agent send`, tagging it with an optional label and
-- the current file:line so the agent knows what it is looking at.
local function send(body, label, line)
  local args = { karya_bin(), "agent", "send" }
  local file = vim.api.nvim_buf_get_name(0)
  if file ~= "" then
    vim.list_extend(args, { "--file", file })
    if line and line ~= "" then
      vim.list_extend(args, { "--line", line })
    end
  end
  if label and label ~= "" then
    vim.list_extend(args, { "--label", label })
  end
  vim.system(args, { stdin = body or "" }, function(res)
    if res.code ~= 0 and res.stderr and res.stderr ~= "" then
      vim.schedule(function()
        vim.notify("agent send: " .. res.stderr, vim.log.levels.WARN)
      end)
    end
  end)
end

local function buffer_text()
  return table.concat(vim.api.nvim_buf_get_lines(0, 0, -1, false), "\n")
end

-- selection returns the current/last visual selection text and its "start-end"
-- line range, or nil when there is none.
local function selection()
  local s, e = vim.fn.getpos("'<"), vim.fn.getpos("'>")
  if s[2] == 0 or e[2] == 0 then
    return nil
  end
  local ok, lines = pcall(vim.fn.getregion, s, e, { type = vim.fn.visualmode() })
  if not ok or not lines or #lines == 0 then
    return nil
  end
  return table.concat(lines, "\n"), string.format("%d-%d", s[2], e[2])
end

local function current_line()
  return vim.api.nvim_get_current_line(), tostring(vim.api.nvim_win_get_cursor(0)[1])
end

-- diagnostics_here returns the joined diagnostic messages on the cursor line and
-- the line number, or nil when the line is clean.
local function diagnostics_here()
  local row = vim.api.nvim_win_get_cursor(0)[1]
  local diags = vim.diagnostic.get(0, { lnum = row - 1 })
  if #diags == 0 then
    return nil, tostring(row)
  end
  local parts = {}
  for _, d in ipairs(diags) do
    table.insert(parts, d.message)
  end
  return table.concat(parts, "\n"), tostring(row)
end

local function map(lhs, mode, fn, desc)
  vim.keymap.set(mode, lhs, fn, { silent = true, desc = desc })
end

map("<leader>aa", "n", function()
  vim.system({ karya_bin(), "agent", "focus" })
end, "Focus agent pane")

map("<leader>ab", "n", function()
  send(buffer_text(), "Here is the current file:")
end, "Send buffer to agent")

map("<leader>as", "v", function()
  local text, range = selection()
  if not text then
    vim.notify("No selection to send", vim.log.levels.WARN)
    return
  end
  send(text, "Here is a selection:", range)
end, "Send selection to agent")

map("<leader>ac", "n", function()
  local text, line = current_line()
  send(text, "Explain this code:", line)
end, "Explain code under cursor")

map("<leader>ac", "v", function()
  local text, range = selection()
  if text then
    send(text, "Explain this code:", range)
  end
end, "Explain selection")

map("<leader>ad", "n", function()
  local text, line = diagnostics_here()
  if not text then
    vim.notify("No diagnostic on this line", vim.log.levels.WARN)
    return
  end
  send(text, "Fix this diagnostic:", line)
end, "Send diagnostic to agent")

map("<leader>af", "n", function()
  send("", "Look at this file:", tostring(vim.api.nvim_win_get_cursor(0)[1]))
end, "Send file reference to agent")

return {}

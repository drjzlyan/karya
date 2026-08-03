-- Shared LLDB debug adapter (codelldb) for C, C++ and Rust. codelldb is installed
-- into karya's isolated tool prefix from the vscode-lldb extension; this wires it
-- into nvim-dap. No Homebrew/system paths — karya never assumes a global install.
local M = {}

local karya = require("util.karya")

-- Locate the codelldb adapter binary inside the extracted extension.
local function codelldb_binary()
  local root = karya.tool("codelldb") or (karya.data_dir() .. "/tools/codelldb")
  local candidates = {
    root .. "/extension/adapter/codelldb",
    root .. "/adapter/codelldb",
  }
  for _, path in ipairs(candidates) do
    if vim.fn.filereadable(path) == 1 then
      return path
    end
  end
  return nil
end

function M.setup()
  local ok, dap = pcall(require, "dap")
  if not ok then
    return
  end
  local cmd = codelldb_binary()
  if not cmd then
    return -- codelldb not installed; leave debugging unconfigured rather than error
  end

  dap.adapters.codelldb = {
    type = "server",
    port = "${port}",
    executable = {
      command = cmd,
      args = { "--port", "${port}" },
    },
  }

  local launch = {
    name = "Launch (codelldb)",
    type = "codelldb",
    request = "launch",
    program = function()
      return vim.fn.input("Path to executable: ", vim.fn.getcwd() .. "/", "file")
    end,
    cwd = "${workspaceFolder}",
    stopOnEntry = false,
    args = {},
  }

  for _, ft in ipairs({ "cpp", "c", "rust" }) do
    dap.configurations[ft] = dap.configurations[ft] or {}
    local already = false
    for _, config in ipairs(dap.configurations[ft]) do
      if config.type == "codelldb" then
        already = true
        break
      end
    end
    if not already then
      table.insert(dap.configurations[ft], launch)
    end
  end
end

return M

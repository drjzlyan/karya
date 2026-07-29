local M = {}

local function find_python()
  if vim.env.VIRTUAL_ENV then
    local bin = vim.env.VIRTUAL_ENV .. "/bin/python"
    if vim.fn.executable(bin) == 1 then
      return bin
    end
  end

  local path = vim.api.nvim_buf_get_name(0)
  local root = vim.fs.root(path, { ".venv", "venv", "pyproject.toml", ".git" }) or vim.fn.getcwd()

  for _, name in ipairs({ ".venv", "venv" }) do
    local bin = root .. "/" .. name .. "/bin/python"
    if vim.fn.executable(bin) == 1 then
      return bin
    end
    local win = root .. "/" .. name .. "/Scripts/python.exe"
    if vim.fn.executable(win) == 1 then
      return win
    end
  end

  return vim.fn.exepath("python3") or vim.fn.exepath("python") or "python"
end

function M.setup()
  local dap = require("dap")
  if dap.adapters.python then
    return
  end

  dap.adapters.python = function(callback, config)
    callback({
      type = "executable",
      command = find_python(),
      args = { "-m", "debugpy.adapter" },
      options = { source_filetype = "python" },
    })
  end

  dap.configurations.python = {
    {
      type = "python",
      request = "launch",
      name = "Launch current file",
      program = "${file}",
      pythonPath = find_python,
    },
    {
      type = "python",
      request = "launch",
      name = "Launch module",
      module = function()
        return vim.fn.input("Module name: ")
      end,
      pythonPath = find_python,
    },
    {
      type = "python",
      request = "attach",
      name = "Attach to process",
      processId = require("dap.utils").pick_process,
      pythonPath = find_python,
    },
    {
      type = "python",
      request = "attach",
      name = "Attach to running debugpy (5678)",
      host = "127.0.0.1",
      port = 5678,
    },
  }
end

return M

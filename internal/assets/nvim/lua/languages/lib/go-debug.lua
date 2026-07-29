local M = {}

function M.setup()
  local dap = require("dap")
  if dap.adapters.go then
    return
  end

  if vim.fn.executable("dlv") == 0 then
    vim.notify(
      "dlv (Delve) not found. Install with: go install github.com/go-delve/delve/cmd/dlv@latest",
      vim.log.levels.WARN
    )
    return
  end

  dap.adapters.go = {
    type = "server",
    port = "${port}",
    executable = {
      command = "dlv",
      args = { "dap", "-l", "127.0.0.1:${port}" },
    },
  }

  dap.configurations.go = {
    {
      type = "go",
      name = "Debug file",
      request = "launch",
      program = "${file}",
    },
    {
      type = "go",
      name = "Debug package",
      request = "launch",
      program = "${fileDirname}",
    },
    {
      type = "go",
      name = "Debug test",
      request = "launch",
      mode = "test",
      program = "${file}",
    },
    {
      type = "go",
      name = "Attach to process",
      request = "attach",
      mode = "local",
      processId = require("dap.utils").pick_process,
    },
  }
end

return M

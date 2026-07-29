local M = {}

-- Register the standard per-language keymap group and user commands so every
-- language offers the same core actions under its own <leader> prefix,
-- mirroring the Java (<leader>j) and Python (<leader>p) groups.
--
-- spec = {
--   lang             = "Go",            -- used for :GoXxx command names
--   prefix           = "<leader>o",
--   format           = function(bufnr)? -- formatter; enables {p}f
--   organize_imports = function(bufnr)? -- enables {p}i
--   run_file         = function()?      -- defaults to util.tasks.run_file
--   debug            = boolean?         -- enables {p}d / {p}D (DAP test debug)
-- }
function M.register(bufnr, spec)
  local prefix = spec.prefix
  local lang = spec.lang

  local map = function(keys, fn, modes, desc)
    vim.keymap.set(modes or "n", keys, fn, { buffer = bufnr, silent = true, desc = desc })
  end

  local command = function(name, fn, desc)
    vim.api.nvim_buf_create_user_command(bufnr, lang .. name, fn, { desc = desc })
  end

  local testing = require("util.testing")
  local tasks = require("util.tasks")

  local run_file = spec.run_file or tasks.run_file

  if spec.format then
    map(prefix .. "f", function()
      spec.format(bufnr)
    end, { "n", "v" }, "Format")
    command("Format", function()
      spec.format(vim.api.nvim_get_current_buf())
    end, "Format " .. lang)
  end

  if spec.organize_imports then
    map(prefix .. "i", function()
      spec.organize_imports(bufnr)
    end, "n", "Organize imports")
    command("OrganizeImports", function()
      spec.organize_imports(vim.api.nvim_get_current_buf())
    end, "Organize " .. lang .. " imports")
  end

  map(prefix .. "r", function()
    vim.lsp.buf.code_action({ context = { only = { "refactor" } } })
  end, { "n", "v" }, "Refactor")
  command("Refactor", function()
    vim.lsp.buf.code_action({ context = { only = { "refactor" } } })
  end, lang .. " refactor actions")

  map(prefix .. "c", tasks.build, "n", "Build / compile project")
  command("Build", tasks.build, "Build " .. lang .. " project")

  map(prefix .. "p", tasks.run_project, "n", "Run project")
  command("RunProject", tasks.run_project, "Run " .. lang .. " project")

  map(prefix .. "R", run_file, "n", "Run current file")
  command("RunFile", run_file, "Run current " .. lang .. " file")

  map(prefix .. "t", testing.run_nearest, "n", "Run nearest test")
  map(prefix .. "T", testing.run_current_class, "n", "Run test file / class")
  command("TestNearest", testing.run_nearest, "Run nearest " .. lang .. " test")
  command("TestFile", testing.run_current_class, "Run " .. lang .. " tests for file / class")
  command("TestProject", testing.run_module, "Run all " .. lang .. " tests")

  if spec.debug then
    map(prefix .. "d", testing.debug_nearest, "n", "Debug nearest test")
    map(prefix .. "D", testing.debug_current_class, "n", "Debug test file / class")
    command("DebugNearest", testing.debug_nearest, "Debug nearest " .. lang .. " test")
    command("DebugClass", testing.debug_current_class, "Debug " .. lang .. " test file / class")
  end

  map(prefix .. "h", vim.lsp.buf.incoming_calls, "n", "Incoming calls")
  map(prefix .. "H", vim.lsp.buf.outgoing_calls, "n", "Outgoing calls")
  command("IncomingCalls", vim.lsp.buf.incoming_calls, lang .. " incoming calls")
  command("OutgoingCalls", vim.lsp.buf.outgoing_calls, lang .. " outgoing calls")
end

return M

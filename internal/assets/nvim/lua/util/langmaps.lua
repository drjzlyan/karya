local M = {}

-- The single, language-agnostic "Code" prefix. Every language registers the
-- identical set of actions under <leader>c so muscle memory transfers across
-- languages: <leader>ct always runs the nearest test, <leader>cf always
-- formats, and so on. Because these maps are buffer-local and only the active
-- buffer's language FileType autocmd fires, <leader>c dispatches to whichever
-- language owns the current buffer — no per-language prefix is needed.
M.prefix = "<leader>c"

-- Register the standard per-language keymap group and user commands so every
-- language offers the same core actions under the shared <leader>c prefix.
--
-- spec = {
--   lang             = "Go",            -- used for :GoXxx command names
--   format           = function(bufnr)? -- formatter; enables cf
--   organize_imports = function(bufnr)? -- enables ci
--   run_file         = function()?      -- defaults to util.tasks.run_file
--   debug            = boolean?         -- enables cd / cD (DAP test debug)
--   extra            = function(map, command)? -- language-specific additions
--                                       -- registered under <leader>c (buffer-local)
-- }
--
-- The map/command helpers passed to spec.extra bind under the same <leader>c
-- prefix and the language's command namespace, so extras stay consistent with
-- the core group (e.g. Java workspace actions land on <leader>cw*).
function M.register(bufnr, spec)
  local prefix = M.prefix
  local lang = spec.lang

  local map = function(keys, fn, modes, desc)
    vim.keymap.set(modes or "n", prefix .. keys, fn, { buffer = bufnr, silent = true, desc = desc })
  end

  local command = function(name, fn, desc)
    vim.api.nvim_buf_create_user_command(bufnr, lang .. name, fn, { desc = desc })
  end

  local testing = require("util.testing")
  local tasks = require("util.tasks")

  local run_file = spec.run_file or tasks.run_file

  if spec.format then
    map("f", function()
      spec.format(bufnr)
    end, { "n", "v" }, "Format")
    command("Format", function()
      spec.format(vim.api.nvim_get_current_buf())
    end, "Format " .. lang)
  end

  if spec.organize_imports then
    map("i", function()
      spec.organize_imports(bufnr)
    end, "n", "Organize imports")
    command("OrganizeImports", function()
      spec.organize_imports(vim.api.nvim_get_current_buf())
    end, "Organize " .. lang .. " imports")
  end

  map("r", function()
    vim.lsp.buf.code_action({ context = { only = { "refactor" } } })
  end, { "n", "v" }, "Refactor")
  command("Refactor", function()
    vim.lsp.buf.code_action({ context = { only = { "refactor" } } })
  end, lang .. " refactor actions")

  map("c", tasks.build, "n", "Build / compile project")
  command("Build", tasks.build, "Build " .. lang .. " project")

  map("p", tasks.run_project, "n", "Run project")
  command("RunProject", tasks.run_project, "Run " .. lang .. " project")

  map("R", run_file, "n", "Run current file")
  command("RunFile", run_file, "Run current " .. lang .. " file")

  map("t", testing.run_nearest, "n", "Run nearest test")
  map("T", testing.run_current_class, "n", "Run test file / class")
  map("l", testing.rerun_last, "n", "Re-run last test")
  command("TestNearest", testing.run_nearest, "Run nearest " .. lang .. " test")
  command("TestFile", testing.run_current_class, "Run " .. lang .. " tests for file / class")
  command("TestProject", testing.run_module, "Run all " .. lang .. " tests")

  if spec.debug then
    map("d", testing.debug_nearest, "n", "Debug nearest test")
    map("D", testing.debug_current_class, "n", "Debug test file / class")
    command("DebugNearest", testing.debug_nearest, "Debug nearest " .. lang .. " test")
    command("DebugClass", testing.debug_current_class, "Debug " .. lang .. " test file / class")
  end

  map("h", vim.lsp.buf.incoming_calls, "n", "Incoming calls")
  map("H", vim.lsp.buf.outgoing_calls, "n", "Outgoing calls")
  command("IncomingCalls", vim.lsp.buf.incoming_calls, lang .. " incoming calls")
  command("OutgoingCalls", vim.lsp.buf.outgoing_calls, lang .. " outgoing calls")

  if spec.extra then
    spec.extra(map, command)
  end
end

return M

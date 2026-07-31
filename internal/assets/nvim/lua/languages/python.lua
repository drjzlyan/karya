local M = {}

-- Keep Python-specific configuration isolated in this file.
-- Generic LSP wiring (diagnostics, keymaps, handlers) lives in lua/features/lsp.lua.

-- ============================================================================
-- Project / virtual environment helpers
-- ============================================================================

local function executable(name)
  return vim.fn.executable(name) == 1
end

local function project_root(bufnr)
  bufnr = bufnr or 0
  local path = vim.api.nvim_buf_get_name(bufnr)
  if path == "" then
    path = vim.fn.getcwd()
  end
  return vim.fs.root(path, {
    ".git",
    "pyproject.toml",
    "setup.py",
    "setup.cfg",
    "requirements.txt",
    "Pipfile",
  }) or vim.fs.dirname(path)
end

-- Search upward from a starting directory for a virtual environment.
-- Supports .venv, venv, and uv-managed virtual environments (which default to
-- .venv). Also respects an active VIRTUAL_ENV if one is set.
local function find_venv_python(start_path)
  local active = vim.env.VIRTUAL_ENV
  if active then
    local sep = package.config:sub(1, 1)
    local candidates = {
      active .. sep .. "bin" .. sep .. "python",
      active .. sep .. "Scripts" .. sep .. "python.exe",
    }
    for _, c in ipairs(candidates) do
      if executable(c) then
        return c
      end
    end
  end

  local dir = start_path
  local root = "/"
  while dir and dir ~= root do
    for _, venv in ipairs({ ".venv", "venv" }) do
      for _, interp in ipairs({ "bin/python", "Scripts/python.exe" }) do
        local candidate = dir .. "/" .. venv .. "/" .. interp
        if executable(candidate) then
          return candidate
        end
      end
    end
    dir = vim.fs.dirname(dir)
  end

  return nil
end

local function python_interpreter(bufnr)
  return find_venv_python(project_root(bufnr))
end

-- Build a command prefix that runs Python inside the project's environment.
-- Prefers `uv run`; falls back to the detected venv interpreter or `python3`.
local function python_cmd(bufnr)
  if executable("uv") then
    return { "uv", "run", "python" }
  end
  local interp = python_interpreter(bufnr) or "python3"
  return { interp }
end

-- ============================================================================
-- Terminal runner
-- ============================================================================

local function send_to_terminal(cmd, cwd, name)
  name = name or "shell"
  cwd = cwd or vim.fn.getcwd()
  local terminal = require("features.terminal")
  if type(cmd) == "table" then
    cmd = table.concat(cmd, " ")
  end
  terminal.send(name, cmd, { dir = cwd })
end

-- ============================================================================
-- Treesitter helpers for Python scopes
-- ============================================================================

local function current_pytest_node()
  local node = vim.treesitter.get_node()
  local parts = {}

  while node do
    local t = node:type()
    if t == "function_definition" or t == "async_function_definition" then
      for child in node:iter_children() do
        if child:type() == "identifier" then
          table.insert(parts, 1, vim.treesitter.get_node_text(child, 0))
          break
        end
      end
    elseif t == "class_definition" then
      for child in node:iter_children() do
        if child:type() == "identifier" then
          table.insert(parts, 1, vim.treesitter.get_node_text(child, 0))
          break
        end
      end
    end
    node = node:parent()
  end

  if #parts == 0 then
    return nil
  end
  return table.concat(parts, "::")
end

-- ============================================================================
-- Run commands
-- ============================================================================

local function run_file()
  local bufnr = vim.api.nvim_get_current_buf()
  local file = vim.api.nvim_buf_get_name(bufnr)
  if file == "" then
    vim.notify("No Python file to run", vim.log.levels.WARN)
    return
  end
  local root = project_root(bufnr)
  local cmd = python_cmd(bufnr)
  table.insert(cmd, file)
  send_to_terminal(cmd, root, "shell")
end

local function relpath(path, base)
  local prefix = base:gsub("([%.%+%-%*%?%[%]%^%$%(%)%%])", "\\%1") .. "[/\\]+"
  return path:gsub("^" .. prefix, "")
end

local function run_module()
  local bufnr = vim.api.nvim_get_current_buf()
  local file = vim.api.nvim_buf_get_name(bufnr)
  if file == "" then
    run_file()
    return
  end
  local root = project_root(bufnr)
  local rel = relpath(file, root)
  if rel == file then
    run_file()
    return
  end
  local module = rel:gsub("%.py$", ""):gsub("[/\\]", ".")
  local cmd = python_cmd(bufnr)
  table.insert(cmd, "-m")
  table.insert(cmd, module)
  send_to_terminal(cmd, root, "shell")
end

local function run_selection()
  local bufnr = vim.api.nvim_get_current_buf()
  local start_pos = vim.fn.getpos("'<")
  local end_pos = vim.fn.getpos("'>")
  if start_pos[2] == 0 or end_pos[2] == 0 then
    vim.notify("No Python code selected", vim.log.levels.WARN)
    return
  end
  local vmode = vim.fn.visualmode()
  local lines = vim.fn.getregion(start_pos, end_pos, { type = vmode })
  if not lines or #lines == 0 then
    vim.notify("No Python code selected", vim.log.levels.WARN)
    return
  end
  local text = table.concat(lines, "\n") .. "\n"
  local tmp = vim.fn.tempname() .. ".py"
  local f, write_err = io.open(tmp, "w")
  if not f then
    vim.notify("Failed to write temporary Python file: " .. tostring(write_err), vim.log.levels.ERROR)
    return
  end
  f:write(text)
  f:close()
  local root = project_root(bufnr)
  local cmd = python_cmd(bufnr)
  table.insert(cmd, tmp)
  send_to_terminal(cmd, root, "shell")
end

-- ============================================================================
-- Test commands
-- ============================================================================

local function pytest_file()
  local bufnr = vim.api.nvim_get_current_buf()
  local file = vim.api.nvim_buf_get_name(bufnr)
  if file == "" then
    vim.notify("No Python file to test", vim.log.levels.WARN)
    return
  end
  local root = project_root(bufnr)
  local cmd = python_cmd(bufnr)
  vim.list_extend(cmd, { "-m", "pytest", file })
  send_to_terminal(cmd, root, "test")
end

local function pytest_function()
  local bufnr = vim.api.nvim_get_current_buf()
  local file = vim.api.nvim_buf_get_name(bufnr)
  if file == "" then
    vim.notify("No Python file to test", vim.log.levels.WARN)
    return
  end
  local node = current_pytest_node()
  local target
  if node then
    target = vim.fn.fnamemodify(file, ":t") .. "::" .. node
  else
    target = vim.fn.fnamemodify(file, ":t")
  end
  local root = project_root(bufnr)
  local cmd = python_cmd(bufnr)
  vim.list_extend(cmd, { "-m", "pytest", target })
  send_to_terminal(cmd, root, "test")
end

local function pytest_project()
  local bufnr = vim.api.nvim_get_current_buf()
  local root = project_root(bufnr)
  local cmd = python_cmd(bufnr)
  vim.list_extend(cmd, { "-m", "pytest" })
  send_to_terminal(cmd, root, "test")
end

-- Enclosing class name only (for class-scoped pytest runs), or nil.
local function current_pytest_class()
  local node = vim.treesitter.get_node()
  while node do
    if node:type() == "class_definition" then
      for child in node:iter_children() do
        if child:type() == "identifier" then
          return vim.treesitter.get_node_text(child, 0)
        end
      end
      return nil
    end
    node = node:parent()
  end
  return nil
end

local function pytest_class()
  local bufnr = vim.api.nvim_get_current_buf()
  local file = vim.api.nvim_buf_get_name(bufnr)
  if file == "" then
    vim.notify("No Python file to test", vim.log.levels.WARN)
    return
  end
  local class_name = current_pytest_class()
  local target
  if class_name then
    target = vim.fn.fnamemodify(file, ":t") .. "::" .. class_name
  else
    target = file
  end
  local root = project_root(bufnr)
  local cmd = python_cmd(bufnr)
  vim.list_extend(cmd, { "-m", "pytest", target })
  send_to_terminal(cmd, root, "test")
end

local function debug_pytest(target_fn)
  local bufnr = vim.api.nvim_get_current_buf()
  local file = vim.api.nvim_buf_get_name(bufnr)
  if file == "" then
    vim.notify("No Python file to debug", vim.log.levels.WARN)
    return
  end
  local ok, dap = pcall(require, "dap")
  if not ok then
    vim.notify("nvim-dap is not available", vim.log.levels.ERROR)
    return
  end
  dap.run({
    type = "python",
    request = "launch",
    name = "Debug pytest",
    module = "pytest",
    args = { target_fn(bufnr, file) },
    pythonPath = function()
      return python_interpreter(bufnr)
    end,
    cwd = project_root(bufnr),
  })
end

-- ============================================================================
-- Ruff organize imports
-- ============================================================================

local function organize_imports()
  local bufnr = vim.api.nvim_get_current_buf()
  local params = {
    textDocument = vim.lsp.util.make_text_document_params(bufnr),
    range = {
      start = { line = 0, character = 0 },
      ["end"] = { line = vim.api.nvim_buf_line_count(bufnr), character = 0 },
    },
    context = {
      diagnostics = {},
      only = { "source.organizeImports" },
    },
  }

  local results = vim.lsp.buf_request_sync(bufnr, "textDocument/codeAction", params, 1500)
  if not results then
    return
  end

  for client_id, response in pairs(results) do
    if response.result then
      local client = vim.lsp.get_client_by_id(client_id)
      for _, action in ipairs(response.result) do
        if action.edit and client then
          vim.lsp.util.apply_workspace_edit(action.edit, client.offset_encoding)
        elseif action.command and client then
          client:exec_cmd(action.command, { bufnr = bufnr })
        end
      end
    end
  end
end

local function format_python()
  vim.lsp.buf.format({
    bufnr = 0,
    async = false,
    filter = function(client)
      return client.name == "ruff"
    end,
  })
end

-- ============================================================================
-- Keymaps and user commands
-- ============================================================================

local function register_commands(bufnr)
  local cmds = {
    { "PythonRunFile", run_file, { desc = "Run current Python file" } },
    { "PythonRunModule", run_module, { desc = "Run current Python module" } },
    { "PythonRunSelection", run_selection, { desc = "Run selected Python code", range = true } },
    { "PythonTestFile", pytest_file, { desc = "pytest current file" } },
    { "PythonTestFunction", pytest_function, { desc = "pytest current function/class" } },
    { "PythonTestClass", pytest_class, { desc = "pytest current class" } },
    { "PythonTestProject", pytest_project, { desc = "pytest whole project" } },
    { "PythonOrganizeImports", organize_imports, { desc = "Organize Python imports" } },
    { "PythonFormat", format_python, { desc = "Format Python with Ruff" } },
  }

  for _, c in ipairs(cmds) do
    vim.api.nvim_buf_create_user_command(bufnr, c[1], c[2], c[3])
  end
end

-- Route Python through the shared <leader>c "Code" interface (via langmaps),
-- adding Python-only actions under the same prefix through the extra hook. The
-- core test/run/build actions resolve through the registered testing and tasks
-- providers below, so <leader>ct, <leader>cR, <leader>cp, etc. work uniformly.
local function register_keymaps(bufnr)
  require("util.langmaps").register(bufnr, {
    lang = "Python",
    format = format_python,
    organize_imports = organize_imports,
    debug = true,
    extra = function(map)
      map("m", run_module, "n", "Run module")
      map("s", run_selection, "v", "Run selection")
      map("v", function()
        local interp = python_interpreter(0)
        if interp then
          vim.notify("Python interpreter: " .. interp, vim.log.levels.INFO)
        else
          vim.notify("No virtual environment detected", vim.log.levels.WARN)
        end
      end, "n", "Show active venv")
    end,
  })
end

-- ============================================================================
-- LSP setup (basedpyright + Ruff)
-- ============================================================================

local function setup_lsp()
  -- Ensure nvim-lspconfig server definitions are registered before
  -- vim.lsp.config is used; requiring the plugin directly does not trigger the
  -- deprecated setup framework.
  local lspconfig = require("lspconfig")
  _ = lspconfig

  local has_basedpyright = vim.fn.executable("basedpyright") == 1
  local has_ruff = vim.fn.executable("ruff") == 1
  if not has_basedpyright and not has_ruff then
    vim.notify("Python LSP servers not found. Install with: brew install basedpyright ruff", vim.log.levels.WARN)
    return
  end

  local capabilities = require("util.lsp").with_blink(vim.lsp.protocol.make_client_capabilities())

  -- basedpyright: type checking, navigation, hover, workspace symbols, etc.
  -- The virtual-env python path is set per-client after attach so each project
  -- uses its own environment.
  if has_basedpyright then
    vim.lsp.config("basedpyright", {
      capabilities = capabilities,
      filetypes = { "python" },
      settings = {
        basedpyright = {
          analysis = {
            autoSearchPaths = true,
            diagnosticMode = "openFilesOnly",
            typeCheckingMode = "standard",
          },
          disableTaggedHints = true,
        },
      },
      on_attach = function(client, bufnr)
        local python = find_venv_python(project_root(bufnr))
        if python then
          local settings = vim.tbl_deep_extend("force", client.config.settings or {}, {
            python = { pythonPath = python },
          })
          client:notify("workspace/didChangeConfiguration", { settings = settings })
        end
      end,
    })
    vim.lsp.enable("basedpyright")
  end

  -- Ruff: linting, formatting, organize imports, code actions.
  -- Ruff is preferred over basedpyright whenever there is overlap.
  if has_ruff then
    vim.lsp.config("ruff", {
      capabilities = capabilities,
      filetypes = { "python" },
      init_options = {
        settings = {
          organizeImports = true,
          fixAll = true,
          lint = { enable = true },
          format = { backend = "internal" },
        },
      },
    })
    vim.lsp.enable("ruff")
  end
end

-- ============================================================================
-- Autocommands: format and organize imports on save (Python only)
-- ============================================================================

local python_augroup = vim.api.nvim_create_augroup("PythonDev", { clear = true })

vim.api.nvim_create_autocmd("FileType", {
  group = python_augroup,
  pattern = "python",
  callback = function(args)
    setup_lsp()
    register_commands(args.buf)
    register_keymaps(args.buf)
  end,
})

vim.api.nvim_create_autocmd("BufWritePre", {
  group = python_augroup,
  pattern = "*.py",
  callback = function()
    organize_imports()
    format_python()
  end,
})

-- ============================================================================
-- Performance: keep searches and navigation out of virtual-env caches
-- ============================================================================

vim.opt.wildignore:append("*/.venv/*,*/__pycache__/*,*/.pytest_cache/*,*/.mypy_cache/*")

-- ============================================================================
-- Generic task provider
-- ============================================================================

local python_provider = {
  detect = function(bufnr)
    local root = project_root(bufnr)
    local markers = {
      "pyproject.toml",
      "setup.py",
      "setup.cfg",
      "requirements.txt",
      "Pipfile",
    }
    for _, marker in ipairs(markers) do
      if vim.fn.filereadable(root .. "/" .. marker) == 1 then
        return true
      end
    end
    return false
  end,

  build = function()
    return nil
  end,

  test = function(bufnr)
    local root = project_root(bufnr)
    local cmd = python_cmd(bufnr)
    vim.list_extend(cmd, { "-m", "pytest" })
    return { cmd = cmd, cwd = root }
  end,

  run_file = function(bufnr)
    local file = vim.api.nvim_buf_get_name(bufnr)
    if file == "" then
      return nil
    end
    local root = project_root(bufnr)
    local cmd = python_cmd(bufnr)
    table.insert(cmd, file)
    return { cmd = cmd, cwd = root }
  end,

  run_project = function(bufnr)
    local file = vim.api.nvim_buf_get_name(bufnr)
    local root = project_root(bufnr)
    if file ~= "" then
      local rel = relpath(file, root)
      if rel ~= file then
        local module = rel:gsub("%.py$", ""):gsub("[/\\]", ".")
        local cmd = python_cmd(bufnr)
        table.insert(cmd, "-m")
        table.insert(cmd, module)
        return { cmd = cmd, cwd = root }
      end
    end
    return { cmd = python_cmd(bufnr), cwd = root }
  end,

  clean = function(bufnr)
    local root = project_root(bufnr)
    return {
      cmd = { "find", root, "-type", "d", "-name", "__pycache__", "-exec", "rm", "-rf", "{}", "+" },
      cwd = root,
    }
  end,
}

local testing = require("util.testing")
if testing then
  testing.register("python", {
    detect = function(bufnr)
      return vim.bo[bufnr or 0].filetype == "python"
    end,
    run_nearest = function()
      pytest_function()
    end,
    run_current_class = function()
      pytest_class()
    end,
    run_package = function()
      pytest_file()
    end,
    run_module = function()
      pytest_project()
    end,
    debug_nearest = function()
      debug_pytest(function(_, file)
        local node = current_pytest_node()
        if node then
          return vim.fn.fnamemodify(file, ":t") .. "::" .. node
        end
        return file
      end)
    end,
    debug_current_class = function()
      debug_pytest(function(_, file)
        local class_name = current_pytest_class()
        if class_name then
          return vim.fn.fnamemodify(file, ":t") .. "::" .. class_name
        end
        return file
      end)
    end,
  })
end

local ok, tasks = pcall(require, "util.tasks")
if ok and tasks.register_provider then
  tasks.register_provider("python", python_provider)
end

return {}

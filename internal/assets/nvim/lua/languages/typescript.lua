-- TypeScript / JavaScript development environment.
-- Activated only when "typescript" is listed in languages.local.
--
-- External tools (install via install-tools.sh or manually):
--   typescript-language-server  — LSP server
--   typescript                   — TypeScript compiler (required by the server)
--   prettier                     — formatter (optional)

local lsp_lib = require("util.lsp")

local M = {}

local function executable(name)
  return vim.fn.executable(name) == 1
end

local function setup_lsp()
  if not executable("typescript-language-server") then
    vim.notify(
      "typescript-language-server not found. Install with: npm install -g typescript-language-server typescript",
      vim.log.levels.WARN
    )
    return
  end

  local capabilities = lsp_lib.with_blink(vim.lsp.protocol.make_client_capabilities())

  lsp_lib.setup_server("ts_ls", {
    capabilities = capabilities,
    filetypes = { "typescript", "typescriptreact", "javascript", "javascriptreact" },
    root_markers = { "tsconfig.json", "package.json", ".git" },
    init_options = {
      hostInfo = "neovim",
      preferences = {
        includeCompletionsForModuleExports = true,
        includeCompletionsWithInsertText = true,
      },
    },
  })
end

-- Format with prettier if available, otherwise fall back to LSP formatting.
local function format_typescript(bufnr)
  bufnr = bufnr or 0
  if executable("prettier") then
    vim.lsp.buf.format({
      bufnr = bufnr,
      async = false,
      filter = function(client)
        return client.name == "ts_ls"
      end,
    })
  else
    vim.lsp.buf.format({ bufnr = bufnr, async = false })
  end
end

-- Organize imports via the ts_ls source action
local function organize_imports(bufnr)
  vim.lsp.buf.code_action({
    context = { only = { "source.organizeImports" } },
    apply = true,
  })
end

local ts_augroup = vim.api.nvim_create_augroup("TypeScriptDev", { clear = true })

vim.api.nvim_create_autocmd("FileType", {
  group = ts_augroup,
  pattern = { "typescript", "typescriptreact", "javascript", "javascriptreact" },
  callback = function(args)
    setup_lsp()
    local bufnr = args.buf
    -- Prettier formatting is exposed via the Code group (<leader>cf) below;
    -- <leader>lf stays the generic "Format with LSP" LspAttach map, avoiding a
    -- duplicate that a buffer-local override would otherwise shadow.
    require("util.langmaps").register(bufnr, {
      lang = "TypeScript",
      format = format_typescript,
      organize_imports = organize_imports,
    })
  end,
})

vim.api.nvim_create_autocmd("BufWritePre", {
  group = ts_augroup,
  pattern = { "*.ts", "*.tsx", "*.js", "*.jsx" },
  callback = function(args)
    format_typescript(args.buf)
  end,
})

-- Keep build artifacts out of search
vim.opt.wildignore:append("*/node_modules/*,*/dist/*,*/build/*")

-- Generic task provider
local ts_provider = {
  detect = function(bufnr)
    local path = vim.api.nvim_buf_get_name(bufnr)
    if path == "" then
      path = vim.fn.getcwd()
    end
    local root = vim.fs.root(path, { "package.json" })
    return root ~= nil
  end,

  build = function()
    return { cmd = { "npm", "run", "build" }, cwd = vim.fn.getcwd() }
  end,

  test = function()
    return { cmd = { "npm", "test" }, cwd = vim.fn.getcwd() }
  end,

  run_file = function(bufnr)
    local file = vim.api.nvim_buf_get_name(bufnr)
    if file == "" then
      return nil
    end
    return { cmd = { "node", file }, cwd = vim.fs.dirname(file) }
  end,

  run_project = function()
    return { cmd = { "npm", "start" }, cwd = vim.fn.getcwd() }
  end,

  clean = function()
    return { cmd = { "rm", "-rf", "node_modules", "dist", "build" }, cwd = vim.fn.getcwd() }
  end,
}

local testing = require("util.testing")
if testing then
  testing.register("typescript", require("languages.lib.ts-testing"))
end

local ok, tasks = pcall(require, "util.tasks")
if ok and tasks.register_provider then
  tasks.register_provider("typescript", ts_provider)
end

return {}

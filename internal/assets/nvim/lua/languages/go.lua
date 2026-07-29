-- Go development environment.
-- Activated only when "go" is listed in languages.local.
--
-- External tools (install via install-tools.sh or manually):
--   gopls      — Go language server (go install golang.org/x/tools/gopls@latest)
--   goimports  — import organizer  (go install golang.org/x/tools/cmd/goimports@latest)
--   delve      — debugger           (go install github.com/go-delve/delve@latest)
-- Go itself is managed by mise.

local lsp_lib = require("util.lsp")

local M = {}

local function executable(name)
  return vim.fn.executable(name) == 1
end

local function project_root(bufnr)
  bufnr = bufnr or 0
  local path = vim.api.nvim_buf_get_name(bufnr)
  if path == "" then
    path = vim.fn.getcwd()
  end
  return vim.fs.root(path, { "go.mod", "go.work", ".git" }) or vim.fs.dirname(path)
end

local function setup_lsp()
  if not executable("gopls") then
    vim.notify("gopls not found. Install with: go install golang.org/x/tools/gopls@latest", vim.log.levels.WARN)
    return
  end

  local capabilities = lsp_lib.with_blink(vim.lsp.protocol.make_client_capabilities())

  vim.lsp.config("gopls", {
    capabilities = capabilities,
    filetypes = { "go", "gomod", "gowork", "gotmpl" },
    root_markers = { "go.mod", "go.work" },
    settings = {
      gopls = {
        analyses = {
          unusedparams = true,
          unusedwrite = true,
          nilness = true,
        },
        staticcheck = true,
        gofumpt = false,
        usePlaceholders = true,
        completeUnimported = true,
      },
    },
  })
  vim.lsp.enable("gopls")
end

-- Format with goimports if available, otherwise gofmt (both via LSP).
local function format_go(bufnr)
  bufnr = bufnr or 0
  vim.lsp.buf.format({
    bufnr = bufnr,
    async = false,
    filter = function(client)
      return client.name == "gopls"
    end,
  })
end

-- Organize imports via gopls code action
local function organize_imports(bufnr)
  bufnr = bufnr or 0
  local params = vim.lsp.util.make_range_params(0, "utf-16")
  params.context = { only = { "source.organizeImports" } }
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

local go_augroup = vim.api.nvim_create_augroup("GoDev", { clear = true })

vim.api.nvim_create_autocmd("FileType", {
  group = go_augroup,
  pattern = "go",
  callback = function(args)
    setup_lsp()
    local bufnr = args.buf
    vim.keymap.set("n", "<leader>lI", function()
      organize_imports(bufnr)
    end, { buffer = bufnr, silent = true, desc = "Organize imports (goimports)" })
    require("util.langmaps").register(bufnr, {
      lang = "Go",
      prefix = "<leader>o",
      format = format_go,
      organize_imports = organize_imports,
      debug = true,
    })
  end,
})

vim.api.nvim_create_autocmd("BufWritePre", {
  group = go_augroup,
  pattern = "*.go",
  callback = function(args)
    organize_imports(args.buf)
    format_go(args.buf)
  end,
})

-- Keep Go build artifacts out of search
vim.opt.wildignore:append("*/bin/*,*/vendor/*")

-- Generic task provider
local go_provider = {
  detect = function(bufnr)
    local root = project_root(bufnr)
    return vim.fn.filereadable(root .. "/go.mod") == 1
  end,

  build = function()
    return { cmd = { "go", "build", "./..." }, cwd = project_root() }
  end,

  test = function()
    return { cmd = { "go", "test", "./..." }, cwd = project_root() }
  end,

  run_file = function(bufnr)
    local file = vim.api.nvim_buf_get_name(bufnr)
    if file == "" then
      return nil
    end
    return { cmd = { "go", "run", file }, cwd = vim.fs.dirname(file) }
  end,

  run_project = function()
    return { cmd = { "go", "run", "." }, cwd = project_root() }
  end,

  clean = function()
    return { cmd = { "go", "clean", "-cache" }, cwd = project_root() }
  end,
}

local testing = require("util.testing")
if testing then
  testing.register("go", require("languages.lib.go-testing"))
end

local ok, tasks = pcall(require, "util.tasks")
if ok and tasks.register_provider then
  tasks.register_provider("go", go_provider)
end

return {}

-- Rust development environment.
-- Activated only when "rust" is listed in languages.local.
--
-- External tools (install via install-tools.sh or manually):
--   rust-analyzer  — language server (rustup component add rust-analyzer)
--   rustfmt        — formatter       (rustup component add rustfmt)
--   cargo          — build tool      (comes with rustup)
-- Rust itself is managed by mise.

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
  return vim.fs.root(path, { "Cargo.toml", ".git" }) or vim.fs.dirname(path)
end

local function setup_lsp()
  if not executable("rust-analyzer") then
    vim.notify("rust-analyzer not found. Install with: rustup component add rust-analyzer", vim.log.levels.WARN)
    return
  end

  local capabilities = lsp_lib.with_blink(vim.lsp.protocol.make_client_capabilities())
  -- Enable auto-import and auto-detect cargo workspace
  capabilities.experimental = {
    hoverActions = true,
    serverStatusNotification = true,
  }

  vim.lsp.config("rust_analyzer", {
    capabilities = capabilities,
    filetypes = { "rust" },
    root_markers = { "Cargo.toml" },
    settings = {
      ["rust-analyzer"] = {
        cargo = {
          allFeatures = true,
          loadOutDirsFromCheck = true,
          buildScripts = { enable = true },
        },
        checkOnSave = {
          command = "clippy",
          extraArgs = { "--no-deps" },
        },
        procMacro = { enable = true },
        hover = { actions = { references = { enable = true } } },
        inlayHints = {
          bindingModeHints = { enable = false },
          chainingHints = { enable = true },
          closingBraceHints = { enable = true, minLines = 25 },
          closureReturnTypeHints = { enable = "never" },
          lifetimeElisionHints = { enable = "never", useParameterNames = false },
          maxLength = 25,
          parameterHints = { enable = true },
          reborrowHints = { enable = "never" },
          renderColons = true,
          typeHints = { enable = true },
        },
      },
    },
  })
  vim.lsp.enable("rust_analyzer")
end

-- Format with rustfmt (via LSP)
local function format_rust(bufnr)
  bufnr = bufnr or 0
  vim.lsp.buf.format({
    bufnr = bufnr,
    async = false,
    filter = function(client)
      return client.name == "rust_analyzer"
    end,
  })
end

local rust_augroup = vim.api.nvim_create_augroup("RustDev", { clear = true })

vim.api.nvim_create_autocmd("FileType", {
  group = rust_augroup,
  pattern = "rust",
  callback = function(args)
    setup_lsp()
    require("util.langmaps").register(args.buf, {
      lang = "Rust",
      prefix = "<leader>r",
      format = format_rust,
    })
  end,
})

vim.api.nvim_create_autocmd("BufWritePre", {
  group = rust_augroup,
  pattern = "*.rs",
  callback = function(args)
    format_rust(args.buf)
  end,
})

-- Keep build artifacts out of search
vim.opt.wildignore:append("*/target/*,*/Cargo.lock")

-- Generic task provider
local rust_provider = {
  detect = function(bufnr)
    local root = project_root(bufnr)
    return vim.fn.filereadable(root .. "/Cargo.toml") == 1
  end,

  build = function()
    return { cmd = { "cargo", "build" }, cwd = project_root() }
  end,

  test = function()
    return { cmd = { "cargo", "test" }, cwd = project_root() }
  end,

  run_file = function()
    return { cmd = { "cargo", "run" }, cwd = project_root() }
  end,

  run_project = function()
    return { cmd = { "cargo", "run" }, cwd = project_root() }
  end,

  clean = function()
    return { cmd = { "cargo", "clean" }, cwd = project_root() }
  end,
}

local testing = require("util.testing")
if testing then
  testing.register("rust", require("languages.lib.rust-testing"))
end

local ok, tasks = pcall(require, "util.tasks")
if ok and tasks.register_provider then
  tasks.register_provider("rust", rust_provider)
end

return {}

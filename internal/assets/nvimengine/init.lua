-- karya embedded Neovim engine config (plugin-free).
--
-- karya owns the UI, windowing, chrome, and all IDE-level keymaps; this file
-- configures ONLY the editing engine: options, syntax/treesitter highlighting,
-- native LSP, and completion. There is no plugin manager and no network access —
-- it relies solely on Neovim's built-in runtime, so the embedded editor starts
-- instantly and offline (DESIGN.md §6.3).

local o = vim.opt
o.number = true
o.signcolumn = "yes"
o.wrap = false
o.expandtab = true
o.shiftwidth = 4
o.tabstop = 4
o.smartindent = true
o.ignorecase = true
o.smartcase = true
o.termguicolors = true
o.scrolloff = 4
o.updatetime = 300
o.undofile = true
o.swapfile = false
o.mouse = "a"

-- karya draws all chrome itself.
o.laststatus = 0
o.showtabline = 0
o.ruler = false

-- Neovim's own leader is used for editor-local text actions only; every
-- IDE-level action is on karya's leader and is intercepted before Neovim.
vim.g.mapleader = " "

-- Fallback regex syntax highlighting; treesitter overrides it where a parser
-- exists (Neovim ships parsers for a handful of languages).
vim.cmd("syntax enable")

vim.api.nvim_create_autocmd("FileType", {
  callback = function(args)
    pcall(vim.treesitter.start, args.buf)
  end,
})

-- Native LSP, plugin-free. Servers are started on FileType (not once at startup)
-- with an executable guard, so a server that karya installs lazily in the
-- background attaches as soon as karya re-fires FileType — no restart needed.
-- vim.lsp.start de-duplicates by name+root, so re-firing is safe.
local servers = {
  gopls = {
    cmd = { "gopls" },
    filetypes = { "go", "gomod", "gowork" },
    root_markers = { "go.mod", ".git" },
  },
  ["lua-language-server"] = {
    cmd = { "lua-language-server" },
    filetypes = { "lua" },
    root_markers = { ".luarc.json", ".luarc.jsonc", ".git" },
  },
  basedpyright = {
    cmd = { "basedpyright-langserver", "--stdio" },
    filetypes = { "python" },
    root_markers = { "pyproject.toml", "setup.py", "requirements.txt", ".git" },
  },
  ts_ls = {
    cmd = { "typescript-language-server", "--stdio" },
    filetypes = { "javascript", "javascriptreact", "typescript", "typescriptreact" },
    root_markers = { "package.json", "tsconfig.json", ".git" },
  },
  rust_analyzer = {
    cmd = { "rust-analyzer" },
    filetypes = { "rust" },
    root_markers = { "Cargo.toml", ".git" },
  },
  clangd = {
    cmd = { "clangd" },
    filetypes = { "c", "cpp", "objc", "objcpp" },
    root_markers = { "compile_commands.json", ".clangd", ".git" },
  },
}

-- Index servers by filetype for quick lookup on FileType.
local by_ft = {}
for name, cfg in pairs(servers) do
  for _, ft in ipairs(cfg.filetypes) do
    by_ft[ft] = by_ft[ft] or {}
    table.insert(by_ft[ft], { name = name, cmd = cfg.cmd, root_markers = cfg.root_markers })
  end
end

vim.api.nvim_create_autocmd("FileType", {
  callback = function(args)
    local ft = vim.bo[args.buf].filetype
    for _, s in ipairs(by_ft[ft] or {}) do
      if vim.fn.executable(s.cmd[1]) == 1 then
        local root = vim.fs.root(args.buf, s.root_markers) or vim.fn.getcwd()
        pcall(vim.lsp.start, { name = s.name, cmd = s.cmd, root_dir = root }, { bufnr = args.buf })
      end
    end
  end,
})

-- On LSP attach: enable built-in autocompletion and editor-local maps. These are
-- Neovim-internal (under Neovim's leader), distinct from karya's IDE keymap.
vim.api.nvim_create_autocmd("LspAttach", {
  callback = function(args)
    local bufnr = args.buf
    local client = vim.lsp.get_client_by_id(args.data.client_id)
    if client and vim.lsp.completion and vim.lsp.completion.enable then
      pcall(vim.lsp.completion.enable, true, client.id, bufnr, { autotrigger = true })
    end
    local map = function(mode, lhs, rhs)
      vim.keymap.set(mode, lhs, rhs, { buffer = bufnr, silent = true })
    end
    map("n", "gd", vim.lsp.buf.definition)
    map("n", "gr", vim.lsp.buf.references)
    map("n", "gi", vim.lsp.buf.implementation)
    map("n", "K", vim.lsp.buf.hover)
    map("n", "<leader>rn", vim.lsp.buf.rename)
    map({ "n", "v" }, "<leader>ca", vim.lsp.buf.code_action)
    map({ "n", "v" }, "<leader>f", function()
      vim.lsp.buf.format({ async = true })
    end)
  end,
})

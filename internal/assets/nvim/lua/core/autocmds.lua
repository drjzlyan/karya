local augroup = vim.api.nvim_create_augroup
local autocmd = vim.api.nvim_create_autocmd

-- Briefly highlight yanked text
local yank_group = augroup("HighlightYank", { clear = true })
autocmd("TextYankPost", {
  group = yank_group,
  callback = function()
    vim.highlight.on_yank({ higroup = "IncSearch", timeout = 120 })
  end,
})

-- Resize splits when the terminal window changes
local resize_group = augroup("ResizeSplits", { clear = true })
autocmd("VimResized", {
  group = resize_group,
  callback = function()
    vim.cmd("wincmd =")
  end,
})

-- Create undo directory if it doesn't exist
local undo_dir = vim.fn.stdpath("state") .. "/undo"
if vim.fn.isdirectory(undo_dir) == 0 then
  vim.fn.mkdir(undo_dir, "p")
end

-- LSP keymaps, bound the moment any server attaches. This lives here (always
-- loaded) rather than in features/lsp.lua, because that plugin is ft-gated to the
-- common languages and only force-loads elsewhere via require("lspconfig"). The
-- per-language modules (Go, Rust, TypeScript, C/C++, Java) enable their servers
-- directly through vim.lsp.enable and never trip that gate, so a keymap autocmd
-- registered inside the plugin would never fire for those buffers — leaving
-- <leader>l*, gd, gr, K, … unbound and the leader combos leaking into insert.
local lsp_keymaps_group = augroup("UserLspKeymaps", { clear = true })
autocmd("LspAttach", {
  group = lsp_keymaps_group,
  callback = function(args)
    local bufnr = args.buf
    local map = function(keys, fn, modes, desc)
      modes = modes or "n"
      vim.keymap.set(modes, keys, fn, {
        buffer = bufnr,
        silent = true,
        desc = desc,
      })
    end

    map("gd", vim.lsp.buf.definition, "n", "Go to definition")
    map("gD", vim.lsp.buf.declaration, "n", "Go to declaration")
    map("gr", vim.lsp.buf.references, "n", "Find references")
    map("gi", vim.lsp.buf.implementation, "n", "Go to implementation")
    map("gt", vim.lsp.buf.type_definition, "n", "Go to type definition")
    map("K", function()
      vim.lsp.buf.hover({ border = "rounded" })
    end, "n", "Hover documentation")
    map("<C-k>", function()
      vim.lsp.buf.signature_help({ border = "rounded" })
    end, "i", "Signature help")

    map("<leader>lr", vim.lsp.buf.rename, "n", "Rename symbol")
    map("<leader>la", vim.lsp.buf.code_action, { "n", "v" }, "Code action")
    map("<leader>lf", function()
      vim.lsp.buf.format({ async = true })
    end, { "n", "v" }, "Format with LSP")
    map("<leader>ls", vim.lsp.buf.workspace_symbol, "n", "Workspace symbols")
    map("<leader>ld", vim.lsp.buf.document_symbol, "n", "Document symbols")
  end,
})

-- Bigfile handling: disable expensive features for large files
local bigfile_group = augroup("Bigfile", { clear = true })
vim.api.nvim_create_autocmd({ "BufReadPre", "BufNewFile" }, {
  group = bigfile_group,
  callback = function(args)
    local ok, stats = pcall(vim.uv.fs_stat, args.file)
    if not ok or not stats then
      return
    end
    if stats.size > 1024 * 1024 then
      vim.b[args.buf].bigfile = true
      vim.opt_local.swapfile = false
      vim.opt_local.undofile = false
      vim.opt_local.bufhidden = "wipe"
    end
  end,
})

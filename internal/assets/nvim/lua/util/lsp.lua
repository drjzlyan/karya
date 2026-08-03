local M = {}

---Merge blink.cmp capabilities into a base capabilities table.
---@param capabilities table vim.lsp.protocol capabilities
---@return table
function M.with_blink(capabilities)
  local ok, blink = pcall(require, "blink.cmp")
  if ok then
    return blink.get_lsp_capabilities(capabilities)
  end
  return capabilities
end

---Ensure nvim-lspconfig is loaded so its shipped `lsp/<server>.lua` definitions
---(cmd, root markers, filetypes) are on the runtimepath. Servers configured
---without an explicit `cmd` (gopls, rust_analyzer, ts_ls, …) rely on those
---defaults; nvim-lspconfig is lazy-loaded, so a language whose setup runs before
---anything requires it would enable a server with no `cmd` and never start it.
function M.ensure_lspconfig()
  pcall(require, "lspconfig")
end

---Configure and enable a language server in one call. lspconfig is force-loaded
---first so the server's default `cmd` is available before `vim.lsp.config` merges
---the caller's options over it.
---@param name string LSP server name
---@param opts table vim.lsp.config options
function M.setup_server(name, opts)
  M.ensure_lspconfig()
  vim.lsp.config(name, opts)
  vim.lsp.enable(name)
end

return M

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

---Configure and enable a language server in one call.
---@param name string LSP server name
---@param opts table vim.lsp.config options
function M.setup_server(name, opts)
  vim.lsp.config(name, opts)
  vim.lsp.enable(name)
end

return M

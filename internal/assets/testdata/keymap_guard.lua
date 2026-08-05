-- Keymap guardrail harness (run via `nvim -l keymap_guard.lua <nvim-dir>`).
--
-- It loads the karya Neovim config's core + language modules WITHOUT bootstrapping
-- plugins (the plugin requires reached on FileType are stubbed), triggers each
-- language's FileType handler on a scratch buffer, and prints the buffer-local
-- <leader>c ("Code") interface it registered. The Go integration test asserts the
-- interface is identical across every language and that the close-buffer key moved
-- off <leader>c — i.e. that the "consistent, unified keymaps" property holds.
--
-- Output is line-oriented and machine-readable:
--   LANG <ft> <comma-separated single-char suffixes under <leader>c>
--   GLOBAL c <present|absent>
--   GLOBAL x <present|absent>
--   OK

local nvim_dir = arg[1]
assert(nvim_dir and nvim_dir ~= "", "usage: nvim -l keymap_guard.lua <nvim-dir>")

vim.opt.runtimepath:prepend(nvim_dir)
vim.g.mapleader = " "
vim.g.maplocalleader = " "

-- Stub the plugin modules language FileType handlers may require, so the test is
-- hermetic (no network, no Lazy bootstrap). Each stub is a table whose fields are
-- no-op callables, which is all the setup paths touch here.
local function stub()
  return setmetatable({}, {
    __index = function()
      return function() end
    end,
  })
end
for _, mod in ipairs({ "lspconfig", "jdtls", "conform", "dap" }) do
  package.preload[mod] = function()
    return stub()
  end
end
-- blink.cmp needs a faithful get_lsp_capabilities: the LSP setup paths pass their
-- capabilities table through it and then index the result.
package.preload["blink.cmp"] = function()
  return {
    get_lsp_capabilities = function(caps)
      return caps or {}
    end,
  }
end

-- Neutralize the LSP setup calls the language FileType handlers make. This test
-- is about keymaps, not LSP, and vim.lsp.config/enable only exist on newer Neovim
-- — stubbing them keeps the guard deterministic across nvim versions (and CI
-- runners where a real LSP server happens to be installed).
vim.lsp.config = function() end
vim.lsp.enable = function() end

-- Global keymaps: core.keymaps binds the top-level leader keys we assert on.
require("core.keymaps")

-- The Karya Tasks feature (<leader>k…) registers its maps at import time. It is
-- hermetic (features.terminal defers its toggleterm require), so loading it here
-- lets the guard assert the task group stays wired.
pcall(require, "features.karyatasks")

local langs = { "go", "rust", "typescript", "cpp", "python", "java" }
local filetype = {
  go = "go",
  rust = "rust",
  typescript = "typescript",
  cpp = "cpp",
  python = "python",
  java = "java",
}

for _, mod in ipairs(langs) do
  pcall(require, "languages." .. mod)
end

-- suffixes returns the sorted single-character suffixes bound under <leader>c
-- (leader is a space, so lhs looks like " cf") for a buffer.
local function suffixes(buf)
  local found = {}
  for _, m in ipairs(vim.api.nvim_buf_get_keymap(buf, "n")) do
    local s = m.lhs:match("^ c(.)$")
    if s then
      found[s] = true
    end
  end
  local list = vim.tbl_keys(found)
  table.sort(list)
  return table.concat(list, ",")
end

for _, mod in ipairs(langs) do
  local buf = vim.api.nvim_create_buf(false, true)
  vim.api.nvim_set_current_buf(buf)
  vim.bo[buf].filetype = filetype[mod]
  pcall(vim.api.nvim_exec_autocmds, "FileType", { buffer = buf, modeline = false })
  print(string.format("LANG %s %s", mod, suffixes(buf)))
end

-- Global-scope assertions: <leader>c must NOT be a global bind anymore (it is the
-- Code group prefix); <leader>x must be the close-buffer key.
local function global_has(lhs)
  for _, m in ipairs(vim.api.nvim_get_keymap("n")) do
    if m.lhs == lhs then
      return true
    end
  end
  return false
end
print("GLOBAL c " .. (global_has(" c") and "present" or "absent"))
print("GLOBAL x " .. (global_has(" x") and "present" or "absent"))
-- Karya Tasks group: a couple of representative <leader>k maps must stay bound.
print("GLOBAL kn " .. (global_has(" kn") and "present" or "absent"))
print("GLOBAL kr " .. (global_has(" kr") and "present" or "absent"))
print("OK")

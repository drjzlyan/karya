-- Global fallbacks for the buffer-local leader groups.
--
-- which-key advertises the LSP (<leader>l) and Code (<leader>c) groups globally,
-- but their real keymaps are buffer-local: <leader>l* is bound on LspAttach
-- (core/autocmds.lua) and <leader>c* per-language on FileType (util/langmaps).
-- In a buffer where the real map is absent (no server attached, or a filetype
-- with no language module), which-key replays the typed keys and the trailing
-- key runs raw -- e.g. <leader>la in a plain buffer drops you into insert via the
-- bare `a`, and <leader>cc changes the line. Registering a harmless,
-- self-explaining fallback for every advertised leaf guarantees the combo always
-- resolves to a real action; the buffer-local maps override these wherever the
-- server / language is active. Fallbacks cover normal AND visual mode because the
-- leaked key is often destructive in visual (`c`/`s` delete the selection).

local M = {}

local modes = { "n", "v" }

local function notify(msg)
  return function()
    vim.notify(msg, vim.log.levels.WARN)
  end
end

-- <leader>l* -- the generic LSP actions bound on LspAttach (core/autocmds.lua).
local lsp_leaves = {
  { "r", "Rename symbol" },
  { "a", "Code action" },
  { "f", "Format with LSP" },
  { "s", "Workspace symbols" },
  { "d", "Document symbols" },
}

-- <leader>c* -- the language-agnostic Code group registered by util.langmaps,
-- plus the extras a few languages add (m/s/v from Python). Keep this in sync with
-- util/langmaps.lua so every advertised Code leaf has a fallback.
local code_leaves = {
  { "f", "Format" },
  { "i", "Organize imports" },
  { "r", "Refactor" },
  { "c", "Build / compile project" },
  { "p", "Run project" },
  { "R", "Run current file" },
  { "t", "Run nearest test" },
  { "T", "Run test file / class" },
  { "l", "Re-run last test" },
  { "d", "Debug nearest test" },
  { "D", "Debug test file / class" },
  { "h", "Incoming calls" },
  { "H", "Outgoing calls" },
  { "m", "Run module" },
  { "s", "Run selection" },
  { "v", "Show active venv" },
}

function M.setup()
  local lsp_msg = "No language server attached to this buffer."
  for _, leaf in ipairs(lsp_leaves) do
    local key, desc = leaf[1], leaf[2]
    vim.keymap.set(modes, "<leader>l" .. key, notify(lsp_msg .. " " .. desc .. " is unavailable here."), {
      silent = true,
      desc = desc,
    })
  end

  local code_msg = "No language actions for this buffer -- enable the language with `karya lang` and reopen the file."
  for _, leaf in ipairs(code_leaves) do
    local key, desc = leaf[1], leaf[2]
    vim.keymap.set(modes, "<leader>c" .. key, notify(code_msg), {
      silent = true,
      desc = desc,
    })
  end
end

return M

-- Language dispatcher: reads the user's language selection and conditionally
-- loads the corresponding language modules.  Common languages (JSON, YAML,
-- Bash, Lua, TOML, Markdown) are always available via lua/features/lsp.lua.
--
-- The selection file is written by `karya lang` into karya's isolated data root
-- (NOT Neovim's data dir): with NVIM_APPNAME=karya/nvim, that root is the parent
-- of stdpath("data"). Reading the wrong path here silently loads no language
-- modules (no per-language LSP, formatting, or the <leader>c Code group).
--   ~/.local/share/karya/languages.local
--
-- Format: key=value, one per line, comments start with #.  Example:
--   python=3.12.7
--   java=17
--   typescript=20.18.0
--   go=1.23.3
--   cpp=system
--   rust=1.81.0

local M = {}

--- Path to the language selection file, in karya's isolated data root.
M.config_path = require("util.karya").data_dir() .. "/languages.local"

--- All languages this configuration knows how to set up.
--- Common languages (json, yaml, bash, lua, toml, markdown) are always on
--- and are NOT listed here because they are handled by features/lsp.lua.
M.available = {
  "python",
  "java",
  "typescript",
  "go",
  "cpp",
  "rust",
}

--- Read the language selection file and return the list of selected languages.
--- If the file does not exist, returns an empty table (all modules stay
--- inactive until the user runs the selector).
---@return string[]
function M.selected()
  local langs = {}
  local f = io.open(M.config_path, "r")
  if not f then
    return langs
  end
  for line in f:lines() do
    line = line:gsub("#.*", ""):gsub("^%s+", ""):gsub("%s+$", "")
    if line ~= "" then
      -- Extract the language name from key=value format
      local lang = line:match("^([%a]+)=")
      if lang then
        table.insert(langs, lang)
      end
    end
  end
  f:close()
  return langs
end

--- Read the version for a selected language.
---@param lang string
---@return string|nil
function M.version(lang)
  local f = io.open(M.config_path, "r")
  if not f then
    return nil
  end
  for line in f:lines() do
    line = line:gsub("#.*", ""):gsub("^%s+", ""):gsub("%s+$", "")
    local l, v = line:match("^([%a]+)=(.+)$")
    if l == lang then
      f:close()
      return v
    end
  end
  f:close()
  return nil
end

--- Check whether a language is selected.
---@param lang string
---@return boolean
function M.is_enabled(lang)
  for _, l in ipairs(M.selected()) do
    if l == lang then
      return true
    end
  end
  return false
end

-- Conditionally require each selected language module.
-- Each module returns a table of lazy.nvim specs (possibly empty) and has
-- side effects (autocmds, LSP setup, keymaps) that run on require.
local specs = {}

for _, lang in ipairs(M.selected()) do
  local ok, mod = pcall(require, "languages." .. lang)
  if ok and type(mod) == "table" then
    vim.list_extend(specs, mod)
  end
end

return specs

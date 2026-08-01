local M = {}

-- bin resolves the absolute path to the running karya binary. Inside a karya
-- session, EDITOR/VISUAL/GIT_EDITOR are set to "<abs path to karya> edit", so we
-- recover the path from there and fall back to $PATH otherwise.
function M.bin()
  for _, var in ipairs({ vim.env.EDITOR, vim.env.VISUAL, vim.env.GIT_EDITOR }) do
    if var then
      local bin = var:match("^(.-)%s+edit$")
      if bin and bin ~= "" then
        return bin
      end
    end
  end
  return "karya"
end

-- data_dir is karya's isolated data root. Neovim runs with NVIM_APPNAME=karya/nvim,
-- so stdpath("data") is <karya-data>/nvim; the karya root is its parent. Everything
-- karya manages (mise installs, tool prefix, languages.local) lives under here, so
-- the editor must resolve tools relative to this — never the user's global dirs.
function M.data_dir()
  return vim.fn.fnamemodify(vim.fn.stdpath("data"), ":h")
end

-- cache_dir is karya's isolated cache root (parent of stdpath("cache")), used for
-- scratch like jdtls workspaces so nothing lands in the user's ~/.cache.
function M.cache_dir()
  return vim.fn.fnamemodify(vim.fn.stdpath("cache"), ":h")
end

local manifest_cache = nil

-- manifest returns karya's resolved-tool map (tool id -> { path, source }),
-- written by the karya binary to <data>/karya-tools.json. Cached per session.
-- Returns an empty table when the manifest is absent or malformed, so callers
-- degrade to PATH resolution.
function M.manifest()
  if manifest_cache ~= nil then
    return manifest_cache
  end
  manifest_cache = {}
  local path = vim.fn.stdpath("data") .. "/karya-tools.json"
  local ok, f = pcall(io.open, path, "r")
  if ok and f then
    local content = f:read("*a")
    f:close()
    local decoded_ok, decoded = pcall(vim.json.decode, content)
    if decoded_ok and type(decoded) == "table" and type(decoded.tools) == "table" then
      manifest_cache = decoded.tools
    end
  end
  return manifest_cache
end

-- tool returns the absolute path of a managed tool by id from the manifest, or
-- nil when it is not resolved (the caller should fall back to PATH).
function M.tool(id)
  local entry = M.manifest()[id]
  if type(entry) == "table" and type(entry.path) == "string" and entry.path ~= "" then
    return entry.path
  end
  return nil
end

return M

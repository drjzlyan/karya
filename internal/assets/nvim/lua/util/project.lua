local M = {}

-- Root markers used to identify project roots.
M.root_markers = {
  ".git",
  "pyproject.toml",
  "setup.py",
  "setup.cfg",
  "requirements.txt",
  "Pipfile",
  "pom.xml",
  "build.gradle",
  "settings.gradle",
  "settings.gradle.kts",
  "go.mod",
  "Cargo.toml",
  "package.json",
}

-- Project-type markers. A directory containing one of these files is treated
-- as a project of the associated type.
M.type_markers = {
  git = { ".git" },
  python = { "pyproject.toml", "setup.py", "setup.cfg", "requirements.txt", "Pipfile" },
  java = { "pom.xml", "build.gradle", "settings.gradle", "settings.gradle.kts" },
  maven = { "pom.xml" },
  gradle = { "build.gradle", "settings.gradle", "settings.gradle.kts" },
  go = { "go.mod" },
  rust = { "Cargo.toml" },
  node = { "package.json" },
}

-- Return the project root for a buffer or path.
function M.root(bufnr_or_path)
  local path
  if type(bufnr_or_path) == "number" then
    path = vim.api.nvim_buf_get_name(bufnr_or_path)
    if path == "" then
      path = vim.fn.getcwd()
    end
  else
    path = tostring(bufnr_or_path)
  end
  return vim.fs.root(path, M.root_markers) or vim.fs.dirname(path)
end

-- Check whether a marker exists in the project root.
function M.has_marker(bufnr_or_path, marker)
  local r = M.root(bufnr_or_path)
  if not r or r == "" then
    return false
  end
  return vim.fn.filereadable(r .. "/" .. marker) == 1 or vim.fn.isdirectory(r .. "/" .. marker) == 1
end

-- Return a list of project types detected for the buffer or path.
function M.types(bufnr_or_path)
  local detected = {}
  for t, markers in pairs(M.type_markers) do
    for _, marker in ipairs(markers) do
      if M.has_marker(bufnr_or_path, marker) then
        table.insert(detected, t)
        break
      end
    end
  end
  return detected
end

-- Return true if the buffer or path is inside a project of the given type.
function M.is_type(bufnr_or_path, t)
  local markers = M.type_markers[t]
  if not markers then
    return false
  end
  for _, marker in ipairs(markers) do
    if M.has_marker(bufnr_or_path, marker) then
      return true
    end
  end
  return false
end

return M

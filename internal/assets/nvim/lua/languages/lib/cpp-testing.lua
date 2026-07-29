local M = {}

local project = require("util.project")

local function send(cmd, cwd)
  local terminal = require("features.terminal")
  if type(cmd) == "table" then
    cmd = table.concat(cmd, " ")
  end
  terminal.send("test", cmd, { dir = cwd })
end

function M.detect(bufnr)
  bufnr = bufnr or 0
  local root = project.root(bufnr)
  return vim.fn.filereadable(root .. "/CMakeLists.txt") == 1
end

-- ctest cannot map buffer positions to tests, so the smallest useful scope
-- is the whole suite. Requires the build directory to be configured first
-- (cmake -S . -B build).
function M.run_module(bufnr)
  local root = project.root(bufnr)
  send({ "ctest", "--output-on-failure" }, root .. "/build")
end

function M.run_package(bufnr)
  M.run_module(bufnr)
end

function M.run_current_class(bufnr)
  M.run_module(bufnr)
end

return M

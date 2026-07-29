local M = {}

local project = require("util.project")

local function send(cmd, cwd)
  local terminal = require("features.terminal")
  if type(cmd) == "table" then
    cmd = table.concat(cmd, " ")
  end
  terminal.send("test", cmd, { dir = cwd })
end

local function pkg_dir(bufnr)
  local file = vim.api.nvim_buf_get_name(bufnr)
  return vim.fs.dirname(file)
end

-- Package pattern relative to the module root, e.g. "./cmd/calc" or ".".
local function rel_pkg(bufnr)
  local root = project.root(bufnr)
  local dir = pkg_dir(bufnr)
  if dir == root then
    return "."
  end
  return "./" .. dir:sub(#root + 2)
end

-- Name of the enclosing Go test/benchmark/example/fuzz function, or nil.
local function current_test_func()
  local node = vim.treesitter.get_node()
  while node do
    local t = node:type()
    if t == "function_declaration" or t == "method_declaration" then
      local name_node = node:field("name")[1]
      if name_node then
        local name = vim.treesitter.get_node_text(name_node, 0)
        if name:match("^Test") or name:match("^Benchmark") or name:match("^Example") or name:match("^Fuzz") then
          return name
        end
      end
      return nil
    end
    node = node:parent()
  end
  return nil
end

local function run_flag(name)
  if name:match("^Benchmark") then
    return "-bench"
  end
  if name:match("^Fuzz") then
    return "-fuzz"
  end
  return "-run"
end

function M.detect(bufnr)
  return project.is_type(bufnr or 0, "go")
end

function M.run_nearest(bufnr)
  local name = current_test_func()
  if not name then
    vim.notify("No Go test function under cursor", vim.log.levels.WARN)
    return
  end
  send({ "go", "test", run_flag(name), "'^" .. name .. "$'", rel_pkg(bufnr) }, project.root(bufnr))
end

function M.run_current_class(bufnr)
  M.run_package(bufnr)
end

function M.run_package(bufnr)
  send({ "go", "test", rel_pkg(bufnr) }, project.root(bufnr))
end

function M.run_module(bufnr)
  send({ "go", "test", "./..." }, project.root(bufnr))
end

function M.debug_nearest(bufnr)
  local name = current_test_func()
  if not name then
    vim.notify("No Go test function under cursor", vim.log.levels.WARN)
    return
  end
  local ok, dap = pcall(require, "dap")
  if not ok then
    vim.notify("nvim-dap is not available", vim.log.levels.ERROR)
    return
  end
  dap.run({
    type = "go",
    name = "Debug nearest test",
    request = "launch",
    mode = "test",
    program = pkg_dir(bufnr),
    args = { "-test.run", "^" .. name .. "$" },
  })
end

function M.debug_current_class(bufnr)
  local ok, dap = pcall(require, "dap")
  if not ok then
    vim.notify("nvim-dap is not available", vim.log.levels.ERROR)
    return
  end
  dap.run({
    type = "go",
    name = "Debug package tests",
    request = "launch",
    mode = "test",
    program = pkg_dir(bufnr),
  })
end

return M

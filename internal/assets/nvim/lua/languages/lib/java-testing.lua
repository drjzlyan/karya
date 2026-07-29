local M = {}

local project = require("util.project")

local function build_tool(root)
  if vim.fn.filereadable(root .. "/pom.xml") == 1 then
    return "maven"
  end
  if
    vim.fn.filereadable(root .. "/build.gradle") == 1
    or vim.fn.filereadable(root .. "/settings.gradle") == 1
    or vim.fn.filereadable(root .. "/settings.gradle.kts") == 1
  then
    return "gradle"
  end
  return nil
end

local function current_package(bufnr)
  local lines = vim.api.nvim_buf_get_lines(bufnr, 0, 20, false)
  for _, line in ipairs(lines) do
    local pkg = line:match("^%s*package%s+([%w%.]+)%s*;")
    if pkg then
      return pkg
    end
  end
  return nil
end

local function send(cmd, cwd, name)
  local terminal = require("features.terminal")
  if type(cmd) == "table" then
    cmd = table.concat(cmd, " ")
  end
  terminal.send(name or "test", cmd, { dir = cwd })
end

function M.detect(bufnr)
  return project.is_type(bufnr or 0, "java")
end

function M.run_nearest(_bufnr)
  local ok, jdtls = pcall(require, "jdtls")
  if not ok then
    vim.notify("nvim-jdtls is not available", vim.log.levels.ERROR)
    return
  end
  jdtls.test_nearest_method({ config_overrides = { noDebug = true } })
end

function M.run_current_class(_bufnr)
  local ok, jdtls = pcall(require, "jdtls")
  if not ok then
    vim.notify("nvim-jdtls is not available", vim.log.levels.ERROR)
    return
  end
  jdtls.test_class({ config_overrides = { noDebug = true } })
end

function M.run_package(bufnr)
  local root = project.root(bufnr)
  local pkg = current_package(bufnr)
  if not pkg then
    vim.notify("Could not determine package name", vim.log.levels.WARN)
    return
  end
  local tool = build_tool(root)
  if tool == "maven" then
    send({ "mvn", "test", "-Dtest=" .. pkg .. ".**" }, root, "test")
  elseif tool == "gradle" then
    send({ "gradle", "test", "--tests", pkg .. ".**" }, root, "test")
  else
    vim.notify("No build tool found for package test run", vim.log.levels.WARN)
  end
end

function M.run_module(bufnr)
  local root = project.root(bufnr)
  local tool = build_tool(root)
  if tool == "maven" then
    send({ "mvn", "test" }, root, "test")
  elseif tool == "gradle" then
    send({ "gradle", "test" }, root, "test")
  else
    vim.notify("No build tool found for module test run", vim.log.levels.WARN)
  end
end

function M.debug_nearest(_bufnr)
  local ok, jdtls = pcall(require, "jdtls")
  if not ok then
    vim.notify("nvim-jdtls is not available", vim.log.levels.ERROR)
    return
  end
  jdtls.test_nearest_method()
end

function M.debug_current_class(_bufnr)
  local ok, jdtls = pcall(require, "jdtls")
  if not ok then
    vim.notify("nvim-jdtls is not available", vim.log.levels.ERROR)
    return
  end
  jdtls.test_class()
end

return M

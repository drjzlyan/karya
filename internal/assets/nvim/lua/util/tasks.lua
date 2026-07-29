local M = {}

local project = require("util.project")
local terminal = require("features.terminal")

-- Registered language providers keyed by name.
-- Providers supply detect/build/test/clean/run_file/run_project functions.
local providers = {}

function M.register_provider(name, provider)
  providers[name] = provider
end

local function filereadable(path)
  return vim.fn.filereadable(path) == 1
end

local function has_just(root)
  return filereadable(root .. "/justfile") or filereadable(root .. "/Justfile")
end

local function has_make(root)
  return filereadable(root .. "/Makefile") or filereadable(root .. "/makefile")
end

local function run(cmd, cwd)
  cwd = cwd or project.root()
  if type(cmd) == "table" then
    cmd = table.concat(cmd, " ")
  end
  terminal.send(nil, cmd, { dir = cwd })
end

local function detect_provider()
  local bufnr = vim.api.nvim_get_current_buf()
  for _, provider in pairs(providers) do
    if provider.detect and provider.detect(bufnr) then
      return provider, bufnr
    end
  end
  return nil, bufnr
end

local function run_provider(result)
  if not result then
    return false
  end
  run(result.cmd, result.cwd)
  return true
end

local function task_or_notify(kind, just_cmd, make_cmd)
  local root = project.root()
  if has_just(root) then
    run(just_cmd, root)
  elseif has_make(root) then
    run(make_cmd, root)
  else
    vim.notify("No task provider found for " .. kind, vim.log.levels.WARN)
  end
end

function M.build()
  local provider, bufnr = detect_provider()
  if provider and provider.build then
    if run_provider(provider.build(bufnr)) then
      return
    end
  end
  task_or_notify("build", "just build", "make build")
end

function M.test()
  local provider, bufnr = detect_provider()
  if provider and provider.test then
    if run_provider(provider.test(bufnr)) then
      return
    end
  end
  task_or_notify("test", "just test", "make test")
end

function M.clean()
  local provider, bufnr = detect_provider()
  if provider and provider.clean then
    if run_provider(provider.clean(bufnr)) then
      return
    end
  end
  task_or_notify("clean", "just clean", "make clean")
end

function M.run_file()
  local provider, bufnr = detect_provider()
  if provider and provider.run_file then
    local result = provider.run_file(bufnr)
    if result and run_provider(result) then
      return
    end
  end
  vim.notify("Run file is not supported for this file type; use language keymaps", vim.log.levels.INFO)
end

function M.run_project()
  local provider, bufnr = detect_provider()
  if provider and provider.run_project then
    if run_provider(provider.run_project(bufnr)) then
      return
    end
  end
  task_or_notify("run", "just run", "make run")
end

function M.setup()
  vim.api.nvim_create_user_command("TaskBuild", M.build, { desc = "Run project build task" })
  vim.api.nvim_create_user_command("TaskTest", M.test, { desc = "Run project test task" })
  vim.api.nvim_create_user_command("TaskClean", M.clean, { desc = "Run project clean task" })
  vim.api.nvim_create_user_command("TaskRun", M.run_project, { desc = "Run project run task" })
end

return M

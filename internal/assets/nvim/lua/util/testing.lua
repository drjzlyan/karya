local M = {}

local providers = {}
local last_run = nil

function M.register(name, provider)
  providers[name] = provider
end

function M.providers()
  return vim.tbl_keys(providers)
end

function M.detect(bufnr)
  bufnr = bufnr or 0
  for _, provider in pairs(providers) do
    if provider.detect and provider.detect(bufnr) then
      return provider
    end
  end
  return nil
end

local function run(kind)
  local bufnr = vim.api.nvim_get_current_buf()
  local provider = M.detect(bufnr)
  if not provider then
    vim.notify("No test provider detected for this buffer", vim.log.levels.WARN)
    return
  end

  local fn = provider[kind]
  if not fn then
    vim.notify("Test provider does not support " .. kind, vim.log.levels.WARN)
    return
  end

  last_run = { provider = provider, kind = kind, bufnr = bufnr }

  local ok, err = pcall(fn, bufnr)
  if not ok then
    vim.notify("Test provider failed: " .. tostring(err), vim.log.levels.ERROR)
  end
end

function M.run_nearest()
  run("run_nearest")
end

function M.run_current_class()
  run("run_current_class")
end

function M.run_package()
  run("run_package")
end

function M.run_module()
  run("run_module")
end

function M.debug_nearest()
  run("debug_nearest")
end

function M.debug_current_class()
  run("debug_current_class")
end

function M.rerun_last()
  if not last_run then
    vim.notify("No previous test run to repeat", vim.log.levels.WARN)
    return
  end
  local ok, err = pcall(last_run.provider[last_run.kind], last_run.bufnr)
  if not ok then
    vim.notify("Re-run failed: " .. tostring(err), vim.log.levels.ERROR)
  end
end

return M

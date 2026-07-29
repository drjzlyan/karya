local health = require("util.health")
local M = {}

---Standard Neovim :checkhealth integration for the dev environment.
function M.check()
  local ok, results = pcall(health.check_all)
  if not ok then
    vim.health.error("Health check failed: " .. tostring(results))
    return
  end

  vim.health.start("Development environment")
  for _, item in ipairs(results) do
    if item.status == "ok" then
      local msg = item.version and (item.version .. (item.message and " (" .. item.message .. ")" or "")) or "installed"
      vim.health.ok(item.name .. ": " .. msg)
    elseif item.status == "warning" then
      vim.health.warn(item.name .. ": " .. (item.message or "warning"))
    else
      vim.health.error(item.name .. ": " .. (item.message or "missing"))
    end
  end
end

return M

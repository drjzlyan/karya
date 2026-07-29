local M = {}

local function find_bundles()
  local bundles = {}
  local home = vim.fn.expand("~")
  local candidates = {
    home .. "/.local/share/ide-tools/java-debug/extensions/debug/*.jar",
    home .. "/.local/share/ide-tools/java-debug/server/*.jar",
    home .. "/.local/share/ide-tools/java-test/extensions/*.jar",
    home .. "/.local/share/ide-tools/java-test/server/*.jar",
    "/opt/homebrew/opt/java-debug-adapter/libexec/extensions/debug/*.jar",
    "/opt/homebrew/opt/java-test/libexec/extensions/*.jar",
    "/usr/local/opt/java-debug-adapter/libexec/extensions/debug/*.jar",
    "/usr/local/opt/java-test/libexec/extensions/*.jar",
  }

  for _, pattern in ipairs(candidates) do
    for _, jar in ipairs(vim.fn.glob(pattern, false, true)) do
      table.insert(bundles, jar)
    end
  end

  return bundles
end

function M.bundles()
  return find_bundles()
end

function M.setup()
  local ok, jdtls_dap = pcall(require, "jdtls.dap")
  if not ok then
    return
  end

  local dap = require("dap")
  dap.configurations.java = dap.configurations.java or {}
  table.insert(dap.configurations.java, {
    type = "java",
    request = "attach",
    name = "Attach to running JVM (localhost:5005)",
    hostName = "127.0.0.1",
    port = 5005,
  })

  jdtls_dap.setup_dap({ hotcodereplace = "auto" })
  jdtls_dap.setup_dap_main_class_configs()
end

return M

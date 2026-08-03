local M = {}

local karya = require("util.karya")

-- Find the java-debug / java-test adapter jars in karya's isolated tool prefix
-- (installed there from the VS Code extensions). No Homebrew/system paths — karya
-- never assumes a global install (PLAN.md §2).
local function find_bundles()
  local bundles = {}
  local tools = karya.data_dir() .. "/tools"
  local roots = { karya.tool("java-debug"), karya.tool("java-test"), tools .. "/java-debug", tools .. "/java-test" }
  local subpatterns = { "/extension/server/*.jar", "/extension/*.jar", "/server/*.jar", "/extensions/debug/*.jar" }

  local seen = {}
  for _, root in ipairs(roots) do
    if root then
      for _, sub in ipairs(subpatterns) do
        for _, jar in ipairs(vim.fn.glob(root .. sub, false, true)) do
          if not seen[jar] then
            seen[jar] = true
            table.insert(bundles, jar)
          end
        end
      end
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

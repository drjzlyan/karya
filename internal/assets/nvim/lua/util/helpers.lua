local M = {}

-- Returns true if a plugin is available.
function M.has(plugin)
  local ok, _ = pcall(require, plugin)
  return ok
end

-- Print a formatted table for quick debugging.
function M.dump(o)
  if type(o) == "table" then
    local s = "{ "
    for k, v in pairs(o) do
      if type(k) ~= "number" then
        k = '"' .. k .. '"'
      end
      s = s .. "[" .. k .. "] = " .. M.dump(v) .. ","
    end
    return s .. "} "
  else
    return tostring(o)
  end
end

return M

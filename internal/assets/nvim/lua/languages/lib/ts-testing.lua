local M = {}

local project = require("util.project")

local function send(cmd, cwd)
  local terminal = require("features.terminal")
  if type(cmd) == "table" then
    cmd = table.concat(cmd, " ")
  end
  terminal.send("test", cmd, { dir = cwd })
end

-- Map a test source file (test/foo.test.ts) to its compiled output
-- (dist/test/foo.test.js). Returns nil when the file is not a test file.
local function compiled_test_path(bufnr)
  local file = vim.api.nvim_buf_get_name(bufnr)
  local root = project.root(bufnr)
  local rel = file:sub(#root + 2)
  if not rel:match("%.test%.tsx?$") and not rel:match("%.spec%.tsx?$") then
    return nil
  end
  return "dist/" .. rel:gsub("%.tsx?$", ".js")
end

-- Name argument of the enclosing node:test it()/test()/describe() block.
local function current_test_name()
  local node = vim.treesitter.get_node()
  while node do
    if node:type() == "call_expression" then
      local fn_node = node:field("function")[1]
      if fn_node then
        local fn_name = vim.treesitter.get_node_text(fn_node, 0)
        if fn_name == "it" or fn_name == "test" or fn_name == "describe" then
          local args = node:field("arguments")[1]
          if args then
            for child in args:iter_children() do
              if child:type() == "string" then
                local text = vim.treesitter.get_node_text(child, 0)
                return text:gsub("^[\"'`]", ""):gsub("[\"'`]$", "")
              end
            end
          end
        end
      end
    end
    node = node:parent()
  end
  return nil
end

function M.detect(bufnr)
  return project.is_type(bufnr or 0, "node")
end

function M.run_nearest(bufnr)
  local root = project.root(bufnr)
  local name = current_test_name()
  local compiled = compiled_test_path(bufnr)
  if not name or not compiled then
    M.run_current_class(bufnr)
    return
  end
  local pattern = name:gsub("'", "'\\''")
  send("npm run build && node --test --test-name-pattern='" .. pattern .. "' " .. compiled, root)
end

function M.run_current_class(bufnr)
  local root = project.root(bufnr)
  local compiled = compiled_test_path(bufnr)
  if not compiled then
    vim.notify("Current file is not a *.test.ts / *.spec.ts file", vim.log.levels.WARN)
    return
  end
  send("npm run build && node --test " .. compiled, root)
end

function M.run_package(bufnr)
  M.run_module(bufnr)
end

function M.run_module(bufnr)
  send({ "npm", "test" }, project.root(bufnr))
end

return M

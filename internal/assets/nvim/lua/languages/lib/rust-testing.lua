local M = {}

local project = require("util.project")

local function send(cmd, cwd)
  local terminal = require("features.terminal")
  if type(cmd) == "table" then
    cmd = table.concat(cmd, " ")
  end
  terminal.send("test", cmd, { dir = cwd })
end

local function is_test_fn(fn_node)
  local sibling = fn_node:prev_named_sibling()
  while sibling and sibling:type() == "attribute_item" do
    if vim.treesitter.get_node_text(sibling, 0):match("#%[test%]") then
      return true
    end
    sibling = sibling:prev_named_sibling()
  end
  return false
end

local function fn_name(fn_node)
  local name_node = fn_node:field("name")[1]
  return name_node and vim.treesitter.get_node_text(name_node, 0) or nil
end

-- Enclosing test function and the mod path leading to it (outermost first).
-- The mod path is returned even when the cursor is not inside a test
-- function, so mod-scoped runs work from anywhere in the module.
local function current_test()
  local node = vim.treesitter.get_node()
  local mods = {}
  local test_fn = nil
  local saw_plain_fn = false
  while node do
    local t = node:type()
    if t == "function_item" then
      if not test_fn and not saw_plain_fn then
        if is_test_fn(node) then
          test_fn = fn_name(node)
        else
          saw_plain_fn = true
        end
      end
    elseif t == "mod_item" then
      local name = fn_name(node)
      if name then
        table.insert(mods, 1, name)
      end
    end
    node = node:parent()
  end
  local modpath = #mods > 0 and table.concat(mods, "::") or nil
  if not test_fn then
    return nil, modpath
  end
  table.insert(mods, test_fn)
  return table.concat(mods, "::"), modpath
end

function M.detect(bufnr)
  return project.is_type(bufnr or 0, "rust")
end

function M.run_nearest(bufnr)
  local path = current_test()
  if not path then
    vim.notify("No Rust test under cursor", vim.log.levels.WARN)
    return
  end
  send({ "cargo", "test", path, "--", "--exact" }, project.root(bufnr))
end

-- Run every test in the enclosing mod (e.g. the `tests` module).
function M.run_current_class(bufnr)
  local _, mod_path = current_test()
  if mod_path then
    send({ "cargo", "test", mod_path .. "::" }, project.root(bufnr))
  else
    M.run_module(bufnr)
  end
end

function M.run_package(bufnr)
  send({ "cargo", "test" }, project.root(bufnr))
end

function M.run_module(bufnr)
  send({ "cargo", "test" }, project.root(bufnr))
end

return M

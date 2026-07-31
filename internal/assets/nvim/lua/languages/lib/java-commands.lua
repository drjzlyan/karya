local M = {}

local project = require("util.project")
local java_util = require("util.java")
local java_project = require("languages.lib.java-project")

local function jdtls_client(bufnr)
  return vim.lsp.get_clients({ bufnr = bufnr or 0, name = "jdtls" })[1]
end

local function source_action(kind)
  return function()
    vim.lsp.buf.code_action({
      context = { only = { kind } },
      apply = true,
    })
  end
end

local function refactor(kind)
  return function()
    vim.lsp.buf.code_action({
      context = { only = { "refactor." .. kind } },
      apply = true,
    })
  end
end

local function set_qflist_from_locations(locations)
  local items = vim.lsp.util.locations_to_items(locations or {}, "utf-16")
  vim.fn.setqflist(items, "r")
  vim.cmd("copen")
end

local function collect_hierarchy_items(item, locations)
  if item.location then
    table.insert(locations, item.location)
  elseif item.uri and item.range then
    table.insert(locations, { uri = item.uri, range = item.range })
  end
  for _, child in ipairs(item.children or {}) do
    collect_hierarchy_items(child, locations)
  end
  for _, parent in ipairs(item.parents or {}) do
    collect_hierarchy_items(parent, locations)
  end
end

function M.open_type_hierarchy()
  local bufnr = vim.api.nvim_get_current_buf()
  local client = jdtls_client(bufnr)
  if not client then
    vim.notify("jdtls is not attached", vim.log.levels.WARN)
    return
  end
  local params = vim.lsp.util.make_position_params(0, "utf-16")
  client:request("java/navigate/openTypeHierarchy", {
    textDocument = params.textDocument,
    position = params.position,
  }, function(err, result)
    if err then
      vim.notify("Type hierarchy failed: " .. tostring(err.message or err), vim.log.levels.ERROR)
      return
    end
    local locations = {}
    collect_hierarchy_items(result, locations)
    set_qflist_from_locations(locations)
  end, bufnr)
end

function M.open_workspace_logs()
  local log = java_util.workspace_dir(project.root(0)) .. "/.metadata/.log"
  if vim.fn.filereadable(log) ~= 1 then
    vim.notify("No jdtls log found at " .. log, vim.log.levels.WARN)
    return
  end
  vim.cmd("edit " .. vim.fn.fnameescape(log))
end

function M.clear_workspace_cache()
  local dir = java_util.workspace_dir(project.root(0))
  local ok, err = pcall(vim.fn.delete, dir, "rf")
  if not ok then
    vim.notify("Failed to clear jdtls workspace cache: " .. tostring(err), vim.log.levels.ERROR)
    return
  end
  vim.notify("Cleared jdtls workspace cache: " .. dir, vim.log.levels.INFO)
  pcall(vim.cmd, "LspRestart jdtls")
end

function M.build_workspace()
  local bufnr = vim.api.nvim_get_current_buf()
  local client = jdtls_client(bufnr)
  if not client then
    vim.notify("jdtls is not attached", vim.log.levels.WARN)
    return
  end
  client:request("java/buildWorkspace", true, function(err, result)
    if err then
      vim.notify("Build workspace failed: " .. tostring(err.message or err), vim.log.levels.ERROR)
    else
      vim.notify("Workspace build status: " .. tostring(result), vim.log.levels.INFO)
    end
  end, bufnr)
end

function M.reload_workspace()
  local bufnr = vim.api.nvim_get_current_buf()
  local client = jdtls_client(bufnr)
  if not client then
    vim.notify("jdtls is not attached", vim.log.levels.WARN)
    return
  end
  client:notify("java/projectConfigurationsUpdate", {
    identifiers = { { uri = vim.uri_from_bufnr(bufnr) } },
  })
  vim.notify("Reloading workspace configuration", vim.log.levels.INFO)
end

function M.restart_jdtls()
  vim.cmd("LspRestart jdtls")
end

function M.run_project_cmd(cmd_name)
  return function()
    local result = java_project.run(cmd_name, 0)
    if not result then
      return
    end
    local terminal = require("features.terminal")
    local cmd = result.cmd
    if type(cmd) == "table" then
      cmd = table.concat(cmd, " ")
    end
    terminal.send(cmd_name == "test" and "test" or "build", cmd, { dir = result.cwd })
  end
end

function M.format_java()
  local ok, conform = pcall(require, "conform")
  if ok then
    conform.format({ bufnr = 0, async = false, lsp_format = "fallback" })
  else
    vim.notify("conform.nvim is not available", vim.log.levels.WARN)
  end
end

function M.register_commands(bufnr)
  local cmds = {
    { "JavaFormat", M.format_java, { desc = "Format Java with google-java-format" } },
    { "JavaOrganizeImports", source_action("source.organizeImports"), { desc = "Organize Java imports" } },
    { "JavaAddMissingImports", source_action("source.addMissingImports"), { desc = "Add missing Java imports" } },
    { "JavaRemoveUnusedImports", source_action("source.removeUnusedImports"), { desc = "Remove unused Java imports" } },
    { "JavaExtractMethod", refactor("extract.function"), { desc = "Extract method" } },
    { "JavaExtractVariable", refactor("extract.variable"), { desc = "Extract variable" } },
    { "JavaExtractConstant", refactor("extract.constant"), { desc = "Extract constant" } },
    { "JavaInlineVariable", refactor("inline"), { desc = "Inline variable" } },
    { "JavaMoveType", refactor("move"), { desc = "Move type" } },
    { "JavaRename", vim.lsp.buf.rename, { desc = "Rename symbol" } },
    { "JavaCompile", M.run_project_cmd("compile"), { desc = "Compile Java project" } },
    { "JavaPackage", M.run_project_cmd("package"), { desc = "Package Java project" } },
    { "JavaVerify", M.run_project_cmd("verify"), { desc = "Verify Java project" } },
    { "JavaBuildWorkspace", M.build_workspace, { desc = "Build workspace" } },
    { "JavaReloadWorkspace", M.reload_workspace, { desc = "Reload workspace configuration" } },
    { "JavaRestartJdtls", M.restart_jdtls, { desc = "Restart jdtls" } },
    { "JavaClearWorkspaceCache", M.clear_workspace_cache, { desc = "Clear jdtls workspace cache" } },
    { "JavaOpenWorkspaceLogs", M.open_workspace_logs, { desc = "Open jdtls workspace logs" } },
    { "JavaIncomingCalls", vim.lsp.buf.incoming_calls, { desc = "Incoming calls" } },
    { "JavaOutgoingCalls", vim.lsp.buf.outgoing_calls, { desc = "Outgoing calls" } },
    { "JavaTypeHierarchy", M.open_type_hierarchy, { desc = "Open type hierarchy" } },
    { "JavaImplementationHierarchy", vim.lsp.buf.implementation, { desc = "Implementation hierarchy" } },
  }

  for _, c in ipairs(cmds) do
    vim.api.nvim_buf_create_user_command(bufnr, c[1], c[2], c[3])
  end
end

-- Route Java through the shared <leader>c "Code" interface (via langmaps). The
-- core actions (format, imports, refactor, build, run, test, debug, calls) match
-- every other language; Java-only workspace and navigation actions live under
-- the <leader>cw* subgroup so they stay discoverable without polluting the core.
function M.register_keymaps(bufnr)
  require("util.langmaps").register(bufnr, {
    lang = "Java",
    format = M.format_java,
    organize_imports = source_action("source.organizeImports"),
    debug = true,
    extra = function(map)
      map("wb", M.build_workspace, "n", "Build workspace")
      map("wr", M.reload_workspace, "n", "Reload workspace configuration")
      map("ww", M.restart_jdtls, "n", "Restart jdtls")
      map("wc", M.clear_workspace_cache, "n", "Clear workspace cache")
      map("wl", M.open_workspace_logs, "n", "Open workspace logs")
      map("wh", M.open_type_hierarchy, "n", "Type hierarchy")
      map("wv", M.run_project_cmd("verify"), "n", "Verify project")
    end,
  })
end

return M

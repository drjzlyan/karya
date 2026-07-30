local M = {}

---Show a floating window containing the health report.
local function dev_health()
  require("util.health").show()
end

---Show project detection information for the current buffer.
local function dev_info()
  local project = require("util.project")
  local bufnr = vim.api.nvim_get_current_buf()
  local root = project.root(bufnr)
  local types = project.types(bufnr)

  local lines = {
    "# DevInfo",
    "",
    "Project root: " .. (root or "<none>"),
    "Detected types: " .. (vim.tbl_isempty(types) and "<none>" or table.concat(types, ", ")),
    "",
    "Root markers:",
  }

  for _, marker in ipairs(project.root_markers) do
    local found = project.has_marker(bufnr, marker)
    table.insert(lines, string.format("  %s %s", found and "✓" or " ", marker))
  end

  local buf = vim.api.nvim_create_buf(false, true)
  vim.api.nvim_buf_set_lines(buf, 0, -1, false, lines)
  vim.bo[buf].buftype = "nofile"
  vim.bo[buf].bufhidden = "wipe"
  vim.bo[buf].modifiable = false
  vim.bo[buf].filetype = "markdown"

  local width = math.min(80, math.floor(vim.o.columns * 0.8))
  local height = math.min(#lines + 2, math.floor(vim.o.lines * 0.8))
  local row = math.floor((vim.o.lines - height) / 2)
  local col = math.floor((vim.o.columns - width) / 2)

  local win = vim.api.nvim_open_win(buf, true, {
    relative = "editor",
    width = width,
    height = height,
    row = row,
    col = col,
    style = "minimal",
    border = "rounded",
    title = " DevInfo ",
    title_pos = "center",
  })

  local close = function()
    vim.api.nvim_win_close(win, true)
  end
  vim.keymap.set("n", "q", close, { buffer = buf, silent = true })
  vim.keymap.set("n", "<Esc>", close, { buffer = buf, silent = true })
end

---Reload user configuration modules without restarting.
---Plugin specs managed by lazy.nvim require a full Neovim restart to reload.
local function dev_reload()
  local modules = {}
  for name, _ in pairs(package.loaded) do
    if
      name:match("^core%.")
      or name:match("^features%.")
      or name:match("^languages%.")
      or name:match("^util%.")
      or name == "dev.health"
    then
      package.loaded[name] = nil
      table.insert(modules, name)
    end
  end

  local ok, err = pcall(function()
    require("core.options")
    require("core.keymaps")
    require("core.autocmds")
    require("core.commands").setup()
  end)

  if ok then
    vim.notify(
      "Configuration reloaded (" .. #modules .. " modules). Restart Neovim to reload plugin specs.",
      vim.log.levels.INFO
    )
  else
    vim.notify("Failed to reload configuration: " .. tostring(err), vim.log.levels.ERROR)
  end
end

---Update editor plugins via lazy.nvim and refresh external tooling via karya.
local function dev_update()
  -- Sync plugins first.
  if vim.fn.exists(":Lazy") == 2 then
    vim.cmd("Lazy sync")
  else
    vim.notify("lazy.nvim is not loaded", vim.log.levels.WARN)
  end

  -- Refresh external tooling (runtimes, LSPs, formatters) through karya, which
  -- reinstalls them idempotently into the isolated karya prefix.
  if vim.fn.executable("karya") == 1 then
    vim.fn.jobstart({ "karya", "install" }, {
      detach = true,
      on_exit = function(_, code)
        vim.schedule(function()
          if code == 0 then
            vim.notify("External tooling updated", vim.log.levels.INFO)
          else
            vim.notify("External tooling update failed (exit " .. code .. ")", vim.log.levels.WARN)
          end
        end)
      end,
    })
  else
    vim.notify("karya not found on PATH; skipping external tooling update", vim.log.levels.INFO)
  end
end

---Profile startup time and display the last lines in a floating window.
local function dev_profile()
  local log = vim.fn.stdpath("cache") .. "/startup.log"
  vim.fn.mkdir(vim.fn.fnamemodify(log, ":h"), "p")

  vim.notify("Profiling startup...", vim.log.levels.INFO)

  local job = vim.fn.jobstart({ "nvim", "--headless", "--startuptime", log, "-c", "qa" }, {
    on_exit = function()
      vim.schedule(function()
        if vim.fn.filereadable(log) ~= 1 then
          vim.notify("Startup profile log not created", vim.log.levels.ERROR)
          return
        end

        local lines = vim.fn.readfile(log, "", 30)
        local buf = vim.api.nvim_create_buf(false, true)
        vim.api.nvim_buf_set_lines(buf, 0, -1, false, lines)
        vim.bo[buf].buftype = "nofile"
        vim.bo[buf].bufhidden = "wipe"
        vim.bo[buf].modifiable = false
        vim.bo[buf].filetype = ""

        local width = math.min(100, math.floor(vim.o.columns * 0.9))
        local height = math.min(#lines + 2, math.floor(vim.o.lines * 0.8))
        local row = math.floor((vim.o.lines - height) / 2)
        local col = math.floor((vim.o.columns - width) / 2)

        local win = vim.api.nvim_open_win(buf, true, {
          relative = "editor",
          width = width,
          height = height,
          row = row,
          col = col,
          style = "minimal",
          border = "rounded",
          title = " Startup Profile ",
          title_pos = "center",
        })

        local close = function()
          vim.api.nvim_win_close(win, true)
        end
        vim.keymap.set("n", "q", close, { buffer = buf, silent = true })
        vim.keymap.set("n", "<Esc>", close, { buffer = buf, silent = true })
        vim.notify("Startup profile written to " .. log, vim.log.levels.INFO)
      end)
    end,
  })

  if job <= 0 then
    vim.notify("Could not start nvim profiling job", vim.log.levels.ERROR)
  end
end

---Clear caches and temporary data.
local function dev_clean_cache(args)
  local what = vim.split(args.args or "", " ", { plain = true })
  if #what == 0 or (what[1] == "") then
    what = { "all" }
  end

  local actions = {}
  local function add(name, fn)
    table.insert(actions, { name = name, fn = fn })
  end

  local function should(name)
    return vim.tbl_contains(what, "all") or vim.tbl_contains(what, name)
  end

  if should("treesitter") then
    add("treesitter cache", function()
      local dir = vim.fn.stdpath("cache") .. "/treesitter"
      if vim.fn.isdirectory(dir) == 1 then
        vim.fn.delete(dir, "rf")
      end
      return "cleared " .. dir
    end)
  end

  if should("jdtls") then
    add("jdtls workspace", function()
      local project = require("util.project")
      local java_util = require("util.java")
      local dir = java_util.workspace_dir(project.root(0))
      if vim.fn.isdirectory(dir) == 1 then
        vim.fn.delete(dir, "rf")
      end
      pcall(vim.cmd, "LspRestart jdtls")
      return "cleared " .. dir
    end)
  end

  if should("swap") then
    add("swap files", function()
      -- Neovim is configured with swapfile = false, but clean stale files just in case.
      local dir = vim.fn.stdpath("state") .. "/swap"
      if vim.fn.isdirectory(dir) == 1 then
        vim.fn.delete(dir, "rf")
      end
      return "cleared swap files"
    end)
  end

  if should("sessions") then
    add("sessions", function()
      local dir = vim.fn.stdpath("data") .. "/sessions"
      if vim.fn.isdirectory(dir) == 1 then
        vim.fn.delete(dir, "rf")
      end
      return "cleared sessions"
    end)
  end

  if should("lazy") then
    add("lazy cache", function()
      if vim.fn.exists(":Lazy") == 2 then
        vim.cmd("Lazy clean")
        return "ran Lazy clean"
      end
      return "lazy.nvim not loaded"
    end)
  end

  local messages = {}
  for _, action in ipairs(actions) do
    local ok, result = pcall(action.fn)
    if ok then
      table.insert(messages, "✓ " .. action.name .. ": " .. tostring(result))
    else
      table.insert(messages, "✗ " .. action.name .. ": " .. tostring(result))
    end
  end

  vim.notify(table.concat(messages, "\n"), vim.log.levels.INFO)
end

---Register all custom Neovim commands.
function M.setup()
  vim.api.nvim_create_user_command("DevHealth", dev_health, { desc = "Show development environment health" })
  vim.api.nvim_create_user_command("DevInfo", dev_info, { desc = "Show project detection info for current buffer" })
  vim.api.nvim_create_user_command("DevReload", dev_reload, { desc = "Reload Neovim configuration" })
  vim.api.nvim_create_user_command("DevUpdate", dev_update, { desc = "Update plugins and external tooling" })
  vim.api.nvim_create_user_command("DevProfile", dev_profile, { desc = "Profile Neovim startup time" })
  vim.api.nvim_create_user_command(
    "DevCleanCache",
    dev_clean_cache,
    { desc = "Clear caches: treesitter, jdtls, swap, sessions, lazy (default: all)", nargs = "?" }
  )
end

return M

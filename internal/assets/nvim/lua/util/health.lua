local M = {}

local karya = require("util.karya")

---@class DevHealthResult
---@field name string Display name of the checked item.
---@field status "ok"|"missing"|"warning"|"error" Current status.
---@field version? string Detected version, if any.
---@field message? string Human-readable explanation or fix suggestion.

local MIN_NVIM_VERSION = "0.11.0"

---Safely run a command and return trimmed stdout.
---@param cmd string[]
---@return string?
local function safe_exec(cmd)
  local ok, output = pcall(vim.fn.system, cmd)
  if not ok or not output then
    return nil
  end
  local trimmed = vim.trim(output or "")
  if trimmed == "" then
    return nil
  end
  return trimmed
end

---Run a version command and extract the first version-looking line.
---@param cmd string[]
---@return string?
local function version_from(cmd)
  local output = safe_exec(cmd)
  if not output then
    return nil
  end
  return output:match("%d+%.?%d*%.?%d*") or output:sub(1, 60)
end

---Check whether an executable is on PATH.
---@param name string
---@return boolean
local function has_exec(name)
  return vim.fn.executable(name) == 1
end

---Find an installed JDK for the requested version, strictly within karya's
---isolated mise prefix (never Homebrew or system JVMs — see DESIGN.md §4).
---@param version integer
---@return string?
function M.find_jdk(version)
  local mise_java = karya.data_dir() .. "/mise/installs/java"

  local paths = {
    mise_java .. "/" .. version,
    mise_java .. "/temurin-" .. version,
    mise_java .. "/openjdk-" .. version,
  }

  for _, path in ipairs(paths) do
    if vim.fn.isdirectory(path) == 1 then
      return path
    end
  end

  -- Glob for mise patch-version installs (e.g. 21.0.5)
  local pattern = mise_java .. "/" .. version .. ".*"
  local matches = vim.fn.glob(pattern, false, true)
  if type(matches) == "table" then
    table.sort(matches, function(a, b)
      return a > b
    end)
    for _, p in ipairs(matches) do
      if vim.fn.isdirectory(p) == 1 then
        return p
      end
    end
  end

  return nil
end

---Run the Java version command and return the version string.
---@param java_path string
---@return string?
local function java_version(java_path)
  local output = safe_exec({ java_path, "-version" })
  if not output then
    return nil
  end
  -- OpenJDK output puts version on the first line like "openjdk version \"17.0.9\" ..."
  return output:match('version "(%d+[%.%d]*)"') or output:match("(%d+%.%d+)")
end

---Check a generic executable.
---@param name string
---@param version_cmd? string[]
---@param install_hint? string
---@return DevHealthResult
local function check_tool(name, version_cmd, install_hint)
  if not has_exec(name) then
    return {
      name = name,
      status = "missing",
      message = install_hint or ("Install " .. name),
    }
  end
  return {
    name = name,
    status = "ok",
    version = version_cmd and version_from(version_cmd) or nil,
  }
end

---Check a Homebrew-installed LSP/formatter binary.
---@param name string
---@param package string
---@param version_cmd? string[]
---@return DevHealthResult
local function check_brew_tool(name, package, version_cmd)
  if not has_exec(name) then
    return {
      name = name,
      status = "missing",
      message = "run `karya install` to provision it",
    }
  end
  return {
    name = name,
    status = "ok",
    version = version_cmd and version_from(version_cmd) or nil,
  }
end

---Check the Neovim version.
---@return DevHealthResult
local function check_neovim()
  local version = vim.version()
  local version_string = string.format("%d.%d.%d", version.major, version.minor, version.patch)
  local ok = vim.version.ge(version, MIN_NVIM_VERSION)
  return {
    name = "Neovim",
    status = ok and "ok" or "error",
    version = version_string,
    message = ok and nil or ("Requires >= " .. MIN_NVIM_VERSION),
  }
end

---Check the Git installation.
---@return DevHealthResult
local function check_git()
  return check_tool("git", { "git", "--version" }, "install git (most systems ship it)")
end

---Check ripgrep.
---@return DevHealthResult
local function check_ripgrep()
  return check_tool("rg", { "rg", "--version" }, "run `karya profile install core`")
end

---Check fd.
---@return DevHealthResult
local function check_fd()
  return check_tool("fd", { "fd", "--version" }, "run `karya profile install core`")
end

---Check fzf.
---@return DevHealthResult
local function check_fzf()
  return check_tool("fzf", { "fzf", "--version" }, "run `karya profile install core`")
end

---Check lazygit.
---@return DevHealthResult
local function check_lazygit()
  return check_tool("lazygit", { "lazygit", "--version" }, "install lazygit for the in-session git UI")
end

---Check Ghostty.
---@return DevHealthResult
local function check_ghostty()
  return check_tool("ghostty", { "ghostty", "--version" }, "brew install --cask ghostty")
end

---Check tmux.
---@return DevHealthResult
local function check_tmux()
  return check_tool("tmux", { "tmux", "-V" }, "run `karya install`")
end

---Check uv.
---@return DevHealthResult
local function check_uv()
  return check_tool("uv", { "uv", "--version" }, "run `karya install`")
end

---Check a specific JDK version.
---@param version integer
---@return DevHealthResult
local function check_jdk(version)
  local path = M.find_jdk(version)
  if not path then
    return {
      name = "JDK " .. version,
      status = "missing",
      message = "run `karya lang add java` to install it",
    }
  end
  local java = path .. "/bin/java"
  return {
    name = "JDK " .. version,
    status = "ok",
    version = java_version(java),
    message = path,
  }
end

---Check JAVA_HOME.
---@return DevHealthResult
local function check_java_home()
  local home = vim.env.JAVA_HOME
  if not home or home == "" then
    return {
      name = "JAVA_HOME",
      status = "missing",
      message = "Set JAVA_HOME to a valid JDK, e.g. /Library/Java/JavaVirtualMachines/temurin-17.jdk/Contents/Home",
    }
  end
  if vim.fn.isdirectory(home) ~= 1 then
    return {
      name = "JAVA_HOME",
      status = "error",
      version = home,
      message = "JAVA_HOME points to a non-existent directory",
    }
  end
  return {
    name = "JAVA_HOME",
    status = "ok",
    version = java_version(home .. "/bin/java"),
    message = home,
  }
end

---Check jdtls.
---@return DevHealthResult
local function check_jdtls()
  return check_brew_tool("jdtls", "jdtls", { "jdtls", "--version" })
end

---Check basedpyright.
---@return DevHealthResult
local function check_basedpyright()
  return check_brew_tool("basedpyright", "basedpyright", { "basedpyright", "--version" })
end

---Check ruff.
---@return DevHealthResult
local function check_ruff()
  return check_brew_tool("ruff", "ruff", { "ruff", "--version" })
end

---Check debugpy.
---@return DevHealthResult
local function check_debugpy()
  -- debugpy is typically installed inside a Python venv or via uv tool install
  if has_exec("debugpy") then
    return {
      name = "debugpy",
      status = "ok",
      version = version_from({ "debugpy", "--version" }),
    }
  end
  local uv_ok = has_exec("uv")
  if uv_ok then
    return {
      name = "debugpy",
      status = "missing",
      message = "uv tool install debugpy  (or install into the active Python venv)",
    }
  end
  return {
    name = "debugpy",
    status = "missing",
    message = "pip install debugpy  (or uv tool install debugpy)",
  }
end

---Check google-java-format.
---@return DevHealthResult
local function check_google_java_format()
  return check_brew_tool("google-java-format", "google-java-format", { "google-java-format", "--version" })
end

---Check Lombok jar via karya's isolated resolver (manifest + tool prefix).
---@return DevHealthResult
local function check_lombok()
  local jar = require("util.java").find_lombok_jar()
  if jar then
    return { name = "Lombok", status = "ok", message = jar }
  end
  return {
    name = "Lombok",
    status = "missing",
    message = "run `karya lang add java` to install it",
  }
end

---Read which languages are selected from languages.local.
---@return table<string, boolean>
local function selected_languages()
  local langs = {}
  local path = karya.data_dir() .. "/languages.local"
  local f = io.open(path, "r")
  if not f then
    return langs
  end
  for line in f:lines() do
    local lang = line:match("^([%a]+)=")
    if lang then
      langs[lang] = true
    end
  end
  f:close()
  return langs
end

---Return all health check results.
---@return DevHealthResult[]
function M.check_all()
  local results = {
    check_neovim(),
    check_git(),
    check_ripgrep(),
    check_fd(),
    check_fzf(),
    check_lazygit(),
    check_ghostty(),
    check_tmux(),
    check_uv(),
  }

  local langs = selected_languages()

  if langs.python then
    vim.list_extend(results, {
      check_basedpyright(),
      check_ruff(),
      check_debugpy(),
    })
  end

  if langs.java then
    vim.list_extend(results, {
      check_jdk(8),
      check_jdk(11),
      check_jdk(17),
      check_jdk(21),
      check_java_home(),
      check_jdtls(),
      check_google_java_format(),
      check_lombok(),
    })
    if vim.fn.executable("mvn") == 1 then
      table.insert(results, check_tool("mvn", { "mvn", "--version" }, "install Maven"))
    end
    if vim.fn.executable("gradle") == 1 then
      table.insert(results, check_tool("gradle", { "gradle", "--version" }, "install Gradle"))
    end
    if vim.fn.executable("mvn") ~= 1 and vim.fn.executable("gradle") ~= 1 then
      table.insert(results, {
        name = "mvn / gradle",
        status = "missing",
        message = "install Maven or Gradle for your Java build",
      })
    end
  end

  if langs.typescript then
    vim.list_extend(results, {
      check_tool("node", { "node", "--version" }, "run `karya lang add typescript`"),
      check_tool(
        "typescript-language-server",
        { "typescript-language-server", "--version" },
        "run `karya lang add typescript`"
      ),
      check_tool("prettier", { "prettier", "--version" }, "run `karya profile install docs`"),
    })
  end

  if langs.go then
    vim.list_extend(results, {
      check_tool("go", { "go", "version" }, "run `karya lang add go`"),
      check_tool("gopls", { "gopls", "version" }, "run `karya lang add go`"),
      check_tool("goimports", nil, "run `karya lang add go`"),
      check_tool("dlv", { "dlv", "version" }, "run `karya lang add go`"),
    })
  end

  if langs.cpp then
    vim.list_extend(results, {
      check_tool("clangd", { "clangd", "--version" }, "install LLVM/clang (system package)"),
    })
  end

  if langs.rust then
    vim.list_extend(results, {
      check_tool("cargo", { "cargo", "--version" }, "run `karya lang add rust`"),
      check_tool("rust-analyzer", { "rust-analyzer", "--version" }, "run `karya lang add rust`"),
      check_tool("rustfmt", { "rustfmt", "--version" }, "run `karya lang add rust`"),
    })
  end

  return results
end

---Render health results into a buffer.
---@param results DevHealthResult[]
local function render(results)
  local lines = { "# DevHealth Report", "" }
  local any_missing = false

  for _, item in ipairs(results) do
    local status_icon = ({ ok = "✓", missing = "✗", warning = "!", error = "✗" })[item.status] or "?"
    table.insert(lines, string.format("%s %s", status_icon, item.name))
    if item.version then
      table.insert(lines, string.format("    version: %s", item.version))
    end
    if item.message then
      table.insert(lines, string.format("    %s%s", item.status == "ok" and "path: " or "fix: ", item.message))
    end
    if item.status ~= "ok" then
      any_missing = true
    end
    table.insert(lines, "")
  end

  if any_missing then
    table.insert(lines, "Run :checkhealth dev for the standard Neovim health report.")
  else
    table.insert(lines, "All checks passed.")
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
    title = " DevHealth ",
    title_pos = "center",
  })

  vim.keymap.set("n", "q", function()
    vim.api.nvim_win_close(win, true)
  end, { buffer = buf, silent = true })
  vim.keymap.set("n", "<Esc>", function()
    vim.api.nvim_win_close(win, true)
  end, { buffer = buf, silent = true })
end

---Open a floating window with the health report.
function M.show()
  local ok, results = pcall(M.check_all)
  if not ok then
    vim.notify("DevHealth failed: " .. tostring(results), vim.log.levels.ERROR)
    return
  end
  render(results)
end

return M

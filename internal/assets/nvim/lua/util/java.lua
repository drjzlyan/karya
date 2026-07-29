local M = {}

-- All LTS / common Java major versions to probe for.
local KNOWN_VERSIONS = { 8, 11, 17, 21, 25 }

---Read selected Java versions from languages.local.
---Returns a list of version integers (major only).
local function selected_java_versions()
  local path = vim.fn.expand("~/.local/share/nvim/languages.local")
  local ok, f = pcall(io.open, path, "r")
  if not ok or not f then
    return {}
  end
  local versions = {}
  for line in f:lines() do
    local value = line:match("^java%s*=%s*(.+)$")
    if value then
      for v in value:gmatch("[^,]+") do
        local major = tonumber(v:match("^%s*(%d+)"))
        if major then
          table.insert(versions, major)
        end
      end
    end
  end
  f:close()
  return versions
end

local function find_jdks()
  local jdks = {}
  local home = vim.fn.expand("~")
  local brew_prefix = vim.fn.exists("$HOMEBREW_PREFIX") == 1 and vim.env.HOMEBREW_PREFIX
    or (vim.fn.isdirectory("/opt/homebrew") == 1 and "/opt/homebrew" or "/usr/local")

  -- Collect all versions to check: user-selected first, then well-known LTS
  local to_check = {}
  local seen = {}
  for _, v in ipairs(selected_java_versions()) do
    if not seen[v] then
      seen[v] = true
      table.insert(to_check, v)
    end
  end
  for _, v in ipairs(KNOWN_VERSIONS) do
    if not seen[v] then
      seen[v] = true
      table.insert(to_check, v)
    end
  end

  for _, version in ipairs(to_check) do
    local paths = {
      -- mise: exact major, temurin-N, openjdk-N, and glob for N.x.y variants
      home .. "/.local/share/mise/installs/java/" .. version,
      home .. "/.local/share/mise/installs/java/temurin-" .. version,
      home .. "/.local/share/mise/installs/java/openjdk-" .. version,
      -- Homebrew
      brew_prefix .. "/opt/openjdk@" .. version .. "/libexec/openjdk.jdk/Contents/Home",
      -- Temurin / AdoptOpenJDK system installs
      "/Library/Java/JavaVirtualMachines/temurin-" .. version .. ".jdk/Contents/Home",
      "/Library/Java/JavaVirtualMachines/adoptopenjdk-" .. version .. ".jdk/Contents/Home",
      "/usr/local/opt/openjdk@" .. version .. "/libexec/openjdk.jdk/Contents/Home",
    }

    for _, p in ipairs(paths) do
      if vim.fn.isdirectory(p) == 1 then
        jdks[version] = p
        break
      end
    end

    -- Glob for mise installs that include patch versions (e.g. 21.0.5)
    if not jdks[version] then
      local pattern = home .. "/.local/share/mise/installs/java/" .. version .. ".*"
      local matches = vim.fn.glob(pattern, false, true)
      if type(matches) == "table" then
        -- Sort descending so newest patch is first
        table.sort(matches, function(a, b)
          return a > b
        end)
        for _, p in ipairs(matches) do
          if vim.fn.isdirectory(p) == 1 then
            jdks[version] = p
            break
          end
        end
      end
    end
  end

  return jdks
end

function M.resolve_jdk()
  if vim.env.JAVA_HOME and vim.fn.isdirectory(vim.env.JAVA_HOME) == 1 then
    return vim.env.JAVA_HOME
  end

  local jdks = find_jdks()

  -- Prefer user-selected Java version
  for _, v in ipairs(selected_java_versions()) do
    if jdks[v] then
      return jdks[v]
    end
  end

  -- Fall back: newest known version first
  for _, v in ipairs({ 25, 21, 17, 11, 8 }) do
    if jdks[v] then
      return jdks[v]
    end
  end
  return nil
end

function M.pick_jdk_for_project(root)
  if not root then
    return M.resolve_jdk()
  end
  local ok, f = pcall(io.open, root .. "/.java-version", "r")
  if ok and f then
    local wanted = tonumber((f:read("*l") or ""):match("(%d+)"))
    f:close()
    if wanted then
      local jdks = find_jdks()
      if jdks[wanted] then
        return jdks[wanted]
      end
    end
  end
  return M.resolve_jdk()
end

function M.find_lombok_jar()
  local home = vim.fn.expand("~")
  local brew_prefix = vim.fn.exists("$HOMEBREW_PREFIX") == 1 and vim.env.HOMEBREW_PREFIX
    or (vim.fn.isdirectory("/opt/homebrew") == 1 and "/opt/homebrew" or "/usr/local")

  local candidates = {
    home .. "/.local/share/ide-tools/lombok.jar",
    brew_prefix .. "/opt/lombok/libexec/lombok.jar",
    brew_prefix .. "/opt/lombok/libexec/lombok-1.18.34.jar",
    "/usr/local/opt/lombok/libexec/lombok.jar",
  }

  for _, c in ipairs(candidates) do
    if vim.fn.filereadable(c) == 1 then
      return c
    end
  end

  local matches = vim.fn.glob(brew_prefix .. "/Cellar/lombok/*/libexec/lombok*.jar", false, true)
  if type(matches) == "table" and #matches > 0 then
    return matches[1]
  end

  return nil
end

function M.workspace_dir(root)
  local name = root and vim.fn.fnamemodify(root, ":t") or "unknown"
  return vim.fn.expand("~/.cache/jdtls") .. "/" .. name
end

function M.jdtls_cmd(root)
  local cmd = { "jdtls" }
  local jdk = M.pick_jdk_for_project(root)
  if jdk then
    vim.list_extend(cmd, { "--java-executable", jdk .. "/bin/java" })
  end
  local lombok = M.find_lombok_jar()
  if lombok then
    vim.list_extend(cmd, { "--jvm-arg", "-javaagent:" .. lombok })
  end
  local xms = vim.env.NVIM_JDTLS_XMS or "-Xms1G"
  local xmx = vim.env.NVIM_JDTLS_XMX or "-Xmx4G"
  local gc = vim.env.NVIM_JDTLS_GC or "-XX:+UseG1GC"
  for _, arg in ipairs({ xms, xmx, gc }) do
    vim.list_extend(cmd, { "--jvm-arg", arg })
  end
  vim.list_extend(cmd, { "-data", M.workspace_dir(root) })
  return cmd
end

return M

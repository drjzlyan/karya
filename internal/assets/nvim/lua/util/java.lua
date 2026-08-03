local M = {}

local karya = require("util.karya")

-- All LTS / common Java major versions to probe for.
local KNOWN_VERSIONS = { 8, 11, 17, 21, 25 }

---Read selected Java versions from karya's isolated languages.local.
---Returns a list of version integers (major only).
local function selected_java_versions()
  local path = karya.data_dir() .. "/languages.local"
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

-- find_jdks discovers installed JDKs strictly within karya's isolated mise
-- prefix. karya never installs into or reads from Homebrew, the system
-- JavaVirtualMachines, or the user's global mise (PLAN.md §2), so only karya's
-- own installs are considered here.
local function find_jdks()
  local jdks = {}
  local mise_java = karya.data_dir() .. "/mise/installs/java"

  -- Collect all versions to check: user-selected first, then well-known LTS.
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
    -- karya's isolated mise installs: exact major, and temurin-/openjdk- variants.
    local paths = {
      mise_java .. "/" .. version,
      mise_java .. "/temurin-" .. version,
      mise_java .. "/openjdk-" .. version,
    }

    for _, p in ipairs(paths) do
      if vim.fn.isdirectory(p) == 1 then
        jdks[version] = p
        break
      end
    end

    -- Glob for installs that include patch versions (e.g. 21.0.5).
    if not jdks[version] then
      local matches = vim.fn.glob(mise_java .. "/" .. version .. ".*", false, true)
      if type(matches) == "table" then
        -- Sort descending so newest patch is first.
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
  -- JAVA_HOME, when set by karya's generated mise config, is authoritative.
  if vim.env.JAVA_HOME and vim.fn.isdirectory(vim.env.JAVA_HOME) == 1 then
    return vim.env.JAVA_HOME
  end

  local jdks = find_jdks()

  -- Prefer user-selected Java version.
  for _, v in ipairs(selected_java_versions()) do
    if jdks[v] then
      return jdks[v]
    end
  end

  -- Fall back: newest known version first.
  for _, v in ipairs({ 25, 21, 17, 11, 8 }) do
    if jdks[v] then
      return jdks[v]
    end
  end
  return nil
end

-- jdtls itself runs on a JVM and requires Java 21+ (independent of the project's
-- target Java version). Pick the highest installed JDK that is >= 21 for the
-- language server's own JVM, so a project pinned to Java 8/11/17 doesn't make
-- jdtls exit (E: "jdtls exited with 2").
local function server_jdk()
  local jdks = find_jdks()
  local best_major, best_path = nil, nil
  for major, path in pairs(jdks) do
    if major >= 21 and (best_major == nil or major > best_major) then
      best_major, best_path = major, path
    end
  end
  return best_path
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

-- find_lombok_jar returns the Lombok jar from karya's isolated tool prefix,
-- preferring the resolved path in karya's tool manifest and falling back to the
-- known location under karya's tools dir. No Homebrew/system paths are consulted.
function M.find_lombok_jar()
  local from_manifest = karya.tool("lombok")
  if from_manifest and vim.fn.filereadable(from_manifest) == 1 then
    return from_manifest
  end
  local jar = karya.data_dir() .. "/tools/lombok.jar"
  if vim.fn.filereadable(jar) == 1 then
    return jar
  end
  return nil
end

function M.workspace_dir(root)
  local name = root and vim.fn.fnamemodify(root, ":t") or "unknown"
  return karya.cache_dir() .. "/jdtls/" .. name
end

function M.jdtls_cmd(root)
  -- Prefer the manifest-resolved jdtls launcher; fall back to PATH (karya's
  -- managed bin is on PATH inside a session).
  local cmd = { karya.tool("jdtls") or "jdtls" }
  -- Run jdtls on a Java 21+ JVM (it requires it); fall back to the project JDK
  -- only if no 21+ is installed. The project's own Java version is configured
  -- separately via jdtls runtimes, not by the server's --java-executable.
  local jdk = server_jdk() or M.pick_jdk_for_project(root)
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

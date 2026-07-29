local M = {}

local project = require("util.project")

local function build_tool(root)
  if vim.fn.filereadable(root .. "/pom.xml") == 1 then
    return "maven"
  end
  if
    vim.fn.filereadable(root .. "/build.gradle") == 1
    or vim.fn.filereadable(root .. "/settings.gradle") == 1
    or vim.fn.filereadable(root .. "/settings.gradle.kts") == 1
  then
    return "gradle"
  end
  return nil
end

local commands = {
  maven = {
    compile = { "mvn", "compile" },
    clean = { "mvn", "clean" },
    package = { "mvn", "package" },
    install = { "mvn", "install" },
    test = { "mvn", "test" },
    verify = { "mvn", "verify" },
  },
  gradle = {
    compile = { "gradle", "build" },
    clean = { "gradle", "clean" },
    package = { "gradle", "assemble" },
    install = { "gradle", "publishToMavenLocal" },
    test = { "gradle", "test" },
    verify = { "gradle", "check" },
  },
}

function M.detect(bufnr)
  return project.is_type(bufnr or 0, "java")
end

function M.run(cmd_name, bufnr)
  local root = project.root(bufnr or 0)
  local tool = build_tool(root)
  if not tool then
    vim.notify("No Maven or Gradle project found", vim.log.levels.WARN)
    return nil
  end
  local cmd = commands[tool][cmd_name]
  if not cmd then
    vim.notify("Unsupported project command: " .. cmd_name, vim.log.levels.WARN)
    return nil
  end
  return { cmd = cmd, cwd = root }
end

function M.tool(bufnr)
  return build_tool(project.root(bufnr or 0))
end

return M

local project = require("util.project")
local java_util = require("util.java")
local java_commands = require("languages.lib.java-commands")
local java_project = require("languages.lib.java-project")
local java_testing = require("languages.lib.java-testing")

local java_augroup = vim.api.nvim_create_augroup("JavaDev", { clear = true })

local function jdtls_config(bufnr)
  local root = project.root(bufnr)
  local capabilities = require("util.lsp").with_blink(vim.lsp.protocol.make_client_capabilities())

  return {
    cmd = java_util.jdtls_cmd(root),
    root_dir = root,
    capabilities = capabilities,
    single_file_support = true,
    init_options = {
      bundles = require("languages.lib.java-debug").bundles(),
    },
    settings = {
      java = {
        signatureHelp = { enabled = true },
        contentProvider = { preferred = "fernflower" },
        codeLens = { enabled = true },
        referencesCodeLens = { enabled = true },
        implementationCodeLens = { enabled = true },
        completion = {
          favoriteStaticMembers = {
            "org.junit.Assert.*",
            "org.junit.Assume.*",
            "org.junit.jupiter.api.Assertions.*",
            "org.junit.jupiter.api.Assumptions.*",
            "org.junit.jupiter.api.DynamicContainer.*",
            "org.junit.jupiter.api.DynamicTest.*",
          },
        },
        sources = {
          organizeImports = {
            starThreshold = 9999,
            staticStarThreshold = 9999,
          },
        },
        codeGeneration = {
          toString = {
            template = "${object.className}{${member.name()}=${member.value}, ${otherMembers}}",
          },
        },
        configuration = {
          updateBuildConfiguration = "automatic",
        },
      },
    },
    on_attach = function(client, bufnr)
      if client:supports_method("textDocument/codeLens") then
        vim.lsp.codelens.enable(true, { bufnr = bufnr })
      end
    end,
  }
end

vim.api.nvim_create_autocmd("FileType", {
  group = java_augroup,
  pattern = "java",
  callback = function(args)
    if vim.fn.executable("jdtls") ~= 1 then
      vim.notify("jdtls is not installed. Install it with: brew install jdtls", vim.log.levels.WARN)
      java_commands.register_commands(args.buf)
      java_commands.register_keymaps(args.buf)
      return
    end
    local ok, jdtls = pcall(require, "jdtls")
    if not ok then
      vim.notify("nvim-jdtls is not available", vim.log.levels.ERROR)
      return
    end
    jdtls.start_or_attach(jdtls_config(args.buf))
    java_commands.register_commands(args.buf)
    java_commands.register_keymaps(args.buf)
  end,
})

vim.api.nvim_create_autocmd({ "BufWritePost", "BufEnter" }, {
  group = java_augroup,
  pattern = "*.java",
  callback = function(args)
    if vim.lsp.codelens then
      vim.lsp.codelens.enable(true, { bufnr = args.buf })
    end
  end,
})

vim.api.nvim_create_autocmd("BufWritePre", {
  group = java_augroup,
  pattern = "*.java",
  callback = function()
    java_commands.format_java()
  end,
})

vim.opt.wildignore:append("*/target/*,*/build/*,*/.gradle/*,*/.idea/*")

local java_provider = {
  detect = function(bufnr)
    return java_project.detect(bufnr)
  end,
  build = function(bufnr)
    -- "compile" is the build action in java-project (mvn compile / gradle build);
    -- there is no "build" command key, so run("build") would warn "Unsupported
    -- project command".
    return java_project.run("compile", bufnr)
  end,
  test = function(bufnr)
    return java_project.run("test", bufnr)
  end,
  run_file = function()
    return nil
  end,
  run_project = function(bufnr)
    return java_project.run("package", bufnr)
  end,
  clean = function(bufnr)
    return java_project.run("clean", bufnr)
  end,
}

local testing = require("util.testing")
if testing then
  testing.register("java", java_testing)
end

local ok, tasks = pcall(require, "util.tasks")
if ok and tasks.register_provider then
  tasks.register_provider("java", java_provider)
end

return {}

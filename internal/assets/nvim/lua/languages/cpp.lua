-- C / C++ development environment.
-- Activated only when "cpp" is listed in languages.local.
--
-- External tools (install via install-tools.sh or manually):
--   clangd        — language server (brew install clangd or llvm)
--   clang-format  — formatter      (comes with clangd/llvm)

local lsp_lib = require("util.lsp")

local M = {}

local function executable(name)
  return vim.fn.executable(name) == 1
end

local function project_root(bufnr)
  bufnr = bufnr or 0
  local path = vim.api.nvim_buf_get_name(bufnr)
  if path == "" then
    path = vim.fn.getcwd()
  end
  return vim.fs.root(path, {
    "CMakeLists.txt",
    "compile_commands.json",
    "Makefile",
    "meson.build",
    ".git",
  }) or vim.fs.dirname(path)
end

local function setup_lsp()
  if not executable("clangd") then
    vim.notify("clangd not found. Install with: brew install clangd", vim.log.levels.WARN)
    return
  end

  local capabilities = lsp_lib.with_blink(vim.lsp.protocol.make_client_capabilities())
  capabilities.offsetEncoding = { "utf-16", "utf-8" }

  vim.lsp.config("clangd", {
    capabilities = capabilities,
    filetypes = { "c", "cpp", "objc", "objcpp", "cuda" },
    root_markers = { "compile_commands.json", "CMakeLists.txt", ".clangd", ".git" },
    cmd = {
      "clangd",
      "--background-index",
      "--clang-tidy",
      "--header-insertion=iwyu",
      "--completion-style=detailed",
      "--function-arg-placeholders",
      "--fallback-style=llvm",
    },
    init_options = {
      usePlaceholders = true,
      completeUnimported = true,
      clangdFileStatus = true,
    },
  })
  vim.lsp.enable("clangd")
end

-- Format with clang-format if available
local function format_cpp(bufnr)
  bufnr = bufnr or 0
  if not executable("clang-format") then
    vim.lsp.buf.format({ bufnr = bufnr, async = false })
    return
  end
  vim.lsp.buf.format({
    bufnr = bufnr,
    async = false,
    filter = function(client)
      return client.name == "clangd"
    end,
  })
end

local cpp_augroup = vim.api.nvim_create_augroup("CppDev", { clear = true })

vim.api.nvim_create_autocmd("FileType", {
  group = cpp_augroup,
  pattern = { "c", "cpp", "objc", "objcpp", "cuda" },
  callback = function(args)
    setup_lsp()
    require("util.langmaps").register(args.buf, {
      lang = "Cpp",
      prefix = "<leader>C",
      format = format_cpp,
    })
  end,
})

vim.api.nvim_create_autocmd("BufWritePre", {
  group = cpp_augroup,
  pattern = { "*.c", "*.cpp", "*.cc", "*.cxx", "*.h", "*.hpp", "*.hh", "*.hxx" },
  callback = function(args)
    format_cpp(args.buf)
  end,
})

-- Keep build artifacts out of search
vim.opt.wildignore:append("*/build/*,*/cmake-build-*/*,*/.cache/*")

-- Generic task provider
local cpp_provider = {
  detect = function(bufnr)
    local root = project_root(bufnr)
    return vim.fn.filereadable(root .. "/CMakeLists.txt") == 1
      or vim.fn.filereadable(root .. "/Makefile") == 1
      or vim.fn.filereadable(root .. "/meson.build") == 1
  end,

  build = function()
    local root = project_root()
    if vim.fn.filereadable(root .. "/CMakeLists.txt") == 1 then
      return { cmd = { "cmake", "--build", "build" }, cwd = root }
    end
    return { cmd = { "make" }, cwd = root }
  end,

  test = function()
    return { cmd = { "ctest", "--output-on-failure" }, cwd = project_root() .. "/build" }
  end,

  run_file = function(bufnr)
    local file = vim.api.nvim_buf_get_name(bufnr)
    if file == "" then
      return nil
    end
    local out = vim.fs.basename(file):gsub("%.%w+$", "")
    return { cmd = { out }, cwd = vim.fs.dirname(file) }
  end,

  run_project = function()
    return { cmd = { "./build/" .. "a.out" }, cwd = project_root() }
  end,

  clean = function()
    return { cmd = { "rm", "-rf", "build" }, cwd = project_root() }
  end,
}

local testing = require("util.testing")
if testing then
  testing.register("cpp", require("languages.lib.cpp-testing"))
end

local ok, tasks = pcall(require, "util.tasks")
if ok and tasks.register_provider then
  tasks.register_provider("cpp", cpp_provider)
end

return {}

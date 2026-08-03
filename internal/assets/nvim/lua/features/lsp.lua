return {
  {
    "neovim/nvim-lspconfig",
    ft = { "lua", "json", "yaml", "sh", "zsh", "toml", "markdown" },
    dependencies = { "saghen/blink.cmp" },
    config = function()
      -- Diagnostics
      vim.diagnostic.config({
        virtual_text = {
          severity = { min = vim.diagnostic.severity.WARN },
        },
        underline = {
          severity = { min = vim.diagnostic.severity.ERROR },
        },
        float = {
          border = "rounded",
          source = "if_many",
        },
        severity_sort = true,
        signs = {
          text = {
            [vim.diagnostic.severity.ERROR] = "E",
            [vim.diagnostic.severity.WARN] = "W",
            [vim.diagnostic.severity.INFO] = "I",
            [vim.diagnostic.severity.HINT] = "H",
          },
        },
      })

      -- The LspAttach keymaps (gd, gr, K, <leader>l*, …) are registered globally
      -- in core/autocmds.lua so they bind for every language's server, not only
      -- the ft-gated ones this plugin loads for.

      local capabilities = require("util.lsp").with_blink(vim.lsp.protocol.make_client_capabilities())

      -- Ensure lspconfig server definitions are registered with vim.lsp.config.
      local lspconfig = require("lspconfig")
      _ = lspconfig

      local flags = { debounce_text_changes = 150 }

      vim.lsp.config("lua_ls", {
        capabilities = capabilities,
        flags = flags,
        settings = {
          Lua = {
            runtime = { version = "LuaJIT" },
            diagnostics = { globals = { "vim" } },
            workspace = {
              checkThirdParty = false,
              library = { vim.env.VIMRUNTIME },
            },
            telemetry = { enable = false },
          },
        },
      })
      vim.lsp.enable("lua_ls")

      vim.lsp.config("jsonls", { capabilities = capabilities, flags = flags })
      vim.lsp.enable("jsonls")

      vim.lsp.config("yamlls", { capabilities = capabilities, flags = flags })
      vim.lsp.enable("yamlls")

      vim.lsp.config("bashls", { capabilities = capabilities, flags = flags })
      vim.lsp.enable("bashls")

      vim.lsp.config("taplo", { capabilities = capabilities, flags = flags })
      vim.lsp.enable("taplo")

      vim.lsp.config("marksman", { capabilities = capabilities, flags = flags })
      vim.lsp.enable("marksman")
    end,
  },
}

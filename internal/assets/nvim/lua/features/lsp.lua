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

      -- Keymaps active only when a language server attaches
      vim.api.nvim_create_autocmd("LspAttach", {
        group = vim.api.nvim_create_augroup("UserLspKeymaps", { clear = true }),
        callback = function(args)
          local bufnr = args.buf
          local map = function(keys, fn, modes, desc)
            modes = modes or "n"
            vim.keymap.set(modes, keys, fn, {
              buffer = bufnr,
              silent = true,
              desc = desc,
            })
          end

          map("gd", vim.lsp.buf.definition, "n", "Go to definition")
          map("gD", vim.lsp.buf.declaration, "n", "Go to declaration")
          map("gr", vim.lsp.buf.references, "n", "Find references")
          map("gi", vim.lsp.buf.implementation, "n", "Go to implementation")
          map("gt", vim.lsp.buf.type_definition, "n", "Go to type definition")
          map("K", function()
            vim.lsp.buf.hover({ border = "rounded" })
          end, "n", "Hover documentation")
          map("<C-k>", function()
            vim.lsp.buf.signature_help({ border = "rounded" })
          end, "i", "Signature help")

          map("<leader>lr", vim.lsp.buf.rename, "n", "Rename symbol")
          map("<leader>la", vim.lsp.buf.code_action, { "n", "v" }, "Code action")
          map("<leader>lf", function()
            vim.lsp.buf.format({ async = true })
          end, { "n", "v" }, "Format with LSP")
          map("<leader>ls", vim.lsp.buf.workspace_symbol, "n", "Workspace symbols")
          map("<leader>ld", vim.lsp.buf.document_symbol, "n", "Document symbols")
        end,
      })

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

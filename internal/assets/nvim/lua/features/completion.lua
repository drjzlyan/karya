return {
  {
    "saghen/blink.cmp",
    build = function()
      require("blink.cmp").build():wait()
    end,
    event = { "InsertEnter", "CmdlineEnter" },
    dependencies = {
      "saghen/blink.lib",
      "L3MON4D3/LuaSnip",
      "rafamadriz/friendly-snippets",
    },
    config = function()
      local luasnip = require("luasnip")
      require("luasnip.loaders.from_vscode").lazy_load()

      require("blink.cmp").setup({
        keymap = { preset = "default" },
        completion = {
          documentation = {
            auto_show = true,
            auto_show_delay_ms = 300,
          },
          ghost_text = {
            enabled = true,
          },
        },
        signature = {
          enabled = true,
        },
        sources = {
          default = { "lsp", "buffer", "path", "snippets" },
        },
        snippets = {
          expand = function(snippet)
            luasnip.lsp_expand(snippet)
          end,
          active = function(filter)
            if filter and filter.direction then
              return luasnip.jumpable(filter.direction)
            end
            return luasnip.in_snippet()
          end,
          jump = function(direction)
            luasnip.jump(direction)
          end,
        },
      })
    end,
  },
}

return {
  {
    "saghen/blink.cmp",
    -- No `build` step: building blink's native fuzzy matcher from source needs a
    -- Rust toolchain (cargo), which karya must never assume exists on the user's
    -- machine (DESIGN.md §4). We use the pure-Lua fuzzy matcher instead (see the
    -- `fuzzy` opt below), so completion works fully isolated and offline.
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
        -- Use the Lua fuzzy matcher so blink never needs its Rust-built native
        -- library (and never shells out to cargo/rustup). Slightly slower than the
        -- native matcher but robust in karya's isolated, toolchain-agnostic env.
        fuzzy = {
          implementation = "lua",
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

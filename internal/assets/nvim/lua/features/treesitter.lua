return {
  {
    "nvim-treesitter/nvim-treesitter",
    tag = "v0.9.3",
    build = ":TSUpdate",
    event = { "BufReadPost", "BufNewFile" },
    dependencies = {
      "nvim-treesitter/nvim-treesitter-textobjects",
      "nvim-treesitter/nvim-treesitter-context",
    },
    config = function()
      -- Base parsers always installed (common/config languages + editing)
      local ensure_installed = {
        "lua",
        "vim",
        "vimdoc",
        "bash",
        "markdown",
        "markdown_inline",
        "json",
        "yaml",
        "toml",
        "dockerfile",
        "gitignore",
      }

      -- Extra parsers keyed by language name (matches languages.local entries)
      local lang_parsers = {
        python = { "python", "requirements" },
        java = { "java" },
        typescript = { "typescript", "tsx", "javascript" },
        go = { "go", "gomod", "gosum" },
        cpp = { "cpp", "c" },
        rust = { "rust" },
      }

      -- Add parsers for selected languages
      local ok, langs = pcall(require, "languages")
      if ok and langs.selected then
        for _, lang in ipairs(langs.selected()) do
          if lang_parsers[lang] then
            vim.list_extend(ensure_installed, lang_parsers[lang])
          end
        end
      end

      require("nvim-treesitter.configs").setup({
        ensure_installed = ensure_installed,
        sync_install = false,
        auto_install = false,

        highlight = { enable = true },
        indent = { enable = true },

        incremental_selection = {
          enable = true,
          keymaps = {
            init_selection = "gnn",
            node_incremental = "grn",
            scope_incremental = "grm",
            node_decremental = "grc",
          },
        },

        textobjects = {
          select = {
            enable = true,
            lookahead = true,
            keymaps = {
              ["af"] = "@function.outer",
              ["if"] = "@function.inner",
              ["ac"] = "@class.outer",
              ["ic"] = "@class.inner",
              ["ab"] = "@block.outer",
              ["ib"] = "@block.inner",
              ["ap"] = "@parameter.outer",
              ["ip"] = "@parameter.inner",
            },
          },
          move = {
            enable = true,
            set_jumps = true,
            goto_next_start = {
              ["]f"] = "@function.outer",
              ["]c"] = "@class.outer",
              ["]b"] = "@block.outer",
              ["]p"] = "@parameter.outer",
            },
            goto_next_end = {
              ["]F"] = "@function.outer",
              ["]C"] = "@class.outer",
              ["]B"] = "@block.outer",
              ["]P"] = "@parameter.outer",
            },
            goto_previous_start = {
              ["[f"] = "@function.outer",
              ["[c"] = "@class.outer",
              ["[b"] = "@block.outer",
              ["[p"] = "@parameter.outer",
            },
            goto_previous_end = {
              ["[F"] = "@function.outer",
              ["[C"] = "@class.outer",
              ["[B"] = "@block.outer",
              ["[P"] = "@parameter.outer",
            },
          },
        },
      })

      require("treesitter-context").setup({
        max_lines = 3,
        multiline_threshold = 1,
      })
    end,
  },
}

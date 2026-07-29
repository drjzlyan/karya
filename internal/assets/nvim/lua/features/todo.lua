return {
  {
    "folke/todo-comments.nvim",
    event = { "BufReadPost", "BufNewFile" },
    dependencies = { "nvim-lua/plenary.nvim" },
    config = function()
      require("todo-comments").setup({
        keywords = {
          TODO = { icon = " " },
          FIXME = { icon = " " },
          BUG = { icon = " " },
          NOTE = { icon = " " },
          HACK = { icon = " " },
          WARN = { icon = " " },
          PERF = { icon = " " },
        },
      })

      vim.keymap.set("n", "<leader>st", function()
        require("todo-comments.fzf").todo()
      end, { desc = "Search TODOs" })
    end,
  },
}

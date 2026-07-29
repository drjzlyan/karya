return {
  {
    "stevearc/oil.nvim",
    cmd = "Oil",
    keys = {
      {
        "<leader>E",
        function()
          require("oil").open()
        end,
        desc = "Open oil (file dir)",
      },
      {
        "<leader>O",
        function()
          require("oil").open(vim.fn.getcwd())
        end,
        desc = "Open oil (cwd)",
      },
    },
    opts = {
      default_file_explorer = true,
      skip_confirm_for_simple_edits = true,
      view_options = {
        show_hidden = true,
      },
      keymaps = {
        ["q"] = "actions.close",
        ["<C-s>"] = "actions.select_split",
        ["<C-v>"] = "actions.select_vsplit",
      },
    },
  },
}

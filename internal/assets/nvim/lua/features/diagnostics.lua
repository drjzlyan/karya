return {
  {
    "folke/trouble.nvim",
    cmd = "Trouble",
    keys = {
      {
        "<leader>ee",
        function()
          require("trouble").toggle("diagnostics")
        end,
        desc = "Diagnostics (Trouble)",
      },
      {
        "<leader>er",
        function()
          require("trouble").toggle("lsp_references")
        end,
        desc = "LSP references (Trouble)",
      },
      {
        "<leader>ei",
        function()
          require("trouble").toggle("lsp_implementations")
        end,
        desc = "LSP implementations (Trouble)",
      },
      {
        "<leader>en",
        function()
          require("trouble").next({ skip_groups = true, jump = true })
        end,
        desc = "Next trouble item",
      },
      {
        "<leader>ep",
        function()
          require("trouble").prev({ skip_groups = true, jump = true })
        end,
        desc = "Previous trouble item",
      },
    },
    opts = {
      auto_close = true,
      auto_preview = true,
      focus = true,
    },
  },
}

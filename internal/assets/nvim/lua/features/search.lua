return {
  {
    "ibhagwan/fzf-lua",
    cmd = "FzfLua",
    event = "VeryLazy",
    dependencies = { "nvim-lua/plenary.nvim" },
    config = function()
      local fzf = require("fzf-lua")
      local map = vim.keymap.set

      fzf.setup({
        fzf_opts = {
          ["--layout"] = "reverse-list",
        },
        files = {
          fd_opts = "--color=never --type f --hidden --follow --exclude .git",
        },
        grep = {
          rg_opts = "--column --line-number --no-heading --color=always --smart-case --hidden --follow -g '!.git'",
        },
        winopts = {
          height = 0.85,
          width = 0.80,
          preview = { horizontal = "right:50%" },
        },
      })

      -- Command / buffer switching
      map("n", "<leader>:", fzf.command_history, { desc = "Command history" })
      map("n", "<leader><space>", fzf.buffers, { desc = "Buffers" })

      -- Files (<leader>f)
      map("n", "<leader>ff", fzf.files, { desc = "Find files" })
      map("n", "<leader>fr", fzf.oldfiles, { desc = "Recent files" })

      -- Search (<leader>s)
      map("n", "<leader>s/", fzf.live_grep, { desc = "Live grep" })
      map("n", "<leader>s*", fzf.grep_cword, { desc = "Grep word under cursor" })
    end,
  },
}

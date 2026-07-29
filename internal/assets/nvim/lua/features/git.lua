local function in_git_repo()
  local ok, result = pcall(vim.fn.system, { "git", "rev-parse", "--is-inside-work-tree" })
  if not ok or not result then
    return false
  end
  return vim.trim(result) == "true"
end

return {
  {
    "lewis6991/gitsigns.nvim",
    cond = in_git_repo,
    event = { "BufReadPost", "BufNewFile" },
    keys = {
      { "<leader>gh", "<cmd>Gitsigns preview_hunk<cr>", desc = "Preview hunk" },
      { "<leader>gb", "<cmd>Gitsigns blame_line<cr>", desc = "Line blame" },
      { "<leader>gs", "<cmd>Gitsigns stage_hunk<cr>", desc = "Stage hunk", mode = { "n", "v" } },
      { "<leader>gr", "<cmd>Gitsigns reset_hunk<cr>", desc = "Reset hunk", mode = { "n", "v" } },
      { "<leader>gu", "<cmd>Gitsigns undo_stage_hunk<cr>", desc = "Undo stage hunk" },
      { "<leader>gn", "<cmd>Gitsigns next_hunk<cr>", desc = "Next hunk" },
      { "<leader>gp", "<cmd>Gitsigns prev_hunk<cr>", desc = "Previous hunk" },
      { "<leader>gD", "<cmd>Gitsigns diffthis<cr>", desc = "Diff against index" },
    },
    config = function()
      require("gitsigns").setup({
        current_line_blame = true,
        current_line_blame_opts = {
          delay = 300,
        },
        preview_config = {
          border = "rounded",
        },
      })
    end,
  },
  {
    "sindrets/diffview.nvim",
    cond = in_git_repo,
    cmd = { "DiffviewOpen", "DiffviewClose", "DiffviewFileHistory" },
    keys = {
      {
        "<leader>gd",
        function()
          vim.cmd("DiffviewOpen")
        end,
        desc = "Diffview",
      },
    },
    config = function()
      require("diffview").setup({})
    end,
  },
}

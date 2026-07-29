local augroup = vim.api.nvim_create_augroup
local autocmd = vim.api.nvim_create_autocmd

-- Briefly highlight yanked text
local yank_group = augroup("HighlightYank", { clear = true })
autocmd("TextYankPost", {
  group = yank_group,
  callback = function()
    vim.highlight.on_yank({ higroup = "IncSearch", timeout = 120 })
  end,
})

-- Resize splits when the terminal window changes
local resize_group = augroup("ResizeSplits", { clear = true })
autocmd("VimResized", {
  group = resize_group,
  callback = function()
    vim.cmd("wincmd =")
  end,
})

-- Create undo directory if it doesn't exist
local undo_dir = vim.fn.stdpath("state") .. "/undo"
if vim.fn.isdirectory(undo_dir) == 0 then
  vim.fn.mkdir(undo_dir, "p")
end

-- Bigfile handling: disable expensive features for large files
local bigfile_group = augroup("Bigfile", { clear = true })
vim.api.nvim_create_autocmd({ "BufReadPre", "BufNewFile" }, {
  group = bigfile_group,
  callback = function(args)
    local ok, stats = pcall(vim.uv.fs_stat, args.file)
    if not ok or not stats then
      return
    end
    if stats.size > 1024 * 1024 then
      vim.b[args.buf].bigfile = true
      vim.opt_local.swapfile = false
      vim.opt_local.undofile = false
      vim.opt_local.bufhidden = "wipe"
    end
  end,
})

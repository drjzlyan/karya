local map = vim.keymap.set
local opts = { noremap = true, silent = true }

-- Unbind space in normal/visual so leader works cleanly
map({ "n", "v" }, "<Space>", "<Nop>", opts)

-- Clear search highlight
map("n", "<Esc>", "<cmd>nohlsearch<CR>", opts)

-- Save / quit
map("n", "<leader>S", "<cmd>w!<CR>", { desc = "Save" })
map("n", "<leader>Z", "<cmd>q!<CR>", { desc = "Quit" })

-- Buffers (close on <leader>x; <leader>c is the language-agnostic Code group)
map("n", "<leader>x", "<cmd>bdelete!<CR>", { desc = "Close buffer" })

-- Window navigation
map("n", "<C-h>", "<C-w>h", opts)
map("n", "<C-j>", "<C-w>j", opts)
map("n", "<C-k>", "<C-w>k", opts)
map("n", "<C-l>", "<C-w>l", opts)

-- Window splits
map("n", "<leader>-", "<C-w>s", { desc = "Split below" })
map("n", "<leader>|", "<C-w>v", { desc = "Split right" })
map("n", "<leader>=", "<C-w>=", { desc = "Equalize splits" })

-- Indent in visual mode
map("v", "<", "<gv", opts)
map("v", ">", ">gv", opts)

-- Move lines
map("n", "<A-j>", "<cmd>m .+1<CR>==", opts)
map("n", "<A-k>", "<cmd>m .-2<CR>==", opts)
map("v", "<A-j>", ":m '>+1<CR>gv=gv", opts)
map("v", "<A-k>", ":m '<-2<CR>gv=gv", opts)

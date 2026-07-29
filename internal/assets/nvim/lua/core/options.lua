local opt = vim.opt
local g = vim.g

-- Leader must be set before lazy.nvim loads
vim.g.mapleader = " "
local leader = vim.g.mapleader
vim.g.maplocalleader = " "

vim.g.loaded_perl_provider = 0
vim.g.loaded_ruby_provider = 0
vim.g.loaded_node_provider = 0

-- Line numbers
opt.number = true
opt.relativenumber = true

-- Appearance
opt.termguicolors = true
opt.mouse = "a"
opt.signcolumn = "yes"
opt.scrolloff = 8
opt.sidescrolloff = 8
opt.cursorline = true

-- Indentation
opt.expandtab = true
opt.shiftwidth = 2
opt.tabstop = 2
opt.softtabstop = 2
opt.smartindent = true
opt.wrap = false

-- Search
opt.ignorecase = true
opt.smartcase = true
opt.hlsearch = true

-- Clipboard
opt.clipboard = "unnamedplus"

-- Splits
opt.splitbelow = true
opt.splitright = true

-- Persistent undo
opt.undofile = true
opt.undodir = vim.fn.stdpath("state") .. "/undo"
opt.swapfile = false
opt.backup = false

-- Responsiveness
opt.updatetime = 250
opt.timeoutlen = 300
opt.ttimeoutlen = 0

-- List chars
opt.list = true
opt.listchars = { tab = "» ", trail = "·", nbsp = "␣" }

-- Completion
opt.completeopt = { "menu", "menuone", "noselect" }

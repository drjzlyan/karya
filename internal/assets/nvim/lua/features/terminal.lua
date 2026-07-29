local M = {}

local terminal = nil

local function get_terminal()
  if terminal == nil then
    local Terminal = require("toggleterm.terminal").Terminal
    terminal = Terminal:new({ direction = "float" })
  end
  return terminal
end

-- Inside tmux, shell commands go to the session's "build/test" pane via the
-- ide-run helper so build/test output stays in tmux, not an editor terminal.
local function use_tmux()
  return vim.env.TMUX ~= nil and vim.env.TMUX ~= "" and vim.fn.executable("ide-run") == 1
end

function M.toggle()
  if use_tmux() then
    vim.system({ "ide-run", "--focus" }, { detach = true })
    return
  end
  get_terminal():toggle()
end

function M.send(_, cmd, opts)
  opts = opts or {}
  if use_tmux() then
    local args = { "ide-run" }
    if opts.dir then
      table.insert(args, "-d")
      table.insert(args, opts.dir)
    end
    table.insert(args, cmd)
    vim.system(args, { detach = true }, function(result)
      if result.code ~= 0 then
        vim.schedule(function()
          vim.notify("ide-run failed: " .. (result.stderr or ""), vim.log.levels.ERROR)
        end)
      end
    end)
    return
  end
  local term = get_terminal()
  if not term:is_open() then
    term:open()
  end
  if opts.dir then
    term:change_dir(opts.dir)
  end
  term:send(cmd)
end

return {
  "akinsho/toggleterm.nvim",
  version = "*",
  keys = {
    {
      "<leader>t",
      function()
        M.toggle()
      end,
      desc = "Terminal (tmux pane / float)",
    },
  },
  config = function()
    require("toggleterm").setup({
      open_mapping = nil,
      direction = "float",
      close_on_exit = true,
      shell = vim.o.shell,
      float_opts = {
        border = "curved",
        width = function()
          return math.floor(vim.o.columns * 0.85)
        end,
        height = function()
          return math.floor(vim.o.lines * 0.85)
        end,
      },
    })
  end,
  toggle = M.toggle,
  send = M.send,
}

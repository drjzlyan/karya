-- The in-editor karya tutorial engine.
--
-- It drives the steps in tutorial.steps against a live IDE session. Two detection
-- strategies let the learner perform the *real* operation and auto-advance:
--
--   * nvim keystroke steps are observed with vim.on_key, which is non-intercepting
--     — the actual karya keymap still fires — and matched against the step's
--     expected byte sequence (leader-substituted). So the user really runs the
--     command, and the tutorial notices.
--   * tmux / lazygit / agent steps can't be seen as keystrokes (the Ctrl-a prefix
--     is consumed by tmux), so the engine polls the karya tmux server on a timer
--     and advances when the real effect appears.
--
-- All state lives in a single module-local `state`; the tutorial is a singleton.

local M = {}

local steps_mod = require("tutorial.steps")
local ns = vim.api.nvim_create_namespace("karya_tutorial")

local state = nil
local activate -- forward declaration (referenced by the panel maps and advance)

-- expected_bytes turns a step's key spec (e.g. "<leader>ff") into the raw bytes
-- vim.on_key reports, substituting <leader> with the configured leader (space).
local function expected_bytes(lhs)
  local leader = vim.g.mapleader
  if leader == nil or leader == "" then
    leader = " "
  end
  lhs = lhs:gsub("<[lL]eader>", leader)
  return vim.api.nvim_replace_termcodes(lhs, true, true, true)
end

-- ── availability gates ──────────────────────────────────────────────────────

local function tmux_ok()
  return vim.env.TMUX ~= nil
end

local function agent_count()
  local a = steps_mod.tmux_display("#{@ide_agents}")
  if not a or a == "" then
    return 0
  end
  return #vim.split(a, "%s+", { trimempty = true })
end

local function agent_ok()
  if not tmux_ok() then
    return false
  end
  local cur = steps_mod.tmux_display("#{@ide_current_agent}")
  return cur ~= nil and cur ~= "" and cur ~= "none"
end

-- gate returns a skip reason string when a step's requirement is unmet, else nil.
local function gate(step)
  if step.needs_tmux and not tmux_ok() then
    return "not in a tmux session"
  end
  if step.needs_lazygit and vim.fn.executable("lazygit") ~= 1 then
    return "lazygit not installed"
  end
  if step.needs_multi_agent and agent_count() < 2 then
    return "needs 2+ agents"
  end
  if step.needs_agent and not agent_ok() then
    return "no coding agent in this session"
  end
  return nil
end

-- ── the panel ───────────────────────────────────────────────────────────────

local function ensure_buf()
  if state.buf and vim.api.nvim_buf_is_valid(state.buf) then
    return
  end
  state.buf = vim.api.nvim_create_buf(false, true)
  vim.bo[state.buf].bufhidden = "wipe"
  vim.bo[state.buf].filetype = "markdown"
  local function mapq(lhs, fn)
    vim.keymap.set("n", lhs, fn, { buffer = state.buf, silent = true, nowait = true })
  end
  mapq("q", M.quit)
  mapq("s", M.skip)
  mapq("n", M.advance)
  mapq("r", function()
    if state then
      activate(state.idx)
    end
  end)
end

local function ensure_win()
  ensure_buf()
  if state.win and vim.api.nvim_win_is_valid(state.win) then
    return
  end
  local width = math.min(56, math.floor(vim.o.columns * 0.5))
  local height = math.max(10, vim.o.lines - 4)
  state.win = vim.api.nvim_open_win(state.buf, false, {
    relative = "editor",
    width = width,
    height = height,
    row = 1,
    col = vim.o.columns - width - 1,
    style = "minimal",
    border = "rounded",
    title = " karya tutorial ",
    title_pos = "center",
  })
  vim.wo[state.win].wrap = true
  vim.wo[state.win].winblend = 0
end

local function render()
  ensure_win()
  local step = state.steps[state.idx]
  local total = #state.steps
  local lines = {
    string.format("karya IDE tutorial            [%d/%d]", state.idx, total),
    string.rep("─", 40),
    "▶ " .. step.title,
    "",
  }
  for _, l in ipairs(step.body or {}) do
    table.insert(lines, l)
  end
  table.insert(lines, "")
  table.insert(lines, "→ " .. (step.action or ""))
  table.insert(lines, "")
  table.insert(lines, string.rep("─", 40))
  table.insert(lines, "stuck? <C-w>w here, then s=skip n=next q=quit")
  if #state.log > 0 then
    table.insert(lines, "")
    -- Show the most recent completed steps (newest last).
    local from = math.max(1, #state.log - 8)
    for i = from, #state.log do
      table.insert(lines, state.log[i])
    end
  end

  vim.bo[state.buf].modifiable = true
  vim.api.nvim_buf_set_lines(state.buf, 0, -1, false, lines)
  vim.bo[state.buf].modifiable = false
end

-- ── flow ────────────────────────────────────────────────────────────────────

-- activate moves to step idx, skipping any gated steps that can't run here.
activate = function(idx)
  while idx <= #state.steps do
    local step = state.steps[idx]
    local reason = gate(step)
    if reason then
      table.insert(state.log, "• " .. step.title .. " (skipped: " .. reason .. ")")
      idx = idx + 1
    else
      state.idx = idx
      state.acc = ""
      step._phase = nil
      step._start = nil
      step._expected = nil
      if step.keys then
        step._expected = expected_bytes(step.keys)
      elseif step.confirm then
        step._expected = expected_bytes("<CR>")
      end
      step._poll_active = step.poll ~= nil
      if step.setup then
        pcall(step.setup, state.ctx)
      end
      render()
      return
    end
  end
  M.finish()
end

function M.advance()
  if not state then
    return
  end
  local step = state.steps[state.idx]
  if step then
    table.insert(state.log, "✓ " .. step.title)
  end
  activate(state.idx + 1)
end

function M.skip()
  if not state then
    return
  end
  local step = state.steps[state.idx]
  if step then
    table.insert(state.log, "• " .. step.title .. " (skipped)")
  end
  activate(state.idx + 1)
end

local function teardown()
  if not state then
    return
  end
  pcall(vim.on_key, nil, ns)
  if state.timer then
    pcall(function()
      state.timer:stop()
      state.timer:close()
    end)
  end
  if state.win and vim.api.nvim_win_is_valid(state.win) then
    pcall(vim.api.nvim_win_close, state.win, true)
  end
  state = nil
end

function M.finish()
  teardown()
  vim.notify("karya tutorial complete — you've run the whole developer loop.", vim.log.levels.INFO)
end

function M.quit()
  teardown()
  vim.notify("karya tutorial stopped.", vim.log.levels.INFO)
end

-- on_key observes typed bytes for keystroke steps. It never consumes input, so
-- the real keymap still runs; we only detect the expected suffix and advance.
local function on_key(_, typed)
  if not state then
    return
  end
  local key = (typed ~= nil and typed ~= "") and typed or _
  if not key or key == "" then
    return
  end
  local step = state.steps[state.idx]
  local exp = step and step._expected
  if not exp or exp == "" then
    return
  end
  state.acc = (state.acc or "") .. key
  if #state.acc > 64 then
    state.acc = state.acc:sub(-64)
  end
  if #state.acc >= #exp and state.acc:sub(-#exp) == exp then
    state.acc = ""
    step._expected = nil -- guard against a duplicate match before we advance
    vim.schedule(M.advance)
  end
end

local function begin(lang)
  local sample = steps_mod.scaffold(lang)
  vim.cmd.cd(vim.fn.fnameescape(sample.dir))
  vim.cmd.edit(vim.fn.fnameescape(sample.file))

  local ctx = { lang = lang, sample = sample }
  state = {
    steps = steps_mod.build(ctx),
    idx = 0,
    ctx = ctx,
    log = {},
    acc = "",
    timer = vim.uv.new_timer(),
  }

  vim.on_key(on_key, ns)

  -- Poll tmux-effect steps on a timer, scheduled onto the main loop so the poll
  -- functions may use the vim API safely.
  state.timer:start(400, 400, function()
    vim.schedule(function()
      if not state then
        return
      end
      local step = state.steps[state.idx]
      if step and step.poll and step._poll_active then
        local ok, done = pcall(step.poll, step, state.ctx)
        if ok and done then
          step._poll_active = false
          M.advance()
        end
      end
    end)
  end)

  activate(1)
end

function M.start()
  if state then
    vim.notify("karya tutorial is already running (:KaryaTutorialQuit to stop).", vim.log.levels.WARN)
    return
  end
  local langs = steps_mod.languages()
  vim.ui.select(langs, { prompt = "Pick a language for the karya tutorial:" }, function(choice)
    if not choice then
      return
    end
    begin(choice)
  end)
end

return M

local M = {}

-- bin resolves the absolute path to the running karya binary. Inside a karya
-- session, EDITOR/VISUAL/GIT_EDITOR are set to "<abs path to karya> edit", so we
-- recover the path from there and fall back to $PATH otherwise.
function M.bin()
  for _, var in ipairs({ vim.env.EDITOR, vim.env.VISUAL, vim.env.GIT_EDITOR }) do
    if var then
      local bin = var:match("^(.-)%s+edit$")
      if bin and bin ~= "" then
        return bin
      end
    end
  end
  return "karya"
end

return M

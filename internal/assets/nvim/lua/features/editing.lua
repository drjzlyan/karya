return {
  {
    "echasnovski/mini.pairs",
    event = "VeryLazy",
    config = function()
      require("mini.pairs").setup({
        modes = { insert = true, command = false, terminal = false },
        mappings = {
          ["<"] = { action = "open", pair = "<>", neigh_pattern = "[^\\]." },
        },
      })
    end,
  },
  {
    "echasnovski/mini.surround",
    event = "VeryLazy",
    config = function()
      -- Default mappings:
      -- sa add, sd delete, sr replace, sf/sF find, sh highlight
      require("mini.surround").setup({})
    end,
  },
}

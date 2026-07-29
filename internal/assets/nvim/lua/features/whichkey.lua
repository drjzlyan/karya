return {
  {
    "folke/which-key.nvim",
    event = "VeryLazy",
    opts = {
      preset = "helix",
      delay = 0,
      icons = {
        mappings = false,
      },
      spec = {
        { "<leader>e", group = "Diagnostics/Errors" },
        { "<leader>E", desc = "Open oil (file dir)" },
        { "<leader>O", desc = "Open oil (cwd)" },
        { "<leader>f", group = "Files" },
        { "<leader>s", group = "Search" },
        { "<leader>g", group = "Git" },
        { "<leader>d", group = "Debug" },
        { "<leader>l", group = "LSP" },
        { "<leader>t", desc = "Terminal (tmux pane / float)" },
        { "<leader>m", group = "Make/Tasks" },
        { "<leader>q", group = "Session" },
        { "<leader>j", group = "Java" },
        { "<leader>p", group = "Python" },
        { "<leader>o", group = "Go" },
        { "<leader>r", group = "Rust" },
        { "<leader>C", group = "C/C++" },
        { "<leader>y", group = "TypeScript" },
        { "<leader>T", group = "Testing" },
        { "<leader>W", group = "Workspace" },
        { "<leader>S", desc = "Save" },
        { "<leader>Z", desc = "Quit" },
        { "<leader>c", desc = "Close buffer" },
        { "<leader>:", desc = "Command history" },
        { "<leader><space>", desc = "Buffers" },
        { "<leader>st", desc = "Search TODOs" },
        {
          "<leader>mb",
          function()
            require("util.tasks").build()
          end,
          desc = "Run build task",
        },
        {
          "<leader>ms",
          function()
            require("util.tasks").test()
          end,
          desc = "Run test task",
        },
        {
          "<leader>mc",
          function()
            require("util.tasks").clean()
          end,
          desc = "Run clean task",
        },
        {
          "<leader>mp",
          function()
            require("util.tasks").run_project()
          end,
          desc = "Run project task",
        },
        { "<leader>ee", desc = "Diagnostics (Trouble)" },
        { "<leader>er", desc = "LSP references (Trouble)" },
        { "<leader>ei", desc = "LSP implementations (Trouble)" },
        { "<leader>en", desc = "Next trouble item" },
        { "<leader>ep", desc = "Previous trouble item" },
        { "<leader>gd", desc = "Diffview" },
        { "<leader>gh", desc = "Preview hunk" },
        { "<leader>gb", desc = "Line blame" },
        { "<leader>gs", desc = "Stage hunk", mode = { "n", "v" } },
        { "<leader>gr", desc = "Reset hunk", mode = { "n", "v" } },
        { "<leader>gu", desc = "Undo stage hunk" },
        { "<leader>gn", desc = "Next hunk" },
        { "<leader>gp", desc = "Previous hunk" },
        { "<leader>gD", desc = "Diff against index" },
        { "<leader>db", desc = "Toggle breakpoint" },
        { "<leader>dB", desc = "Conditional breakpoint" },
        { "<leader>dc", desc = "Continue" },
        { "<leader>di", desc = "Step into" },
        { "<leader>do", desc = "Step over" },
        { "<leader>dO", desc = "Step out" },
        { "<leader>dr", desc = "Open REPL" },
        { "<leader>du", desc = "Toggle UI" },
        { "<leader>dt", desc = "Terminate" },
        { "<leader>dx", desc = "Clear breakpoints" },
        {
          "<leader>Tt",
          function()
            require("util.testing").run_nearest()
          end,
          desc = "Run nearest test",
        },
        {
          "<leader>Tc",
          function()
            require("util.testing").run_current_class()
          end,
          desc = "Run test class",
        },
        {
          "<leader>Tp",
          function()
            require("util.testing").run_package()
          end,
          desc = "Run package tests",
        },
        {
          "<leader>Tm",
          function()
            require("util.testing").run_module()
          end,
          desc = "Run module tests",
        },
        {
          "<leader>Tl",
          function()
            require("util.testing").rerun_last()
          end,
          desc = "Re-run last test",
        },
        {
          "<leader>Td",
          function()
            require("util.testing").debug_nearest()
          end,
          desc = "Debug nearest test",
        },
        {
          "<leader>TD",
          function()
            require("util.testing").debug_current_class()
          end,
          desc = "Debug test class",
        },
        {
          "<leader>Wb",
          function()
            require("languages.lib.java-commands").build_workspace()
          end,
          desc = "Build workspace",
        },
        {
          "<leader>Wr",
          function()
            require("languages.lib.java-commands").reload_workspace()
          end,
          desc = "Reload workspace",
        },
        {
          "<leader>Ww",
          function()
            require("languages.lib.java-commands").restart_jdtls()
          end,
          desc = "Restart jdtls",
        },
        {
          "<leader>Wc",
          function()
            require("languages.lib.java-commands").clear_workspace_cache()
          end,
          desc = "Clear workspace cache",
        },
        {
          "<leader>Wl",
          function()
            require("languages.lib.java-commands").open_workspace_logs()
          end,
          desc = "Open workspace logs",
        },
        { "<leader>jf", desc = "Format Java", mode = { "n", "v" } },
        { "<leader>ji", desc = "Organize imports" },
        { "<leader>jr", desc = "Refactor", mode = { "n", "v" } },
        { "<leader>jc", desc = "Compile project" },
        { "<leader>jp", desc = "Package project" },
        { "<leader>jv", desc = "Verify project" },
        { "<leader>jt", desc = "Run nearest test" },
        { "<leader>jT", desc = "Run test class" },
        { "<leader>jd", desc = "Debug nearest test" },
        { "<leader>jD", desc = "Debug test class" },
        { "<leader>jh", desc = "Call / type hierarchy" },
        { "<leader>jl", desc = "Workspace logs" },
        { "<leader>jw", desc = "Restart workspace" },
        { "gd", desc = "Definition" },
        { "gD", desc = "Declaration" },
        { "gr", desc = "References" },
        { "gi", desc = "Implementation" },
        { "gt", desc = "Type definition" },
        { "K", desc = "Hover documentation" },
        { "<C-k>", desc = "Signature help", mode = "i" },
        { "<leader>lr", desc = "Rename symbol" },
        { "<leader>la", desc = "Code action", mode = { "n", "v" } },
        { "<leader>lf", desc = "Format with LSP", mode = { "n", "v" } },
        { "<leader>ls", desc = "Workspace symbols" },
        { "<leader>ld", desc = "Document symbols" },
      },
    },
  },
}

return {
  {
    "mfussenegger/nvim-dap",
    dependencies = {
      "rcarriga/nvim-dap-ui",
      "nvim-neotest/nvim-nio",
      "mfussenegger/nvim-jdtls",
    },
    cmd = {
      "DapContinue",
      "DapToggleBreakpoint",
      "DapTerminate",
      "DapStepOver",
      "DapStepInto",
      "DapStepOut",
      "DapClearBreakpoints",
      "DapReplOpen",
    },
    keys = {
      {
        "<leader>db",
        function()
          require("dap").toggle_breakpoint()
        end,
        desc = "Toggle breakpoint",
      },
      {
        "<leader>dB",
        function()
          require("dap").set_breakpoint(vim.fn.input("Condition: "))
        end,
        desc = "Conditional breakpoint",
      },
      {
        "<leader>dc",
        function()
          require("dap").continue()
        end,
        desc = "Continue",
      },
      {
        "<leader>di",
        function()
          require("dap").step_into()
        end,
        desc = "Step into",
      },
      {
        "<leader>do",
        function()
          require("dap").step_over()
        end,
        desc = "Step over",
      },
      {
        "<leader>dO",
        function()
          require("dap").step_out()
        end,
        desc = "Step out",
      },
      {
        "<leader>dr",
        function()
          require("dap").repl.open()
        end,
        desc = "Open REPL",
      },
      {
        "<leader>du",
        function()
          require("dapui").toggle()
        end,
        desc = "Toggle UI",
      },
      {
        "<leader>dt",
        function()
          require("dap").terminate()
        end,
        desc = "Terminate",
      },
      {
        "<leader>dx",
        function()
          require("dap").clear_breakpoints()
        end,
        desc = "Clear breakpoints",
      },
    },
    config = function()
      local dap = require("dap")
      local dapui = require("dapui")

      vim.fn.sign_define("DapBreakpoint", { text = "B", texthl = "DapBreakpoint" })
      vim.fn.sign_define("DapBreakpointCondition", { text = "C", texthl = "DapBreakpointCondition" })
      vim.fn.sign_define("DapLogPoint", { text = "L", texthl = "DapLogPoint" })
      vim.fn.sign_define("DapStopped", { text = "→", texthl = "DapStopped", linehl = "DapStopped" })

      dapui.setup()

      dap.listeners.after.event_initialized["dapui_config"] = function()
        dapui.open()
      end
      dap.listeners.before.event_terminated["dapui_config"] = function()
        dapui.close()
      end
      dap.listeners.before.event_exited["dapui_config"] = function()
        dapui.close()
      end

      local function setup_adapters()
        local ft = vim.bo.filetype
        local ok, adapter
        if ft == "python" then
          ok, adapter = pcall(require, "languages.lib.python-debug")
        elseif ft == "java" then
          ok, adapter = pcall(require, "languages.lib.java-debug")
        elseif ft == "go" then
          ok, adapter = pcall(require, "languages.lib.go-debug")
        end
        if ok and adapter and adapter.setup then
          adapter.setup()
        end
      end

      setup_adapters()

      local augroup = vim.api.nvim_create_augroup("DapLanguageAdapters", { clear = true })
      vim.api.nvim_create_autocmd("FileType", {
        group = augroup,
        pattern = "python",
        callback = function()
          local ok, adapter = pcall(require, "languages.lib.python-debug")
          if ok and adapter and adapter.setup then
            adapter.setup()
          end
        end,
      })
      vim.api.nvim_create_autocmd("FileType", {
        group = augroup,
        pattern = "java",
        callback = function()
          local ok, adapter = pcall(require, "languages.lib.java-debug")
          if ok and adapter and adapter.setup then
            adapter.setup()
          end
        end,
      })
      vim.api.nvim_create_autocmd("FileType", {
        group = augroup,
        pattern = "go",
        callback = function()
          local ok, adapter = pcall(require, "languages.lib.go-debug")
          if ok and adapter and adapter.setup then
            adapter.setup()
          end
        end,
      })
    end,
  },
}

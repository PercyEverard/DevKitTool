return {
  "nvimdev/dashboard-nvim",
  event = "VimEnter",
  dependencies = { "nvim-tree/nvim-web-devicons", "nvim-telescope/telescope.nvim" },
  config = function()
    require("dashboard").setup({
      theme = "hyper",
      config = {
        header = {
          " ███╗   ██╗███████╗ ██████╗ ██╗   ██╗██╗███╗   ███╗",
          " ████╗  ██║██╔════╝██╔═══██╗██║   ██║██║████╗ ████║",
          " ██╔██╗ ██║█████╗  ██║   ██║██║   ██║██║██╔████╔██║",
          " ██║╚██╗██║██╔══╝  ██║   ██║╚██╗ ██╔╝██║██║╚██╔╝██║",
          " ██║ ╚████║███████╗╚██████╔╝ ╚████╔╝ ██║██║ ╚═╝ ██║",
          " ╚═╝  ╚═══╝╚══════╝ ╚═════╝   ╚═══╝  ╚═╝╚═╝     ╚═╝",
        },
        shortcut = {
          { desc = "󰚰 Update", group = "DiagnosticHint", action = "Lazy update", key = "u" },
          { desc = " Files", group = "Label", action = "Telescope find_files", key = "f" },
          { desc = " Search", group = "Label", action = "Telescope live_grep", key = "g" },
          { desc = "󰈚 Recent", group = "Label", action = "Telescope oldfiles", key = "r" },
        },
      },
    })
  end,
}

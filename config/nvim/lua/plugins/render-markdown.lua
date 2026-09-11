return {
  "MeanderingProgrammer/render-markdown.nvim",
  dependencies = {
    "nvim-treesitter/nvim-treesitter",
    "nvim-tree/nvim-web-devicons",
  },
  ft = { "markdown", "mdx" },
  opts = {
    enabled = true,
    heading = {
      enabled = true,
      sign = true,
      position = "overlay",
      icons = { "① ", "② ", "③ ", "④ ", "⑤ ", "⑥ " },
    },
    code = {
      enabled = true,
      sign = true,
      style = "full",
    },
    bullet = {
      enabled = true,
      icons = { "●", "○", "◆", "◇" },
    },
    checkbox = {
      enabled = true,
      unchecked = { icon = "󰄱 " },
      checked = { icon = "󰱒 " },
    },
  },
}

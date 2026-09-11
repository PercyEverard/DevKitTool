return {
  "nvim-treesitter/nvim-treesitter",
  tag = "v0.9.3",
  build = ":TSUpdate",
  config = function()
    require("nvim-treesitter.configs").setup({
      ensure_installed = {
        -- 用户常用语言
        "c", "cpp", "python", "bash", "javascript", "css", "html", "lua", "go", "asm",
        -- 其他常用
        "rust", "typescript", "json", "yaml", "toml", "markdown",
        -- Neovim 自身
        "vim", "vimdoc",
      },
      highlight = { enable = true },
      indent = { enable = true },
      incremental_selection = {
        enable = true,
        keymaps = {
          init_selection = "<CR>",
          node_incremental = "<CR>",
          node_decremental = "<BS>",
          scope_incremental = "<TAB>",
        },
      },
    })
  end,
}

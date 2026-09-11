return {
  "stevearc/conform.nvim",
  dependencies = { "williamboman/mason.nvim" },
  config = function()
    require("conform").setup({
      formatters_by_ft = {
        lua = { "stylua" },
        python = { "black" },
        javascript = { "prettier" },
        typescript = { "prettier" },
        html = { "prettier" },
        css = { "prettier" },
        json = { "prettier" },
        yaml = { "prettier" },
        markdown = { "prettier" },
        c = { "clang-format" },
        cpp = { "clang-format" },
        go = { "gofmt" },
        rust = { "rustfmt" },
        sh = { "shfmt" },
        bash = { "shfmt" },
      },
      -- 刻意不开启保存时自动格式化：会重排整个文件、让 git diff 混入非本意改动；格式化改用 <leader>fm 手动触发
    })
  end,
}

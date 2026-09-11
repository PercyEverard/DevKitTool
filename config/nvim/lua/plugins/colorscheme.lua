return {
	{
		"folke/tokyonight.nvim",
		lazy = false,
		priority = 1000,
		config = function()
			require("tokyonight").setup({
				transparent = false,
				styles = {
					sidebars = "dark",
					floats = "dark",
				},
			})
			vim.cmd("colorscheme tokyonight")
		end,
	},
	{
		-- 主题切换器：:ThemeHub 打开选择主题，自动记住上次选择
		"erl-koenig/theme-hub.nvim",
		dependencies = { "nvim-lua/plenary.nvim" },
		config = function()
			require("theme-hub").setup({
				persistent = true,
			})
		end,
	},
}

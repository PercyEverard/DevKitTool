-- ========== Neovim 配置入口 ==========
-- Leader 键：空格（必须在插件加载之前设置）
vim.g.mapleader = " "
vim.g.maplocalleader = " "

-- 引导 lazy.nvim 插件管理器
local lazypath = vim.fn.stdpath("data") .. "/lazy/lazy.nvim"
local uv = vim.uv or vim.loop
if not uv.fs_stat(lazypath) then
	vim.fn.system({
		"git",
		"clone",
		"--filter=blob:none",
		"https://github.com/folke/lazy.nvim.git",
		"--branch=stable",
		lazypath,
	})
end
vim.opt.rtp:prepend(lazypath)

-- 加载核心配置
require("config.options")
require("config.keymaps")
require("config.autocmds")

-- 加载插件
require("lazy").setup("plugins")

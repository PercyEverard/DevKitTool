-- 基础选项
local opt = vim.opt

opt.number = true
opt.relativenumber = true
opt.cursorline = true
opt.termguicolors = true
opt.signcolumn = "yes"
opt.wrap = false
opt.scrolloff = 8
opt.sidescrolloff = 8

-- 缩进
opt.tabstop = 2
opt.shiftwidth = 2
opt.expandtab = true
opt.smartindent = true

-- 搜索
opt.ignorecase = true
opt.smartcase = true
opt.hlsearch = true
opt.incsearch = true

-- 文件与剪贴板
opt.encoding = "utf-8"
opt.fileencoding = "utf-8"
opt.undofile = true
opt.swapfile = false
opt.backup = false
opt.clipboard = "unnamedplus" -- 与系统剪贴板打通
opt.fixendofline = false -- 保存时不自动补文件末尾换行（否则 git diff 会出现非本意改动）

-- 状态栏
opt.laststatus = 3
opt.showmode = false

-- 补全行为
opt.completeopt = { "menu", "menuone", "noselect" }
opt.shortmess:append("c")

-- 分屏更自然
opt.splitright = true
opt.splitbelow = true

-- 响应速度（LSP 诊断/补全更快）
opt.updatetime = 250
opt.timeoutlen = 400

local map = vim.keymap.set
local opts = { noremap = true, silent = true }

-- ========== VS Code 风格快捷键 ==========

-- 保存 / 退出
map("n", "<C-s>", ":w<CR>", opts)
map("i", "<C-s>", "<Esc>:w<CR>a", opts)
map("n", "<C-q>", ":q<CR>", opts)

-- 撤销 / 重做
map("n", "<C-z>", "u", opts)
map("i", "<C-z>", "<C-o>u", opts)
map("n", "<C-S-z>", "<C-r>", opts)

-- 复制 / 粘贴 / 剪切 / 全选
map("v", "<C-c>", '"+y', opts)
map("n", "<C-v>", '"+p', opts)
map("i", "<C-v>", "<C-r>+", opts)
map("v", "<C-x>", '"+d', opts)
map("n", "<C-a>", "ggVG", opts)

-- 查找 / 替换
map("n", "<C-f>", "/", opts)
map("n", "<Esc>", ":nohlsearch<CR>", opts)

-- 侧边栏文件树
map("n", "<C-b>", ":NvimTreeToggle<CR>", opts)

-- 终端
map("n", "<C-`>", ":split | terminal<CR>", opts)
map("t", "<Esc>", "<C-\\><C-n>", opts)

-- 行移动（Alt + 上下）
map("n", "<A-Up>", ":m .-2<CR>==", opts)
map("n", "<A-Down>", ":m .+1<CR>==", opts)
map("v", "<A-Up>", ":m '<-2<CR>gv=gv", opts)
map("v", "<A-Down>", ":m '>+1<CR>gv=gv", opts)

-- 注释（Comment.nvim 自带 gc/gcc，这里加一组 VS Code 风格）
map("n", "<C-/>", "gcc", { remap = true, silent = true })
map("v", "<C-/>", "gc", { remap = true, silent = true })

-- 缓冲区切换
map("n", "<S-l>", ":bnext<CR>", opts)
map("n", "<S-h>", ":bprevious<CR>", opts)
map("n", "<leader>bd", ":bdelete<CR>", { desc = "关闭当前缓冲区" })

-- ========== Leader（空格）快捷键 ==========

-- Telescope 模糊查找 / 项目搜索
map("n", "<leader>ff", ":Telescope find_files<CR>", { desc = "查找文件（项目内）" })
map("n", "<leader>fg", ":Telescope live_grep<CR>", { desc = "全文搜索（项目内）" })
map("n", "<leader>fw", ":Telescope grep_string<CR>", { desc = "搜索光标下的单词" })
map("n", "<leader>/", ":Telescope current_buffer_fuzzy_find<CR>", { desc = "当前文件内模糊搜索" })
map("n", "<leader>fb", ":Telescope buffers<CR>", { desc = "已打开的缓冲区" })
map("n", "<leader>fr", ":Telescope oldfiles<CR>", { desc = "最近打开的文件" })
map("n", "<leader>fd", ":Telescope diagnostics<CR>", { desc = "项目内所有诊断/错误" })
map("n", "<leader>fs", ":Telescope lsp_document_symbols<CR>", { desc = "当前文件的符号大纲" })
map("n", "<leader>fS", ":Telescope lsp_workspace_symbols<CR>", { desc = "项目内搜索符号" })
map("n", "<leader>gf", ":Telescope git_files<CR>", { desc = "Git 文件列表" })
map("n", "<leader>gS", ":Telescope git_status<CR>", { desc = "Git 修改状态" })
map("n", "<leader>fR", ":Telescope resume<CR>", { desc = "继续上次搜索" })
map("n", "<leader>fh", ":Telescope help_tags<CR>", { desc = "帮助文档" })
map("n", "<leader>fk", ":Telescope keymaps<CR>", { desc = "查看所有快捷键" })

-- 格式化（LSP 快捷键在打开支持的语言文件后自动生效，见 lua/plugins/lsp.lua）
map("n", "<leader>fm", function()
  require("conform").format({ async = true })
end, { desc = "格式化当前文件" })

-- Quickfix 列表（查找引用 / 调用关系的结果都在这里）
map("n", "]q", ":cnext<CR>", { desc = "下一个 quickfix 项" })
map("n", "[q", ":cprev<CR>", { desc = "上一个 quickfix 项" })
map("n", "<leader>qo", ":copen<CR>", { desc = "打开 quickfix 列表" })
map("n", "<leader>qc", ":cclose<CR>", { desc = "关闭 quickfix 列表" })

-- 窗口导航
map("n", "<C-h>", "<C-w>h", opts)
map("n", "<C-j>", "<C-w>j", opts)
map("n", "<C-k>", "<C-w>k", opts)
map("n", "<C-l>", "<C-w>l", opts)

-- 分屏
map("n", "<leader>sv", ":vsplit<CR>", opts)
map("n", "<leader>sh", ":split<CR>", opts)

-- 调整分屏大小
map("n", "<C-Left>", ":vertical resize -2<CR>", opts)
map("n", "<C-Right>", ":vertical resize +2<CR>", opts)
map("n", "<C-Up>", ":resize +2<CR>", opts)
map("n", "<C-Down>", ":resize -2<CR>", opts)

-- Markdown
map("n", "<leader>mp", ":MarkdownPreviewToggle<CR>", { desc = "Markdown 预览" })
map("n", "<leader>mr", function()
  require("render-markdown").toggle()
end, { desc = "切换 Markdown 渲染" })

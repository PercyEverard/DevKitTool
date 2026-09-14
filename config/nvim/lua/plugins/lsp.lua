return {
	{
		"williamboman/mason.nvim",
		config = function()
			require("mason").setup()
		end,
	},
	{
		"williamboman/mason-lspconfig.nvim",
		dependencies = { "williamboman/mason.nvim", "neovim/nvim-lspconfig" },
		config = function()
			require("mason-lspconfig").setup({
				-- 自动安装的 LSP 服务器（都可在 :Mason 里管理/更新）
				ensure_installed = {
					"clangd",   -- C / C++
					"pyright",  -- Python
					"ts_ls",    -- JavaScript / TypeScript
					"html",     -- HTML
					"cssls",    -- CSS
					"jsonls",   -- JSON
					"gopls",    -- Go
					"bashls",   -- Bash / Shell
					"lua_ls",   -- Lua
					"asm_lsp",  -- 汇编（x86 / x86-64 / ARM / RISC-V）
				},
				-- Mason 里新装的服务器自动启用，装完即用
				--（stylua 只是格式化器，不是 LSP，排除掉免得它抢注 Lua 文件）
				automatic_enable = {
					exclude = { "stylua" },
				},
			})
		end,
	},
	{
		"neovim/nvim-lspconfig",
		dependencies = { "hrsh7th/cmp-nvim-lsp", "williamboman/mason-lspconfig.nvim" },
		config = function()
			-- ===== 所有服务器共用的补全能力（来自 nvim-cmp）=====
			local capabilities = require("cmp_nvim_lsp").default_capabilities()
			vim.lsp.config("*", { capabilities = capabilities })

			-- ===== 各服务器专属设置 =====

			-- Lua：识别 vim 全局变量和 Neovim 运行时库，避免误报
			vim.lsp.config("lua_ls", {
				settings = {
					Lua = {
						runtime = { version = "LuaJIT" },
						workspace = {
							checkThirdParty = false,
							library = vim.api.nvim_get_runtime_file("", true),
						},
						diagnostics = { globals = { "vim" } },
						telemetry = { enable = false },
					},
				},
			})

			-- C/C++：后台建索引（跨文件跳转更快），开启 clang-tidy 静态检查
			vim.lsp.config("clangd", {
				cmd = {
					"clangd",
					"--background-index",
					"--clang-tidy",
					"--completion-style=detailed",
					"--header-insertion=iwyu",
				},
			})

			-- ===== 诊断显示样式 =====
			-- 报错在光标行下方整行展开（同一行多条也能看全），其他行只有 ✘/▲ 标记
			vim.diagnostic.config({
				virtual_text = false,
				virtual_lines = { current_line = true },
				severity_sort = true,
				update_in_insert = false,
				float = { border = "rounded", source = true },
				signs = {
					text = {
						[vim.diagnostic.severity.ERROR] = "✘",
						[vim.diagnostic.severity.WARN] = "▲",
					},
				},
			})

			-- 悬浮窗口圆角
			vim.lsp.handlers["textDocument/hover"] =
				vim.lsp.with(vim.lsp.handlers.hover, { border = "rounded" })
			vim.lsp.handlers["textDocument/signatureHelp"] =
				vim.lsp.with(vim.lsp.handlers.signature_help, { border = "rounded" })

			-- ===== LSP 快捷键（仅在服务器附加到当前文件时生效）=====
			vim.api.nvim_create_autocmd("LspAttach", {
				callback = function(args)
					local bufnr = args.buf
					local map = function(lhs, rhs, desc)
						vim.keymap.set("n", lhs, rhs, { buffer = bufnr, remap = false, desc = desc })
					end

					-- 跳转
					map("gd", vim.lsp.buf.definition, "跳转到定义")
					map("gD", vim.lsp.buf.declaration, "跳转到声明")
					map("gi", vim.lsp.buf.implementation, "跳转到实现")
					map("gy", vim.lsp.buf.type_definition, "跳转到类型定义")
					map("gr", "<cmd>Telescope lsp_references<CR>", "查找引用（列表）")
					map("K", vim.lsp.buf.hover, "悬停文档")

					-- 调用关系（结果在 quickfix 列表里，用 ]q [q 翻）
					map("<leader>ci", vim.lsp.buf.incoming_calls, "查看调用者（谁调用了它）")
					map("<leader>co", vim.lsp.buf.outgoing_calls, "查看被调用（它调用了谁）")

					-- 编辑与诊断
					map("<leader>cr", vim.lsp.buf.rename, "重命名符号")
					map("<F2>", vim.lsp.buf.rename, "重命名符号")
					map("<leader>ca", vim.lsp.buf.code_action, "代码操作 / 快速修复")
					map("<leader>cd", vim.diagnostic.open_float, "显示当前行诊断")
					map("[d", vim.diagnostic.goto_prev, "上一个诊断")
					map("]d", vim.diagnostic.goto_next, "下一个诊断")
				end,
			})
		end,
	},
}

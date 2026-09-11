# Terminal Dev Environment — User Manual

> 适用系统：Fedora 43 ｜ 用户：root ｜ 实时配置位置：`/root/.config/nvim`
> 本文档 + 配置备份位置：本仓库（`DevKitPack/`）

---

## 1. What's Inside

| 工具 | 作用 | 怎么启动 |
|---|---|---|
| **Neovim** (0.11+) | 主编辑器：写代码、看代码、LSP 跳转 | `nvim 文件` / `nvim .`（打开整个目录）|
| **lazygit** | 终端 Git 图形界面（提交、查看 diff、分支） | `lazygit` |
| **yazi** | 终端文件管理器（带预览） | `yazi` |
| **ripgrep (rg)** | 极速文本搜索（Telescope 后台用的就是它） | `rg 关键词` |
| **fd** | 极速文件名搜索 | `fd 名字` |
| **fzf** | 命令行模糊查找（管道、历史记录） | `历史命令 Ctrl+R` |
| **bear** | 给 C/C++ 项目生成编译数据库（clangd 用） | `bear -- make` |

LSP 服务器和代码格式化工具由 **Mason** 自动安装维护（在 Neovim 里输 `:Mason` 可查看/更新）。
所有 LSP 与格式化工具放在 `~/.local/share/nvim/mason/`，不污染系统。

---

## 2. Most-Used Actions (Start Here)

### Open a Project

```bash
cd ~/你的项目目录
nvim .
```

### Search Code

| 我要…… | 按键（空格 = Leader 键） |
|---|---|
| 按文件名找文件 | `空格` `f` `f` |
| 在整个项目里搜内容 | `空格` `f` `g`（实时出结果，回车跳转）|
| 搜光标下的单词/函数名 | 光标放词上，按 `空格` `f` `w` |
| 在当前文件里模糊搜 | `空格` `/` |
| 当前文件的函数大纲 | `空格` `f` `s` |
| 全项目搜函数名/类名 | `空格` `f` `S` |
| 文件内查找（原生） | `/关键词` 回车，`n`/`N` 下一个/上一个 |

### Jump & Peek (LSP, Works on Code Files)

| 我要…… | 按键 |
|---|---|
| 跳到**定义** | `g` `d` |
| 跳到**声明** | `g` `D` |
| 跳到**实现** | `g` `i` |
| 跳到**类型定义** | `g` `y` |
| 查看**引用**（谁用了它） | `g` `r`（弹出列表，回车跳转）|
| 查看**调用者**（谁调用了这个函数）| `空格` `c` `i` |
| 查看**被调用**（这个函数调用了谁）| `空格` `c` `o` |
| 看函数文档/签名 | `K`（光标放在函数上）|
| 在结果列表里翻 | `]` `q` 下一条 / `[` `q` 上一条 |
| 跳回刚才的位置 | `Ctrl` `o`；再跳进去 `Ctrl` `i` |

> `空格 c i` / `空格 c o` 的结果会显示在下方 **quickfix 列表**里，按 `空格` `q` `o` 可展开查看，回车跳转。
> 查看调用关系需要语言服务器支持：**C/C++ (clangd)、Python (pyright)、Go (gopls) 都支持**。

### Write Code

| 我要…… | 按键 |
|---|---|
| 补全 | 直接打字自动弹出；`Tab`/`Shift+Tab` 切换候选，回车选中 |
| 重命名一个函数/变量（全项目一起改）| `空格` `c` `r` 或 `F2` |
| 快速修复 / 代码操作 | `空格` `c` `a` |
| 看当前行的错误提示 | `空格` `c` `d` |
| 跳到下一个/上一个错误 | `]` `d` / `[` `d` |
| 手动格式化 | `空格` `f` `m` |
| 打开/关闭文件树 | `Ctrl` `b` |
| 打开终端 | `Ctrl` `` ` ``，退出终端窗口用 `Esc` 再 `:q` |

---

## 3. Language Support (Installed Servers)

| 语言 | LSP 服务器 | 能做什么 |
|---|---|---|
| C / C++ | **clangd** | 跳转、补全、引用、调用关系、clang-tidy 检查 |
| Python | **pyright** | 跳转、补全、引用、调用关系、类型检查 |
| JavaScript / TypeScript | **ts_ls** | 跳转、补全、引用、重命名 |
| HTML / CSS / JSON | html / cssls / jsonls | 标签、类名、属性补全与检查 |
| Go | **gopls** | 跳转、补全、引用、调用关系、重命名 |
| Bash / Shell | **bashls** | 命令、变量补全，跳转，引用 |
| Lua | **lua_ls** | 跳转、补全、检查（已适配 Neovim 开发）|
| 汇编 (asm) | **asm_lsp** | 标签/符号跳转、悬停提示（x86/x86-64/ARM/RISC-V）|

**格式化**：手动执行 `空格` `f` `m`（Lua→stylua，Python→black，C/C++→clang-format，Go→gofmt，JS/HTML/CSS/JSON/YAML→prettier，Shell→shfmt）。刻意不开 "保存时自动格式化"，避免重排整个文件、让 git diff 混入非本意改动。

保存绝不隐式改动文件：不自动格式化、不修剪空白、**不自动补文件末尾换行**（`fixendofline=false`），保存只会写入你亲手改的内容。

**语法高亮**：由 treesitter 提供，覆盖上述所有语言 + Rust/Markdown 等。

---

## 4. C/C++: compile_commands.json

clangd 要精确跨文件跳转，需要项目里有 `compile_commands.json`（编译数据库）。
生成方法（二选一）：

**CMake 项目：**
```bash
cmake -B build -DCMAKE_EXPORT_COMPILE_COMMANDS=ON
ln -s build/compile_commands.json .    # 放到项目根目录
```

**Make 项目**（bear 已装好）：
```bash
bear -- make
```

没有这个文件时 clangd 也能工作（自动猜编译参数），但跳转精度会下降，标准库头文件可能报错。

---

## 5. Neovim Config Layout

```
/root/.config/nvim/
├── init.lua                     # 入口
├── lazy-lock.json               # 插件版本锁文件
└── lua/
    ├── config/
    │   ├── options.lua          # 基础设置（缩进、行号、剪贴板…）
    │   ├── keymaps.lua          # 全局快捷键（本文件是快捷键总表）
    │   └── autocmds.lua         # 自动命令（各语言缩进风格等）
    └── plugins/                 # 每个插件一个文件
        ├── lsp.lua              # ★ LSP 服务器 + 跳转快捷键（最重要）
        ├── telescope.lua        # 模糊搜索
        ├── treesitter.lua       # 语法高亮
        ├── cmp.lua              # 代码补全
        ├── conform.lua          # 格式化
        ├── nvim-tree.lua        # 文件树
        └── ...                  # 其余插件
```

常用管理命令（在 Neovim 里输入）：

| 命令 | 作用 |
|---|---|
| `:Mason` | 管理 LSP 服务器/工具（安装、更新、卸载）|
| `:Lazy` | 管理插件（更新、查看状态）|
| `:LspInfo` | 查看当前文件挂上了哪个 LSP 服务器 |
| `:MasonInstall 名字` | 安装新的服务器/工具，如 `:MasonInstall gopls` |
| `:TSInstall 语言` | 安装语法高亮解析器，如 `:TSInstall rust` |
| `:checkhealth` | 全面体检，出问题先跑这个 |

---

## 6. FAQ

**Q：打开文件后 gd / gr 没反应？**
A：① 等几秒（大项目 clangd 要先建索引）；② `:LspInfo` 看服务器有没有挂上；③ 确认该语言装了服务器（`:Mason` 里搜）。

**Q：C/C++ 标准库头文件标红？**
A：没生成 `compile_commands.json`（见第四节），或 clangd 还没建完索引（首次打开大项目会等一会儿，之后有缓存就快了）。

**Q：终端里按 Ctrl+S 卡住不动？**
A：这是终端流控。按 `Ctrl+Q` 恢复。在 Neovim 里已把 Ctrl+S 映射为保存，不会卡。

**Q：想装别的语言的 LSP？**
A：`:MasonInstall 包名`，装完自动启用（比如 `:MasonInstall rust-analyzer` 已经有了，`clangd`、`gopls` 同理）。

**Q：插件出问题怎么恢复？**
A：这份文档旁边的 `config/` 目录就是配置备份，重新复制回去即可（见 install.sh）。

**Q：怎么换主题？**
A：输入 `:ThemeHub` 打开主题选择器，选择后自动记住。

**Q：保存后 git diff 里多了 "\ No newline at end of file" 之类、我没动过的改动？**
A：旧配置里 Neovim 会在保存时自动给文件末尾补换行，现已关闭（`fixendofline=false`）：文件原本有没有末尾换行，保存后保持原样，不会再出现这类改动。

---

## 7. Config Backup & Restore

- **实时配置**（Neovim 实际读取的）：`/root/.config/nvim/`
- **备份副本**（本目录）：`DevKitPack/config/nvim/`

改完配置如果想让备份同步，执行：
```bash
cp -a ~/.config/nvim/. config/nvim/    # 在仓库根目录执行
```

换新机器 / 重装时，把 `config/nvim` 复制到 `~/.config/nvim`，再运行本目录的 `install.sh` 即可。

---

## 8. lazygit & yazi Cheatsheet

**lazygit**（在项目目录里执行 `lazygit`）：

| 按键 | 作用 |
|---|---|
| `空格` | 暂存/取消暂存文件 |
| `c` | 提交（写消息后回车）|
| `P` | 推送 / `p` 拉取 |
| `2` `3` `4` | 切换 文件 / 分支 / 提交 面板 |
| `q` | 退出 |
| `x` | 打开菜单（更多操作）|

**yazi**（`yazi` 启动）：

| 按键 | 作用 |
|---|---|
| `hjkl` 或方向键 | 移动 |
| `Enter` / `l` | 打开文件或进入目录 |
| `y` / `x` / `p` | 复制 / 剪切 / 粘贴 |
| `d` | 删除（会问确认）|
| `/` | 搜索 |
| `.` | 显示/隐藏隐藏文件 |
| `q` | 退出；`Q` 退出且不保存当前目录 |
| `~` | 回到 home |

---

## 9. Migrating to Another Machine (Offline Ubuntu)

如果要把这套环境整套搬到**没有外网**的 Ubuntu 机器上，用同目录 `devkit/` 里的迁移工具：
Fedora 上 `./devkit pack` 生成单个可执行文件 → 拷过去 → Ubuntu 上 `./devkit apply && ./devkit verify`。
详见 [DEVKIT.md](./DEVKIT.md)。

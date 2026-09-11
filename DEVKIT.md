# devkit — Offline Migration to an Air-Gapped Ubuntu

> 源码位置：本仓库 `devkit/` 目录
> 适用：Fedora（源机，联网）→ Ubuntu 24.04（目标机，**完全无网**，用普通用户运行）

---

## 1. What It Is

把 Fedora 上这套终端开发环境（Neovim + 30 个插件 + 13 个 LSP 服务器 + ripgrep / fd / fzf / lazygit / yazi / node 等）打包成**单个可执行文件** `devkit`。拷到目标机运行一条命令，在**零网络**的情况下还原出一模一样的环境；以后在 Fedora 改了配置、加了插件，重新打包再拷一次即可。

实测数据（Fedora → 模拟目标机）：

| 指标 | 数值 |
|---|---|
| 打包内容 | 37,872 个文件 / 993.1 MB |
| 压缩后 payload | 348.4 MB |
| 最终单文件 | 约 355 MB |
| 打包耗时 | 约 6 分钟（缓存命中，含压缩与编译） |
| 还原耗时 | 约 15 秒 |
| 冒烟验证 | 约 6 秒，10 种文件类型 LSP 全部 ✔ |

---

## 2. Three-Step Usage

### Step 1: Pack on Fedora (Needs Network + Proxy)

```bash
cd <仓库目录>/devkit
export HTTP_PROXY=http://127.0.0.1:7890 HTTPS_PROXY=http://127.0.0.1:7890
./devkit pack
```

产物是**同目录的 `devkit`**（同名覆盖，内含全部环境）。如果你改过 Go 源码，先进目录 `go build -o devkit .` 再 pack；pack 内部也会自动重新编译。

| 选项 | 作用 |
|---|---|
| `--out 名字` | 输出文件名（默认 `devkit`） |
| `--trim-web` | 不带 HTML/CSS/JSON 三个 LSP，省约 280 MB |

打包做的事：收集 `~/.config/{nvim,lazygit,yazi}`、lazy 插件、mason 里 13 个服务器（html/css/json 可选）；下载 Neovim 0.11.6、Node 22、ripgrep / fd / fzf / ruff / yazi 的静态版；从 pip wheel 里提取 clang-format 的真 ELF；生成 `embed/payload.tar.gz` 后自动 `go build` 出单文件。**下载缓存**在 `devkit/build/cache/`，重复打包不会重复下载。

### Step 2: Copy to Ubuntu

一个文件即可（U 盘 / scp 都行）：

```bash
scp devkit <user>@目标机:~/
```

### Step 3: Apply on Ubuntu (No sudo, No Network)

```bash
chmod +x devkit
./devkit apply          # 装到当前用户 ~/.local，全程用户级，不碰系统目录
./devkit verify         # 冒烟：工具版本 + 10 种文件类型的 LSP 附加
nvim                    # 开写
```

`apply` 完成后**重开一个终端**（或 `source ~/.bashrc`）让 PATH 生效。

---

## 3. Where Things Go

| 目标路径 | 内容 |
|---|---|
| `~/.config/{nvim,lazygit,yazi}` | 配置文件 |
| `~/.local/share/nvim/lazy` | 30 个 lazy 插件（版本由 lazy-lock.json 锁定） |
| `~/.local/share/nvim/mason` | 13 个 LSP / 格式化服务器（含离线必需的 registries） |
| `~/.local/opt/nvim` | Neovim 本体（含 runtime） |
| `~/.local/opt/node` | Node.js 22（pyright / ts_ls / css / html / json LSP 依赖它） |
| `~/.local/bin` | nvim、node、lazygit、yazi、rg、fd、fzf、ruff、black、clang-format 的入口 |
| `~/.bashrc` 末尾 | `# >>> devkit >>>` 块：PATH 前置、EDITOR=nvim、`lg`/`y` 别名（幂等，可反复 apply） |

> lazy / mason 必须放在 `~/.local/share/nvim/` 下 —— 这是 Neovim `stdpath("data")` 的固定约定，放别处插件找不到。

---

## 4. Common Commands

| 命令 | 作用 |
|---|---|
| `./devkit list` | 查看包内清单（组件、版本、大小） |
| `./devkit apply --dry-run` | 只预览将做什么，不写磁盘 |
| `./devkit apply --home /home/<user> --owner <user>` | 以 root 运行 `apply` 时，指定目标家目录与属主 |
| `./devkit verify --timeout 120` | 每个语言 LSP 附加检查的等待秒数（默认 90） |
| `./devkit verify --home DIR` | 验证指定家目录里的这套环境 |

> `apply` / `verify` 会自动识别**执行者**（用户名与家目录取自系统用户库，在哪个目录下执行都一样）；不传 `--home` 时装进 / 检查执行者本人的家目录。以 root 运行时会有提示（推荐改用普通用户）。

**再次强调：正常情况下在 Ubuntu 上用普通用户（`<user>`）直接跑，不要加 sudo。**

---

## 5. How to Update Later

1. 在 Fedora 上正常用（改配置、`:Lazy sync` 装新插件、`:Mason` 装新服务器）；
2. 重新打包：`cd .../devkit && ./devkit pack`
3. `devkit` 拷到 Ubuntu，跑 `./devkit apply && ./devkit verify`。

`apply` 是**幂等**的：旧配置会先整体挪到 `~/.devkit-backup-<时间戳>/`（可随时翻回去），插件 / LSP / 工具目录直接重装；`.bashrc` 的 devkit 块是替换式更新，不会重复堆叠。确认新环境没问题后，备份目录可自行删除。

> 注：pack 里打包的 mason 包是白名单制（当前 13 个）。如果你在 Fedora 用 `:Mason` 装了**新的** LSP，需要把它加进 `devkit/pack.go` 的 `masonKeep` 列表再重新打包，否则不会随包。

---

## 6. Notes & Known Differences

- **推荐用 Ubuntu 上的普通用户直接跑 apply**：会自动识别执行者（用户名、家目录取自系统用户库，与在哪个目录执行、`$HOME` 是否被改写无关）；用 sudo / root 运行时会提示，并装进 root 的家目录——确需 root 安装时用 `--home /home/<user> --owner <user>`。
- **black 实际是 ruff 的包装**：离线环境不带 Python 运行时（mason 里的 black 是绑定 Python 3.14 的编译产物，搬到 Python 3.12 的机器上必挂），因此改用静态的 ruff，并生成同名 `black` 命令转发给它。日常格式化无感，极少数边角格式与真 black 略有差异。
- 若用 `--trim-web` 打包，verify 里 HTML / CSS 会显示「跳过：未打包」。
- Ubuntu 系统自带 node 16 不受影响，只是 PATH 里随包的 node 22 优先。
- yazi 用官方 musl 静态版（避免 Fedora glibc 2.42 → Ubuntu 2.39 的动态链接风险）；lazygit 本来就是静态链接，直接拷。
- 兼容性最终以目标机 `./devkit verify` 全 ✔ 为准；每一步独立报错，绝不静默失败。

---

## 7. Design Notes

- **单文件**：payload 用 `go:embed` 嵌进 Go 二进制，不再依赖外部 tar 包。
- **离线可用**：mason 的 `registries/` 必须随包（否则 `:Mason` 与 mason-lspconfig 会尝试联网）；插件目录自带 `.git`，lazy 启动时不会去 clone。
- **二进制可移植**：外网工具一律用 musl 静态版；asm-lsp 需要 GLIBC_2.39（Ubuntu 24.04 恰好满足）；clangd / lua-ls / gopls / treesitter parser 实测均 ≤ 2.39。
- **node 类 LSP 即插即用**：mason 里 node 系包全是纯 JS + 相对软链，无原生模块，PATH 里有 node 22 即可。
- **路径兼容**：Fedora 是 root（`/root`）、Ubuntu 是普通用户（`/home/<user>`）；apply 会自动把包内残留的 `/root/` 文本路径改写成目标家目录，文件属主也可用 `--owner` 一并处理。

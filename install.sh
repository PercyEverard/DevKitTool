#!/bin/bash
# ==============================================================
# 终端开发环境一键安装脚本（Fedora）
# 作用：装齐所有工具 + 恢复 Neovim 配置 + 自动下载插件和 LSP
# 用法：bash install.sh
# ==============================================================
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"

echo "==> [1/5] 安装系统工具（dnf）"
dnf install -y \
    neovim ripgrep fd-find fzf yazi bear \
    git gcc gcc-c++ clang clang-tools-extra make cmake \
    python3 nodejs nodejs-npm golang \
    unzip tar curl wget openssl-devel pkgconf-pkg-config

echo "==> [2/5] 安装 lazygit（Fedora 仓库没有，从 GitHub 下载）"
if ! command -v lazygit >/dev/null 2>&1; then
    LAZYGIT_VERSION=$(curl -s "https://api.github.com/repos/jesseduffield/lazygit/releases/latest" \
        | grep -Po '"tag_name": "v\K[^"]*')
    curl -Lo /tmp/lazygit.tar.gz \
        "https://github.com/jesseduffield/lazygit/releases/latest/download/lazygit_${LAZYGIT_VERSION}_Linux_x86_64.tar.gz"
    tar -xf /tmp/lazygit.tar.gz -C /tmp lazygit
    install /tmp/lazygit /usr/local/bin/lazygit
    rm -f /tmp/lazygit /tmp/lazygit.tar.gz
    echo "    lazygit ${LAZYGIT_VERSION} 安装完成"
else
    echo "    lazygit 已存在，跳过"
fi

echo "==> [3/5] 恢复 Neovim 配置到 ~/.config/nvim"
if [ -d "$HOME/.config/nvim" ]; then
    BACKUP="$HOME/.config/nvim.bak.$(date +%Y%m%d%H%M%S)"
    echo "    已有配置先备份到 $BACKUP"
    mv "$HOME/.config/nvim" "$BACKUP"
fi
mkdir -p "$HOME/.config"
cp -a "$SCRIPT_DIR/config/nvim" "$HOME/.config/nvim"

echo "==> [4/5] 首次启动 Neovim，自动下载插件（lazy.nvim）"
nvim --headless "+Lazy! sync" +qa 2>/dev/null || true

echo "==> [5/5] 自动下载 LSP 服务器和语法解析器（Mason + treesitter）"
# 启动 Neovim 让 mason-lspconfig / treesitter 自动安装，最多等 10 分钟
nvim --headless "+qa" 2>/dev/null || true
echo "    （这一步是后台异步的：首次打开 nvim 后等它装完即可；"
echo "      可用 :Mason 查看进度，或 :TSInstall <语言> 手动补装解析器）"

cat <<'EOF'

============================================================
安装完成！快速验证：
  nvim 任意代码文件
  :LspInfo     查看当前文件挂上的 LSP 服务器
  :Mason       查看/管理所有 LSP 服务器
  :checkhealth 全面体检

文档在：本目录 README.md；快捷键：本目录 KEYMAPS.md
============================================================
EOF

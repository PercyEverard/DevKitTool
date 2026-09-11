// devkit：把 Fedora 上的终端开发环境（Neovim 配置 / 插件 / LSP / 工具链）
// 打包成单个可执行文件，拷贝到无外网的 Ubuntu 机器上离线还原。
//
// 典型流程：
//
//	Fedora 上:   go build -o devkit . && ./devkit pack
//	拷贝:       devkit  →  Ubuntu
//	Ubuntu 上:   ./devkit apply && ./devkit verify
package main

import (
	"flag"
	"fmt"
	"os"
	"os/user"
	"path/filepath"
)

const devkitVersion = "1.0.0"

func main() {
	if len(os.Args) < 2 {
		printHelp()
		os.Exit(1)
	}

	cmd := os.Args[1]
	args := os.Args[2:]

	var err error
	switch cmd {
	case "pack":
		err = cmdPack(args)
	case "apply":
		err = cmdApply(args)
	case "verify":
		err = cmdVerify(args)
	case "list":
		err = cmdList(args)
	case "version", "-v", "--version":
		fmt.Printf("devkit %s\n", devkitVersion)
		return
	case "help", "-h", "--help":
		printHelp()
		return
	default:
		fmt.Fprintf(os.Stderr, "未知命令: %s\n\n", cmd)
		printHelp()
		os.Exit(1)
	}

	if err != nil {
		fmt.Fprintf(os.Stderr, "\n✘ 失败: %v\n", err)
		os.Exit(1)
	}
}

func printHelp() {
	fmt.Print(`devkit - 终端开发环境离线迁移工具

用法:
  devkit <命令> [选项]

命令:
  pack     在 Fedora（联网）上打包环境并编译出可迁移的单文件工具
  apply    在目标机（离线）上还原环境
  verify   还原后冒烟测试：工具版本 + 各语言 LSP 附加检查
  list     查看本文件内已打包的内容清单
  version  显示版本

常用流程:
  1) Fedora:   ./devkit pack                 # 产物还是 devkit（同名，内含全部环境）
  2) 拷贝:     devkit 一个文件拷到 Ubuntu
  3) Ubuntu:   ./devkit apply                # 默认装到 ~/.local，无需 sudo
  4) Ubuntu:   ./devkit verify               # 检查是否全部可用

pack 常用选项:
  --out NAME      输出文件名（默认 devkit）
  --trim-web      不打包 HTML/CSS/JSON 的 LSP 服务器（省约 280MB）

apply 常用选项:
  --home DIR      目标家目录（默认：执行者本人的家目录，取自系统用户库）
  --owner USER    装完后把文件属主改为该用户（以 root 运行时有意义）
  --dry-run       只显示将要做什么，不写磁盘

执行者识别:
  apply / verify 在任意目录下执行都会自动识别执行者本人（用户名 + 家目录，
  与 $HOME 是否被改写无关）；推荐用目标机的普通用户直接运行，以 root 运行时会提示。
`)
}

// currentUser 返回执行者的系统用户记录：用户名与家目录来自系统用户库，
// 与在哪个目录执行、$HOME 是否被改写（sudo / 脚本）无关。
func currentUser() (*user.User, error) {
	u, err := user.Current()
	if err != nil {
		return nil, fmt.Errorf("无法识别当前执行用户（可用 --home 显式指定目标家目录）: %w", err)
	}
	return u, nil
}

// resolveHome 决定目标家目录：优先 --home，否则取执行者本人的家目录。
func resolveHome(flagHome string, u *user.User) (string, error) {
	home := flagHome
	if home == "" {
		home = u.HomeDir
	}
	if home == "" || home == "/" {
		return "", fmt.Errorf("用户 %s 的家目录无法确定，请用 --home 指定", u.Username)
	}
	return filepath.Abs(home)
}

func parseFlags(fs *flag.FlagSet, args []string) {
	fs.SetOutput(os.Stderr)
	if err := fs.Parse(args); err != nil {
		os.Exit(1)
	}
}

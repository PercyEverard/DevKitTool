package main

import (
	"bytes"
	"flag"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

type verifyCtx struct {
	home    string
	env     []string
	path    string
	timeout time.Duration
	fails   int
}

// resolve 优先用托管路径，不存在时退回 PATH（方便在源机器上直接对比自测）。
func (v *verifyCtx) resolve(name, preferred string) string {
	if fileExists(preferred) {
		return preferred
	}
	for _, dir := range filepath.SplitList(v.path) {
		p := filepath.Join(dir, name)
		if fileExists(p) {
			return p
		}
	}
	return preferred
}

func (v *verifyCtx) bin(name string) string {
	return filepath.Join(v.home, ".local/bin", name)
}

func (v *verifyCtx) masonBin(name string) string {
	return filepath.Join(v.home, ".local/share/nvim/mason/bin", name)
}

func (v *verifyCtx) failf(format string, a ...any) {
	v.fails++
	fmt.Printf("  ✘ "+format+"\n", a...)
}

// run 执行命令并返回合并输出。
func (v *verifyCtx) run(name string, args ...string) (string, error) {
	cmd := exec.Command(name, args...)
	cmd.Env = v.env
	var buf bytes.Buffer
	cmd.Stdout = &buf
	cmd.Stderr = &buf
	err := cmd.Run()
	return strings.TrimSpace(buf.String()), err
}

func (v *verifyCtx) runStdin(input string, name string, args ...string) (string, error) {
	cmd := exec.Command(name, args...)
	cmd.Env = v.env
	cmd.Stdin = strings.NewReader(input)
	var buf bytes.Buffer
	cmd.Stdout = &buf
	cmd.Stderr = &buf
	err := cmd.Run()
	return strings.TrimSpace(buf.String()), err
}

func firstLine(s string) string {
	if i := strings.IndexByte(s, '\n'); i >= 0 {
		return s[:i]
	}
	return s
}

func cmdVerify(args []string) error {
	fs := flag.NewFlagSet("verify", flag.ContinueOnError)
	homeFlag := fs.String("home", "", "目标家目录（默认 $HOME）")
	timeoutFlag := fs.Int("timeout", 90, "每个语言 LSP 附加等待秒数")
	parseFlags(fs, args)

	me, err := currentUser()
	if err != nil {
		return err
	}
	home, err := resolveHome(*homeFlag, me)
	if err != nil {
		return err
	}

	pathEnv := filepath.Join(home, ".local/bin") + ":" +
		filepath.Join(home, ".local/opt/node/bin") + ":" +
		filepath.Join(home, ".local/share/nvim/mason/bin") + ":" +
		os.Getenv("PATH")
	v := &verifyCtx{home: home, path: pathEnv, timeout: time.Duration(*timeoutFlag) * time.Second}
	v.env = append(os.Environ(),
		"HOME="+home,
		"XDG_CONFIG_HOME="+filepath.Join(home, ".config"),
		"XDG_DATA_HOME="+filepath.Join(home, ".local/share"),
		"XDG_STATE_HOME="+filepath.Join(home, ".local/state"),
		"XDG_CACHE_HOME="+filepath.Join(home, ".local/cache"),
		"PATH="+pathEnv,
	)

	infof("运行身份: %s（uid %s），目标家目录: %s", me.Username, me.Uid, home)

	// ---- 1) 工具版本 ----
	step("1/3", "工具版本")
	tools := []struct {
		label string
		name  string
		pref  string
		args  []string
	}{
		{"neovim", "nvim", v.bin("nvim"), []string{"--version"}},
		{"lazygit", "lazygit", v.bin("lazygit"), []string{"--version"}},
		{"yazi", "yazi", v.bin("yazi"), []string{"--version"}},
		{"ripgrep", "rg", v.bin("rg"), []string{"--version"}},
		{"fd", "fd", v.bin("fd"), []string{"--version"}},
		{"fzf", "fzf", v.bin("fzf"), []string{"--version"}},
		{"node", "node", v.bin("node"), []string{"--version"}},
		{"ruff", "ruff", v.bin("ruff"), []string{"--version"}},
		{"clang-format", "clang-format", v.bin("clang-format"), []string{"--version"}},
		{"clangd", "clangd", v.masonBin("clangd"), []string{"--version"}},
		{"gopls", "gopls", v.masonBin("gopls"), []string{"version"}},
	}
	for _, t := range tools {
		p := v.resolve(t.name, t.pref)
		if !fileExists(p) {
			v.failf("%-14s 不存在: %s", t.label, p)
			continue
		}
		out, err := v.run(p, t.args...)
		if err != nil {
			v.failf("%-14s 运行失败: %v（%s）", t.label, err, firstLine(out))
			continue
		}
		fmt.Printf("  ✓ %-14s %s\n", t.label, firstLine(out))
	}

	// ---- 2) 格式化工具冒烟 ----
	step("2/3", "格式化工具冒烟")
	if out, err := v.runStdin("x=1\n", v.resolve("black", v.bin("black")), "-"); err != nil || !strings.Contains(out, "x = 1") {
		v.failf("black（ruff 包装）格式化失败: %v（%q）", err, out)
	} else {
		fmt.Printf("  ✓ %-14s black 兼容包装可用（ruff format）\n", "black")
	}
	if out, err := v.runStdin("int  main(){return 0;}\n", v.resolve("clang-format", v.bin("clang-format"))); err != nil || !strings.Contains(out, "int main()") {
		v.failf("clang-format 格式化失败: %v（%q）", err, out)
	} else {
		fmt.Printf("  ✓ %-14s 格式化可用\n", "clang-format")
	}

	// ---- 3) 各语言 LSP 附加冒烟 ----
	step("3/3", "LSP 附加检查（每语言最多等 %ds）", int(v.timeout.Seconds()))
	lazyDir := filepath.Join(home, ".local/share/nvim/lazy")
	if des, err := os.ReadDir(lazyDir); err == nil {
		n := 0
		for _, d := range des {
			if d.IsDir() {
				n++
			}
		}
		fmt.Printf("  · lazy 插件 %d 个\n", n)
	} else {
		v.failf("lazy 插件目录不存在: %s", lazyDir)
	}

	tmp, err := os.MkdirTemp("", "devkit-verify-")
	if err != nil {
		return err
	}
	defer os.RemoveAll(tmp)

	cases := []struct {
		lang, file, server, pkg, content string
	}{
		{"C", "test.c", "clangd", "clangd",
			"#include <stdio.h>\nint add(int a, int b) { return a + b; }\nint main(void) { printf(\"%d\\n\", add(1, 2)); return 0; }\n"},
		{"C++", "test.cpp", "clangd", "clangd",
			"#include <vector>\nint main() { std::vector<int> v{1, 2}; return (int)v.size(); }\n"},
		{"Python", "test.py", "pyright", "pyright",
			"def add(a, b):\n    return a + b\n\n\nprint(add(1, 2))\n"},
		{"Go", "test.go", "gopls", "gopls",
			"package main\n\nimport \"fmt\"\n\nfunc main() { fmt.Println(\"hi\") }\n"},
		{"Bash", "test.sh", "bashls", "bash-language-server",
			"#!/bin/bash\nset -e\necho \"hi\" | grep hi\n"},
		{"JavaScript", "test.js", "ts_ls", "typescript-language-server",
			"const add = (a, b) => a + b;\nconsole.log(add(1, 2));\n"},
		{"HTML", "test.html", "html", "html-lsp",
			"<!DOCTYPE html>\n<html><body><h1>hi</h1></body></html>\n"},
		{"CSS", "test.css", "cssls", "css-lsp",
			"body { color: red; }\n"},
		{"Lua", "test.lua", "lua_ls", "lua-language-server",
			"local function add(a, b)\n  return a + b\nend\nprint(add(1, 2))\n"},
		{"汇编", "test.s", "asm_lsp", "asm-lsp",
			".globl main\nmain:\n    mov $60, %rax\n    xor %rdi, %rdi\n    syscall\n"},
	}

	for _, tc := range cases {
		if !fileExists(filepath.Join(home, ".local/share/nvim/mason/packages", tc.pkg)) {
			fmt.Printf("  ~ %-10s 跳过：未打包（--trim-web）\n", tc.lang)
			continue
		}
		src := filepath.Join(tmp, tc.file)
		if err := os.WriteFile(src, []byte(tc.content), 0o644); err != nil {
			return err
		}
		script := filepath.Join(tmp, "check_"+tc.server+".lua")
		scriptBody := fmt.Sprintf(`local want = %q
local ok = vim.wait(%d, function()
  for _, c in ipairs(vim.lsp.get_clients({ bufnr = 0 })) do
    if c.name == want then return true end
  end
  return false
end, 200)
if ok then
  print("DEVKIT_LSP_OK")
  vim.cmd("qa!")
else
  local got = {}
  for _, c in ipairs(vim.lsp.get_clients({ bufnr = 0 })) do got[#got + 1] = c.name end
  print("DEVKIT_LSP_FAIL attached:[" .. table.concat(got, ",") .. "]")
  vim.cmd("cquit 1")
end
`, tc.server, v.timeout.Milliseconds())
		if err := os.WriteFile(script, []byte(scriptBody), 0o644); err != nil {
			return err
		}

		out, err := v.run(v.resolve("nvim", v.bin("nvim")), "--headless", src, "-c", "luafile "+script)
		switch {
		case strings.Contains(out, "DEVKIT_LSP_OK"):
			fmt.Printf("  ✓ %-10s LSP %s 已附加\n", tc.lang, tc.server)
		case err != nil:
			v.failf("%-10s LSP %s 未附加（%v）%s", tc.lang, tc.server, err, lspHint(firstLine(out)))
		default:
			v.failf("%-10s LSP %s 未附加 %s", tc.lang, tc.server, lspHint(firstLine(out)))
		}
	}

	fmt.Println()
	if v.fails > 0 {
		return fmt.Errorf("%d 项检查未通过（见上方 ✘ 与提示）", v.fails)
	}
	fmt.Println("全部检查通过 ✔")
	return nil
}

func lspHint(first string) string {
	if first == "" {
		return ""
	}
	if strings.Contains(first, "DEVKIT_LSP_FAIL") {
		return "（" + first + "，可试：nvim 里 :LspInfo / :Mason）"
	}
	return "（" + first + "）"
}

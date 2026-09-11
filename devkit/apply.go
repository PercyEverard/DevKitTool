package main

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"os/user"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

const (
	blockStart = "# >>> devkit >>>"
	blockEnd   = "# <<< devkit <<<"
)

// 管理清单：payload 内 prefix → 落到目标机的相对位置
var applyPrefixes = map[string]string{
	"config": ".config",
	"data":   ".local/share/nvim",
	"opt":    ".local/opt",
	"bin":    ".local/bin",
}

// 需要整体替换的大目录（不可再现内容才值得备份）
var replaceDirs = []string{
	".local/share/nvim/lazy",
	".local/share/nvim/mason",
	".local/opt/nvim",
	".local/opt/node",
}

// 旧配置目录：存在则先备份
var backupDirs = []string{
	".config/nvim",
	".config/lazygit",
	".config/yazi",
}

type applyCtx struct {
	home    string
	backup  string
	dry     bool
	owner   *user.User
	managed []string
}

func cmdApply(args []string) error {
	fs := flag.NewFlagSet("apply", flag.ContinueOnError)
	homeFlag := fs.String("home", "", "目标家目录（默认 $HOME）")
	ownerFlag := fs.String("owner", "", "装完后把文件属主改为该用户")
	dryFlag := fs.Bool("dry-run", false, "只显示将要做什么，不写磁盘")
	parseFlags(fs, args)

	if !payloadReady() {
		return errors.New("本文件目前是占位状态，还没有打包任何环境（请先在 Fedora 上执行 ./devkit pack）")
	}

	me, err := currentUser()
	if err != nil {
		return err
	}
	home, err := resolveHome(*homeFlag, me)
	if err != nil {
		return err
	}
	c := &applyCtx{home: home, dry: *dryFlag}
	c.backup = filepath.Join(home, ".devkit-backup-"+time.Now().Format("20060102-150405"))
	c.managed = append(c.managed, replaceDirs...)
	c.managed = append(c.managed, backupDirs...)

	if *ownerFlag != "" {
		u, err := user.Lookup(*ownerFlag)
		if err != nil {
			return fmt.Errorf("找不到用户 %q: %w", *ownerFlag, err)
		}
		c.owner = u
	}

	step("1/5", "预检")
	m, _ := loadManifest()
	if m != nil && !m.Stub {
		infof("包内容: %s 于 %s 打包（源主机 %s / 用户 %s）",
			humanBytes(m.PayloadSize), m.CreatedAt, m.SourceHost, m.SourceUser)
	}
	if !c.dry {
		if err := os.MkdirAll(home, 0o755); err != nil {
			return fmt.Errorf("家目录不可写: %w", err)
		}
		test := filepath.Join(home, ".devkit-write-test")
		if err := os.WriteFile(test, []byte("x"), 0o644); err != nil {
			return fmt.Errorf("家目录不可写: %w", err)
		}
		os.Remove(test)
	}
	infof("运行身份: %s（uid %s, gid %s）", me.Username, me.Uid, me.Gid)
	okf("目标家目录: %s", home)
	if me.Uid == "0" {
		if c.owner == nil {
			warnf("以 root 运行：更推荐用目标用户的普通身份执行本命令（属主/权限更合适）")
			warnf("如确需以 root 安装，可加 --owner <用户名>，把文件属主改给目标用户")
		} else {
			infof("以 root 安装，文件属主将改为 %s（uid %s）", c.owner.Username, c.owner.Uid)
		}
	}
	if c.dry {
		warnf("dry-run 模式：只预览，不写磁盘")
	}

	// ---- 2) 备份 / 清理现有内容 ----
	step("2/5", "处理已有内容")
	for _, rel := range backupDirs {
		p := filepath.Join(home, rel)
		if !fileExists(p) {
			continue
		}
		dst := filepath.Join(c.backup, rel)
		if c.dry {
			infof("将备份 %s → %s", rel, c.backup)
			continue
		}
		if err := os.MkdirAll(filepath.Dir(dst), 0o755); err != nil {
			return err
		}
		if err := os.Rename(p, dst); err != nil {
			return fmt.Errorf("备份 %s 失败: %w", rel, err)
		}
		infof("备份 %s → %s", rel, filepath.Join(c.backup, rel))
	}
	for _, rel := range replaceDirs {
		p := filepath.Join(home, rel)
		if !fileExists(p) {
			continue
		}
		if c.dry {
			infof("将删除并重装 %s", rel)
			continue
		}
		if err := os.RemoveAll(p); err != nil {
			return fmt.Errorf("清理 %s 失败: %w", rel, err)
		}
		infof("清理 %s（由 payload 重新展开）", rel)
	}
	for _, name := range []string{"nvim", "node", "lazygit", "yazi", "rg", "fd", "fzf", "ruff", "black", "clang-format"} {
		p := filepath.Join(home, ".local/bin", name)
		if _, err := os.Lstat(p); err != nil { // Lstat：软链即使悬空也算存在
			continue
		}
		if c.dry {
			continue
		}
		dst := filepath.Join(c.backup, ".local/bin", name)
		if err := os.MkdirAll(filepath.Dir(dst), 0o755); err != nil {
			return err
		}
		os.Rename(p, dst)
	}
	if fileExists(c.backup) && !c.dry {
		infof("旧内容已挪到: %s（确认没问题后可自行删除）", c.backup)
	}

	// ---- 3) 解包 ----
	step("3/5", "展开 payload")
	count, rawBytes, err := c.extract()
	if err != nil {
		return err
	}
	okf("%d 个文件，%s", count, humanBytes(rawBytes))

	// ---- 4) 修复残留路径 ----
	step("4/5", "修正包内残留的源机路径")
	fixed, err := c.fixSourcePaths(m)
	if err != nil {
		return err
	}
	if fixed > 0 {
		okf("重写 %d 处文本路径", fixed)
	} else {
		okf("无需修正")
	}

	// ---- 5) bashrc + 属主 ----
	step("5/5", "shell 集成与属主")
	if err := c.updateBashrc(); err != nil {
		return err
	}
	if c.owner != nil {
		if err := c.chownAll(); err != nil {
			return err
		}
		okf("属主改为 %s（uid %s）", c.owner.Username, c.owner.Uid)
	}

	fmt.Printf("\n环境已还原到 %s。\n", home)
	if m != nil && !m.Stub {
		fmt.Printf("清单: ./%s list\n", filepath.Base(os.Args[0]))
	}
	fmt.Println("下一步:")
	fmt.Println("  1) 重开一个终端（或 source ~/.bashrc）让 PATH 生效")
	fmt.Printf("  2) ./%s verify    # 冒烟测试全部工具与 LSP\n", filepath.Base(os.Args[0]))
	fmt.Println("  3) nvim            # 开始写代码")
	return nil
}

// extract 从内嵌 payload.tar.gz 展开到目标家目录。
func (c *applyCtx) extract() (count int64, rawBytes int64, err error) {
	gz, err := gzip.NewReader(bytes.NewReader(payloadGz))
	if err != nil {
		return 0, 0, fmt.Errorf("payload 损坏: %w", err)
	}
	defer gz.Close()

	type dirMode struct {
		path string
		mode os.FileMode
	}
	var dirModes []dirMode

	tr := tar.NewReader(gz)
	for {
		hdr, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return 0, 0, err
		}

		name := strings.TrimPrefix(hdr.Name, "./")
		parts := strings.SplitN(name, "/", 2)
		if len(parts) < 2 || parts[1] == "" {
			continue // 顶层目录自身的条目（如 "bin/"），子项会创建它
		}
		base, ok := applyPrefixes[parts[0]]
		if !ok {
			return 0, 0, fmt.Errorf("payload 内有未知顶层目录: %q", parts[0])
		}
		dest, err := safeJoin(filepath.Join(c.home, base), parts[1])
		if err != nil {
			return 0, 0, err
		}

		if !c.dry {
			if err := os.MkdirAll(filepath.Dir(dest), 0o755); err != nil {
				return 0, 0, err
			}
		}

		switch hdr.Typeflag {
		case tar.TypeDir:
			if !c.dry {
				if err := os.MkdirAll(dest, 0o755); err != nil {
					return 0, 0, err
				}
			}
			dirModes = append(dirModes, dirMode{dest, os.FileMode(hdr.Mode) & 0o777})
		case tar.TypeReg:
			count++
			rawBytes += hdr.Size
			if c.dry {
				continue
			}
			out, err := os.OpenFile(dest, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, os.FileMode(hdr.Mode)&0o777)
			if err != nil {
				return 0, 0, err
			}
			if _, err := io.Copy(out, tr); err != nil {
				out.Close()
				return 0, 0, err
			}
			if err := out.Close(); err != nil {
				return 0, 0, err
			}
			// 显式 chmod，防 umask 干扰
			if err := os.Chmod(dest, os.FileMode(hdr.Mode)&0o777); err != nil {
				return 0, 0, err
			}
		case tar.TypeSymlink:
			if c.dry {
				continue
			}
			os.Remove(dest)
			if err := os.Symlink(hdr.Linkname, dest); err != nil {
				return 0, 0, err
			}
		}
	}

	if !c.dry {
		for _, d := range dirModes {
			os.Chmod(d.path, d.mode)
		}
		// bin/ 下的可执行文件兜底 +x
		binDir := filepath.Join(c.home, ".local/bin")
		if des, err := os.ReadDir(binDir); err == nil {
			for _, e := range des {
				p := filepath.Join(binDir, e.Name())
				if fi, err := os.Lstat(p); err == nil && fi.Mode().IsRegular() {
					os.Chmod(p, 0o755)
				}
			}
		}
	}
	return count, rawBytes, nil
}

func sourceHome(m *Manifest) string {
	if m == nil || m.SourceUser == "" {
		return ""
	}
	if m.SourceUser == "root" {
		return "/root"
	}
	return "/home/" + m.SourceUser
}

// fixSourcePaths 把配置文件里残留的源机家目录（如 /root/）改成本机家目录。
func (c *applyCtx) fixSourcePaths(m *Manifest) (int, error) {
	srcHome := sourceHome(m)
	if srcHome == "" || srcHome == c.home {
		return 0, nil
	}
	from := srcHome + "/"
	to := c.home + "/"

	scanRoots := []string{
		filepath.Join(c.home, ".config/nvim"),
		filepath.Join(c.home, ".config/lazygit"),
		filepath.Join(c.home, ".config/yazi"),
		filepath.Join(c.home, ".local/share/nvim/mason/bin"),
		filepath.Join(c.home, ".local/share/nvim/mason/packages"),
		filepath.Join(c.home, ".local/share/nvim/mason/registries"),
	}
	fixed := 0
	for _, root := range scanRoots {
		if !fileExists(root) {
			continue
		}
		err := filepath.WalkDir(root, func(p string, d os.DirEntry, err error) error {
			if err != nil || d.IsDir() {
				return nil
			}
			if !d.Type().IsRegular() {
				return nil
			}
			fi, err := d.Info()
			if err != nil || fi.Size() > 2<<20 {
				return nil
			}
			data, err := os.ReadFile(p)
			if err != nil || bytes.IndexByte(data[:min(len(data), 8192)], 0) >= 0 {
				return nil // 二进制，跳过
			}
			if !bytes.Contains(data, []byte(from)) {
				return nil
			}
			fixed++
			if c.dry {
				return nil
			}
			return os.WriteFile(p, bytes.ReplaceAll(data, []byte(from), []byte(to)), fi.Mode().Perm())
		})
		if err != nil {
			return fixed, err
		}
	}
	return fixed, nil
}

func (c *applyCtx) updateBashrc() error {
	path := filepath.Join(c.home, ".bashrc")
	block := strings.Join([]string{
		blockStart,
		`export PATH="$HOME/.local/bin:$HOME/.local/opt/node/bin:$PATH"`,
		`export EDITOR=nvim`,
		`export VISUAL=nvim`,
		`alias lg=lazygit`,
		`alias y=yazi`,
		blockEnd,
		"",
	}, "\n")

	old, err := os.ReadFile(path)
	hadFile := err == nil
	content := string(old)

	// 幂等：先移除旧 block
	if i := strings.Index(content, blockStart); i >= 0 {
		if j := strings.Index(content, blockEnd); j > i {
			content = content[:i] + content[j+len(blockEnd):]
		}
	}
	content = strings.TrimRight(content, "\n")
	var newContent string
	if content == "" {
		newContent = block
	} else {
		newContent = content + "\n\n" + block
	}

	if c.dry {
		infof("将更新 %s 的 devkit 块", path)
		return nil
	}
	if hadFile && newContent != string(old) {
		// 首次写入（之前没有 devkit 块）时留个备份
		if !strings.Contains(string(old), blockStart) {
			if err := os.MkdirAll(c.backup, 0o755); err == nil {
				os.WriteFile(filepath.Join(c.backup, "bashrc.bak"), old, 0o644)
			}
		}
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	if err := os.WriteFile(path, []byte(newContent), 0o644); err != nil {
		return err
	}
	okf("%s 的 devkit 块已写入（PATH/EDITOR/别名）", path)
	return nil
}

func (c *applyCtx) chownAll() error {
	uid, err := strconv.Atoi(c.owner.Uid)
	if err != nil {
		return err
	}
	gid, err := strconv.Atoi(c.owner.Gid)
	if err != nil {
		return err
	}
	targets := []string{
		filepath.Join(c.home, ".config/nvim"),
		filepath.Join(c.home, ".config/lazygit"),
		filepath.Join(c.home, ".config/yazi"),
		filepath.Join(c.home, ".local/share/nvim"),
		filepath.Join(c.home, ".local/opt"),
		filepath.Join(c.home, ".local/bin"),
	}
	if c.dry {
		return nil
	}
	for _, root := range targets {
		if !fileExists(root) {
			continue
		}
		err := filepath.WalkDir(root, func(p string, d os.DirEntry, err error) error {
			if err != nil {
				return nil
			}
			return os.Lchown(p, uid, gid)
		})
		if err != nil {
			return fmt.Errorf("chown %s: %w", root, err)
		}
	}
	return nil
}

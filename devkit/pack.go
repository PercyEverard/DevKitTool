package main

import (
	"archive/tar"
	"archive/zip"
	"compress/gzip"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"os/user"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"syscall"
	"time"
)

const (
	nodeTag         = "v22.23.2"
	nvimFallbackTag = "v0.11.6"
	minFreeBytes    = 3 << 30
)

// 随包携带的 mason 包白名单（其余一律剔除，省体积）。
var masonKeep = []string{
	"clangd", "gopls", "lua-language-server", "asm-lsp",
	"bash-language-server", "pyright", "typescript-language-server",
	"prettier", "stylua", "shfmt",
}

// --trim-web 时跳过的三个 Web LSP（约 280MB）。
var masonWeb = []string{"css-lsp", "html-lsp", "json-lsp"}

var nvimReleaseRe = regexp.MustCompile(`^v\d+\.\d+\.\d+$`)

func step(idx, format string, a ...any) {
	fmt.Printf("\n▍%s %s\n", idx, fmt.Sprintf(format, a...))
}

func okf(format string, a ...any) {
	fmt.Printf("  ✓ "+format+"\n", a...)
}

func infof(format string, a ...any) {
	fmt.Printf("  · "+format+"\n", a...)
}

func warnf(format string, a ...any) {
	fmt.Printf("  ! "+format+"\n", a...)
}

func isTTY(f *os.File) bool {
	fi, err := f.Stat()
	return err == nil && fi.Mode()&os.ModeCharDevice != 0
}

func fileExists(p string) bool {
	_, err := os.Stat(p)
	return err == nil
}

func humanBytes(n int64) string { return humanSize(n) }

// findSourceDir 定位 devkit 源码目录（含 go.mod）。优先 $DEVKIT_SRC，
// 其次是可执行文件所在目录，最后是当前工作目录。
func findSourceDir() (string, error) {
	if d := os.Getenv("DEVKIT_SRC"); d != "" {
		return d, nil
	}
	if exe, err := os.Executable(); err == nil {
		d := filepath.Dir(exe)
		if fileExists(filepath.Join(d, "go.mod")) {
			return d, nil
		}
	}
	if wd, err := os.Getwd(); err == nil && fileExists(filepath.Join(wd, "go.mod")) {
		return wd, nil
	}
	return "", errors.New("找不到 devkit 源码目录（没有 go.mod），请设置 DEVKIT_SRC 或在源码目录内运行")
}

func diskFree(path string) (int64, error) {
	var st syscall.Statfs_t
	if err := syscall.Statfs(path, &st); err != nil {
		return 0, err
	}
	return int64(st.Bavail) * int64(st.Bsize), nil
}

func detectNvimTag() string {
	out, err := exec.Command("nvim", "--version").Output()
	if err != nil {
		return nvimFallbackTag
	}
	line := strings.SplitN(string(out), "\n", 2)[0]
	fields := strings.Fields(line) // NVIM v0.11.6
	if len(fields) >= 2 && nvimReleaseRe.MatchString(fields[1]) {
		return fields[1]
	}
	return nvimFallbackTag
}

func hostAndUser() (string, string) {
	h, _ := os.Hostname()
	u := "unknown"
	if cu, err := user.Current(); err == nil {
		u = cu.Username
	}
	return h, u
}

// ---------- 下载 ----------

func (c *packCtx) latestTag(repo string) (string, error) {
	req, err := http.NewRequest(http.MethodHead, "https://github.com/"+repo+"/releases/latest", nil)
	if err != nil {
		return "", err
	}
	client := &http.Client{
		Timeout: 60 * time.Second,
		CheckRedirect: func(*http.Request, []*http.Request) error {
			return http.ErrUseLastResponse
		},
	}
	resp, err := client.Do(req)
	if err != nil {
		return "", fmt.Errorf("访问 github.com 失败（代理是否可用？）: %w", err)
	}
	resp.Body.Close()
	loc := resp.Header.Get("Location")
	if loc == "" {
		return "", fmt.Errorf("没有拿到 %s 的最新版本跳转", repo)
	}
	tag := loc[strings.LastIndex(loc, "/")+1:]
	if tag == "" {
		return "", fmt.Errorf("无法从 %q 解析版本号", loc)
	}
	return tag, nil
}

type artifact struct {
	key   string // 代号，决定如何解包
	label string
	ver   string
	url   string
	file  string // 缓存文件完整路径
}

type progressReader struct {
	r     io.Reader
	label string
	total int64
	read  int64
	last  time.Time
	show  bool
}

func (p *progressReader) Read(b []byte) (int, error) {
	n, err := p.r.Read(b)
	p.read += int64(n)
	if p.show && time.Since(p.last) > 2*time.Second {
		fmt.Fprintf(os.Stderr, "\r    下载 %s … %s", p.label, humanBytes(p.read))
		p.last = time.Now()
	}
	return n, err
}

func (c *packCtx) ensure(a *artifact) error {
	dest := filepath.Join(c.cache, filepath.Base(a.url))
	a.file = dest
	if fi, err := os.Stat(dest); err == nil && fi.Size() > 0 {
		infof("%s %s（使用缓存 %s）", a.label, a.ver, humanBytes(fi.Size()))
		return nil
	}

	fmt.Printf("  ↓ %s %s\n", a.label, a.ver)
	var lastErr error
	for attempt := 1; attempt <= 2; attempt++ {
		lastErr = c.download(a.url, dest, a.label)
		if lastErr == nil {
			return nil
		}
		if attempt == 1 {
			warnf("下载失败（%v），重试一次…", lastErr)
			time.Sleep(2 * time.Second)
		}
	}
	return fmt.Errorf("下载 %s 失败: %w", a.label, lastErr)
}

func (c *packCtx) download(url, dest, label string) error {
	resp, err := c.client.Get(url)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("HTTP %s", resp.Status)
	}

	tmp := dest + ".part"
	f, err := os.Create(tmp)
	if err != nil {
		return err
	}
	defer os.Remove(tmp)

	pr := &progressReader{r: resp.Body, label: label, total: resp.ContentLength, last: time.Now(), show: isTTY(os.Stderr)}
	n, err := io.Copy(f, pr)
	closeErr := f.Close()
	if err == nil {
		err = closeErr
	}
	if err != nil {
		return err
	}
	if resp.ContentLength > 0 && n != resp.ContentLength {
		return fmt.Errorf("下载不完整: %d/%d 字节", n, resp.ContentLength)
	}
	if pr.show {
		fmt.Fprintf(os.Stderr, "\r    下载 %s … %s\n", label, humanBytes(n))
	}
	if err := os.Rename(tmp, dest); err != nil {
		return err
	}
	okf("%s（%s）", label, humanBytes(n))
	return nil
}

// ---------- 解包 ----------

func safeJoin(dest, name string) (string, error) {
	clean := filepath.Clean(name)
	if clean == "." || strings.HasPrefix(clean, ".."+string(os.PathSeparator)) || clean == ".." || filepath.IsAbs(clean) {
		return "", fmt.Errorf("tar 内存在可疑路径: %q", name)
	}
	return filepath.Join(dest, clean), nil
}

// extractStrip 解包 tar.gz 并去掉最外层目录（nvim / node 官方包结构）。
func extractStrip(tgz, destDir string, strip int) error {
	f, err := os.Open(tgz)
	if err != nil {
		return err
	}
	defer f.Close()
	gz, err := gzip.NewReader(f)
	if err != nil {
		return err
	}
	defer gz.Close()

	var dirModes []struct {
		path string
		mode os.FileMode
	}
	tr := tar.NewReader(gz)
	for {
		hdr, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return err
		}
		name := strings.TrimPrefix(hdr.Name, "./")
		parts := strings.Split(name, "/")
		if len(parts) <= strip {
			continue
		}
		rel := strings.Join(parts[strip:], "/")
		if rel == "" || strings.HasPrefix(rel, "pax_global_header") {
			continue
		}
		dst, err := safeJoin(destDir, rel)
		if err != nil {
			return err
		}
		switch hdr.Typeflag {
		case tar.TypeDir:
			if err := os.MkdirAll(dst, 0o755); err != nil {
				return err
			}
			dirModes = append(dirModes, struct {
				path string
				mode os.FileMode
			}{dst, os.FileMode(hdr.Mode) & 0o777})
		case tar.TypeReg:
			if err := os.MkdirAll(filepath.Dir(dst), 0o755); err != nil {
				return err
			}
			out, err := os.OpenFile(dst, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, os.FileMode(hdr.Mode)&0o777)
			if err != nil {
				return err
			}
			if _, err := io.Copy(out, tr); err != nil {
				out.Close()
				return err
			}
			if err := out.Close(); err != nil {
				return err
			}
		case tar.TypeSymlink:
			link := hdr.Linkname
			if strings.HasPrefix(link, parts[0]+"/") {
				link = strings.TrimPrefix(link, parts[0]+"/")
			}
			os.Remove(dst)
			if err := os.MkdirAll(filepath.Dir(dst), 0o755); err != nil {
				return err
			}
			if err := os.Symlink(link, dst); err != nil {
				return err
			}
		case tar.TypeLink:
			target, err := safeJoin(destDir, strings.Join(strings.Split(strings.TrimPrefix(hdr.Linkname, "./"), "/")[strip:], "/"))
			if err != nil {
				return err
			}
			os.Remove(dst)
			if err := os.Link(target, dst); err != nil {
				return err
			}
		}
	}
	for _, d := range dirModes {
		os.Chmod(d.path, d.mode)
	}
	return nil
}

// extractFind 在 tar.gz 里找名为 want 的普通文件，解到 destDir/want。
func extractFind(tgz, want, destDir string) error {
	f, err := os.Open(tgz)
	if err != nil {
		return err
	}
	defer f.Close()
	gz, err := gzip.NewReader(f)
	if err != nil {
		return err
	}
	defer gz.Close()

	var names []string
	tr := tar.NewReader(gz)
	for {
		hdr, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return err
		}
		if hdr.Typeflag != tar.TypeReg {
			continue
		}
		base := filepath.Base(hdr.Name)
		if len(names) < 20 {
			names = append(names, hdr.Name)
		}
		if base != want {
			continue
		}
		dst := filepath.Join(destDir, want)
		out, err := os.OpenFile(dst, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, os.FileMode(hdr.Mode)&0o777|0o100)
		if err != nil {
			return err
		}
		if _, err := io.Copy(out, tr); err != nil {
			out.Close()
			return err
		}
		if err := out.Close(); err != nil {
			return err
		}
		return os.Chmod(dst, 0o755)
	}
	return fmt.Errorf("%s 里没找到 %q，实际内容: %s", filepath.Base(tgz), want, strings.Join(names, ", "))
}

// extractZipFind 在 zip 里找名为 want 的文件（yazi 的 musl 包是 zip）。
func extractZipFind(zipPath, want, destDir string) error {
	zr, err := zip.OpenReader(zipPath)
	if err != nil {
		return err
	}
	defer zr.Close()
	var names []string
	for _, zf := range zr.File {
		if len(names) < 20 {
			names = append(names, zf.Name)
		}
		if filepath.Base(zf.Name) != want || zf.FileInfo().IsDir() {
			continue
		}
		rc, err := zf.Open()
		if err != nil {
			return err
		}
		dst := filepath.Join(destDir, want)
		out, err := os.OpenFile(dst, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, os.FileMode(zf.Mode())&0o777|0o100)
		if err != nil {
			rc.Close()
			return err
		}
		if _, err := io.Copy(out, rc); err != nil {
			out.Close()
			rc.Close()
			return err
		}
		out.Close()
		rc.Close()
		return os.Chmod(dst, 0o755)
	}
	return fmt.Errorf("%s 里没找到 %q，实际内容: %s", filepath.Base(zipPath), want, strings.Join(names, ", "))
}

// ---------- 写 payload ----------

type tarEntry struct {
	src string // 磁盘路径（文件 / 目录 / 符号链接）
	dst string // tar 内的相对路径
}

func writePayload(dest string, items []tarEntry) (fileCount, rawBytes int64, err error) {
	f, err := os.Create(dest)
	if err != nil {
		return 0, 0, err
	}
	defer f.Close()
	gz, err := gzip.NewWriterLevel(f, gzip.BestCompression)
	if err != nil {
		return 0, 0, err
	}
	tw := tar.NewWriter(gz)

	var walk func(src, dst string) error
	walk = func(src, dst string) error {
		fi, err := os.Lstat(src)
		if err != nil {
			return err
		}
		switch {
		case fi.IsDir():
			hdr := &tar.Header{
				Name:     dst + "/",
				Typeflag: tar.TypeDir,
				Mode:     int64(fi.Mode().Perm()),
				ModTime:  fi.ModTime(),
				Uid:      0,
				Gid:      0,
			}
			if err := tw.WriteHeader(hdr); err != nil {
				return err
			}
			entries, err := os.ReadDir(src)
			if err != nil {
				return err
			}
			sort.Slice(entries, func(i, j int) bool { return entries[i].Name() < entries[j].Name() })
			for _, e := range entries {
				if err := walk(filepath.Join(src, e.Name()), dst+"/"+e.Name()); err != nil {
					return err
				}
			}
			return nil
		case fi.Mode()&os.ModeSymlink != 0:
			link, err := os.Readlink(src)
			if err != nil {
				return err
			}
			hdr := &tar.Header{
				Name:     dst,
				Typeflag: tar.TypeSymlink,
				Linkname: link,
				Mode:     0o777,
				ModTime:  fi.ModTime(),
				Uid:      0,
				Gid:      0,
			}
			return tw.WriteHeader(hdr)
		case fi.Mode().IsRegular():
			hdr := &tar.Header{
				Name:     dst,
				Typeflag: tar.TypeReg,
				Mode:     int64(fi.Mode().Perm()),
				Size:     fi.Size(),
				ModTime:  fi.ModTime(),
				Uid:      0,
				Gid:      0,
			}
			if err := tw.WriteHeader(hdr); err != nil {
				return err
			}
			in, err := os.Open(src)
			if err != nil {
				return err
			}
			if _, err := io.Copy(tw, in); err != nil {
				in.Close()
				return err
			}
			in.Close()
			fileCount++
			rawBytes += fi.Size()
			return nil
		default:
			warnf("跳过特殊文件: %s", src)
			return nil
		}
	}

	for _, it := range items {
		if err := walk(it.src, it.dst); err != nil {
			return 0, 0, fmt.Errorf("打包 %s: %w", it.src, err)
		}
	}
	if err := tw.Close(); err != nil {
		return 0, 0, err
	}
	if err := gz.Close(); err != nil {
		return 0, 0, err
	}
	return fileCount, rawBytes, f.Close()
}

// ---------- pack 主流程 ----------

type packCtx struct {
	src     string
	home    string
	stage   string
	cache   string
	embed   string
	trimWeb bool
	client  *http.Client
}

func cmdPack(args []string) error {
	fs := flag.NewFlagSet("pack", flag.ContinueOnError)
	outName := fs.String("out", "devkit", "输出文件名")
	trimWeb := fs.Bool("trim-web", false, "不打包 HTML/CSS/JSON 的 LSP 服务器（省约 280MB）")
	parseFlags(fs, args)

	c := &packCtx{trimWeb: *trimWeb, client: &http.Client{Timeout: 30 * time.Minute}}

	// ---- 1) 预检 ----
	step("1/7", "预检")
	src, err := findSourceDir()
	if err != nil {
		return err
	}
	c.src = src
	infof("源码目录: %s", src)

	me, err := currentUser()
	if err != nil {
		return err
	}
	home, err := resolveHome("", me)
	if err != nil {
		return err
	}
	c.home = home
	infof("运行身份: %s（uid %s），家目录: %s", me.Username, me.Uid, home)

	if _, err := exec.LookPath("go"); err != nil {
		return errors.New("找不到 go 命令（pack 需要 Go 工具链）")
	}
	nvimCfg := filepath.Join(home, ".config/nvim")
	lazyDir := filepath.Join(home, ".local/share/nvim/lazy")
	masonDir := filepath.Join(home, ".local/share/nvim/mason")
	if !fileExists(filepath.Join(nvimCfg, "lazy-lock.json")) {
		return fmt.Errorf("没有找到 %s/lazy-lock.json，请先在该机器上配好 nvim 环境", nvimCfg)
	}
	if !fileExists(lazyDir) {
		return fmt.Errorf("没有找到 %s", lazyDir)
	}
	if !fileExists(masonDir) {
		return fmt.Errorf("没有找到 %s", masonDir)
	}
	nvimTag := detectNvimTag()
	okf("本机 nvim %s", nvimTag)

	free, err := diskFree(src)
	if err != nil {
		return err
	}
	if free < minFreeBytes {
		return fmt.Errorf("磁盘可用空间不足 3GB（当前 %s）", humanBytes(free))
	}
	okf("磁盘可用 %s", humanBytes(free))

	// ---- 2) 构建目录 ----
	step("2/7", "准备构建目录")
	buildDir := filepath.Join(src, "build")
	c.stage = filepath.Join(buildDir, "stage")
	c.cache = filepath.Join(buildDir, "cache")
	c.embed = filepath.Join(src, "embed")
	if err := os.RemoveAll(c.stage); err != nil {
		return err
	}
	for _, d := range []string{
		filepath.Join(c.stage, "config"),
		filepath.Join(c.stage, "data"),
		filepath.Join(c.stage, "opt"),
		filepath.Join(c.stage, "bin"),
		c.cache,
		c.embed,
	} {
		if err := os.MkdirAll(d, 0o755); err != nil {
			return err
		}
	}
	okf("%s", buildDir)

	// ---- 3) 下载外部组件 ----
	step("3/7", "下载外部组件（走 HTTP_PROXY / HTTPS_PROXY）")

	rgTag, err := c.latestTag("BurntSushi/ripgrep")
	if err != nil {
		return err
	}
	fdTag, err := c.latestTag("sharkdp/fd")
	if err != nil {
		return err
	}
	fzfTag, err := c.latestTag("junegunn/fzf")
	if err != nil {
		return err
	}
	ruffTag, err := c.latestTag("astral-sh/ruff")
	if err != nil {
		return err
	}
	yaziTag, err := c.latestTag("sxyazi/yazi")
	if err != nil {
		return err
	}
	gh := func(repo, tag, asset string) string {
		return fmt.Sprintf("https://github.com/%s/releases/download/%s/%s", repo, tag, asset)
	}
	arts := []*artifact{
		{key: "nvim", label: "Neovim", ver: nvimTag,
			url: fmt.Sprintf("https://github.com/neovim/neovim/releases/download/%s/nvim-linux-x86_64.tar.gz", nvimTag)},
		{key: "node", label: "Node.js", ver: nodeTag,
			url: fmt.Sprintf("https://nodejs.org/dist/%s/node-%s-linux-x64.tar.gz", nodeTag, nodeTag)},
		{key: "ripgrep", label: "ripgrep", ver: rgTag,
			url: gh("BurntSushi/ripgrep", rgTag, fmt.Sprintf("ripgrep-%s-x86_64-unknown-linux-musl.tar.gz", rgTag))},
		{key: "fd", label: "fd", ver: strings.TrimPrefix(fdTag, "v"),
			url: gh("sharkdp/fd", fdTag, fmt.Sprintf("fd-%s-x86_64-unknown-linux-musl.tar.gz", fdTag))},
		{key: "fzf", label: "fzf", ver: strings.TrimPrefix(fzfTag, "v"),
			url: gh("junegunn/fzf", fzfTag, fmt.Sprintf("fzf-%s-linux_amd64.tar.gz", strings.TrimPrefix(fzfTag, "v")))},
		{key: "ruff", label: "ruff（black 替代）", ver: ruffTag,
			url: gh("astral-sh/ruff", ruffTag, "ruff-x86_64-unknown-linux-musl.tar.gz")},
		{key: "yazi", label: "yazi", ver: strings.TrimPrefix(yaziTag, "v"),
			url: gh("sxyazi/yazi", yaziTag, "yazi-x86_64-unknown-linux-musl.zip")},
	}
	for _, a := range arts {
		if err := c.ensure(a); err != nil {
			return err
		}
	}
	artByKey := map[string]*artifact{}
	for _, a := range arts {
		artByKey[a.key] = a
	}

	// ---- 4) 收集本地内容 ----
	step("4/7", "收集本地内容")

	// 4.1 mason 包
	keptPkgs := append([]string{}, masonKeep...)
	if !c.trimWeb {
		keptPkgs = append(keptPkgs, masonWeb...)
	}
	for _, p := range keptPkgs {
		if !fileExists(filepath.Join(masonDir, "packages", p)) {
			return fmt.Errorf("mason 缺包 %s，请先在 Fedora 上用 :Mason 安装", p)
		}
	}
	sort.Strings(keptPkgs)
	infof("mason 包 %d 个: %s", len(keptPkgs), strings.Join(keptPkgs, ", "))

	// 4.2 mason/bin：只保留指向白名单包的相对软链
	type binLink struct{ name, target string }
	var binLinks []binLink
	entries, err := os.ReadDir(filepath.Join(masonDir, "bin"))
	if err != nil {
		return err
	}
	keepSet := map[string]bool{}
	for _, p := range keptPkgs {
		keepSet[p] = true
	}
	for _, e := range entries {
		p := filepath.Join(masonDir, "bin", e.Name())
		fi, err := os.Lstat(p)
		if err != nil || fi.Mode()&os.ModeSymlink == 0 {
			continue
		}
		target, err := os.Readlink(p)
		if err != nil {
			continue
		}
		parts := strings.Split(target, "/")
		if len(parts) >= 3 && parts[0] == ".." && parts[1] == "packages" && keepSet[parts[2]] {
			binLinks = append(binLinks, binLink{e.Name(), target})
		}
	}
	sort.Slice(binLinks, func(i, j int) bool { return binLinks[i].name < binLinks[j].name })
	infof("mason/bin 软链 %d 个", len(binLinks))

	// 4.3 clang-format：从 pip wheel 里取真 ELF（venv 的 python 入口跨版本不可用）
	cfSrc := ""
	cfGlob, _ := filepath.Glob(filepath.Join(masonDir, "packages/clang-format/venv/lib/python*/site-packages/clang_format/data/bin/clang-format"))
	if len(cfGlob) > 0 {
		cfSrc = cfGlob[0]
	}
	if cfSrc == "" {
		return errors.New("找不到 clang-format 的 ELF 二进制（mason 里未安装 clang-format？）")
	}
	cfVer := "clang-format"
	if out, err := exec.Command(cfSrc, "--version").Output(); err == nil {
		fields := strings.Fields(strings.TrimSpace(string(out)))
		if len(fields) > 0 {
			cfVer = fields[len(fields)-1]
		}
	}

	// 4.4 lazygit（本地静态二进制）
	lazygitSrc := "/usr/local/bin/lazygit"
	if !fileExists(lazygitSrc) {
		if p, err := exec.LookPath("lazygit"); err == nil {
			lazygitSrc = p
		} else {
			return errors.New("找不到 lazygit")
		}
	}
	lazygitVer := "lazygit"
	if out, err := exec.Command(lazygitSrc, "--version").Output(); err == nil {
		for _, f := range strings.Fields(string(out)) {
			if v, ok := strings.CutPrefix(f, "version="); ok {
				lazygitVer = strings.TrimSuffix(strings.TrimSuffix(v, ","), "\n")
				break // 后面的 version= 是内部 git 的版本，不能要
			}
		}
	}
	infof("本地收集: lazygit %s / clang-format %s", lazygitVer, cfVer)

	// ---- 5) 组装 stage/bin + opt ----
	step("5/7", "解包与组装")
	if err := extractStrip(artByKey["nvim"].file, filepath.Join(c.stage, "opt/nvim"), 1); err != nil {
		return err
	}
	okf("opt/nvim")
	if err := extractStrip(artByKey["node"].file, filepath.Join(c.stage, "opt/node"), 1); err != nil {
		return err
	}
	okf("opt/node")
	for _, x := range []struct{ key, want string }{
		{"ripgrep", "rg"}, {"fd", "fd"}, {"fzf", "fzf"}, {"ruff", "ruff"},
	} {
		if err := extractFind(artByKey[x.key].file, x.want, filepath.Join(c.stage, "bin")); err != nil {
			return err
		}
		okf("bin/%s", x.want)
	}
	if err := extractZipFind(artByKey["yazi"].file, "yazi", filepath.Join(c.stage, "bin")); err != nil {
		return err
	}
	okf("bin/yazi")

	binDir := filepath.Join(c.stage, "bin")
	if err := copyFile(cfSrc, filepath.Join(binDir, "clang-format")); err != nil {
		return err
	}
	okf("bin/clang-format")
	if err := copyFile(lazygitSrc, filepath.Join(binDir, "lazygit")); err != nil {
		return err
	}
	okf("bin/lazygit")

	blackWrapper := "#!/bin/sh\n# black 兼容包装：离线环境用随包的 ruff format 代替\nexec \"$(dirname \"$0\")/ruff\" format \"$@\"\n"
	if err := os.WriteFile(filepath.Join(binDir, "black"), []byte(blackWrapper), 0o755); err != nil {
		return err
	}
	okf("bin/black（ruff format 包装）")

	if err := os.Symlink("../opt/nvim/bin/nvim", filepath.Join(binDir, "nvim")); err != nil {
		return err
	}
	if err := os.Symlink("../opt/node/bin/node", filepath.Join(binDir, "node")); err != nil {
		return err
	}
	okf("bin/nvim, bin/node（相对软链）")

	// ---- 6) 打 payload + manifest ----
	step("6/7", "生成 payload.tar.gz 与 manifest.json")

	items := []tarEntry{
		{filepath.Join(home, ".config/nvim"), "config/nvim"},
		{filepath.Join(home, ".config/lazygit"), "config/lazygit"},
		{filepath.Join(home, ".config/yazi"), "config/yazi"},
		{lazyDir, "data/lazy"},
		{filepath.Join(masonDir, "registries"), "data/mason/registries"},
		{filepath.Join(masonDir, "share/mason-schemas"), "data/mason/share/mason-schemas"},
	}
	for _, p := range keptPkgs {
		items = append(items, tarEntry{filepath.Join(masonDir, "packages", p), "data/mason/packages/" + p})
	}
	for _, bl := range binLinks {
		items = append(items, tarEntry{filepath.Join(masonDir, "bin", bl.name), "data/mason/bin/" + bl.name})
	}
	items = append(items,
		tarEntry{filepath.Join(c.stage, "opt/nvim"), "opt/nvim"},
		tarEntry{filepath.Join(c.stage, "opt/node"), "opt/node"},
		tarEntry{filepath.Join(c.stage, "bin"), "bin"},
	)

	payloadTmp := filepath.Join(buildDir, "payload.tar.gz")
	t0 := time.Now()
	fileCount, rawBytes, err := writePayload(payloadTmp, items)
	if err != nil {
		return err
	}
	pfi, err := os.Stat(payloadTmp)
	if err != nil {
		return err
	}
	okf("%d 个文件，原始 %s → 压缩 %s（耗时 %s）",
		fileCount, humanBytes(rawBytes), humanBytes(pfi.Size()), time.Since(t0).Round(time.Second))

	host, uname := hostAndUser()
	pluginCount := 0
	if des, err := os.ReadDir(lazyDir); err == nil {
		for _, d := range des {
			if d.IsDir() {
				pluginCount++
			}
		}
	}
	m := &Manifest{
		CreatedAt:   time.Now().Format("2006-01-02 15:04:05"),
		SourceHost:  host,
		SourceUser:  uname,
		NvimVersion: nvimTag,
		FileCount:   fileCount,
		RawBytes:    rawBytes,
		PayloadSize: pfi.Size(),
		Components: []Component{
			{Name: "Neovim", Version: nvimTag, Path: "opt/nvim", Note: "编辑器本体（含 runtime）"},
			{Name: "nvim 配置", Version: fmt.Sprintf("%d 个 lazy 插件", pluginCount), Path: "config/nvim", Note: "init.lua + lua/ + lazy-lock.json"},
			{Name: "lazy.nvim 插件", Version: fmt.Sprintf("%d 个", pluginCount), Path: "data/lazy", Note: "lazy-lock.json 锁定版本"},
			{Name: "LSP 服务器", Version: fmt.Sprintf("%d 个", len(keptPkgs)), Path: "data/mason", Note: strings.Join(keptPkgs, ", ")},
			{Name: "Node.js", Version: nodeTag, Path: "opt/node", Note: "供 pyright / ts_ls / css / html / json LSP 使用"},
			{Name: "ripgrep", Version: rgTag, Path: "bin/rg", Note: "静态（musl）"},
			{Name: "fd", Version: strings.TrimPrefix(fdTag, "v"), Path: "bin/fd", Note: "静态（musl）"},
			{Name: "fzf", Version: strings.TrimPrefix(fzfTag, "v"), Path: "bin/fzf", Note: "静态"},
			{Name: "ruff", Version: ruffTag, Path: "bin/ruff", Note: "静态（musl），black 的替代"},
			{Name: "clang-format", Version: cfVer, Path: "bin/clang-format", Note: "从 pip wheel 提取的真 ELF"},
			{Name: "lazygit", Version: lazygitVer, Path: "bin/lazygit", Note: "静态"},
			{Name: "yazi", Version: strings.TrimPrefix(yaziTag, "v"), Path: "bin/yazi", Note: "静态（musl）"},
		},
	}
	mj, err := json.MarshalIndent(m, "", "  ")
	if err != nil {
		return err
	}
	mj = append(mj, '\n')

	payloadDst := filepath.Join(c.embed, "payload.tar.gz")
	if err := os.Rename(payloadTmp, payloadDst); err != nil {
		// 跨文件系统时退化为拷贝
		if err := copyFile(payloadTmp, payloadDst); err != nil {
			return err
		}
		os.Remove(payloadTmp)
	}
	if err := os.WriteFile(filepath.Join(c.embed, "manifest.json"), mj, 0o644); err != nil {
		return err
	}
	okf("embed/payload.tar.gz + embed/manifest.json")

	// ---- 7) 编译 ----
	step("7/7", "编译单文件工具")
	wd, err := os.Getwd()
	if err != nil {
		return err
	}
	outPath := *outName
	if !filepath.IsAbs(outPath) {
		outPath = filepath.Join(wd, outPath)
	}
	tmpOut := outPath + ".new"
	payloadSize := int64(0)
	if pfi, err := os.Stat(payloadDst); err == nil {
		payloadSize = pfi.Size()
	}
	infof("go build -trimpath -ldflags \"-s -w\"")
	infof("嵌入 payload %s；通常需要 1~3 分钟，期间每 15 秒报一次进度", humanBytes(payloadSize))
	buildStart := time.Now()
	buildTick := make(chan struct{})
	go func() {
		t := time.NewTicker(15 * time.Second)
		defer t.Stop()
		for {
			select {
			case <-buildTick:
				return
			case <-t.C:
				fmt.Printf("  · 仍在编译… 已耗时 %s\n", time.Since(buildStart).Round(time.Second))
			}
		}
	}()
	cmd := exec.Command("go", "build", "-trimpath", "-ldflags", "-s -w", "-o", tmpOut, ".")
	cmd.Dir = src
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	buildErr := cmd.Run()
	close(buildTick)
	if buildErr != nil {
		os.Remove(tmpOut)
		return fmt.Errorf("go build 失败: %w", buildErr)
	}
	if err := os.Rename(tmpOut, outPath); err != nil {
		return err
	}
	ofi, err := os.Stat(outPath)
	if err != nil {
		return err
	}
	okf("%s（%s，编译耗时 %s）", outPath, humanBytes(ofi.Size()), time.Since(buildStart).Round(time.Second))

	fmt.Printf("\n打包完成。把这个文件拷到目标机（Ubuntu）后执行：\n")
	fmt.Printf("    ./%s apply && ./%s verify\n", filepath.Base(outPath), filepath.Base(outPath))
	return nil
}

func copyFile(src, dst string) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()
	fi, err := in.Stat()
	if err != nil {
		return err
	}
	out, err := os.OpenFile(dst, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, fi.Mode().Perm()|0o100)
	if err != nil {
		return err
	}
	if _, err := io.Copy(out, in); err != nil {
		out.Close()
		return err
	}
	return out.Close()
}

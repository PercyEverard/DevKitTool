package main

import (
	"encoding/json"
	"fmt"
	"text/tabwriter"
	"os"
)

// Component 描述打包内容里的一个组件（工具 / 服务器 / 一组文件）。
type Component struct {
	Name    string `json:"name"`
	Version string `json:"version,omitempty"`
	Path    string `json:"path,omitempty"`
	Note    string `json:"note,omitempty"`
}

// Manifest 是随包携带的清单，apply / verify / list 都读它。
type Manifest struct {
	Stub        bool        `json:"stub,omitempty"`
	CreatedAt   string      `json:"created_at,omitempty"`
	SourceHost  string      `json:"source_host,omitempty"`
	SourceUser  string      `json:"source_user,omitempty"`
	NvimVersion string      `json:"nvim_version,omitempty"`
	Components  []Component `json:"components,omitempty"`
	FileCount   int64       `json:"file_count,omitempty"`
	RawBytes    int64       `json:"raw_bytes,omitempty"`
	PayloadSize int64       `json:"payload_size,omitempty"`
}

func loadManifest() (*Manifest, error) {
	var m Manifest
	if err := json.Unmarshal(manifestJSON, &m); err != nil {
		return nil, fmt.Errorf("读取清单失败: %w", err)
	}
	return &m, nil
}

func cmdList(args []string) error {
	m, err := loadManifest()
	if err != nil {
		return err
	}
	if m.Stub || len(m.Components) == 0 {
		fmt.Println("本文件目前是占位状态，还没有打包任何环境。")
		fmt.Println("请在 Fedora 上执行：  ./devkit pack")
		return nil
	}

	fmt.Printf("打包时间:   %s\n", m.CreatedAt)
	fmt.Printf("来源机器:   %s（用户 %s）\n", m.SourceHost, m.SourceUser)
	fmt.Printf("Neovim:     %s\n", m.NvimVersion)
	fmt.Printf("文件数:     %d 个\n", m.FileCount)
	fmt.Printf("原始大小:   %s   压缩后: %s\n\n", humanSize(m.RawBytes), humanSize(m.PayloadSize))

	w := tabwriter.NewWriter(os.Stdout, 2, 4, 2, ' ', 0)
	fmt.Fprintln(w, "组件\t版本\t说明")
	fmt.Fprintln(w, "----\t----\t----")
	for _, c := range m.Components {
		ver := c.Version
		if ver == "" {
			ver = "-"
		}
		note := c.Note
		if note == "" {
			note = c.Path
		}
		fmt.Fprintf(w, "%s\t%s\t%s\n", c.Name, ver, note)
	}
	return w.Flush()
}

func humanSize(n int64) string {
	const unit = 1024
	if n < unit {
		return fmt.Sprintf("%d B", n)
	}
	div, exp := int64(unit), 0
	for v := n / unit; v >= unit; v /= unit {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.1f %cB", float64(n)/float64(div), "KMGT"[exp])
}

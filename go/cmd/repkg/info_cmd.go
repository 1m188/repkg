// 本文件提供 info 子命令的实现：PKG/TEX 文件元数据查看。
package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"

	"github.com/1m188/repkg-go/internal/pkgfile"
	"github.com/1m188/repkg-go/internal/tex"
)

// actionInfo 执行 info 命令。
func actionInfo(input string) {
	fileInfo, err := os.Stat(input)
	if err != nil {
		_, _ = fmt.Fprintf(os.Stderr, "输入路径不存在: %s\n", input)
		os.Exit(1) //nolint:revive // CLI 命令入口允许直接 os.Exit
	}

	if fileInfo.IsDir() {
		if texInfo {
			infoTexDirectory(input)
		} else {
			infoPkgDirectory(input)
		}
		_, _ = fmt.Println("完成")
		return
	}

	ext := strings.ToLower(filepath.Ext(input))
	switch ext {
	case ".pkg":
		infoPkg(input, filepath.Base(input))
	case extDotTex:
		infoTex(input)
	default:
		_, _ = fmt.Fprintf(os.Stderr, "不支持的文件扩展名: %s\n", ext)
	}
	_, _ = fmt.Println("完成")
}

// infoPkg 显示 PKG 文件信息。
//
//nolint:gocognit,gocyclo,cyclop,nestif // CLI 命令函数，逻辑流程清晰
func infoPkg(filePath, name string) {
	_, _ = fmt.Printf("\n### 包信息: %s\n", name)

	f, err := os.Open(filePath) // #nosec G304 -- CLI 工具打开用户指定文件
	if err != nil {
		_, _ = fmt.Fprintf(os.Stderr, "打开文件失败: %v\n", err)
		return
	}
	defer func() { _ = f.Close() }() //nolint:errcheck // defer 中文件关闭错误可忽略

	reader := pkgfile.NewReader()
	reader.ReadEntryBytes = false
	pkg, err := reader.ReadPackage(f)
	if err != nil {
		_, _ = fmt.Fprintf(os.Stderr, "读取 PKG 失败: %v\n", err)
		return
	}

	_, _ = fmt.Printf("格式: %s\n", pkg.Magic)
	_, _ = fmt.Printf("条目数: %d\n", len(pkg.Entries))

	// 读取 project.json（如果需要）
	var projectData map[string]any
	if projectInfo != "" || titleFilter != "" {
		projectPath := filepath.Join(filepath.Dir(filePath), "project.json")
		//nolint:gosec // projectPath 由 CLI 内部构造
		data, err := os.ReadFile(projectPath)
		if err != nil {
			_, _ = fmt.Fprintf(os.Stderr, "读取 project.json 失败: %v\n", err)
		} else {
			err = json.Unmarshal(data, &projectData)
			if err != nil {
				_, _ = fmt.Fprintf(os.Stderr, "解析 project.json 失败: %v\n", err)
				projectData = nil
			}
		}
	}

	// 标题过滤
	if titleFilter != "" {
		title, ok := projectData["title"].(string)
		if !ok {
			return
		}
		if !containsString(title, titleFilter) {
			return
		}
	}

	// 输出 project.json 信息
	if projectInfo != "" {
		if projectInfo == "*" {
			keys := getPropertyKeys(projectData)
			for _, k := range keys {
				_, _ = fmt.Printf("  %s: %v\n", k, projectData[k])
			}
		} else {
			requestedKeys := strings.Split(projectInfo, ",")
			for _, k := range requestedKeys {
				k = strings.TrimSpace(k)
				found := false
				for jsonKey, jsonVal := range projectData {
					if strings.EqualFold(jsonKey, k) {
						_, _ = fmt.Printf("  %s: %v\n", jsonKey, jsonVal)
						found = true
						break
					}
				}
				if !found {
					_, _ = fmt.Printf("  %s: (未找到)\n", k)
				}
			}
		}
	}

	if !printEntries {
		return
	}

	printPkgEntries(filePath)
}

// printPkgEntries 重新打开 PKG 文件并打印条目列表。
func printPkgEntries(filePath string) {
	f, err := os.Open(filePath) // #nosec G304 -- CLI 工具打开用户指定文件
	if err != nil {
		return
	}
	defer func() { _ = f.Close() }() //nolint:errcheck // defer 中文件关闭错误可忽略

	reader := pkgfile.NewReader()
	pkg, err := reader.ReadPackage(f)
	if err != nil {
		return
	}

	entries := pkg.Entries

	if sortEntries {
		switch sortBy {
		case "extension":
			slices.SortFunc(entries, func(a, b *pkgfile.Entry) int {
				return strings.Compare(filepath.Ext(a.FullPath), filepath.Ext(b.FullPath))
			})
		case "size":
			slices.SortFunc(entries, func(a, b *pkgfile.Entry) int {
				if a.Length < b.Length {
					return -1
				}
				if a.Length > b.Length {
					return 1
				}
				return 0
			})
		default: // name
			slices.SortFunc(entries, func(a, b *pkgfile.Entry) int {
				return strings.Compare(a.FullPath, b.FullPath)
			})
		}
	}

	_, _ = fmt.Println("包条目:")
	for _, entry := range entries {
		_, _ = fmt.Printf("* %s - %d 字节\n", entry.FullPath, entry.Length)
	}
}

// infoTex 显示 TEX 文件信息。
func infoTex(filePath string) {
	data, err := os.ReadFile(filePath) // #nosec G304 -- CLI 工具读取用户指定文件
	if err != nil {
		_, _ = fmt.Fprintf(os.Stderr, "读取 TEX 文件失败: %v\n", err)
		return
	}

	texReader := tex.NewReader()
	texData, err := texReader.ReadTex(readSeekerFromBytes(data))
	if err != nil {
		_, _ = fmt.Fprintf(os.Stderr, "解析 TEX 失败: %v\n", err)
		return
	}

	_, _ = fmt.Printf("\n### TEX 信息: %s\n", filePath)
	_, _ = fmt.Printf("格式: %d\n", int32(texData.Header.Format))
	_, _ = fmt.Printf("纹理尺寸: %d x %d\n", texData.Header.TextureWidth, texData.Header.TextureHeight)
	_, _ = fmt.Printf("图片尺寸: %d x %d\n", texData.Header.ImageWidth, texData.Header.ImageHeight)
	_, _ = fmt.Printf("标志位: 0x%X\n", texData.Header.Flags)
	_, _ = fmt.Printf("动画: %t\n", texData.IsGif())
	_, _ = fmt.Printf("视频: %t\n", texData.IsVideoTexture())

	if texData.IsGif() && texData.FrameInfoContainer != nil {
		_, _ = fmt.Printf("帧数: %d\n", len(texData.FrameInfoContainer.Frames))
		_, _ = fmt.Printf("GIF 尺寸: %d x %d\n", texData.FrameInfoContainer.GifWidth, texData.FrameInfoContainer.GifHeight)
	}
}

// infoPkgDirectory 批量显示目录下 PKG 的信息。
func infoPkgDirectory(dirPath string) {
	entries, err := os.ReadDir(dirPath)
	if err != nil {
		return
	}
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		subDir := filepath.Join(dirPath, entry.Name())
		pkgFiles, err := filepath.Glob(filepath.Join(subDir, "*.pkg"))
		if err != nil {
			_, _ = fmt.Fprintf(os.Stderr, "搜索 PKG 文件失败 (%s): %v\n", subDir, err)
			return
		}
		for _, pf := range pkgFiles {
			relPath, err := filepath.Rel(dirPath, pf)
			if err != nil {
				_, _ = fmt.Fprintf(os.Stderr, "计算相对路径失败: %v\n", err)
				continue
			}
			infoPkg(pf, relPath)
		}
	}
}

// infoTexDirectory 批量显示目录下 TEX 的信息。
func infoTexDirectory(dirPath string) {
	err := filepath.Walk(dirPath, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if info.IsDir() {
			return nil
		}
		if !strings.EqualFold(filepath.Ext(path), extDotTex) {
			return nil
		}
		if !recursive && filepath.Dir(path) != dirPath {
			return nil
		}
		if closing.Load() {
			return filepath.SkipAll
		}
		infoTex(path)
		return nil
	})
	if err != nil {
		_, _ = fmt.Fprintf(os.Stderr, "遍历目录失败: %v\n", err)
	}
}

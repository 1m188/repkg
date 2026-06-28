// 本文件提供 extract 子命令的实现：PKG 提取、TEX 转换、文件过滤和目录批量处理。
package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"slices"
	"strings"

	"github.com/1m188/repkg-go/internal/pkgfile"
	"github.com/1m188/repkg-go/internal/tex"
)

const (
	fileWritePerm = 0o644
	dirPerm       = 0o755
	extDotTex     = ".tex"
)

// gSkipExts 和 gOnlyExts 为包级别的扩展名过滤列表，
// 由 actionExtract 设置，供目录提取函数使用。
var (
	gSkipExts []string
	gOnlyExts []string
)

// actionExtract 执行 extract 命令。
func actionExtract(input string) {
	if outputDir == "" {
		var err error
		outputDir, err = os.Getwd()
		if err != nil {
			_, _ = fmt.Fprintln(os.Stderr, "获取当前目录失败:", err)
			os.Exit(1) //nolint:revive // CLI 命令入口允许直接 os.Exit
		}
	}

	// 规范化扩展名过滤
	var skipExts, onlyExtList []string
	if ignoreExts != "" {
		skipExts = normalizeExtensions(strings.Split(ignoreExts, ","))
	}
	if onlyExts != "" {
		onlyExtList = normalizeExtensions(strings.Split(onlyExts, ","))
	}
	// 设置包级别变量供目录提取函数使用
	gSkipExts = skipExts
	gOnlyExts = onlyExtList

	// 判断输入是文件还是目录
	fileInfo, err := os.Stat(input)
	if err != nil {
		_, _ = fmt.Fprintf(os.Stderr, "输入路径不存在: %s\n", input)
		os.Exit(1) //nolint:revive // CLI 命令入口允许直接 os.Exit
	}

	if fileInfo.IsDir() {
		if texDir {
			extractTexDirectory(input)
		} else {
			extractPkgDirectory(input)
		}
		_, _ = fmt.Println("完成")
		return
	}

	extractFile(input, skipExts, onlyExtList)
	_, _ = fmt.Println("完成")
}

// extractFile 处理单个文件。
func extractFile(filePath string, skipExts, onlyExtList []string) {
	ext := strings.ToLower(filepath.Ext(filePath))

	switch ext {
	case ".pkg":
		extractPkg(filePath, false, "", skipExts, onlyExtList)
	case extDotTex:
		extractTexFile(filePath)
	default:
		_, _ = fmt.Fprintf(os.Stderr, "不支持的文件扩展名: %s\n", ext)
	}
}

// extractPkg 提取 PKG 文件。
//
//nolint:revive // 函数参数命名保持与 C# 原版一致
func extractPkg(filePath string, appendFolderName bool, defaultProjectName string, skipExts, onlyExtList []string) {
	_, _ = fmt.Printf("\n### 提取包: %s\n", filePath)

	// #nosec G304 -- CLI 工具需要打开用户指定的文件
	f, err := os.Open(filePath)
	if err != nil {
		_, _ = fmt.Fprintf(os.Stderr, "打开文件失败: %v\n", err)
		return
	}
	defer func() { _ = f.Close() }() //nolint:errcheck // defer 中文件关闭错误可忽略

	reader := pkgfile.NewReader()
	pkg, err := reader.ReadPackage(f)
	if err != nil {
		_, _ = fmt.Fprintf(os.Stderr, "读取 PKG 失败: %v\n", err)
		return
	}

	// 确定输出目录
	outDir := outputDir
	if appendFolderName {
		dn := defaultProjectName
		if useName {
			if t := readProjectTitle(filePath); t != "" {
				dn = getSafeFilename(t)
			}
		}
		outDir = filepath.Join(outputDir, dn)
	}

	// 过滤条目
	entries := filterEntries(pkg.Entries, skipExts, onlyExtList)

	// 提取每个条目
	for _, entry := range entries {
		if closing.Load() {
			return
		}
		extractEntry(entry, outDir)
	}

	// 复制 project.json 和预览图
	if copyProject && !singleDir {
		copyProjectFiles(filePath, outDir)
	}
}

// readProjectTitle 从 PKG 文件所在目录的 project.json 中读取标题。
func readProjectTitle(pkgPath string) string {
	projectPath := filepath.Join(filepath.Dir(pkgPath), "project.json")
	//nolint:gosec // projectPath 由 CLI 内部构造
	data, err := os.ReadFile(projectPath)
	if err != nil {
		return ""
	}
	var projectData map[string]any
	err = json.Unmarshal(data, &projectData)
	if err != nil {
		return ""
	}
	title, ok := projectData["title"].(string)
	if !ok {
		return ""
	}
	return title
}

// copyProjectFiles 将 project.json 和预览图复制到输出目录。
func copyProjectFiles(pkgPath, outDir string) {
	projectPath := filepath.Join(filepath.Dir(pkgPath), "project.json")
	//nolint:gosec // projectPath 由 CLI 内部构造
	data, err := os.ReadFile(projectPath)
	if err != nil {
		_, _ = fmt.Fprintf(os.Stderr, "读取 project.json 失败: %v\n", err)
		return
	}

	destPath := filepath.Join(outDir, "project.json")
	if !overwrite {
		_, err = os.Stat(destPath)
		if err == nil {
			_, _ = fmt.Printf("* 跳过, 已存在: %s\n", destPath)
			return
		}
	}
	err = os.MkdirAll(filepath.Dir(destPath), dirPerm)
	if err != nil {
		_, _ = fmt.Fprintf(os.Stderr, "创建目录失败: %v\n", err)
		return
	}
	//nolint:gosec // destPath 由 CLI 内部构造
	err = os.WriteFile(destPath, data, fileWritePerm)
	if err != nil {
		_, _ = fmt.Fprintf(os.Stderr, "复制 project.json 失败: %v\n", err)
	} else {
		_, _ = fmt.Println("* 复制: project.json")
	}

	// 解析 preview 字段并复制预览图
	var projectData map[string]any
	err = json.Unmarshal(data, &projectData)
	if err != nil {
		return
	}
	preview, ok := projectData["preview"].(string)
	if !ok || preview == "" {
		return
	}

	previewPath := filepath.Join(filepath.Dir(pkgPath), preview)
	//nolint:gosec // previewPath 由 CLI 内部构造
	previewData, err := os.ReadFile(previewPath)
	if err != nil {
		_, _ = fmt.Fprintf(os.Stderr, "读取预览图失败 (%s): %v\n", preview, err)
		return
	}
	previewDest := filepath.Join(outDir, preview)
	if !overwrite {
		_, err = os.Stat(previewDest)
		if err == nil {
			_, _ = fmt.Printf("* 跳过, 已存在: %s\n", previewDest)
			return
		}
	}
	//nolint:gosec // previewDest 由 CLI 内部构造
	err = os.WriteFile(previewDest, previewData, fileWritePerm)
	if err != nil {
		_, _ = fmt.Fprintf(os.Stderr, "复制预览图失败 (%s): %v\n", preview, err)
	} else {
		_, _ = fmt.Printf("* 复制: %s\n", preview)
	}
}

// extractEntry 提取单个 PKG 条目。
func extractEntry(entry *pkgfile.Entry, outDir string) {
	if closing.Load() {
		return
	}
	var filePath string
	if singleDir {
		filePath = filepath.Join(outDir, filepath.Base(entry.FullPath))
	} else {
		dir := filepath.Dir(entry.FullPath)
		if dir == "." {
			filePath = filepath.Join(outDir, filepath.Base(entry.FullPath))
		} else {
			filePath = filepath.Join(outDir, dir, filepath.Base(entry.FullPath))
		}
	}

	// 创建目录
	err := os.MkdirAll(filepath.Dir(filePath), dirPerm)
	if err != nil {
		_, _ = fmt.Fprintf(os.Stderr, "创建目录失败 (%s): %v\n", filepath.Dir(filePath), err)
		return
	}

	// 检查文件是否已存在
	if !overwrite {
		_, err := os.Stat(filePath)
		if err == nil {
			_, _ = fmt.Printf("* 跳过, 已存在: %s\n", filePath)
			return
		}
	}

	// 写入文件
	_, _ = fmt.Printf("* 提取: %s\n", entry.FullPath)
	err = os.WriteFile(filePath, entry.Bytes, fileWritePerm)
	if err != nil {
		_, _ = fmt.Fprintf(os.Stderr, "写入文件失败 (%s): %v\n", filePath, err)
		return
	}

	// 如果不是 TEX 文件或禁用了转换，跳过转换
	if noTexConvert || entry.Type != pkgfile.EntryTypeTex {
		return
	}

	// 转换 TEX 为图片
	texReader := tex.NewReader()
	texData, err := texReader.ReadTex(bytes.NewReader(entry.Bytes))
	if err != nil {
		_, _ = fmt.Fprintf(os.Stderr, "读取 TEX 失败 (%s): %v\n", entry.FullPath, err)
		return
	}

	saveTexAsImage(texData, filePath[:len(filePath)-len(filepath.Ext(filePath))])
}

// extractTexFile 处理单个 TEX 文件。
func extractTexFile(filePath string) {
	if closing.Load() {
		return
	}
	// #nosec G304 -- CLI 工具需要读取用户指定的文件
	data, err := os.ReadFile(filePath)
	if err != nil {
		_, _ = fmt.Fprintf(os.Stderr, "读取 TEX 文件失败: %v\n", err)
		return
	}

	texReader := tex.NewReader()
	texData, err := texReader.ReadTex(readSeekerFromBytes(data))
	if err != nil {
		_, _ = fmt.Fprintf(os.Stderr, "解析 TEX 失败 (%s): %v\n", filePath, err)
		return
	}

	baseName := filepath.Base(filePath)
	baseName = baseName[:len(baseName)-len(filepath.Ext(baseName))]
	outPath := filepath.Join(outputDir, baseName)
	saveTexAsImage(texData, outPath)
}

// extractTexDirectory 批量转换目录下的 TEX 文件。
func extractTexDirectory(dirPath string) {
	err := os.MkdirAll(outputDir, dirPerm)
	if err != nil {
		_, _ = fmt.Fprintf(os.Stderr, "创建输出目录失败: %v\n", err)
		return
	}
	err = filepath.Walk(dirPath, func(path string, info os.FileInfo, err error) error {
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
		extractTexFile(path)
		return nil
	})
	if err != nil {
		_, _ = fmt.Fprintf(os.Stderr, "遍历目录失败: %v\n", err)
	}
}

// extractPkgDirectory 批量提取目录下的 PKG 文件。
func extractPkgDirectory(dirPath string) {
	if recursive {
		extractPkgRecursive(dirPath)
		return
	}
	extractPkgNonRecursive(dirPath)
}

// extractPkgRecursive 递归提取目录下的 PKG 文件。
func extractPkgRecursive(dirPath string) {
	walkFn := func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if info.IsDir() {
			return nil
		}
		if !strings.EqualFold(filepath.Ext(path), ".pkg") {
			return nil
		}
		if closing.Load() {
			return filepath.SkipAll
		}
		extractPkgByRelDir(dirPath, path)
		return nil
	}
	err := filepath.Walk(dirPath, walkFn)
	if err != nil {
		_, _ = fmt.Fprintf(os.Stderr, "遍历目录失败: %v\n", err)
	}
}

// extractPkgByRelDir 根据路径与基础目录的关系提取 PKG。
func extractPkgByRelDir(dirPath, path string) {
	relDir := filepath.Dir(path)
	if relDir == dirPath {
		extractPkg(path, false, "", gSkipExts, gOnlyExts)
		return
	}
	subDir, err := filepath.Rel(dirPath, relDir)
	if err != nil {
		_, _ = fmt.Fprintf(os.Stderr, "计算相对路径失败: %v\n", err)
		return
	}
	extractPkg(path, true, subDir, gSkipExts, gOnlyExts)
}

// extractPkgNonRecursive 非递归提取直接子目录下的 PKG 文件。
func extractPkgNonRecursive(dirPath string) {
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
			continue
		}
		for _, pf := range pkgFiles {
			extractPkg(pf, true, entry.Name(), gSkipExts, gOnlyExts)
		}
	}
}

// saveTexAsImage 将 TEX 数据转换为图片并保存。
func saveTexAsImage(texData *tex.TEX, basePath string) {
	result, err := texData.Convert()
	if err != nil {
		_, _ = fmt.Fprintf(os.Stderr, "转换 TEX 失败: %v\n", err)
		return
	}

	ext := result.Format.GetFileExtension()
	outPath := basePath + "." + ext

	if !overwrite {
		_, err := os.Stat(outPath) //nolint:gosec // outPath 由 CLI 内部构造
		if err == nil {
			return
		}
	}

	err = os.WriteFile(outPath, result.Bytes, fileWritePerm) //nolint:gosec // outPath 由 CLI 内部构造
	if err != nil {
		_, _ = fmt.Fprintf(os.Stderr, "写入图片失败 (%s): %v\n", outPath, err)
		return
	}

	// 生成 .tex-json 元数据
	jsonInfo, err := tex.GenerateJSONInfo(texData)
	if err == nil {
		wErr := os.WriteFile(basePath+".tex-json", []byte(jsonInfo), fileWritePerm) //nolint:gosec // basePath 由 CLI 内部构造
		if wErr != nil {
			_, _ = fmt.Fprintf(os.Stderr, "写入 tex-json 失败 (%s): %v\n", basePath+".tex-json", wErr)
		}
	}
}

// filterEntries 过滤条目列表。
func filterEntries(entries []*pkgfile.Entry, skipExts, onlyExtList []string) []*pkgfile.Entry {
	if len(skipExts) > 0 {
		filtered := make([]*pkgfile.Entry, 0, len(entries))
		for _, entry := range entries {
			ext := strings.ToLower(filepath.Ext(entry.FullPath))
			if slices.Contains(skipExts, ext) {
				continue
			}
			filtered = append(filtered, entry)
		}
		return filtered
	}

	if len(onlyExtList) > 0 {
		filtered := make([]*pkgfile.Entry, 0, len(entries))
		for _, entry := range entries {
			ext := strings.ToLower(filepath.Ext(entry.FullPath))
			if slices.Contains(onlyExtList, ext) {
				filtered = append(filtered, entry)
			}
		}
		return filtered
	}

	return entries
}

// normalizeExtensions 为没有点号的扩展名添加前缀。
func normalizeExtensions(exts []string) []string {
	for i, ext := range exts {
		ext = strings.TrimSpace(ext)
		if !strings.HasPrefix(ext, ".") {
			ext = "." + ext
		}
		exts[i] = strings.ToLower(ext)
	}
	return exts
}

// readSeekerFromBytes 从字节切片创建 io.ReadSeeker。
func readSeekerFromBytes(data []byte) *byteReadSeeker {
	return &byteReadSeeker{data: data}
}

type byteReadSeeker struct {
	data   []byte
	offset int
}

func (b *byteReadSeeker) Read(p []byte) (int, error) {
	if b.offset >= len(b.data) {
		return 0, io.EOF
	}
	n := copy(p, b.data[b.offset:])
	b.offset += n
	return n, nil
}

func (b *byteReadSeeker) Seek(offset int64, whence int) (int64, error) {
	var newOffset int64
	switch whence {
	case io.SeekStart:
		newOffset = offset
	case io.SeekCurrent:
		newOffset = int64(b.offset) + offset
	case io.SeekEnd:
		newOffset = int64(len(b.data)) + offset
	default:
		return 0, fmt.Errorf("无效的 whence 值: %d", whence)
	}
	if newOffset < 0 {
		return 0, errors.New("负偏移")
	}
	b.offset = int(newOffset)
	return newOffset, nil
}

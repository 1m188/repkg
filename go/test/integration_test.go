// Package test 提供使用实景 test.pkg 文件的集成测试。
//
// 本测试使用go/testdata/test.pkg（来自 Wallpaper Engine 的真实 .pkg 文件）
// 验证完整的 extract 流程：读取 → 解压条目 → TEX 转换。
package test

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/1m188/repkg-go/internal/pkgfile"
	"github.com/1m188/repkg-go/internal/tex"
)

var testDataDir string

func init() {
	// 测试数据目录（从 test/ 向上一级找到 testdata/）
	wd, err := os.Getwd()
	if err != nil {
		panic(fmt.Sprintf("获取当前目录失败: %v", err))
	}
	testDataDir = filepath.Join(wd, "..", "testdata")
}

func TestRealPKG_读取成功(t *testing.T) {
	pkgPath := filepath.Join(testDataDir, "test.pkg")

	f, err := os.Open(pkgPath) //nolint:gosec // 测试中使用变量路径打开文件是预期行为
	if err != nil {
		t.Skipf("测试数据 %s 不存在，跳过集成测试", pkgPath)
	}
	defer func() { _ = f.Close() }() //nolint:errcheck // defer 中文件关闭错误可忽略

	reader := pkgfile.NewReader()
	pkg, err := reader.ReadPackage(f)
	if err != nil {
		t.Fatalf("读取 test.pkg 失败: %v", err)
	}

	// 验证基本属性
	if pkg.Magic == "" {
		t.Error("PKG magic 为空")
	}

	entryCount := len(pkg.Entries)
	t.Logf("test.pkg 条目数: %d", entryCount)
	if entryCount == 0 {
		t.Fatal("test.pkg 没有任何条目")
	}

	// 统计各类型条目
	var texCount, binCount int
	for _, entry := range pkg.Entries {
		if len(entry.Bytes) == 0 {
			t.Errorf("条目 %s 字节为空", entry.FullPath)
		}

		if entry.Type == pkgfile.EntryTypeTex {
			texCount++
		} else {
			binCount++
		}

		t.Logf("  %s (%d 字节, 类型=%d)", entry.FullPath, entry.Length, entry.Type)
	}

	t.Logf("TEX 条目: %d, 二进制条目: %d", texCount, binCount)
}

func TestRealPKG_提取并验证(t *testing.T) {
	pkgPath := filepath.Join(testDataDir, "test.pkg")

	f, err := os.Open(pkgPath) //nolint:gosec // 测试中使用变量路径打开文件是预期行为
	if err != nil {
		t.Skipf("测试数据 %s 不存在，跳过集成测试", pkgPath)
	}
	defer func() { _ = f.Close() }() //nolint:errcheck // defer 中文件关闭错误可忽略

	reader := pkgfile.NewReader()
	pkg, err := reader.ReadPackage(f)
	if err != nil {
		t.Fatalf("读取 test.pkg 失败: %v", err)
	}

	// 验证每个非空条目
	for _, entry := range pkg.Entries {
		t.Run(entry.FullPath, func(t *testing.T) {
			validateEntry(t, entry)
		})
	}
}

// validateEntry 验证单个 PKG 条目。
func validateEntry(t *testing.T, entry *pkgfile.Entry) {
	t.Helper()
	if len(entry.Bytes) == 0 {
		t.Errorf("条目 %s 字节为空", entry.FullPath)
		return
	}
	if entry.Length <= 0 {
		t.Errorf("条目 %s 长度无效: %d", entry.FullPath, entry.Length)
	}

	if entry.Type != pkgfile.EntryTypeTex {
		return
	}

	texReader := tex.NewReader()
	texData, err := texReader.ReadTex(bytes.NewReader(entry.Bytes))
	if err != nil {
		t.Errorf("TEX 条目 %s 解析失败: %v", entry.FullPath, err)
		return
	}

	if texData.Header == nil {
		t.Error("TEX Header 为空")
	}
	if texData.ImagesContainer == nil {
		t.Error("TEX ImagesContainer 为空")
		return
	}
	if len(texData.ImagesContainer.Images) == 0 {
		return
	}
	m := texData.FirstImage().FirstMipmap()
	if m != nil && len(m.Bytes) > 0 {
		t.Logf("  解压成功: %dx%d, 格式=%s, 字节=%d",
			m.Width, m.Height, m.Format.String(), len(m.Bytes))
	}
}

func TestRealPKG_TEX转换测试(t *testing.T) {
	pkgPath := filepath.Join(testDataDir, "test.pkg")

	f, err := os.Open(pkgPath) //nolint:gosec // 测试中使用变量路径打开文件是预期行为
	if err != nil {
		t.Skipf("测试数据 %s 不存在，跳过集成测试", pkgPath)
	}
	defer func() { _ = f.Close() }() //nolint:errcheck // defer 中文件关闭错误可忽略

	reader := pkgfile.NewReader()
	pkg, err := reader.ReadPackage(f)
	if err != nil {
		t.Fatalf("读取 test.pkg 失败: %v", err)
	}

	// 对第一个 TEX 条目进行完整转换测试
	var firstTex []byte
	var texName string
	for _, entry := range pkg.Entries {
		if entry.Type == pkgfile.EntryTypeTex && len(entry.Bytes) > 0 {
			firstTex = entry.Bytes
			texName = entry.FullPath
			break
		}
	}

	if firstTex == nil {
		t.Skip("test.pkg 中没有 TEX 条目，跳过转换测试")
	}

	t.Logf("测试转换: %s", texName)

	texReader := tex.NewReader()
	texData, err := texReader.ReadTex(bytes.NewReader(firstTex))
	if err != nil {
		t.Fatalf("解析 TEX %s 失败: %v", texName, err)
	}

	// 转换
	result, err := texData.Convert()
	if err != nil {
		t.Fatalf("转换 TEX %s 失败: %v", texName, err)
	}

	if len(result.Bytes) == 0 {
		t.Error("转换结果为空")
	}

	t.Logf("转换成功: 格式=%s, 大小=%d 字节", result.Format.String(), len(result.Bytes))

	// 生成 JSON 元数据
	jsonInfo, err := tex.GenerateJSONInfo(texData)
	if err != nil {
		t.Fatalf("生成 JSON 失败: %v", err)
	}

	if jsonInfo == "" {
		t.Error("JSON 信息为空")
	}

	t.Logf("JSON 信息:\n%s", jsonInfo)
}

func TestRealPKG_WriteReadRoundTrip(t *testing.T) {
	// 验证 PKG 写回→读取的往返一致性
	pkgPath := filepath.Join(testDataDir, "test.pkg")

	origBytes, err := os.ReadFile(pkgPath) //nolint:gosec // 测试中使用变量路径是预期行为
	if err != nil {
		t.Skipf("测试数据 %s 不存在，跳过集成测试", pkgPath)
	}

	// 读取
	reader := pkgfile.NewReader()
	pkg, err := reader.ReadPackage(bytes.NewReader(origBytes))
	if err != nil {
		t.Fatalf("读取 test.pkg 失败: %v", err)
	}

	// 写回
	writer := pkgfile.NewWriter()
	var written bytes.Buffer
	err = writer.WriteTo(&written, pkg)
	if err != nil {
		t.Fatalf("写回失败: %v", err)
	}

	// 验证重新写回后条目数一致
	reader2 := pkgfile.NewReader()
	pkg2, err := reader2.ReadPackage(bytes.NewReader(written.Bytes()))
	if err != nil {
		t.Fatalf("重新读取失败: %v", err)
	}

	if len(pkg.Entries) != len(pkg2.Entries) {
		t.Errorf("往返后条目数不一致: %d → %d", len(pkg.Entries), len(pkg2.Entries))
	}

	// 验证每个条目的字节一致
	for i := range pkg.Entries {
		if pkg.Entries[i].FullPath != pkg2.Entries[i].FullPath {
			t.Errorf("条目 %d 路径不一致:\n  原始: %s\n  往返: %s",
				i, pkg.Entries[i].FullPath, pkg2.Entries[i].FullPath)
		}
		if !bytes.Equal(pkg.Entries[i].Bytes, pkg2.Entries[i].Bytes) {
			t.Errorf("条目 %s 数据不一致 (原始=%d 字节, 往返=%d 字节)",
				pkg.Entries[i].FullPath,
				len(pkg.Entries[i].Bytes),
				len(pkg2.Entries[i].Bytes))
		}
	}
}

func TestRealPKG_Info输出(t *testing.T) {
	pkgPath := filepath.Join(testDataDir, "test.pkg")

	f, err := os.Open(pkgPath) //nolint:gosec // 测试中使用变量路径打开文件是预期行为
	if err != nil {
		t.Skipf("测试数据 %s 不存在，跳过集成测试", pkgPath)
	}
	defer func() { _ = f.Close() }() //nolint:errcheck // defer 中文件关闭错误可忽略

	reader := pkgfile.NewReader()
	pkg, err := reader.ReadPackage(f)
	if err != nil {
		t.Fatalf("读取 test.pkg 失败: %v", err)
	}

	// 验证 info 命令所需的信息完整
	var info strings.Builder
	_, _ = fmt.Fprintf(&info, "包格式: %s\n", pkg.Magic)
	_, _ = fmt.Fprintf(&info, "条目总数: %d\n", len(pkg.Entries))

	for _, entry := range pkg.Entries {
		_, _ = fmt.Fprintf(&info, "  * %s - %d 字节\n", entry.FullPath, entry.Length)
	}

	t.Log(info.String())

	if pkg.Magic == "" {
		t.Error("Magic 为空")
	}
}

// ==================== Fix #3-6 对应测试 ====================

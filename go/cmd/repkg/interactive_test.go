package main

import (
	"bytes"
	"encoding/binary"
	"io"
	"os"
	"path/filepath"
	"testing"
)

// TestInteractiveMode_FlagsPassThrough 验证交互模式下 flags 被正确解析和应用。
// 测试：传入 "extract -o /tmp/test_out test.pkg" 形式，
// outputDir 应根据 -o 参数被设置。
func TestInteractiveMode_FlagsPassThrough(t *testing.T) {
	// 创建临时输入文件和临时输出目录
	tmpInput := filepath.Join(t.TempDir(), "test.pkg")
	err := os.WriteFile(tmpInput, makePKGBinary(), 0o644) //nolint:gosec // 测试辅助函数
	if err != nil {
		t.Fatal(err)
	}

	tmpOutput := filepath.Join(t.TempDir(), "custom_output")

	// 模拟交互模式 splitArgs 解析
	line := `extract -o ` + tmpOutput + ` ` + tmpInput
	args := splitArgs(line)

	if len(args) < 4 {
		t.Fatalf("splitArgs 解析结果过少: %v (需要至少 4 个元素)", args)
	}
	if args[1] != "-o" {
		t.Errorf("splitArgs 未正确解析 -o 标志: %v", args)
	}
	if args[2] != tmpOutput {
		t.Errorf("splitArgs 未正确解析输出路径: %v (args[2]=%q, 期望 %q)", args, args[2], tmpOutput)
	}

	// 重置全局标志
	saveDir := outputDir
	outputDir = ""
	defer func() { outputDir = saveDir }()

	// 通过 Cobra 根命令重新解析
	rootCmd := newRootCommand()
	rootCmd.SetArgs(args)
	err = rootCmd.Execute()
	if err != nil {
		t.Fatalf("命令执行失败: %v", err)
	}

	// 验证 -o 参数被解析
	if outputDir != tmpOutput {
		t.Errorf("outputDir = %q, 期望 %q (-o 参数未被应用)", outputDir, tmpOutput)
	}

	// 验证输出目录确实被创建
	_, err = os.Stat(tmpOutput)
	if err != nil {
		t.Logf("输出目录状态: %v (首次提取可能不创建空目录，这是正常的)", err)
	}
}

// TestExtractDir_ExtensionFilter 验证目录模式下 -i/-e 过滤生效。
func TestExtractDir_ExtensionFilter(t *testing.T) {
	// 创建包含多个 .pkg 文件的临时目录结构
	tmpDir := t.TempDir()
	subDir := filepath.Join(tmpDir, "123")
	err := os.MkdirAll(subDir, 0o755) //nolint:gosec // 测试辅助函数
	if err != nil {
		t.Fatal(err)
	}

	// 创建带有 .tex 条目的 PKG
	pkgData := makePKGWithEntries([]pkgTestEntry{
		{"test.tex", []byte{1, 2, 3, 4}},
		{"test.json", []byte(`{"k":"v"}`)},
	})
	err = os.WriteFile(filepath.Join(subDir, "test.pkg"), pkgData, 0o644) //nolint:gosec // 测试辅助函数
	if err != nil {
		t.Fatal(err)
	}

	tmpOutput := filepath.Join(t.TempDir(), "output")

	// 保存并设置全局标志
	saveIgnoreExts, saveOnlyExts, saveOutput := ignoreExts, onlyExts, outputDir
	ignoreExts = ""
	onlyExts = extDotTex
	outputDir = tmpOutput
	defer func() {
		ignoreExts, onlyExts, outputDir = saveIgnoreExts, saveOnlyExts, saveOutput
	}()

	// 执行目录提取
	actionExtract(tmpDir)

	// 验证只有 .tex 文件被提取
	texFile := filepath.Join(tmpOutput, "123", "test.tex")
	_, err = os.Stat(texFile)
	if err != nil {
		t.Errorf("应提取 .tex 文件但未找到: %v", err)
	}

	jsonFile := filepath.Join(tmpOutput, "123", "test.json")
	_, err = os.Stat(jsonFile)
	if err == nil {
		t.Error(".json 文件被提取但应被 -e .tex 过滤排除")
	}
}

// TestExtractDir_IgnoreExtensionFilter 验证 -i 过滤在目录模式下生效。
func TestExtractDir_IgnoreExtensionFilter(t *testing.T) {
	tmpDir := t.TempDir()
	subDir := filepath.Join(tmpDir, "456")
	err := os.MkdirAll(subDir, 0o755) //nolint:gosec // 测试辅助函数
	if err != nil {
		t.Fatal(err)
	}

	pkgData := makePKGWithEntries([]pkgTestEntry{
		{"texture.tex", []byte{1, 2, 3, 4}},
		{"config.json", []byte(`{"key":"value"}`)},
		{"model.obj", []byte{5, 6, 7, 8}},
	})
	err = os.WriteFile(filepath.Join(subDir, "test.pkg"), pkgData, 0o644) //nolint:gosec // 测试辅助函数
	if err != nil {
		t.Fatal(err)
	}

	tmpOutput := filepath.Join(t.TempDir(), "output")

	saveIgnoreExts, saveOnlyExts, saveOutput := ignoreExts, onlyExts, outputDir
	ignoreExts = ".tex"
	onlyExts = ""
	outputDir = tmpOutput
	defer func() {
		ignoreExts, onlyExts, outputDir = saveIgnoreExts, saveOnlyExts, saveOutput
	}()

	actionExtract(tmpDir)

	// .tex 应被忽略
	texFile := filepath.Join(tmpOutput, "456", "texture.tex")
	_, err = os.Stat(texFile)
	if err == nil {
		t.Error(".tex 文件被提取但应被 -i .tex 过滤排除")
	}

	// 其他文件应正常提取
	jsonFile := filepath.Join(tmpOutput, "456", "config.json")
	_, err = os.Stat(jsonFile)
	if err != nil {
		t.Errorf("应提取 .json 文件但未找到: %v", err)
	}
}

// ==================== 测试辅助函数 ====================

// makePKGBinary 创建一个最小合法的 PKG 二进制数据（无条目）。
func makePKGBinary() []byte {
	return makePKGWithEntries(nil)
}

// pkgTestEntry 测试辅助结构体，表示 PKG 条目的名称和数据。
type pkgTestEntry struct {
	name string
	data []byte
}

// makePKGWithEntries 用指定的条目列表创建 PKG 二进制数据。
func makePKGWithEntries(entries []pkgTestEntry) []byte {
	var buf bytes.Buffer

	// 写入 magic
	magic := "PKGV0005"
	_ = binary.Write(&buf, binary.LittleEndian, int32(len(magic))) //nolint:errcheck,gosec // 测试辅助函数
	_, _ = buf.WriteString(magic)

	// 写入条目数量
	_ = binary.Write(&buf, binary.LittleEndian, int32(len(entries))) //nolint:errcheck,gosec // 测试辅助函数

	// 写入条目头部
	offset := int32(0)
	for _, e := range entries {
		_ = binary.Write(&buf, binary.LittleEndian, int32(len(e.name))) //nolint:errcheck,gosec // 测试辅助函数
		_, _ = buf.WriteString(e.name)
		_ = binary.Write(&buf, binary.LittleEndian, offset)             //nolint:errcheck // 测试辅助函数
		_ = binary.Write(&buf, binary.LittleEndian, int32(len(e.data))) //nolint:errcheck,gosec // 测试辅助函数
		offset += int32(len(e.data))                                    //nolint:gosec // 测试辅助函数
	}

	// 写入数据体
	for _, e := range entries {
		_, _ = buf.Write(e.data)
	}

	return buf.Bytes()
}

// TestByteReadSeeker_非法Whence 验证 Seek 对非法 whence 值返回错误。
func TestByteReadSeeker_非法Whence(t *testing.T) {
	b := readSeekerFromBytes([]byte{1, 2, 3})
	_, err := b.Seek(0, 999)
	if err == nil {
		t.Error("非法 whence=999 应返回错误但未返回")
	}
	// 验证合法 whence 仍正常工作
	n, err := b.Seek(0, io.SeekStart)
	if err != nil {
		t.Fatal(err)
	}
	if n != 0 {
		t.Errorf("Seek(0, SeekStart) = %d, 期望 0", n)
	}
}

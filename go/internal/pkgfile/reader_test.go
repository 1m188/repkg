package pkgfile

import (
	"bytes"
	"encoding/binary"
	"io"
	"testing"
)

const (
	testMagic       = "PKGV0005"
	testFileName    = "test.txt"
	testTexFileName = "materials/sky.tex"
)

// 辅助函数：构造 PKG 二进制数据
func makePKGData(entries []*Entry) []byte {
	var buf bytes.Buffer

	magic := testMagic
	length := int32(len(magic))                         //nolint:gosec // 测试辅助函数，magic 长度受控
	_ = binary.Write(&buf, binary.LittleEndian, length) //nolint:errcheck // 测试辅助函数，写入缓冲区不会失败
	_, _ = buf.WriteString(magic)

	// 写入条目数量
	_ = binary.Write(&buf, binary.LittleEndian, int32(len(entries))) //nolint:errcheck,gosec // 测试辅助函数

	// 写入条目头部 + 收集偏移和长度
	currentOffset := int32(0)
	for _, entry := range entries {
		_ = binary.Write(&buf, binary.LittleEndian, int32(len(entry.FullPath))) //nolint:errcheck,gosec // 测试辅助函数
		_, _ = buf.WriteString(entry.FullPath)
		_ = binary.Write(&buf, binary.LittleEndian, currentOffset)           //nolint:errcheck // 测试辅助函数
		_ = binary.Write(&buf, binary.LittleEndian, int32(len(entry.Bytes))) //nolint:errcheck,gosec // 测试辅助函数
		entry.Offset = currentOffset
		entry.Length = int32(len(entry.Bytes)) //nolint:gosec // 测试辅助函数，数据长度受控
		currentOffset += entry.Length
	}

	// 写入数据体
	for _, entry := range entries {
		_, _ = buf.Write(entry.Bytes)
	}

	// 写入数据体
	for _, entry := range entries {
		_, _ = buf.Write(entry.Bytes)
	}

	return buf.Bytes()
}

func TestReader_空包(t *testing.T) {
	data := makePKGData(nil)
	r := NewReader()
	pkg, err := r.ReadPackage(bytes.NewReader(data))
	if err != nil {
		t.Fatal(err)
	}
	if pkg.Magic != testMagic {
		t.Errorf("Magic = %q, 期望 PKGV0005", pkg.Magic)
	}
	if len(pkg.Entries) != 0 {
		t.Errorf("条目数 = %d, 期望 0", len(pkg.Entries))
	}
}

func TestReader_单条目(t *testing.T) {
	entry := &Entry{
		FullPath: "hello.txt",
		Bytes:    []byte("Hello, World!"),
	}
	data := makePKGData([]*Entry{entry})

	r := NewReader()
	pkg, err := r.ReadPackage(bytes.NewReader(data))
	if err != nil {
		t.Fatal(err)
	}

	if len(pkg.Entries) != 1 {
		t.Fatalf("条目数 = %d, 期望 1", len(pkg.Entries))
	}
	e := pkg.Entries[0]
	if e.FullPath != "hello.txt" {
		t.Errorf("FullPath = %q, 期望 hello.txt", e.FullPath)
	}
	if string(e.Bytes) != "Hello, World!" {
		t.Errorf("Bytes = %q, 期望 Hello, World!", string(e.Bytes))
	}
	if e.Type != EntryTypeBinary {
		t.Errorf("EntryType = %d, 期望 Binary", e.Type)
	}
	if e.Name() != "hello" {
		t.Errorf("Name() = %q, 期望 hello", e.Name())
	}
	if e.Extension() != ".txt" {
		t.Errorf("Extension() = %q, 期望 .txt", e.Extension())
	}
}

func TestReader_多条目(t *testing.T) {
	entries := []*Entry{
		{FullPath: "file1.bin", Bytes: []byte("AAAA")},
		{FullPath: testTexFileName, Bytes: make([]byte, 100)},
		{FullPath: "sub/deep/data.json", Bytes: []byte(`{"k":"v"}`)},
	}
	data := makePKGData(entries)

	r := NewReader()
	pkg, err := r.ReadPackage(bytes.NewReader(data))
	if err != nil {
		t.Fatal(err)
	}

	if len(pkg.Entries) != 3 {
		t.Fatalf("条目数 = %d, 期望 3", len(pkg.Entries))
	}

	// 验证 TEX 类型识别
	if pkg.Entries[1].Type != EntryTypeTex {
		t.Errorf("第二个条目类型 = %d, 期望 Tex", pkg.Entries[1].Type)
	}

	// 验证路径解析
	e3 := pkg.Entries[2]
	if e3.DirectoryPath() != "sub/deep" {
		t.Errorf("DirectoryPath() = %q, 期望 sub/deep", e3.DirectoryPath())
	}
}

func TestReader_大条目(t *testing.T) {
	// 1MB 条目
	bigData := make([]byte, 1024*1024)
	for i := range bigData {
		bigData[i] = byte(i % 256)
	}

	entries := []*Entry{
		{FullPath: "big.bin", Bytes: bigData},
	}
	data := makePKGData(entries)

	r := NewReader()
	pkg, err := r.ReadPackage(bytes.NewReader(data))
	if err != nil {
		t.Fatal(err)
	}

	if len(pkg.Entries[0].Bytes) != 1024*1024 {
		t.Errorf("大条目大小 = %d, 期望 %d", len(pkg.Entries[0].Bytes), 1024*1024)
	}

	// 验证随机位置正确
	if pkg.Entries[0].Bytes[0] != 0 || pkg.Entries[0].Bytes[512] != 0 {
		t.Error("大条目数据验证失败")
	}
}

func TestReader_不读取字节(t *testing.T) {
	entry := &Entry{
		FullPath: testFileName,
		Bytes:    []byte("data"),
	}
	data := makePKGData([]*Entry{entry})

	r := NewReader()
	r.ReadEntryBytes = false
	pkg, err := r.ReadPackage(bytes.NewReader(data))
	if err != nil {
		t.Fatal(err)
	}

	if pkg.Entries[0].Bytes != nil {
		t.Error("ReadEntryBytes=false 但 Bytes 不为 nil")
	}
	if pkg.Entries[0].Length != 4 {
		t.Errorf("Length = %d, 期望 4", pkg.Entries[0].Length)
	}
}

func TestEntryType识别(t *testing.T) {
	tests := []struct {
		fileName  string
		entryType EntryType
	}{
		{"texture.tex", EntryTypeTex},
		{"TEXTURE.TEX", EntryTypeTex},
		{"scene.pkg", EntryTypeBinary},
		{"model.obj", EntryTypeBinary},
		{"materials/sky.mat", EntryTypeBinary},
		{"no-extension", EntryTypeBinary},
	}

	for _, tt := range tests {
		t.Run(tt.fileName, func(t *testing.T) {
			got := EntryTypeFromFileName(tt.fileName)
			if got != tt.entryType {
				t.Errorf("EntryTypeFromFileName(%q) = %d, 期望 %d", tt.fileName, got, tt.entryType)
			}
		})
	}
}

func TestWriter_空包返回错误(t *testing.T) {
	w := NewWriter()
	pkg := &Package{Magic: testMagic}
	var buf bytes.Buffer
	err := w.WriteTo(&buf, pkg)
	if err == nil {
		t.Error("空条目应返回错误但未返回")
	}
}

func TestWriter_读写成环(t *testing.T) {
	tests := []struct {
		name string
		pkg  *Package
	}{
		{
			"单条目",
			&Package{
				Magic: testMagic,
				Entries: []*Entry{
					{FullPath: testFileName, Bytes: []byte("hello")},
				},
			},
		},
		{
			"多条目",
			&Package{
				Magic: testMagic,
				Entries: []*Entry{
					{FullPath: "a.txt", Bytes: []byte("aaa")},
					{FullPath: "b.txt", Bytes: []byte("bbb")},
					{FullPath: "c.txt", Bytes: make([]byte, 1000)},
				},
			},
		},
		{
			"含TEX条目",
			&Package{
				Magic: testMagic,
				Entries: []*Entry{
					{FullPath: "data.bin", Bytes: []byte{1, 2, 3}},
					{FullPath: "tex/sky.tex", Bytes: []byte{4, 5, 6, 7}},
				},
			},
		},
		{
			"嵌套路径",
			&Package{
				Magic: testMagic,
				Entries: []*Entry{
					{FullPath: "a/b/c/d/e.txt", Bytes: []byte("deep")},
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// 写入
			w := NewWriter()
			var buf bytes.Buffer
			err := w.WriteTo(&buf, tt.pkg)
			if err != nil {
				t.Fatalf("写入失败: %v", err)
			}

			// 读取
			r := NewReader()
			readPkg, err := r.ReadPackage(bytes.NewReader(buf.Bytes()))
			if err != nil {
				t.Fatalf("读取失败: %v", err)
			}

			// 验证
			if readPkg.Magic != tt.pkg.Magic {
				t.Errorf("Magic = %q, 期望 %q", readPkg.Magic, tt.pkg.Magic)
			}
			if len(readPkg.Entries) != len(tt.pkg.Entries) {
				t.Fatalf("条目数 = %d, 期望 %d", len(readPkg.Entries), len(tt.pkg.Entries))
			}

			for i := range tt.pkg.Entries {
				orig := tt.pkg.Entries[i]
				read := readPkg.Entries[i]

				if read.FullPath != orig.FullPath {
					t.Errorf("条目 %d FullPath = %q, 期望 %q", i, read.FullPath, orig.FullPath)
				}
				if !bytes.Equal(read.Bytes, orig.Bytes) {
					t.Errorf("条目 %d Bytes 不匹配", i)
				}
			}
		})
	}
}

func TestWriter_nilEntry返回错误(t *testing.T) {
	w := NewWriter()
	pkg := &Package{
		Magic:   testMagic,
		Entries: []*Entry{nil},
	}
	var buf bytes.Buffer
	err := w.WriteTo(&buf, pkg)
	if err == nil {
		t.Error("nil 条目应返回错误但未返回")
	}
}

func TestWriter_空FullPath返回错误(t *testing.T) {
	w := NewWriter()
	pkg := &Package{
		Magic: testMagic,
		Entries: []*Entry{
			{FullPath: "", Bytes: []byte("test")},
		},
	}
	var buf bytes.Buffer
	err := w.WriteTo(&buf, pkg)
	if err == nil {
		t.Error("空 FullPath 应返回错误但未返回")
	}
}

func TestWriter_nilBytes返回错误(t *testing.T) {
	w := NewWriter()
	pkg := &Package{
		Magic: testMagic,
		Entries: []*Entry{
			{FullPath: testFileName, Bytes: nil},
		},
	}
	var buf bytes.Buffer
	err := w.WriteTo(&buf, pkg)
	if err == nil {
		t.Error("nil Bytes 应返回错误但未返回")
	}
}

func TestPackageEntry属性(t *testing.T) {
	e := &Entry{
		FullPath: testTexFileName,
		Bytes:    []byte{1, 2, 3},
	}

	if e.Name() != "sky" {
		t.Errorf("Name() = %q, 期望 sky", e.Name())
	}
	if e.Extension() != ".tex" {
		t.Errorf("Extension() = %q, 期望 .tex", e.Extension())
	}
	if e.DirectoryPath() != "materials" {
		t.Errorf("DirectoryPath() = %q, 期望 materials", e.DirectoryPath())
	}
}

func TestReader_非法输入(t *testing.T) {
	tests := []struct {
		name string
		data []byte
	}{
		{"空数据", []byte{}},
		{"截断magic", make([]byte, 2)},
		{"非法条目数", makeIllegalData()},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := NewReader()
			_, err := r.ReadPackage(bytes.NewReader(tt.data))
			if err == nil {
				t.Error("期望错误但未获取到")
			}
		})
	}
}

// 辅助函数
func makeIllegalData() []byte {
	var buf bytes.Buffer
	//nolint:errcheck // 测试辅助函数，构造非法数据
	_ = binary.Write(&buf, binary.LittleEndian, int32(5))
	_, _ = buf.WriteString("TEST")
	// 100 个条目但无数据
	_ = binary.Write(&buf, binary.LittleEndian, int32(100)) //nolint:errcheck // 测试辅助函数，构造非法数据
	return buf.Bytes()
}

func TestReader_HeaderSize(t *testing.T) {
	entries := []*Entry{
		{FullPath: "file1.bin", Bytes: []byte("AAAA")},
		{FullPath: testTexFileName, Bytes: make([]byte, 100)},
	}
	data := makePKGData(entries)

	r := NewReader()
	pkg, err := r.ReadPackage(bytes.NewReader(data))
	if err != nil {
		t.Fatalf("读取 PKG 失败: %v", err)
	}

	if pkg.HeaderSize <= 0 {
		t.Errorf("HeaderSize = %d, 期望 > 0", pkg.HeaderSize)
	}

	// 计算期望的头部大小并验证
	expected := int32(4 + len(testMagic) + 4) // magic长度 + magic内容 + 条目数
	for _, e := range entries {
		expected += 4 + int32(len(e.FullPath)) + 4 + 4 //nolint:gosec // 测试中路径长度受控 // 路径长度 + 路径内容 + 偏移 + 长度
	}
	if pkg.HeaderSize != expected {
		t.Errorf("HeaderSize = %d, 期望 %d", pkg.HeaderSize, expected)
	}
}

func TestReader_溢出Offset(t *testing.T) {
	// 构造 PKG：off=2147483647, length=2，int32 加法溢出会绕过原 bounds check
	// 验证 reader 返回错误而非 panic
	var buf bytes.Buffer
	magic := testMagic
	_ = binary.Write(&buf, binary.LittleEndian, int32(len(magic))) //nolint:errcheck,gosec // 测试辅助函数
	_, _ = buf.WriteString(magic)
	_ = binary.Write(&buf, binary.LittleEndian, int32(1)) //nolint:errcheck // 1 条目
	// entry: 路径 "test"
	_ = binary.Write(&buf, binary.LittleEndian, int32(4)) //nolint:errcheck // 路径长度
	_, _ = buf.WriteString("test")
	_ = binary.Write(&buf, binary.LittleEndian, int32(2147483647)) //nolint:errcheck // offset 接近 int32 最大值
	_ = binary.Write(&buf, binary.LittleEndian, int32(2))          //nolint:errcheck // length=2
	// body: 仅 2 字节
	_, _ = buf.Write([]byte{0, 0})

	r := NewReader()
	_, err := r.ReadPackage(bytes.NewReader(buf.Bytes()))
	if err == nil {
		t.Error("溢出 offset 应返回错误但未返回")
	}
}

func TestReader_空数据EOF错误(t *testing.T) {
	r := NewReader()
	_, err := r.ReadPackage(bytes.NewReader([]byte{}))
	if err == nil || err.Error() == "" {
		t.Error("空数据应返回错误")
	}
}

func TestWriter_WriteTo_nilCheck(t *testing.T) {
	w := NewWriter()
	// nil package
	err := w.WriteTo(io.Discard, nil)
	if err == nil {
		t.Error("nil package 应返回错误")
	}
}

// TestReader_乱序Offset 验证 Reader 通过 Offset 字段正确定位条目数据，
// 而非依赖顺序读取。构造数据体条目顺序与头部不同的 PKG，确认每个条目字节正确。
func TestReader_乱序Offset(t *testing.T) {
	// 手工构造 PKG：头部条目顺序为 [a, b]，但数据体顺序为 [b, a]
	// 验证 reader 通过 Offset 字段正确提取每个条目数据
	var buf bytes.Buffer

	// Header: magic
	magic := testMagic
	_ = binary.Write(&buf, binary.LittleEndian, int32(len(magic))) //nolint:errcheck,gosec // 测试辅助函数
	_, _ = buf.WriteString(magic)

	// Header: 2 个条目
	_ = binary.Write(&buf, binary.LittleEndian, int32(2)) //nolint:errcheck // 测试辅助函数

	bytesA := []byte("AAAA")
	bytesB := []byte("BBBB")

	// 条目 0 (a.txt): Offset=len(bytesB) (因为数据体中 B 在 A 前面)
	_ = binary.Write(&buf, binary.LittleEndian, int32(len("a.txt"))) //nolint:errcheck // 测试辅助函数
	_, _ = buf.WriteString("a.txt")
	_ = binary.Write(&buf, binary.LittleEndian, int32(len(bytesB))) //nolint:errcheck,gosec // offset = 4 (B 的长度)
	_ = binary.Write(&buf, binary.LittleEndian, int32(len(bytesA))) //nolint:errcheck,gosec // length = 4

	// 条目 1 (b.txt): Offset=0 (因为 B 在数据体开头)
	_ = binary.Write(&buf, binary.LittleEndian, int32(len("b.txt"))) //nolint:errcheck // 测试辅助函数
	_, _ = buf.WriteString("b.txt")
	_ = binary.Write(&buf, binary.LittleEndian, int32(0))           //nolint:errcheck // offset = 0 (B 在数据体最前面)
	_ = binary.Write(&buf, binary.LittleEndian, int32(len(bytesB))) //nolint:errcheck,gosec // length = 4

	// 数据体: [B, A]（顺序与头部条目 [a, b] 不同）
	_, _ = buf.Write(bytesB) // offset 0: B
	_, _ = buf.Write(bytesA) // offset 4: A

	reader := NewReader()
	pkg, err := reader.ReadPackage(bytes.NewReader(buf.Bytes()))
	if err != nil {
		t.Fatalf("读取 PKG 失败: %v", err)
	}

	if len(pkg.Entries) != 2 {
		t.Fatalf("条目数 = %d, 期望 2", len(pkg.Entries))
	}

	// 条目 0 应为 a.txt，数据应为 bytesA
	e0 := pkg.Entries[0]
	if e0.FullPath != "a.txt" {
		t.Errorf("条目 0 FullPath = %q, 期望 a.txt", e0.FullPath)
	}
	if !bytes.Equal(e0.Bytes, bytesA) {
		t.Errorf("条目 0 Bytes = %q, 期望 %q (Offset 字段未被正确使用)", e0.Bytes, bytesA)
	}

	// 条目 1 应为 b.txt，数据应为 bytesB
	e1 := pkg.Entries[1]
	if e1.FullPath != "b.txt" {
		t.Errorf("条目 1 FullPath = %q, 期望 b.txt", e1.FullPath)
	}
	if !bytes.Equal(e1.Bytes, bytesB) {
		t.Errorf("条目 1 Bytes = %q, 期望 %q (Offset 字段未被正确使用)", e1.Bytes, bytesB)
	}
}

// TestReader_负数Length 验证恶意 PKG（条目长度为负数）应返回错误而非 panic。
func TestReader_负数Length(t *testing.T) {
	var buf bytes.Buffer
	magic := testMagic
	_ = binary.Write(&buf, binary.LittleEndian, int32(len(magic))) //nolint:errcheck,gosec // 测试辅助函数
	_, _ = buf.WriteString(magic)
	_ = binary.Write(&buf, binary.LittleEndian, int32(1)) //nolint:errcheck // 测试辅助函数
	// 条目：长度为 -5
	_ = binary.Write(&buf, binary.LittleEndian, int32(len("bad.tex"))) //nolint:errcheck // 测试辅助函数
	_, _ = buf.WriteString("bad.tex")
	_ = binary.Write(&buf, binary.LittleEndian, int32(0))  //nolint:errcheck // 测试辅助函数
	_ = binary.Write(&buf, binary.LittleEndian, int32(-5)) //nolint:errcheck // 测试辅助函数

	reader := NewReader()
	_, err := reader.ReadPackage(bytes.NewReader(buf.Bytes()))
	if err == nil {
		t.Error("负数 Length 应返回错误但未返回")
	}
}

// TestReader_超大EntryCount 验证超大条目数应返回错误而非溢出。
func TestReader_超大EntryCount(t *testing.T) {
	var buf bytes.Buffer
	magic := testMagic
	_ = binary.Write(&buf, binary.LittleEndian, int32(len(magic))) //nolint:errcheck,gosec // 测试辅助函数
	_, _ = buf.WriteString(magic)
	_ = binary.Write(&buf, binary.LittleEndian, int32(10_000_000)) //nolint:errcheck // 测试辅助函数
	reader := NewReader()
	_, err := reader.ReadPackage(bytes.NewReader(buf.Bytes()))
	if err == nil {
		t.Error("超大 entryCount 应返回错误但未返回")
	}
}

// TestReader_负数EntryCount 验证 entryCount 为负数时返回错误而非 panic。
// MPLR L1: make([]*Entry, -1) 会直接 panic。
func TestReader_负数EntryCount(t *testing.T) {
	var buf bytes.Buffer
	magic := testMagic
	_ = binary.Write(&buf, binary.LittleEndian, int32(len(magic))) //nolint:errcheck,gosec // 测试辅助函数
	_, _ = buf.WriteString(magic)
	_ = binary.Write(&buf, binary.LittleEndian, int32(-1)) //nolint:errcheck // 负数 entryCount
	reader := NewReader()
	_, err := reader.ReadPackage(bytes.NewReader(buf.Bytes()))
	if err == nil {
		t.Error("负数 entryCount 应返回错误但未返回")
	}
}

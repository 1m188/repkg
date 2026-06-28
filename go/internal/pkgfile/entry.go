// Package pkgfile 提供 PKG 文件的读写功能。
// 对应 C# RePKG.Application.Package 和 RePKG.Core.Package。
package pkgfile

import (
	"path/filepath"
	"strings"
)

// EntryType 表示 PKG 条目的类型。
type EntryType int32

const (
	// EntryTypeBinary 表示普通二进制文件。
	EntryTypeBinary EntryType = iota
	// EntryTypeTex 表示 TEX 纹理文件。
	EntryTypeTex
)

// Entry 表示 PKG 包中的一个条目。
// 对应 C# PackageEntry。
type Entry struct {
	// FullPath 完整路径（如 "materials/sky.tex"）。
	FullPath string
	// Offset 数据体中的偏移量。
	Offset int32
	// Length 数据长度（字节数）。
	Length int32
	// Bytes 条目的原始字节数据。
	Bytes []byte
	// Type 条目类型。
	Type EntryType
}

// Name 返回文件名（不含路径和扩展名）。
func (e *Entry) Name() string {
	name := filepath.Base(e.FullPath)
	ext := filepath.Ext(name)
	return name[:len(name)-len(ext)]
}

// Extension 返回文件扩展名（含点号）。
func (e *Entry) Extension() string {
	return filepath.Ext(e.FullPath)
}

// DirectoryPath 返回条目所在的目录路径。
func (e *Entry) DirectoryPath() string {
	dir := filepath.Dir(e.FullPath)
	if dir == "." {
		return ""
	}
	return dir
}

// Package 表示一个 PKG 包。
// 对应 C# Package。
type Package struct {
	// Magic 包格式标识（如 "PKGV0005"）。
	Magic string
	// HeaderSize 头部大小（字节数）。
	HeaderSize int32
	// Entries 包中的条目列表。
	Entries []*Entry
}

// EntryTypeFromFileName 根据文件名判断条目类型。
// .tex 扩展名 → EntryTypeTex，其余 → EntryTypeBinary。
func EntryTypeFromFileName(fileName string) EntryType {
	if strings.EqualFold(filepath.Ext(fileName), ".tex") {
		return EntryTypeTex
	}
	return EntryTypeBinary
}

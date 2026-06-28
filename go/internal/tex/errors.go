// Package tex 提供 Wallpaper Engine TEX 纹理文件的读写、压缩解压和格式转换功能。
//
// 本文件定义 TEX 相关的自定义错误类型。
package tex

import "fmt"

const (
	// SourceTexReader 错误来源：TEX 读取器。
	SourceTexReader = "TexReader"
	// SourceTexWriter 错误来源：TEX 写入器。
	SourceTexWriter = "TexWriter"
	// SourceTexConverter 错误来源：TEX 转换器。
	SourceTexConverter = "TexConverter"
	// SourceReadMipmapV4 错误来源：V4 mipmap 读取器。
	SourceReadMipmapV4 = "ReadMipmapV4"
)

// UnknownMagicError 表示遇到未知的文件魔数。
type UnknownMagicError struct {
	Source   string // Source 错误来源（如 "TexReader"）。
	Property string // Property 属性名（如 "Magic1"）。
	Magic    string // Magic 实际读取的魔数。
}

func (e *UnknownMagicError) Error() string {
	return fmt.Sprintf("%s: 未知的 %s: %s", e.Source, e.Property, e.Magic)
}

// UnsafeTexError 表示检测到不安全的 TEX 数据。
type UnsafeTexError struct {
	Reason string // Reason 不安全的具体原因。
}

func (e *UnsafeTexError) Error() string {
	return "不安全的 TEX 数据: " + e.Reason
}

// EnumNotValidError 表示枚举值无效。
type EnumNotValidError struct {
	Value  int32  // Value 实际值。
	Source string // Source 枚举类型名。
}

func (e *EnumNotValidError) Error() string {
	return fmt.Sprintf("无效的 %s: %d", e.Source, e.Value)
}

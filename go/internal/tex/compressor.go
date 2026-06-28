// Package tex 提供 Wallpaper Engine TEX 纹理文件的读写、压缩解压和格式转换功能。
package tex

import (
	"errors"
	"fmt"

	"github.com/pierrec/lz4/v4"
)

// Compressor 负责压缩 mipmap 数据（LZ4 压缩）。
// 对应 C# TexMipmapCompressor。
type Compressor struct{}

// NewCompressor 创建默认的压缩器。
func NewCompressor() *Compressor {
	return &Compressor{}
}

// Compress 对 mipmap 数据进行 LZ4 压缩。
// 压缩后设置 IsLZ4Compressed = true 和 DecompressedBytesCount。
func (c *Compressor) Compress(m *Mipmap) error {
	_ = c
	if m == nil {
		return errors.New("mipmap 不能为 nil")
	}
	if m.IsLZ4Compressed {
		return nil // 已经压缩
	}

	originalSize := len(m.Bytes)
	compressed := make([]byte, lz4.CompressBlockBound(originalSize))

	n, err := lz4.CompressBlock(m.Bytes, compressed, nil)
	if err != nil {
		return fmt.Errorf("LZ4 压缩失败: %w", err)
	}

	m.DecompressedBytesCount = int32(originalSize) //nolint:gosec // originalSize 来自上游已验证的数据大小
	m.Bytes = compressed[:n]
	m.IsLZ4Compressed = true

	return nil
}

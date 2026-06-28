package tex

import (
	"errors"
	"fmt"

	"github.com/pierrec/lz4/v4"

	"github.com/1m188/repkg-go/internal/dxt"
	"github.com/1m188/repkg-go/pkg/mipmap"
)

// Decompressor 负责解压 mipmap 数据（LZ4 + DXT）。
// 对应 C# TexMipmapDecompressor。
type Decompressor struct{}

// NewDecompressor 创建默认的解压器。
func NewDecompressor() *Decompressor {
	return &Decompressor{}
}

// Decompress 解压 mipmap 数据：
// 1. 如果 LZ4 压缩，先 LZ4 解压
// 2. 如果是 DXT 压缩格式，再 DXT 解码为 RGBA8888
func (d *Decompressor) Decompress(m *Mipmap) error {
	_ = d
	if m == nil {
		return errors.New("mipmap 不能为 nil")
	}

	// 步骤 1：LZ4 解压
	if m.IsLZ4Compressed {
		if m.DecompressedBytesCount < 0 {
			return errors.New("LZ4 解压目标大小无效: DecompressedBytesCount 为负数")
		}
		decompressed := make([]byte, m.DecompressedBytesCount)
		n, err := lz4.UncompressBlock(m.Bytes, decompressed)
		if err != nil {
			return fmt.Errorf("LZ4 解压失败: %w", err)
		}
		if n != int(m.DecompressedBytesCount) {
			return fmt.Errorf("LZ4 解压后大小不匹配: 期望 %d, 实际 %d", m.DecompressedBytesCount, n)
		}
		m.Bytes = decompressed[:n]
		m.IsLZ4Compressed = false
	}

	// 步骤 2：DXT 解码
	if m.Format.IsCompressed() && (m.Width < 0 || m.Height < 0) {
		return fmt.Errorf("mipmap 尺寸无效: width=%d height=%d", m.Width, m.Height)
	}
	switch m.Format {
	case mipmap.FormatCompressedDXT5:
		m.Bytes = dxt.DecompressImage(int(m.Width), int(m.Height), m.Bytes, dxt.FlagDXT5)
		m.Format = mipmap.FormatRGBA8888
	case mipmap.FormatCompressedDXT3:
		m.Bytes = dxt.DecompressImage(int(m.Width), int(m.Height), m.Bytes, dxt.FlagDXT3)
		m.Format = mipmap.FormatRGBA8888
	case mipmap.FormatCompressedDXT1:
		m.Bytes = dxt.DecompressImage(int(m.Width), int(m.Height), m.Bytes, dxt.FlagDXT1)
		m.Format = mipmap.FormatRGBA8888
	default:
	}

	return nil
}

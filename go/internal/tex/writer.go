package tex

import (
	"errors"
	"fmt"
	"io"

	"github.com/1m188/repkg-go/internal/binutil"
)

// Writer 用于写入 TEX 文件。
// 对应 C# TexWriter。
type Writer struct {
	compressor *Compressor
}

// NewWriter 创建默认的 TEX 写入器。
func NewWriter() *Writer {
	return &Writer{
		compressor: NewCompressor(),
	}
}

// WriteTo 将 TEX 对象写入 writer。
func (w *Writer) WriteTo(writer io.Writer, tex *TEX) error {
	_ = w
	if tex == nil {
		return errors.New("TEX 不能为 nil")
	}
	if tex.Magic1 != MagicTEXV0005 {
		return &UnknownMagicError{Source: SourceTexWriter, Property: "Magic1", Magic: tex.Magic1}
	}
	if tex.Magic2 != MagicTEXI0001 {
		return &UnknownMagicError{Source: SourceTexWriter, Property: "Magic2", Magic: tex.Magic2}
	}
	if tex.Header == nil {
		return errors.New("TEX Header 不能为 nil")
	}
	if tex.ImagesContainer == nil {
		return errors.New("TEX ImagesContainer 不能为 nil")
	}

	// 写入 Magic1 和 Magic2
	err := binutil.WriteNString(writer, tex.Magic1)
	if err != nil {
		return fmt.Errorf("写入 Magic1 失败: %w", err)
	}
	err = binutil.WriteNString(writer, tex.Magic2)
	if err != nil {
		return fmt.Errorf("写入 Magic2 失败: %w", err)
	}

	// 写入 Header
	err = tex.Header.write(writer)
	if err != nil {
		return fmt.Errorf("写入 Header 失败: %w", err)
	}

	// 写入 ImageContainer
	err = tex.ImagesContainer.write(writer)
	if err != nil {
		return fmt.Errorf("写入 ImageContainer 失败: %w", err)
	}

	// 写入 FrameInfoContainer（仅动画）
	if tex.IsGif() {
		if tex.FrameInfoContainer == nil {
			return errors.New("TEX 是动画但 FrameInfoContainer 为 nil")
		}
		err = tex.FrameInfoContainer.write(writer)
		if err != nil {
			return fmt.Errorf("写入 FrameInfoContainer 失败: %w", err)
		}
	}

	return nil
}

// CompressAndWrite 先压缩 mipmap 再写入（用于反向打包）。
func (w *Writer) CompressAndWrite(writer io.Writer, tex *TEX) error {
	if tex == nil {
		return errors.New("TEX 不能为 nil")
	}
	if tex.ImagesContainer == nil {
		return errors.New("TEX ImagesContainer 不能为 nil")
	}
	// 对每个 mipmap 进行 LZ4 压缩
	for _, img := range tex.ImagesContainer.Images {
		for _, m := range img.Mipmaps {
			if !m.IsLZ4Compressed {
				err := w.compressor.Compress(m)
				if err != nil {
					return fmt.Errorf("压缩 mipmap 失败: %w", err)
				}
			}
		}
	}
	return w.WriteTo(writer, tex)
}

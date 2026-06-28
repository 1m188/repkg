package tex

import (
	"fmt"
	"io"

	"github.com/1m188/repkg-go/internal/binutil"
	"github.com/1m188/repkg-go/pkg/mipmap"
)

// byteReader 是读取二进制数据所需的接口。
type byteReader interface {
	io.Reader
}

// byteWriter 是写入二进制数据所需的接口。
type byteWriter interface {
	io.Writer
}

// ioRuneReader 是可以读取 null 结尾字符串的接口。
type ioRuneReader interface {
	io.Reader
}

// TEX 表示一个完整的 TEX 纹理文件。
// 对应 C# Tex。
type TEX struct {
	Magic1             string              // 始终为 MagicTEXV0005
	Magic2             string              // 始终为 MagicTEXI0001
	Header             *Header             // 文件头
	ImagesContainer    *ImageContainer     // 图片容器
	FrameInfoContainer *FrameInfoContainer // 帧信息容器（仅动画）
}

// IsGif 判断是否为 GIF 动画纹理。
func (t *TEX) IsGif() bool {
	return t.Header != nil && (t.Header.Flags&FlagIsGif) != 0
}

// IsVideoTexture 判断是否为视频纹理。
func (t *TEX) IsVideoTexture() bool {
	return t.Header != nil && (t.Header.Flags&FlagIsVideoTexture) != 0
}

// FirstImage 返回第一张图片，如果没有图片数据则返回 nil。
func (t *TEX) FirstImage() *Image {
	if t.ImagesContainer == nil || len(t.ImagesContainer.Images) == 0 {
		return nil
	}
	return t.ImagesContainer.Images[0]
}

// Reader 用于读取 TEX 文件。
// 对应 C# TexReader。
type Reader struct {
	// ReadMipmapBytes 是否读取 mipmap 字节数据。默认 true。
	ReadMipmapBytes bool
	// DecompressMipmapBytes 是否自动解压 mipmap 数据。默认 true。
	DecompressMipmapBytes bool
	decompressor          *Decompressor
}

// NewReader 创建默认的 TEX 读取器（启用 mipmap 读取和解压）。
func NewReader() *Reader {
	return &Reader{
		ReadMipmapBytes:       true,
		DecompressMipmapBytes: true,
		decompressor:          NewDecompressor(),
	}
}

// NewReaderNoDecompress 创建不解压的 TEX 读取器（用于写入往返测试）。
func NewReaderNoDecompress() *Reader {
	return &Reader{
		ReadMipmapBytes:       true,
		DecompressMipmapBytes: false,
		decompressor:          NewDecompressor(),
	}
}

// ReadTex 从 reader 中读取并解析 TEX 数据。
func (r *Reader) ReadTex(reader io.Reader) (*TEX, error) {
	tex := &TEX{}

	// 读取 Magic1
	magic1, err := binutil.ReadNString(reader, 16)
	if err != nil {
		return nil, fmt.Errorf("读取 TEX Magic1 失败: %w", err)
	}
	if magic1 != MagicTEXV0005 {
		return nil, &UnknownMagicError{Source: SourceTexReader, Property: "Magic1", Magic: magic1}
	}
	tex.Magic1 = magic1

	// 读取 Magic2
	magic2, err := binutil.ReadNString(reader, 16)
	if err != nil {
		return nil, fmt.Errorf("读取 TEX Magic2 失败: %w", err)
	}
	if magic2 != MagicTEXI0001 {
		return nil, &UnknownMagicError{Source: SourceTexReader, Property: "Magic2", Magic: magic2}
	}
	tex.Magic2 = magic2

	// 读取 Header
	header, err := readHeader(reader)
	if err != nil {
		return nil, fmt.Errorf("读取 TEX Header 失败: %w", err)
	}
	tex.Header = header

	// 读取 ImageContainer
	imageContainer, err := readImageContainer(reader, header.Format, r.decompressor, r.ReadMipmapBytes, r.DecompressMipmapBytes)
	if err != nil {
		return nil, fmt.Errorf("读取 TEX ImageContainer 失败: %w", err)
	}
	tex.ImagesContainer = imageContainer

	// 读取 FrameInfoContainer（仅当是 GIF 动画时）
	if tex.IsGif() {
		frameContainer, err := readFrameInfoContainer(reader)
		if err != nil {
			return nil, fmt.Errorf("读取 TEX FrameInfoContainer 失败: %w", err)
		}
		tex.FrameInfoContainer = frameContainer
	}

	return tex, nil
}

// HasFlag 检查头部是否包含指定标志位。
func (t *TEX) HasFlag(flag int32) bool {
	if t.Header == nil {
		return false
	}
	return (t.Header.Flags & flag) == flag
}

// Convert 便捷方法：将 TEX 转换为图片。
func (t *TEX) Convert() (*ImageResult, error) {
	converter := NewConverter()
	return converter.ConvertToImage(t)
}

// ConvertFormat 便捷方法：获取转换后的格式。
func (t *TEX) ConvertFormat() mipmap.Format {
	converter := NewConverter()
	return converter.GetConvertedFormat(t)
}

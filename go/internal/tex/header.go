package tex

import (
	"fmt"

	"github.com/1m188/repkg-go/internal/binutil"
	"github.com/1m188/repkg-go/pkg/mipmap"
)

// TEX 文件格式 magic 字符串常量。
const (
	MagicTEXV0005 = "TEXV0005" // TEX 主体 magic。
	MagicTEXI0001 = "TEXI0001" // TEX 图片格式 magic。
	MagicTEXB0001 = "TEXB0001" // 图片容器 V1 magic。
	MagicTEXB0002 = "TEXB0002" // 图片容器 V2 magic。
	MagicTEXB0003 = "TEXB0003" // 图片容器 V3 magic。
	MagicTEXB0004 = "TEXB0004" // 图片容器 V4 magic。
	MagicTEXS0001 = "TEXS0001" // 帧信息容器 V1 magic。
	MagicTEXS0002 = "TEXS0002" // 帧信息容器 V2 magic。
	MagicTEXS0003 = "TEXS0003" // 帧信息容器 V3 magic。
)

// FlagNoInterpolation 表示禁用纹理插值。
const FlagNoInterpolation int32 = 1

// FlagClampUVs 表示 UV 坐标钳制。
const FlagClampUVs int32 = 2

// FlagIsGif 表示纹理为 GIF 动画。
const FlagIsGif int32 = 4

// FlagIsVideoTexture 表示纹理为视频纹理。
const FlagIsVideoTexture int32 = 32

// ImageContainerVersion 表示 TEX 图片容器的版本。
type ImageContainerVersion int32

const (
	// Version1 原始版本，无 LZ4 压缩。
	Version1 ImageContainerVersion = 1
	// Version2 增加 LZ4 压缩支持。
	Version2 ImageContainerVersion = 2
	// Version3 增加 FreeImage 格式字段。
	Version3 ImageContainerVersion = 3
	// Version4 增加 conditionJson 和额外参数。
	Version4 ImageContainerVersion = 4
)

// FreeImageFormat 表示 TEX 图片容器中存储的 FreeImage 格式 ID。
type FreeImageFormat int32

// FreeImage 格式常量。值域与 C# FreeImageFormat 枚举完全一致。
const (
	FIFUnknown FreeImageFormat = -1 // 未知格式。
	FIFBmp     FreeImageFormat = 0  // BMP 位图。
	FIFIco     FreeImageFormat = 1  // ICO 图标。
	FIFJpeg    FreeImageFormat = 2  // JPEG 图片。
	FIFJng     FreeImageFormat = 3  // JNG 图片。
	FIFKoala   FreeImageFormat = 4  // Koala 格式。
	FIFLbm     FreeImageFormat = 5  // Amiga IFF/LBM。
	FIFMng     FreeImageFormat = 6  // MNG 图片。
	FIFPbm     FreeImageFormat = 7  // PBM (ASCII)。
	FIFPbmraw  FreeImageFormat = 8  // PBM (Binary)。
	FIFPcd     FreeImageFormat = 9  // PhotoCD。
	FIFPcx     FreeImageFormat = 10 // PCX。
	FIFPgm     FreeImageFormat = 11 // PGM (ASCII)。
	FIFPgmraw  FreeImageFormat = 12 // PGM (Binary)。
	FIFPng     FreeImageFormat = 13 // PNG 图片。
	FIFPpm     FreeImageFormat = 14 // PPM (ASCII)。
	FIFPpmraw  FreeImageFormat = 15 // PPM (Binary)。
	FIFRas     FreeImageFormat = 16 // Sun Rasterfile。
	FIFTarga   FreeImageFormat = 17 // Targa TGA。
	FIFTiff    FreeImageFormat = 18 // TIFF。
	FIFWbmp    FreeImageFormat = 19 // Wireless Bitmap。
	FIFPsd     FreeImageFormat = 20 // Photoshop PSD。
	FIFCut     FreeImageFormat = 21 // Dr. Halo CUT。
	FIFXbm     FreeImageFormat = 22 // X11 Bitmap。
	FIFXpm     FreeImageFormat = 23 // X11 Pixmap。
	FIFDds     FreeImageFormat = 24 // DirectDraw Surface。
	FIFGif     FreeImageFormat = 25 // GIF 图片。
	FIFHdr     FreeImageFormat = 26 // HDR 图片。
	FIFFaxg3   FreeImageFormat = 27 // CCITT G3 传真。
	FIFSgi     FreeImageFormat = 28 // SGI 图片。
	FIFExr     FreeImageFormat = 29 // OpenEXR。
	FIFJ2k     FreeImageFormat = 30 // JPEG-2000 Code Stream。
	FIFJp2     FreeImageFormat = 31 // JPEG-2000 Image。
	FIFPfm     FreeImageFormat = 32 // Portable FloatMap。
	FIFPict    FreeImageFormat = 33 // Macintosh PICT。
	FIFRaw     FreeImageFormat = 34 // RAW 相机图片。
	FIFMp4     FreeImageFormat = 35 // MP4 视频。
)

// isValidFreeImageFormat 检查 FreeImageFormat 是否有效。
// 值域 -1（FIF_UNKNOWN）到 35（FIF_MP4），与 C# 一致。
func isValidFreeImageFormat(f FreeImageFormat) bool {
	return f >= -1 && f <= 35
}

// isValidTexFormat 检查 TexFormat 是否有效。
func isValidTexFormat(f mipmap.TexFormat) bool {
	switch f {
	case mipmap.TexFormatRGBA8888, mipmap.TexFormatDXT5, mipmap.TexFormatDXT3,
		mipmap.TexFormatDXT1, mipmap.TexFormatRG88, mipmap.TexFormatR8:
		return true
	}
	return false
}

// ==================== TexHeader ====================

// Header 表示 TEX 文件的头部信息。
// 对应 C# TexHeader。
type Header struct {
	Format        mipmap.TexFormat // 纹理格式
	Flags         int32            // 标志位（按位组合）
	TextureWidth  int32            // 纹理宽度
	TextureHeight int32            // 纹理高度
	ImageWidth    int32            // 实际图片宽度
	ImageHeight   int32            // 实际图片高度
	UnkInt0       uint32           // 未知字段（原样读写）
}

// readHeader 从 reader 中读取 TexHeader。对应 C# TexHeaderReader。
func readHeader(r byteReader) (*Header, error) {
	format, err := binutil.ReadInt32(r)
	if err != nil {
		return nil, fmt.Errorf("读取 TEX 格式失败: %w", err)
	}
	flags, err := binutil.ReadInt32(r)
	if err != nil {
		return nil, fmt.Errorf("读取 TEX 标志位失败: %w", err)
	}
	texW, err := binutil.ReadInt32(r)
	if err != nil {
		return nil, fmt.Errorf("读取纹理宽度失败: %w", err)
	}
	texH, err := binutil.ReadInt32(r)
	if err != nil {
		return nil, fmt.Errorf("读取纹理高度失败: %w", err)
	}
	imgW, err := binutil.ReadInt32(r)
	if err != nil {
		return nil, fmt.Errorf("读取图片宽度失败: %w", err)
	}
	imgH, err := binutil.ReadInt32(r)
	if err != nil {
		return nil, fmt.Errorf("读取图片高度失败: %w", err)
	}
	unk, err := binutil.ReadUInt32(r)
	if err != nil {
		return nil, fmt.Errorf("读取 UnkInt0 失败: %w", err)
	}

	tf := mipmap.TexFormat(format)
	if !isValidTexFormat(tf) {
		return nil, &EnumNotValidError{Value: format, Source: "TexFormat"}
	}

	return &Header{
		Format:        tf,
		Flags:         flags,
		TextureWidth:  texW,
		TextureHeight: texH,
		ImageWidth:    imgW,
		ImageHeight:   imgH,
		UnkInt0:       unk,
	}, nil
}

// writeHeader 将 Header 写入 writer。对应 C# TexHeaderWriter。
func (h *Header) write(w byteWriter) error {
	err := binutil.WriteInt32(w, int32(h.Format))
	if err != nil {
		return fmt.Errorf("写入Format失败: %w", err)
	}
	err = binutil.WriteInt32(w, h.Flags)
	if err != nil {
		return fmt.Errorf("写入Flags失败: %w", err)
	}
	err = binutil.WriteInt32(w, h.TextureWidth)
	if err != nil {
		return fmt.Errorf("写入TextureWidth失败: %w", err)
	}
	err = binutil.WriteInt32(w, h.TextureHeight)
	if err != nil {
		return fmt.Errorf("写入TextureHeight失败: %w", err)
	}
	err = binutil.WriteInt32(w, h.ImageWidth)
	if err != nil {
		return fmt.Errorf("写入ImageWidth失败: %w", err)
	}
	err = binutil.WriteInt32(w, h.ImageHeight)
	if err != nil {
		return fmt.Errorf("写入ImageHeight失败: %w", err)
	}
	err = binutil.WriteUInt32(w, h.UnkInt0)
	if err != nil {
		return fmt.Errorf("写入UnkInt0失败: %w", err)
	}
	return nil
}

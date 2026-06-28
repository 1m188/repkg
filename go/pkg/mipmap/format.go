// Package mipmap 定义了 MipmapFormat 枚举及相关扩展方法。
// 这是从 C# RePKG.Core/Texture/Enums/MipmapFormat.cs 移植而来。
package mipmap

// Format 表示 mipmap 的像素格式。
// 值与 C# 版本保持一致以确保二进制兼容。
type Format int32

// extBin 为原始/压缩格式的默认扩展名。
const extBin = "bin"

const (
	extJpg = "jpg"
	extPng = "png"
	extGif = "gif"
	extMp4 = "mp4"
)

const (
	// ==================== 原始像素格式 ====================

	// FormatRGBA8888 表示 32 位 RGBA 格式（每通道 8 位）。
	FormatRGBA8888 Format = 1
	// FormatR8 表示 8 位单通道（灰度）格式。
	FormatR8 Format = 2
	// FormatRG88 表示 16 位双通道格式（用于法线贴图等）。
	FormatRG88 Format = 3

	// ==================== 压缩格式 ====================

	// FormatCompressedDXT5 表示 BC3/DXT5 压缩（插值 alpha）。
	FormatCompressedDXT5 Format = 4
	// FormatCompressedDXT3 表示 BC2/DXT3 压缩（显式 alpha）。
	FormatCompressedDXT3 Format = 5
	// FormatCompressedDXT1 表示 BC1/DXT1 压缩（无 alpha 或 1 位 alpha）。
	FormatCompressedDXT1 Format = 6
)

const (
	// ==================== 图片格式 ====================
	// 值为 1000 + FreeImageFormat ID，与 C# 保持二进制兼容。

	// FormatImageBMP 表示 Windows Bitmap。
	FormatImageBMP Format = 1000 + iota
	// FormatImageICO 表示 Windows Icon。
	FormatImageICO
	// FormatImageJPEG 表示 JPEG 图片。
	FormatImageJPEG
	// FormatImageJNG 表示 JPEG Network Graphics。
	FormatImageJNG
	// FormatImageKOALA 表示 Commodore 64 Koala 格式。
	FormatImageKOALA
	// FormatImageLBM 表示 Amiga IFF/LBM。
	FormatImageLBM
	// FormatImageMNG 表示 Multiple Network Graphics。
	FormatImageMNG
	// FormatImagePBM 表示 Portable Bitmap (ASCII)。
	FormatImagePBM
	// FormatImagePBMRAW 表示 Portable Bitmap (Binary)。
	FormatImagePBMRAW
	// FormatImagePCD 表示 Kodak PhotoCD。
	FormatImagePCD
	// FormatImagePCX 表示 Zsoft Paintbrush PCX。
	FormatImagePCX
	// FormatImagePGM 表示 Portable Graymap (ASCII)。
	FormatImagePGM
	// FormatImagePGMRAW 表示 Portable Graymap (Binary)。
	FormatImagePGMRAW
	// FormatImagePNG 表示 Portable Network Graphics。
	FormatImagePNG
	// FormatImagePPM 表示 Portable Pixelmap (ASCII)。
	FormatImagePPM
	// FormatImagePPMRAW 表示 Portable Pixelmap (Binary)。
	FormatImagePPMRAW
	// FormatImageRAS 表示 Sun Rasterfile。
	FormatImageRAS
	// FormatImageTARGA 表示 Truevision Targa。
	FormatImageTARGA
	// FormatImageTIFF 表示 Tagged Image File Format。
	FormatImageTIFF
	// FormatImageWBMP 表示 Wireless Bitmap。
	FormatImageWBMP
	// FormatImagePSD 表示 Adobe Photoshop。
	FormatImagePSD
	// FormatImageCUT 表示 Dr. Halo CUT。
	FormatImageCUT
	// FormatImageXBM 表示 X11 Bitmap。
	FormatImageXBM
	// FormatImageXPM 表示 X11 Pixmap。
	FormatImageXPM
	// FormatImageDDS 表示 DirectDraw Surface。
	FormatImageDDS
	// FormatImageGIF 表示 Graphics Interchange Format。
	FormatImageGIF
	// FormatImageHDR 表示 High Dynamic Range。
	FormatImageHDR
	// FormatImageFAXG3 表示 Raw Fax CCITT G3。
	FormatImageFAXG3
	// FormatImageSGI 表示 Silicon Graphics SGI。
	FormatImageSGI
	// FormatImageEXR 表示 OpenEXR。
	FormatImageEXR
	// FormatImageJ2K 表示 JPEG-2000 Code Stream。
	FormatImageJ2K
	// FormatImageJP2 表示 JPEG-2000 Image。
	FormatImageJP2
	// FormatImagePFM 表示 Portable FloatMap。
	FormatImagePFM
	// FormatImagePICT 表示 Macintosh PICT。
	FormatImagePICT
	// FormatImageRAW 表示 RAW Camera Image。
	FormatImageRAW
	// FormatImageMP4 表示 MP4 图片帧。
	FormatImageMP4
)

const (
	// ==================== 视频格式 ====================

	// FormatVideoMp4 表示 MP4 视频。
	FormatVideoMp4 Format = 1036
)

// IsRawFormat 判断是否为原始像素格式（RGBA8888、R8、RG88）。
func (f Format) IsRawFormat() bool {
	return f >= FormatRGBA8888 && f <= FormatRG88
}

// IsCompressed 判断是否为压缩格式（DXT1/3/5）。
func (f Format) IsCompressed() bool {
	return f == FormatCompressedDXT5 || f == FormatCompressedDXT3 || f == FormatCompressedDXT1
}

// IsImage 判断是否为图片格式。
func (f Format) IsImage() bool {
	return f >= FormatImageBMP && f < FormatVideoMp4
}

// IsVideo 判断是否为视频格式。
func (f Format) IsVideo() bool {
	return f == FormatVideoMp4
}

// String 返回格式的可读名称。
func (f Format) String() string {
	switch f {
	case FormatRGBA8888:
		return "RGBA8888"
	case FormatR8:
		return "R8"
	case FormatRG88:
		return "RG88"
	case FormatCompressedDXT1:
		return "DXT1"
	case FormatCompressedDXT3:
		return "DXT3"
	case FormatCompressedDXT5:
		return "DXT5"
	case FormatImagePNG:
		return "PNG"
	case FormatImageJPEG:
		return "JPEG"
	case FormatImageGIF:
		return "GIF"
	case FormatVideoMp4:
		return "MP4"
	default:
		return "其他图片"
	}
}

// GetFileExtension 返回该格式对应的文件扩展名（不含点号）。
//
//nolint:gocyclo,cyclop,funlen // 全格式映射表，复杂度不可避免
func (f Format) GetFileExtension() string {
	switch f {
	case FormatImageBMP:
		return "bmp"
	case FormatImageICO:
		return "ico"
	case FormatImageJPEG:
		return extJpg
	case FormatImageJNG:
		return "jng"
	case FormatImageKOALA:
		return "koa"
	case FormatImageLBM:
		return "lbm"
	case FormatImageMNG:
		return "mng"
	case FormatImagePBM, FormatImagePBMRAW:
		return "pbm"
	case FormatImagePCD:
		return "pcd"
	case FormatImagePCX:
		return "pcx"
	case FormatImagePGM, FormatImagePGMRAW:
		return "pgm"
	case FormatImagePNG:
		return extPng
	case FormatImagePPM, FormatImagePPMRAW:
		return "ppm"
	case FormatImageRAS:
		return "ras"
	case FormatImageTARGA:
		return "tga"
	case FormatImageTIFF:
		return "tif"
	case FormatImageWBMP:
		return "wbmp"
	case FormatImagePSD:
		return "psd"
	case FormatImageCUT:
		return "cut"
	case FormatImageXBM:
		return "xbm"
	case FormatImageXPM:
		return "xpm"
	case FormatImageDDS:
		return "dds"
	case FormatImageGIF:
		return extGif
	case FormatImageHDR:
		return "hdr"
	case FormatImageFAXG3:
		return "g3"
	case FormatImageSGI:
		return "sgi"
	case FormatImageEXR:
		return "exr"
	case FormatImageJ2K:
		return "j2k"
	case FormatImageJP2:
		return "jp2"
	case FormatImagePFM:
		return "pfm"
	case FormatImagePICT:
		return "pict"
	case FormatImageRAW:
		return "raw"
	case FormatImageMP4, FormatVideoMp4:
		return extMp4
	default:
		return extBin
	}
}

// TexFormat 表示 TEX 文件头中声明的纹理格式。
// 值与 C# TexFormat 枚举保持一致。
type TexFormat int32

const (
	// TexFormatRGBA8888 RGBA 8 位每通道。
	TexFormatRGBA8888 TexFormat = 0
	// TexFormatDXT5 BC3/DXT5 压缩。
	TexFormatDXT5 TexFormat = 4
	// TexFormatDXT3 BC2/DXT3 压缩。
	TexFormatDXT3 TexFormat = 6
	// TexFormatDXT1 BC1/DXT1 压缩。
	TexFormatDXT1 TexFormat = 7
	// TexFormatRG88 双通道 16 位。
	TexFormatRG88 TexFormat = 8
	// TexFormatR8 单通道 8 位。
	TexFormatR8 TexFormat = 9
)

// FormatForTex 从 FreeImage 格式和 TEX 格式派生出 MipmapFormat。
// 对应 C# TexMipmapFormatGetter.GetFormatForTex。
// 重要：当 freeImageFormat != FIF_UNKNOWN 时，FreeImageFormat 优先于 TexFormat。
func FormatForTex(freeImageFormat int32, texFormat TexFormat) Format {
	// 当 FreeImage 格式已知时，优先使用 FreeImage 格式映射
	if freeImageFormat != -1 {
		return freeImageFormatToMipmapFormat(freeImageFormat)
	}

	// 否则由 TexFormat 决定
	switch texFormat {
	case TexFormatRGBA8888:
		return FormatRGBA8888
	case TexFormatDXT5:
		return FormatCompressedDXT5
	case TexFormatDXT3:
		return FormatCompressedDXT3
	case TexFormatDXT1:
		return FormatCompressedDXT1
	case TexFormatRG88:
		return FormatRG88
	case TexFormatR8:
		return FormatR8
	}
	return FormatRGBA8888
}

// freeImageFormatToMipmapFormat 将 FreeImageFormat ID 映射到 MipmapFormat。
// FreeImageFormat 值域为 0-35，映射为 FormatImageBMP(1000)~FormatImageRAW(1034)。
// FIF_MP4(35) 特殊处理，返回 FormatVideoMp4(1036) 而非图片格式。
func freeImageFormatToMipmapFormat(fif int32) Format {
	if fif == 35 {
		return FormatVideoMp4
	}
	if fif >= 0 && fif < 35 {
		return FormatImageBMP + Format(fif)
	}
	return FormatRGBA8888
}

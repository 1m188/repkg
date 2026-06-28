package mipmap

import (
	"testing"
)

func TestFormat分类(t *testing.T) {
	tests := []struct {
		name    string
		format  Format
		isRaw   bool
		isComp  bool
		isImage bool
		isVideo bool
		ext     string
	}{
		{"RGBA8888是原始格式", FormatRGBA8888, true, false, false, false, extBin},
		{"R8是原始格式", FormatR8, true, false, false, false, extBin},
		{"RG88是原始格式", FormatRG88, true, false, false, false, extBin},
		{"DXT1是压缩格式", FormatCompressedDXT1, false, true, false, false, extBin},
		{"DXT3是压缩格式", FormatCompressedDXT3, false, true, false, false, extBin},
		{"DXT5是压缩格式", FormatCompressedDXT5, false, true, false, false, extBin},
		{"PNG是图片格式", FormatImagePNG, false, false, true, false, extPng},
		{"JPEG是图片格式", FormatImageJPEG, false, false, true, false, extJpg},
		{"GIF是图片格式", FormatImageGIF, false, false, true, false, extGif},
		{"MP4是视频格式", FormatVideoMp4, false, false, false, true, extMp4},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.format.IsRawFormat() != tt.isRaw {
				t.Errorf("IsRawFormat() = %v, 期望 %v", tt.format.IsRawFormat(), tt.isRaw)
			}
			if tt.format.IsCompressed() != tt.isComp {
				t.Errorf("IsCompressed() = %v, 期望 %v", tt.format.IsCompressed(), tt.isComp)
			}
			if tt.format.IsImage() != tt.isImage {
				t.Errorf("IsImage() = %v, 期望 %v", tt.format.IsImage(), tt.isImage)
			}
			if tt.format.IsVideo() != tt.isVideo {
				t.Errorf("IsVideo() = %v, 期望 %v", tt.format.IsVideo(), tt.isVideo)
			}
			if tt.format.GetFileExtension() != tt.ext {
				t.Errorf("GetFileExtension() = %s, 期望 %s", tt.format.GetFileExtension(), tt.ext)
			}
		})
	}
}

func TestFormatForTex(t *testing.T) {
	tests := []struct {
		name            string
		freeImageFormat int32
		texFormat       TexFormat
		want            Format
	}{
		{"RGBA8888无特殊格式→RGBA8888", -1, TexFormatRGBA8888, FormatRGBA8888},
		{"RGBA8888+JPEG→JPEG", 2, TexFormatRGBA8888, FormatImageJPEG},
		{"RGBA8888+PNG→PNG", 13, TexFormatRGBA8888, FormatImagePNG},
		{"DXT5→压缩DXT5", -1, TexFormatDXT5, FormatCompressedDXT5},
		{"DXT3→压缩DXT3", -1, TexFormatDXT3, FormatCompressedDXT3},
		{"DXT1→压缩DXT1", -1, TexFormatDXT1, FormatCompressedDXT1},
		{"RG88→RG88", -1, TexFormatRG88, FormatRG88},
		{"R8→R8", -1, TexFormatR8, FormatR8},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := FormatForTex(tt.freeImageFormat, tt.texFormat)
			if got != tt.want {
				t.Errorf("FormatForTex(%d, %d) = %d, 期望 %d",
					tt.freeImageFormat, tt.texFormat, got, tt.want)
			}
		})
	}
}

// TestMipmapFormatEnumValues 验证 MipmapFormat 枚举值与 C# 原版一致。
func TestMipmapFormatEnumValues(t *testing.T) {
	tests := []struct {
		name  string
		value Format
		want  int32
	}{
		// 原始像素格式 (C#: auto 1-3)
		{"RGBA8888=1", FormatRGBA8888, 1},
		{"R8=2", FormatR8, 2},
		{"RG88=3", FormatRG88, 3},

		// 压缩格式 (C#: auto 4-6)
		{"CompressedDXT5=4", FormatCompressedDXT5, 4},
		{"CompressedDXT3=5", FormatCompressedDXT3, 5},
		{"CompressedDXT1=6", FormatCompressedDXT1, 6},

		// 图片格式从 1000 起步 (与 C# FreeImage 格式 ID 对应)
		{"ImageBMP=1000", FormatImageBMP, 1000},
		{"ImageJPEG=1002", FormatImageJPEG, 1002},
		{"ImagePNG=1013", FormatImagePNG, 1013},
		{"ImageGIF=1025", FormatImageGIF, 1025},
		{"ImageDDS=1024", FormatImageDDS, 1024},
		{"ImageTIFF=1018", FormatImageTIFF, 1018},
		{"ImageMP4=1035", FormatImageMP4, 1035},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if int32(tt.value) != tt.want {
				t.Errorf("%s: 枚举值 = %d, 期望 %d (与 C# 不一致)", tt.name, tt.value, tt.want)
			}
		})
	}
}

// TestFreeImageFormatToMipmap 验证完整的 FreeImageFormat → MipmapFormat 映射表。
// 对应 C# FreeImageFormatToMipmapFormat 方法。
func TestFreeImageFormatToMipmap(t *testing.T) {
	tests := []struct {
		name            string
		freeImageFormat int32
		texFormat       TexFormat
		want            Format
	}{
		// 当 freeImageFormat == FIF_UNKNOWN(-1) 时，由 TexFormat 决定
		{"FIF_UNKNOWN+RGBA8888→RGBA8888", -1, TexFormatRGBA8888, FormatRGBA8888},
		{"FIF_UNKNOWN+DXT5→CompressedDXT5", -1, TexFormatDXT5, FormatCompressedDXT5},
		{"FIF_UNKNOWN+DXT3→CompressedDXT3", -1, TexFormatDXT3, FormatCompressedDXT3},
		{"FIF_UNKNOWN+DXT1→CompressedDXT1", -1, TexFormatDXT1, FormatCompressedDXT1},
		{"FIF_UNKNOWN+RG88→RG88", -1, TexFormatRG88, FormatRG88},
		{"FIF_UNKNOWN+R8→R8", -1, TexFormatR8, FormatR8},

		// 当 freeImageFormat != FIF_UNKNOWN 时，优先使用 FreeImageFormat 映射
		// C# 关键行为：FreeImageFormat 优先级高于 TexFormat
		{"FIF_BMP(0)→ImageBMP", 0, TexFormatRGBA8888, FormatImageBMP},
		{"FIF_JPEG(2)→ImageJPEG", 2, TexFormatRGBA8888, FormatImageJPEG},
		{"FIF_JPEG(2)+DXT5→ImageJPEG(FreeFormat优先)", 2, TexFormatDXT5, FormatImageJPEG},
		{"FIF_PNG(13)→ImagePNG", 13, TexFormatRGBA8888, FormatImagePNG},
		{"FIF_PNG(13)+DXT3→ImagePNG(FreeFormat优先)", 13, TexFormatDXT3, FormatImagePNG},
		{"FIF_DDS(24)→ImageDDS", 24, TexFormatRGBA8888, FormatImageDDS},
		{"FIF_GIF(25)→ImageGIF", 25, TexFormatRGBA8888, FormatImageGIF},
		{"FIF_TIFF(18)→ImageTIFF", 18, TexFormatRGBA8888, FormatImageTIFF},
		{"FIF_TARGA(17)→ImageTARGA", 17, TexFormatRGBA8888, FormatImageTARGA},
		{"FIF_ICO(1)→ImageICO", 1, TexFormatRGBA8888, FormatImageICO},
		// FIF_MP4(35) 特殊处理，应返回 FormatVideoMp4 而非图片格式
		{"FIF_MP4(35)→VideoMp4", 35, TexFormatRGBA8888, FormatVideoMp4},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := FormatForTex(tt.freeImageFormat, tt.texFormat)
			if got != tt.want {
				t.Errorf("FormatForTex(%d, %d) = %d(%s), 期望 %d(%s)",
					tt.freeImageFormat, tt.texFormat, got, got.String(), tt.want, tt.want.String())
			}
		})
	}
}

// TestMP4NotImage 验证 MP4 视频格式的 IsImage() 返回 false（与 C# 一致）。
func TestMP4NotImage(t *testing.T) {
	got := FormatForTex(35, TexFormatRGBA8888)
	if got.IsImage() {
		t.Error("MP4 格式.IsImage() = true, 期望 false (C# VideoMp4 不是图片格式)")
	}
	if !got.IsVideo() {
		t.Error("MP4 格式.IsVideo() = false, 期望 true")
	}
}

// TestAllImageFormatsHaveExtension 验证所有图片格式都有正确的文件扩展名。
func TestAllImageFormatsHaveExtension(t *testing.T) {
	formats := map[Format]string{
		FormatImageBMP:   "bmp",
		FormatImageICO:   "ico",
		FormatImageJPEG:  "jpg",
		FormatImageJNG:   "jng",
		FormatImageKOALA: "koa",
		FormatImageLBM:   "lbm",
		FormatImageMNG:   "mng",
		FormatImagePBM:   "pbm",
		FormatImagePCD:   "pcd",
		FormatImagePCX:   "pcx",
		FormatImagePGM:   "pgm",
		FormatImagePNG:   "png",
		FormatImagePPM:   "ppm",
		FormatImageRAS:   "ras",
		FormatImageTARGA: "tga",
		FormatImageTIFF:  "tif",
		FormatImageWBMP:  "wbmp",
		FormatImagePSD:   "psd",
		FormatImageCUT:   "cut",
		FormatImageXBM:   "xbm",
		FormatImageXPM:   "xpm",
		FormatImageDDS:   "dds",
		FormatImageGIF:   "gif",
		FormatImageHDR:   "hdr",
		FormatImageFAXG3: "g3",
		FormatImageSGI:   "sgi",
		FormatImageEXR:   "exr",
		FormatImageJ2K:   "j2k",
		FormatImageJP2:   "jp2",
		FormatImagePFM:   "pfm",
		FormatImagePICT:  "pict",
		FormatImageRAW:   "raw",
		FormatImageMP4:   "mp4",
	}

	for f, wantExt := range formats {
		t.Run(f.String(), func(t *testing.T) {
			if !f.IsImage() {
				t.Errorf("%s.IsImage() = false, 期望 true", f.String())
			}
			gotExt := f.GetFileExtension()
			if gotExt != wantExt {
				t.Errorf("%s.GetFileExtension() = %q, 期望 %q", f.String(), gotExt, wantExt)
			}
		})
	}
}

// TestNoImageFormatsNotIsImage 验证非图片格式不返回 IsImage=true。
func TestNoImageFormatsNotIsImage(t *testing.T) {
	noImages := []Format{
		FormatRGBA8888, FormatR8, FormatRG88,
		FormatCompressedDXT1, FormatCompressedDXT3, FormatCompressedDXT5,
		FormatVideoMp4,
	}
	for _, f := range noImages {
		if f.IsImage() {
			t.Errorf("%s.IsImage() = true, 期望 false", f.String())
		}
	}
}

func TestFormatString(t *testing.T) {
	if s := FormatRGBA8888.String(); s != "RGBA8888" {
		t.Errorf("FormatRGBA8888.String() = %q, 期望 %q", s, "RGBA8888")
	}
	if s := FormatCompressedDXT5.String(); s != "DXT5" {
		t.Errorf("FormatCompressedDXT5.String() = %q, 期望 %q", s, "DXT5")
	}
	if s := FormatImageGIF.String(); s != "GIF" {
		t.Errorf("FormatImageGIF.String() = %q, 期望 %q", s, "GIF")
	}
	if s := FormatVideoMp4.String(); s != "MP4" {
		t.Errorf("FormatVideoMp4.String() = %q, 期望 %q", s, "MP4")
	}
}

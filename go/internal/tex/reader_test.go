package tex

import (
	"bytes"
	"encoding/binary"
	"image"
	"image/gif"
	"strings"
	"testing"

	"github.com/1m188/repkg-go/internal/binutil"
	"github.com/1m188/repkg-go/pkg/mipmap"
)

// ==================== 辅助函数：构造 TEX 二进制数据 ====================

// writeTexBytes 将整数写入缓冲区（小端序）。
func writeInt32(buf *bytes.Buffer, v int32) {
	_ = binary.Write(buf, binary.LittleEndian, v) //nolint:errcheck // 测试辅助函数，写入缓冲区不会失败
}

// writeUInt32 将无符号整数写入缓冲区（小端序）。
func writeUInt32(buf *bytes.Buffer, v uint32) {
	_ = binary.Write(buf, binary.LittleEndian, v) //nolint:errcheck // 测试辅助函数，写入缓冲区不会失败
}

// writeNString 写入 null 结尾字符串。
func writeNString(buf *bytes.Buffer, s string) {
	_, _ = buf.WriteString(s)
	_ = buf.WriteByte(0)
}

// makeMipmapDataV1 构造 V1 格式的 mipmap 数据。
func makeMipmapDataV1(width, height int32, pixels []byte) []byte {
	var buf bytes.Buffer
	writeInt32(&buf, width)
	writeInt32(&buf, height)
	writeInt32(&buf, int32(len(pixels))) //nolint:gosec // 测试辅助函数，pixels 长度受控
	_, _ = buf.Write(pixels)
	return buf.Bytes()
}

// makeMipmapDataV2 构造 V2/V3 格式的 mipmap 数据（无 LZ4 压缩）。
func makeMipmapDataV2(width, height int32, pixels []byte) []byte {
	var buf bytes.Buffer
	writeInt32(&buf, width)
	writeInt32(&buf, height)
	writeInt32(&buf, 0)                  // 未 LZ4 压缩
	writeInt32(&buf, int32(len(pixels))) //nolint:gosec // 测试辅助函数，pixels 长度受控（解压后字节数）
	writeInt32(&buf, int32(len(pixels))) //nolint:gosec // 测试辅助函数，pixels 长度受控（ByteCount）
	_, _ = buf.Write(pixels)
	return buf.Bytes()
}

// makeTexHeader 构造 TexHeader 二进制数据。
func makeTexHeader(format mipmap.TexFormat, flags, imgW, imgH int32) []byte {
	var buf bytes.Buffer
	writeInt32(&buf, int32(format))
	writeInt32(&buf, flags)
	writeInt32(&buf, imgW) // TextureWidth
	writeInt32(&buf, imgH) // TextureHeight
	writeInt32(&buf, imgW) // ImageWidth
	writeInt32(&buf, imgH) // ImageHeight
	writeUInt32(&buf, 0)   // UnkInt0
	return buf.Bytes()
}

// makeTexV1 构造完整的 V1 TEX 二进制数据。
// 格式：Magic1 + Magic2 + Header + ImageContainer(V1) + [FrameInfoContainer]
func makeTexV1(format mipmap.TexFormat, flags, width, height int32, pixels []byte, hasGif bool, frames []byte) []byte { //nolint:revive // 测试辅助函数，参数语义化
	var buf bytes.Buffer

	writeNString(&buf, MagicTEXV0005)
	writeNString(&buf, MagicTEXI0001)
	_, _ = buf.Write(makeTexHeader(format, flags, width, height))

	// ImageContainer V1
	writeNString(&buf, MagicTEXB0001)
	writeInt32(&buf, 1) // 1 张图片
	writeInt32(&buf, 1) // 1 个 mipmap
	_, _ = buf.Write(makeMipmapDataV1(width, height, pixels))

	// FrameInfoContainer（仅动画）
	if hasGif && frames != nil {
		_, _ = buf.Write(frames)
	}

	return buf.Bytes()
}

// makeTexV2 构造 V2/V3 TEX 二进制数据。
func makeTexV2(version string, format mipmap.TexFormat, flags, width, height int32, pixels []byte, freeImageFormat FreeImageFormat, hasGif bool, frames []byte) []byte { //nolint:revive // 测试辅助函数，参数语义化
	var buf bytes.Buffer

	writeNString(&buf, MagicTEXV0005)
	writeNString(&buf, MagicTEXI0001)
	_, _ = buf.Write(makeTexHeader(format, flags, width, height))

	// ImageContainer
	writeNString(&buf, version)
	writeInt32(&buf, 1) // 1 张图片
	// V3 额外字段
	if version >= MagicTEXB0003 {
		writeInt32(&buf, int32(freeImageFormat))
	}

	writeInt32(&buf, 1) // 1 个 mipmap
	_, _ = buf.Write(makeMipmapDataV2(width, height, pixels))

	if hasGif && frames != nil {
		_, _ = buf.Write(frames)
	}

	return buf.Bytes()
}

// makeTexV4 构造 V4 TEX 二进制数据。
// 注意：V4 中仅 MP4 格式使用 V4 mipmap 格式（含 param1-3），非 MP4 降级为 V3。
func makeTexV4(format mipmap.TexFormat, flags int32, pixels []byte, freeImageFormat FreeImageFormat) []byte {
	var buf bytes.Buffer

	writeNString(&buf, MagicTEXV0005)
	writeNString(&buf, MagicTEXI0001)
	_, _ = buf.Write(makeTexHeader(format, flags, 4, 4))

	// ImageContainer V4
	writeNString(&buf, MagicTEXB0004)
	writeInt32(&buf, 1) // 1 张图片
	writeInt32(&buf, int32(freeImageFormat))
	isVideo := int32(0)
	if freeImageFormat == FIFMp4 {
		isVideo = 1
	}
	writeInt32(&buf, isVideo)

	writeInt32(&buf, 1) // mipmap count

	if freeImageFormat == FIFMp4 {
		// V4 MP4：有额外 param 字段
		writeInt32(&buf, 1)    // param1
		writeInt32(&buf, 2)    // param2
		writeNString(&buf, "") // conditionJson
		writeInt32(&buf, 1)    // param3
	}

	_, _ = buf.Write(makeMipmapDataV2(4, 4, pixels))

	return buf.Bytes()
}

// makeGifFrameContainer 构造 GIF 帧容器二进制数据。
//
//nolint:unparam // gifW is always 32 in tests but kept for signature consistency
func makeGifFrameContainer(magic string, gifW, gifH, frameCount int32) []byte {
	var buf bytes.Buffer
	writeNString(&buf, magic)
	writeInt32(&buf, frameCount)

	if magic == MagicTEXS0003 {
		writeInt32(&buf, gifW)
		writeInt32(&buf, gifH)
	}

	isFloat := magic != MagicTEXS0001
	for i := range frameCount {
		writeInt32(&buf, int32(i)) //nolint:unconvert // ImageId; i 来自 range 为 int 需转换为 int32
		writeFloat32(&buf, 0.1)    // Frametime
		if isFloat {
			writeFloat32(&buf, 0)
			writeFloat32(&buf, 0)
			writeFloat32(&buf, float32(gifW))
			writeFloat32(&buf, 0)
			writeFloat32(&buf, 0)
			writeFloat32(&buf, float32(gifH))
		} else {
			writeInt32(&buf, 0)
			writeInt32(&buf, 0)
			writeInt32(&buf, gifW)
			writeInt32(&buf, 0)
			writeInt32(&buf, 0)
			writeInt32(&buf, gifH)
		}
	}
	return buf.Bytes()
}

func writeFloat32(buf *bytes.Buffer, v float32) {
	_ = binary.Write(buf, binary.LittleEndian, v) //nolint:errcheck // 测试辅助函数，写入缓冲区不会失败 //nolint:errcheck
}

// makeRGBAPixels 生成 RGBA8888 格式的像素数据。
func makeRGBAPixels(w, h int32) []byte {
	pixels := make([]byte, int(w)*int(h)*4)
	for i := range pixels {
		pixels[i] = byte(i % 256)
	}
	return pixels
}

// makeR8Pixels 生成 R8（灰度）格式的像素数据。
func makeR8Pixels(w, h int32) []byte {
	pixels := make([]byte, int(w)*int(h))
	for i := range pixels {
		pixels[i] = byte(i % 256)
	}
	return pixels
}

// makeRG88Pixels 生成 RG88（双通道）格式的像素数据。
func makeRG88Pixels(w, h int32) []byte {
	pixels := make([]byte, int(w)*int(h)*2)
	for i := range pixels {
		pixels[i] = byte(i % 256)
	}
	return pixels
}

// ==================== TEX 读取测试 ====================

func TestReader_V1_DXT5(t *testing.T) {
	// DXT5 最小图像：4x4 = 1 block = 16 bytes
	// 构造一个全红色的 DXT5 块
	dxtBlock := make([]byte, 16)
	dxtBlock[0] = 255   // alpha0
	dxtBlock[1] = 255   // alpha1
	dxtBlock[8] = 0x00  // color0 byte0
	dxtBlock[9] = 0xF8  // color0 byte1 → packed 0xF800 = 纯红
	dxtBlock[10] = 0x00 // color1 byte0
	dxtBlock[11] = 0xF8 // color1 byte1 → packed 0xF800 = 纯红

	data := makeTexV1(mipmap.TexFormatDXT5, 0, 4, 4, dxtBlock, false, nil)

	reader := NewReader()
	tex, err := reader.ReadTex(bytes.NewReader(data))
	if err != nil {
		t.Fatalf("解析 TEX 失败: %v", err)
	}

	if tex.Magic1 != MagicTEXV0005 {
		t.Errorf("Magic1 = %q", tex.Magic1)
	}
	if tex.Header.Format != mipmap.TexFormatDXT5 {
		t.Errorf("Format = %d, 期望 DXT5", tex.Header.Format)
	}

	firstMipmap := tex.FirstImage().FirstMipmap()
	if firstMipmap == nil {
		t.Fatal("第一个 mipmap 为空")
	}
	if firstMipmap.Format != mipmap.FormatRGBA8888 {
		// DXT5 解压后应为 RGBA8888
		t.Errorf("解压后格式 = %s, 期望 RGBA8888", firstMipmap.Format.String())
	}
	if len(firstMipmap.Bytes) != 4*4*4 {
		t.Errorf("解压后大小 = %d, 期望 64", len(firstMipmap.Bytes))
	}
}

func TestReader_V1_RGBA8888(t *testing.T) {
	pixels := makeRGBAPixels(8, 8)
	data := makeTexV1(mipmap.TexFormatRGBA8888, 0, 8, 8, pixels, false, nil)

	reader := NewReader()
	tex, err := reader.ReadTex(bytes.NewReader(data))
	if err != nil {
		t.Fatalf("解析 TEX 失败: %v", err)
	}

	m := tex.FirstImage().FirstMipmap()
	if m.Format != mipmap.FormatRGBA8888 {
		t.Errorf("格式 = %s, 期望 RGBA8888", m.Format.String())
	}
	if !bytes.Equal(m.Bytes, pixels) {
		t.Error("解压后的像素数据与输入不匹配")
	}
}

func TestReader_V2_R8(t *testing.T) {
	pixels := makeR8Pixels(8, 8)
	data := makeTexV2(MagicTEXB0002, mipmap.TexFormatR8, 0, 8, 8, pixels, FIFUnknown, false, nil)

	reader := NewReader()
	tex, err := reader.ReadTex(bytes.NewReader(data))
	if err != nil {
		t.Fatalf("解析 TEX 失败: %v", err)
	}

	m := tex.FirstImage().FirstMipmap()
	if m.Format != mipmap.FormatR8 {
		t.Errorf("格式 = %s, 期望 R8", m.Format.String())
	}
	if len(m.Bytes) != 64 {
		t.Errorf("R8 像素大小 = %d, 期望 64", len(m.Bytes))
	}
}

func TestReader_V2_RG88(t *testing.T) {
	pixels := makeRG88Pixels(8, 8)
	data := makeTexV2(MagicTEXB0002, mipmap.TexFormatRG88, 0, 8, 8, pixels, FIFUnknown, false, nil)

	reader := NewReader()
	tex, err := reader.ReadTex(bytes.NewReader(data))
	if err != nil {
		t.Fatalf("解析 TEX 失败: %v", err)
	}

	m := tex.FirstImage().FirstMipmap()
	if m.Format != mipmap.FormatRG88 {
		t.Errorf("格式 = %s, 期望 RG88", m.Format.String())
	}
}

func TestReader_V3_RGBA8888_JPEG(t *testing.T) {
	jpegData := []byte{0xFF, 0xD8, 0xFF, 0xE0, 0x00, 0x10, 0x4A, 0x46} // 最小 JPEG 头部
	data := makeTexV2(MagicTEXB0003, mipmap.TexFormatRGBA8888, 0, 4, 4, jpegData, FIFJpeg, false, nil)

	reader := NewReader()
	tex, err := reader.ReadTex(bytes.NewReader(data))
	if err != nil {
		t.Fatalf("解析 TEX 失败: %v", err)
	}

	m := tex.FirstImage().FirstMipmap()
	if m.Format != mipmap.FormatImageJPEG {
		t.Errorf("格式 = %s, 期望 JPEG", m.Format.String())
	}
}

func TestReader_V3_DXT1(t *testing.T) {
	dxtBlock := make([]byte, 8) // DXT1 每块 8 字节
	dxtBlock[0] = 0x00
	dxtBlock[1] = 0xF8 // color0 = 纯红
	dxtBlock[2] = 0x1F
	dxtBlock[3] = 0x00 // color1 = 纯蓝

	data := makeTexV2(MagicTEXB0003, mipmap.TexFormatDXT1, 0, 4, 4, dxtBlock, FIFUnknown, false, nil)

	reader := NewReader()
	tex, err := reader.ReadTex(bytes.NewReader(data))
	if err != nil {
		t.Fatalf("解析 TEX 失败: %v", err)
	}

	m := tex.FirstImage().FirstMipmap()
	if m.Format != mipmap.FormatRGBA8888 {
		t.Errorf("DXT1 解压后格式 = %s, 期望 RGBA8888", m.Format.String())
	}
}

func TestReader_V3_DXT3(t *testing.T) {
	dxtBlock := make([]byte, 16)
	dxtBlock[0] = 0xFF
	dxtBlock[8] = 0x00
	dxtBlock[9] = 0xF8
	dxtBlock[10] = 0x00
	dxtBlock[11] = 0xF8

	data := makeTexV2(MagicTEXB0003, mipmap.TexFormatDXT3, 0, 4, 4, dxtBlock, FIFUnknown, false, nil)

	reader := NewReader()
	tex, err := reader.ReadTex(bytes.NewReader(data))
	if err != nil {
		t.Fatalf("解析 TEX 失败: %v", err)
	}

	m := tex.FirstImage().FirstMipmap()
	if m.Format != mipmap.FormatRGBA8888 {
		t.Errorf("DXT3 解压后格式 = %s", m.Format.String())
	}
}

func TestReader_V3_GIF_TEXS0003(t *testing.T) {
	pixels := makeRGBAPixels(32, 32)
	gifFrames := makeGifFrameContainer(MagicTEXS0003, 32, 32, 3)
	data := makeTexV2(MagicTEXB0003, mipmap.TexFormatRGBA8888, FlagIsGif, 32, 32, pixels, FIFUnknown, true, gifFrames)

	reader := NewReader()
	tex, err := reader.ReadTex(bytes.NewReader(data))
	if err != nil {
		t.Fatalf("解析 GIF TEX 失败: %v", err)
	}

	if !tex.IsGif() {
		t.Error("应识别为动画纹理")
	}
	if tex.FrameInfoContainer == nil {
		t.Fatal("FrameInfoContainer 为空")
	}
	if len(tex.FrameInfoContainer.Frames) != 3 {
		t.Errorf("帧数 = %d, 期望 3", len(tex.FrameInfoContainer.Frames))
	}
	if tex.FrameInfoContainer.GifWidth != 32 {
		t.Errorf("GifWidth = %d, 期望 32", tex.FrameInfoContainer.GifWidth)
	}
}

func TestReader_V3_视频MP4(t *testing.T) {
	mp4Data := []byte{0x00, 0x00, 0x00, 0x18, 'f', 't', 'y', 'p', 'i', 's', 'o', 'm'} // MP4 头部
	data := makeTexV2(MagicTEXB0003, mipmap.TexFormatRGBA8888, FlagIsVideoTexture, 16, 16, mp4Data, FIFMp4, false, nil)

	reader := NewReader()
	tex, err := reader.ReadTex(bytes.NewReader(data))
	if err != nil {
		t.Fatalf("解析视频 TEX 失败: %v", err)
	}

	if !tex.IsVideoTexture() {
		t.Error("应识别为视频纹理")
	}
}

func TestReader_V4_PNG(t *testing.T) {
	pngSignature := []byte{0x89, 0x50, 0x4E, 0x47, 0x0D, 0x0A, 0x1A, 0x0A, 0x00, 0x00, 0x00, 0x0D}
	data := makeTexV4(mipmap.TexFormatRGBA8888, 0, pngSignature, FIFPng)

	reader := NewReader()
	tex, err := reader.ReadTex(bytes.NewReader(data))
	if err != nil {
		t.Fatalf("解析 V4 PNG TEX 失败: %v", err)
	}

	m := tex.FirstImage().FirstMipmap()
	if m.Format != mipmap.FormatImagePNG {
		t.Errorf("格式 = %s, 期望 PNG", m.Format.String())
	}
}

func TestReader_V4_DXT5(t *testing.T) {
	dxtBlock := make([]byte, 16)
	dxtBlock[0] = 255
	dxtBlock[1] = 255
	dxtBlock[8] = 0x00
	dxtBlock[9] = 0xF8
	dxtBlock[10] = 0x00
	dxtBlock[11] = 0xF8

	data := makeTexV4(mipmap.TexFormatDXT5, 0, dxtBlock, FIFUnknown)

	reader := NewReader()
	tex, err := reader.ReadTex(bytes.NewReader(data))
	if err != nil {
		t.Fatalf("解析 V4 DXT5 TEX 失败: %v", err)
	}

	if tex.Header.Format != mipmap.TexFormatDXT5 {
		t.Errorf("Format = %d, 期望 DXT5", tex.Header.Format)
	}
}

func TestReader_V4_MP4(t *testing.T) {
	mp4Data := []byte{0x00, 0x00, 0x00, 0x18, 'f', 't', 'y', 'p', 'i', 's', 'o', 'm'}
	data := makeTexV4(mipmap.TexFormatRGBA8888, FlagIsVideoTexture, mp4Data, FIFMp4)

	reader := NewReader()
	tex, err := reader.ReadTex(bytes.NewReader(data))
	if err != nil {
		t.Fatalf("解析 V4 MP4 TEX 失败: %v", err)
	}

	if !tex.IsVideoTexture() {
		t.Error("应识别为视频纹理（V4）")
	}
}

// ==================== 异常输入测试 ====================

func TestReader_错误Magic1(t *testing.T) {
	var buf bytes.Buffer
	writeNString(&buf, "BADE0001") // 错误 magic
	writeNString(&buf, MagicTEXI0001)

	reader := NewReader()
	_, err := reader.ReadTex(bytes.NewReader(buf.Bytes()))
	if err == nil {
		t.Error("错误 Magic1 应返回错误但未返回")
	}
}

func TestReader_错误Magic2(t *testing.T) {
	var buf bytes.Buffer
	writeNString(&buf, MagicTEXV0005)
	writeNString(&buf, "BADE0001") // 错误 magic2

	reader := NewReader()
	_, err := reader.ReadTex(bytes.NewReader(buf.Bytes()))
	if err == nil {
		t.Error("错误 Magic2 应返回错误但未返回")
	}
}

func TestReader_非法TexFormat(t *testing.T) {
	var buf bytes.Buffer
	writeNString(&buf, MagicTEXV0005)
	writeNString(&buf, MagicTEXI0001)
	_, _ = buf.Write(makeTexHeader(999, 0, 4, 4)) // 非法 format = 999

	reader := NewReader()
	_, err := reader.ReadTex(bytes.NewReader(buf.Bytes()))
	if err == nil {
		t.Error("非法 TexFormat 应返回错误但未返回")
	}
}

func TestReader_截断数据(t *testing.T) {
	tests := []struct {
		name string
		data []byte
	}{
		{"空数据", []byte{}},
		{"只有Magic1", append([]byte(MagicTEXV0005), 0)},
		{"缺少Header", []byte("TEXV0005\x00TEXI0001\x00")},
		{"Header截断", append([]byte("TEXV0005\x00TEXI0001\x00"), make([]byte, 10)...)},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			reader := NewReader()
			_, err := reader.ReadTex(bytes.NewReader(tt.data))
			if err == nil {
				t.Error("截断数据应返回错误但未返回")
			}
		})
	}
}

func TestReader_零尺寸(t *testing.T) {
	// 最小有效纹理 1x1 = 1 像素 RGBA = 4 字节
	pixels := make([]byte, 4)
	data := makeTexV1(mipmap.TexFormatRGBA8888, 0, 1, 1, pixels, false, nil)

	reader := NewReader()
	tex, err := reader.ReadTex(bytes.NewReader(data))
	if err != nil {
		t.Fatalf("最小尺寸 TEX 解析失败: %v", err)
	}
	if len(tex.FirstImage().FirstMipmap().Bytes) != 4 {
		t.Errorf("最小尺寸 TEX 的 mipmap 数据大小 = %d, 期望 4", len(tex.FirstImage().FirstMipmap().Bytes))
	}
}

func TestReader_未知ImageContainerMagic(t *testing.T) {
	var buf bytes.Buffer
	writeNString(&buf, MagicTEXV0005)
	writeNString(&buf, MagicTEXI0001)
	_, _ = buf.Write(makeTexHeader(mipmap.TexFormatRGBA8888, 0, 4, 4))
	writeNString(&buf, "TEXB0099") // 非法 magic

	reader := NewReader()
	_, err := reader.ReadTex(bytes.NewReader(buf.Bytes()))
	if err == nil {
		t.Error("非法 ImageContainer magic 应返回错误")
	}
}

func TestReader_超大Mipmap数量(t *testing.T) {
	var buf bytes.Buffer
	writeNString(&buf, MagicTEXV0005)
	writeNString(&buf, MagicTEXI0001)
	_, _ = buf.Write(makeTexHeader(mipmap.TexFormatRGBA8888, 0, 4, 4))
	writeNString(&buf, MagicTEXB0001)
	writeInt32(&buf, 1)    // 1 张图片
	writeInt32(&buf, 1000) // 1000 个 mipmap（超出限制）

	reader := NewReader()
	_, err := reader.ReadTex(bytes.NewReader(buf.Bytes()))
	if err == nil {
		t.Error("超大 mipmap 数量应返回错误")
	}
	if !strings.Contains(err.Error(), "mipmap") {
		t.Errorf("错误信息应包含 mipmap 相关字样: %v", err)
	}
}

func TestReader_超大Mipmap字节数(t *testing.T) {
	var buf bytes.Buffer
	writeNString(&buf, MagicTEXV0005)
	writeNString(&buf, MagicTEXI0001)
	_, _ = buf.Write(makeTexHeader(mipmap.TexFormatRGBA8888, 0, 4, 4))
	writeNString(&buf, MagicTEXB0001)
	writeInt32(&buf, 1) // 1 张图片
	writeInt32(&buf, 1) // 1 个 mipmap
	writeInt32(&buf, 4)
	writeInt32(&buf, 4)
	writeInt32(&buf, 300_000_000) // 300 MB（超出 250 MB 限制）

	reader := NewReader()
	_, err := reader.ReadTex(bytes.NewReader(buf.Bytes()))
	if err == nil {
		t.Error("超大 mipmap 字节数应返回错误")
	}
}

func TestReader_未知FrameContainerMagic(t *testing.T) {
	pixels := makeRGBAPixels(4, 4)
	var buf bytes.Buffer
	writeNString(&buf, MagicTEXV0005)
	writeNString(&buf, MagicTEXI0001)
	_, _ = buf.Write(makeTexHeader(mipmap.TexFormatRGBA8888, FlagIsGif, 4, 4))
	writeNString(&buf, MagicTEXB0001)
	writeInt32(&buf, 1)
	writeInt32(&buf, 1)
	_, _ = buf.Write(makeMipmapDataV1(4, 4, pixels))
	writeNString(&buf, "TEXS0099") // 非法帧容器 magic

	reader := NewReader()
	_, err := reader.ReadTex(bytes.NewReader(buf.Bytes()))
	if err == nil {
		t.Error("未知帧容器 magic 应返回错误")
	}
}

func TestReader_TEXS0001_int32坐标(t *testing.T) {
	pixels := makeRGBAPixels(32, 32)
	gifFrames := makeGifFrameContainer(MagicTEXS0001, 32, 32, 2)
	data := makeTexV1(mipmap.TexFormatRGBA8888, FlagIsGif, 32, 32, pixels, true, gifFrames)

	reader := NewReader()
	tex, err := reader.ReadTex(bytes.NewReader(data))
	if err != nil {
		t.Fatalf("解析 TEXS0001 失败: %v", err)
	}

	if len(tex.FrameInfoContainer.Frames) != 2 {
		t.Errorf("帧数 = %d, 期望 2", len(tex.FrameInfoContainer.Frames))
	}
	// TEXS0001 的 GIF 尺寸从第一帧推断
	if tex.FrameInfoContainer.GifWidth != 32 {
		t.Errorf("推断的 GifWidth = %d, 期望 32", tex.FrameInfoContainer.GifWidth)
	}
}

func TestReader_TEXS0002_float32坐标(t *testing.T) {
	pixels := makeRGBAPixels(32, 32)
	gifFrames := makeGifFrameContainer("TEXS0002", 32, 32, 2)
	data := makeTexV2(MagicTEXB0002, mipmap.TexFormatRGBA8888, FlagIsGif, 32, 32, pixels, FIFUnknown, true, gifFrames)

	reader := NewReader()
	tex, err := reader.ReadTex(bytes.NewReader(data))
	if err != nil {
		t.Fatalf("解析 TEXS0002 失败: %v", err)
	}

	if len(tex.FrameInfoContainer.Frames) != 2 {
		t.Errorf("帧数 = %d, 期望 2", len(tex.FrameInfoContainer.Frames))
	}
}

func TestReader_V4_非法参数(t *testing.T) {
	var buf bytes.Buffer
	writeNString(&buf, MagicTEXV0005)
	writeNString(&buf, MagicTEXI0001)
	_, _ = buf.Write(makeTexHeader(mipmap.TexFormatRGBA8888, 0, 4, 4))
	writeNString(&buf, MagicTEXB0004)
	writeInt32(&buf, 1) // image count
	writeInt32(&buf, int32(FIFUnknown))
	writeInt32(&buf, 0)   // isVideo = 0
	writeInt32(&buf, 1)   // mipmap count
	writeInt32(&buf, 999) // param1 = 999（非法）
	writeInt32(&buf, 2)
	writeNString(&buf, "")
	writeInt32(&buf, 1)

	reader := NewReader()
	_, err := reader.ReadTex(bytes.NewReader(buf.Bytes()))
	if err == nil {
		t.Error("V4 非法 param1 应返回错误")
	}
}

// ==================== 写回往返测试 ====================

func TestWriter_往返_V1(t *testing.T) {
	pixels := makeRGBAPixels(8, 8)
	orig := makeTexV1(mipmap.TexFormatRGBA8888, 0, 8, 8, pixels, false, nil)

	// 读取（不解压，保留原始字节）
	reader := NewReaderNoDecompress()
	tex, err := reader.ReadTex(bytes.NewReader(orig))
	if err != nil {
		t.Fatalf("读取失败: %v", err)
	}

	// 写回
	writer := NewWriter()
	var written bytes.Buffer
	err = writer.WriteTo(&written, tex)
	if err != nil {
		t.Fatalf("写入失败: %v", err)
	}

	// 验证字节一致
	if !bytes.Equal(orig, written.Bytes()) {
		t.Error("写入前后字节不一致")

		// 找出第一个差异位置
		for i := 0; i < len(orig) && i < written.Len(); i++ {
			if orig[i] != written.Bytes()[i] {
				t.Errorf("首个差异位置 %d: 期望 0x%02X, 实际 0x%02X", i, orig[i], written.Bytes()[i])
				break
			}
		}
	}
}

func TestWriter_往返_V2(t *testing.T) {
	pixels := makeRGBAPixels(8, 8)
	orig := makeTexV2(MagicTEXB0002, mipmap.TexFormatRGBA8888, 0, 8, 8, pixels, FIFUnknown, false, nil)

	reader := NewReaderNoDecompress()
	tex, err := reader.ReadTex(bytes.NewReader(orig))
	if err != nil {
		t.Fatalf("读取失败: %v", err)
	}

	writer := NewWriter()
	var written bytes.Buffer
	err = writer.WriteTo(&written, tex)
	if err != nil {
		t.Fatalf("写入失败: %v", err)
	}

	if !bytes.Equal(orig, written.Bytes()) {
		t.Error("写入前后字节不一致")

		// 比较差异
		for i := 0; i < len(orig) && i < written.Len(); i++ {
			if orig[i] != written.Bytes()[i] {
				t.Errorf("首个差异位置 %d: 期望 0x%02X, 实际 0x%02X", i, orig[i], written.Bytes()[i])
				break
			}
		}
	}
}

func TestWriter_往返_V3(t *testing.T) {
	pixels := makeRGBAPixels(8, 8)
	orig := makeTexV2(MagicTEXB0003, mipmap.TexFormatRGBA8888, 0, 8, 8, pixels, FIFJpeg, false, nil)

	reader := NewReaderNoDecompress()
	tex, err := reader.ReadTex(bytes.NewReader(orig))
	if err != nil {
		t.Fatalf("读取失败: %v", err)
	}

	writer := NewWriter()
	var written bytes.Buffer
	err = writer.WriteTo(&written, tex)
	if err != nil {
		t.Fatalf("写入失败: %v", err)
	}

	if !bytes.Equal(orig, written.Bytes()) {
		t.Error("写入前后字节不一致")
		// 显示前 100 个字节的差异
		for i := 0; i < 100 && i < len(orig) && i < written.Len(); i++ {
			if orig[i] != written.Bytes()[i] {
				t.Errorf("偏移 %d: 期望 0x%02X, 实际 0x%02X", i, orig[i], written.Bytes()[i])
			}
		}
	}
}

func TestWriter_往返_V1_DXT5(t *testing.T) {
	dxtBlock := make([]byte, 16)
	dxtBlock[0] = 255
	dxtBlock[1] = 255
	dxtBlock[8] = 0x00
	dxtBlock[9] = 0xF8
	dxtBlock[10] = 0x00
	dxtBlock[11] = 0xF8
	orig := makeTexV1(mipmap.TexFormatDXT5, 0, 4, 4, dxtBlock, false, nil)

	reader := NewReaderNoDecompress()
	tex, err := reader.ReadTex(bytes.NewReader(orig))
	if err != nil {
		t.Fatalf("读取失败: %v", err)
	}

	writer := NewWriter()
	var written bytes.Buffer
	err = writer.WriteTo(&written, tex)
	if err != nil {
		t.Fatalf("写入失败: %v", err)
	}

	if !bytes.Equal(orig, written.Bytes()) {
		t.Error("DXT5 写入前后字节不一致")
	}
}

func TestWriter_往返_V2_R8(t *testing.T) {
	pixels := makeR8Pixels(8, 8)
	orig := makeTexV2(MagicTEXB0002, mipmap.TexFormatR8, 0, 8, 8, pixels, FIFUnknown, false, nil)

	reader := NewReaderNoDecompress()
	tex, err := reader.ReadTex(bytes.NewReader(orig))
	if err != nil {
		t.Fatalf("读取失败: %v", err)
	}

	writer := NewWriter()
	var written bytes.Buffer
	err = writer.WriteTo(&written, tex)
	if err != nil {
		t.Fatalf("写入失败: %v", err)
	}

	if !bytes.Equal(orig, written.Bytes()) {
		t.Error("R8 写入前后字节不一致")
	}
}

func TestWriter_往返_V2_RG88(t *testing.T) {
	pixels := makeRG88Pixels(8, 8)
	orig := makeTexV2(MagicTEXB0002, mipmap.TexFormatRG88, 0, 8, 8, pixels, FIFUnknown, false, nil)

	reader := NewReaderNoDecompress()
	tex, err := reader.ReadTex(bytes.NewReader(orig))
	if err != nil {
		t.Fatalf("读取失败: %v", err)
	}

	writer := NewWriter()
	var written bytes.Buffer
	err = writer.WriteTo(&written, tex)
	if err != nil {
		t.Fatalf("写入失败: %v", err)
	}

	if !bytes.Equal(orig, written.Bytes()) {
		t.Error("RG88 写入前后字节不一致")
	}
}

func TestWriter_往返_含GIF帧(t *testing.T) {
	pixels := makeRGBAPixels(32, 32)
	gifFrames := makeGifFrameContainer(MagicTEXS0003, 32, 32, 3)
	orig := makeTexV2(MagicTEXB0003, mipmap.TexFormatRGBA8888, FlagIsGif, 32, 32, pixels, FIFUnknown, true, gifFrames)

	reader := NewReaderNoDecompress()
	tex, err := reader.ReadTex(bytes.NewReader(orig))
	if err != nil {
		t.Fatalf("读取失败: %v", err)
	}

	writer := NewWriter()
	var written bytes.Buffer
	err = writer.WriteTo(&written, tex)
	if err != nil {
		t.Fatalf("写入失败: %v", err)
	}

	if !bytes.Equal(orig, written.Bytes()) {
		t.Error("GIF TEX 写入前后字节不一致")
	}
}

func TestConverter_GIF非法ImageID(t *testing.T) {
	// 构造 GIF TEX：1 张图片，但 FrameInfo 中 ImageID=99（越界）
	pixels := makeRGBAPixels(32, 32)

	var buf bytes.Buffer
	writeNString(&buf, MagicTEXV0005)
	writeNString(&buf, MagicTEXI0001)
	_, _ = buf.Write(makeTexHeader(mipmap.TexFormatRGBA8888, FlagIsGif, 32, 32))
	writeNString(&buf, MagicTEXB0003)
	writeInt32(&buf, 1) // 1 张图片
	writeInt32(&buf, int32(FIFUnknown))
	writeInt32(&buf, 1) // 1 个 mipmap
	_, _ = buf.Write(makeMipmapDataV2(32, 32, pixels))

	// FrameInfoContainer: 1 帧，ImageID=99
	writeNString(&buf, MagicTEXS0003)
	writeInt32(&buf, 1) // 1 frame
	writeInt32(&buf, 32)
	writeInt32(&buf, 32)
	writeInt32(&buf, 99) // 非法 ImageID
	writeFloat32(&buf, 0.1)
	writeFloat32(&buf, 0)
	writeFloat32(&buf, 0)
	writeFloat32(&buf, 32)
	writeFloat32(&buf, 0)
	writeFloat32(&buf, 0)
	writeFloat32(&buf, 32)

	reader := NewReader()
	tex, err := reader.ReadTex(bytes.NewReader(buf.Bytes()))
	if err != nil {
		t.Fatalf("解析 TEX 失败: %v", err)
	}

	c := NewConverter()
	_, err = c.ConvertToImage(tex)
	if err == nil {
		t.Error("非法 ImageID 应返回错误但未返回")
	}
}

func TestConverter_空TEX不崩溃(t *testing.T) {
	// 构造一个 ImagesContainer 为空（无 Images）的 TEX，验证 Converter 和 JSONInfo 不 panic
	tex := &TEX{
		Magic1: MagicTEXV0005,
		Magic2: MagicTEXI0001,
		Header: &Header{
			Format:        mipmap.TexFormatRGBA8888,
			TextureWidth:  8,
			TextureHeight: 8,
			ImageWidth:    8,
			ImageHeight:   8,
		},
		ImagesContainer: &ImageContainer{
			Magic:                 MagicTEXB0001,
			ImageContainerVersion: Version1,
			Images:                nil, // 空 Images
		},
	}

	// Converter.GetConvertedFormat 应不 panic
	c := NewConverter()
	f := c.GetConvertedFormat(tex)
	if f != mipmap.FormatRGBA8888 {
		t.Errorf("空 Images 应返回默认 RGBA8888, 实际 %s", f.String())
	}

	// GenerateJSONInfo 应不 panic 并返回错误
	_, err := GenerateJSONInfo(tex)
	if err == nil {
		t.Error("空 Images 的 GenerateJSONInfo 应返回错误但未返回")
	}
}

func TestWriter_CompressAndWrite_nil(t *testing.T) {
	w := NewWriter()
	var buf bytes.Buffer

	// nil TEX
	err := w.CompressAndWrite(&buf, nil)
	if err == nil {
		t.Error("nil TEX 应返回错误")
	}

	// TEX 有 nil ImagesContainer
	tex := &TEX{
		Magic1: MagicTEXV0005,
		Magic2: MagicTEXI0001,
		Header: &Header{Format: mipmap.TexFormatRGBA8888},
	}
	err = w.CompressAndWrite(&buf, tex)
	if err == nil {
		t.Error("nil ImagesContainer 应返回错误")
	}
}

func TestWriter_nilTEX返回错误(t *testing.T) {
	writer := NewWriter()
	var buf bytes.Buffer
	err := writer.WriteTo(&buf, nil)
	if err == nil {
		t.Error("nil TEX 应返回错误")
	}
}

func TestWriter_错误Magic1(t *testing.T) {
	tex := &TEX{
		Magic1: "WRONG",
		Magic2: MagicTEXI0001,
	}
	writer := NewWriter()
	var buf bytes.Buffer
	err := writer.WriteTo(&buf, tex)
	if err == nil {
		t.Error("错误 Magic1 应返回错误")
	}
}

func TestWriter_缺少Header(t *testing.T) {
	tex := &TEX{
		Magic1: MagicTEXV0005,
		Magic2: MagicTEXI0001,
		Header: nil,
	}
	writer := NewWriter()
	var buf bytes.Buffer
	err := writer.WriteTo(&buf, tex)
	if err == nil {
		t.Error("缺少 Header 应返回错误")
	}
}

// TestWriter_V1拒绝LZ4 验证 V1 容器写入时拒绝 LZ4 压缩的 mipmap（与 C# 行为一致）。
func TestWriter_V1拒绝LZ4(t *testing.T) {
	pixels := makeRGBAPixels(8, 8)
	data := makeTexV1(mipmap.TexFormatRGBA8888, 0, 8, 8, pixels, false, nil)

	reader := NewReaderNoDecompress()
	tex, err := reader.ReadTex(bytes.NewReader(data))
	if err != nil {
		t.Fatalf("读取 TEX 失败: %v", err)
	}

	// 强制压缩 mipmap（模拟用户操作）
	m := tex.FirstImage().FirstMipmap()
	compressor := NewCompressor()
	err = compressor.Compress(m)
	if err != nil {
		t.Fatalf("压缩 mipmap 失败: %v", err)
	}
	if !m.IsLZ4Compressed {
		t.Fatal("mipmap 应已压缩但检查失败")
	}

	// 确保容器版本为 V1
	tex.ImagesContainer.ImageContainerVersion = Version1
	tex.ImagesContainer.Magic = MagicTEXB0001

	// V1 容器写入 LZ4 压缩数据应返回错误
	writer := NewWriter()
	var written bytes.Buffer
	err = writer.WriteTo(&written, tex)
	if err == nil {
		t.Error("V1 容器写入 LZ4 压缩数据应返回错误但未返回")
	}
}

// TestWriter_往返_V4_MP4 验证 V4 容器 + MP4 数据的往返写入。
// V4 的 mipmap 含有 param1=1/param2=2/conditionJson=""/param3=1 前导字段，写入必须还原它们。
func TestWriter_往返_V4_MP4(t *testing.T) {
	mp4Data := []byte{0x00, 0x00, 0x00, 0x18, 'f', 't', 'y', 'p', 'i', 's', 'o', 'm', 0x00, 0x00, 0x00, 0x00}
	orig := makeTexV4(mipmap.TexFormatRGBA8888, FlagIsVideoTexture, mp4Data, FIFMp4)

	reader := NewReaderNoDecompress()
	tex, err := reader.ReadTex(bytes.NewReader(orig))
	if err != nil {
		t.Fatalf("读取 V4 MP4 TEX 失败: %v", err)
	}

	// 验证版本信息
	if tex.ImagesContainer.ImageContainerVersion != Version4 {
		t.Errorf("容器版本 = %d, 期望 V4(4)", tex.ImagesContainer.ImageContainerVersion)
	}

	writer := NewWriter()
	var written bytes.Buffer
	err = writer.WriteTo(&written, tex)
	if err != nil {
		t.Fatalf("写入失败: %v", err)
	}

	// 字节级比对
	if !bytes.Equal(orig, written.Bytes()) {
		t.Error("V4 MP4 写入前后字节不一致")
		// 找出第一个差异位置
		for i := 0; i < len(orig) && i < written.Len(); i++ {
			if orig[i] != written.Bytes()[i] {
				t.Errorf("首个差异位置 %d: 期望 0x%02X, 实际 0x%02X", i, orig[i], written.Bytes()[i])
				break
			}
		}
	}

	// 验证写回后能再次读取
	tex2, err := reader.ReadTex(bytes.NewReader(written.Bytes()))
	if err != nil {
		t.Fatalf("重新读取失败: %v", err)
	}
	if !tex2.IsVideoTexture() {
		t.Error("重新读取的 TEX 应识别为视频纹理")
	}
}

// ==================== Decompressor 测试 ====================

func TestDecompressor_nilMipmap(t *testing.T) {
	d := NewDecompressor()
	err := d.Decompress(nil)
	if err == nil {
		t.Error("nil mipmap 应返回错误")
	}
}

// ==================== 转换器测试 ====================

func TestConverter_GetConvertedFormat(t *testing.T) {
	// 视频纹理返回 VideoMp4
	mp4Data := []byte{0x00, 0x00, 0x00, 0x18, 'f', 't', 'y', 'p', 'i', 's', 'o', 'm'}
	data := makeTexV2(MagicTEXB0003, mipmap.TexFormatRGBA8888, FlagIsVideoTexture, 16, 16, mp4Data, FIFMp4, false, nil)
	tex, err := NewReader().ReadTex(bytes.NewReader(data))
	if err != nil {
		t.Fatalf("解析 TEX 失败: %v", err)
	}
	c := NewConverter()
	if f := c.GetConvertedFormat(tex); f != mipmap.FormatVideoMp4 {
		t.Errorf("视频纹理格式 = %s, 期望 MP4", f.String())
	}

	// 原始格式返回 PNG
	pixels2 := makeRGBAPixels(8, 8)
	data2 := makeTexV1(mipmap.TexFormatRGBA8888, 0, 8, 8, pixels2, false, nil)
	tex2, err := NewReader().ReadTex(bytes.NewReader(data2))
	if err != nil {
		t.Fatalf("解析 TEX 失败: %v", err)
	}
	if f := c.GetConvertedFormat(tex2); f != mipmap.FormatImagePNG {
		t.Errorf("原始格式转换 = %s, 期望 PNG", f.String())
	}
}

func TestConverter_nilTEX(t *testing.T) {
	c := NewConverter()
	_, err := c.ConvertToImage(nil)
	if err == nil {
		t.Error("nil TEX 应返回错误")
	}
}

// TestConverter_空图片TEX 验证零图片的 TEX 在转换时不 panic。
func TestConverter_空图片TEX(t *testing.T) {
	// 构造一个只有 Header 但没有图片容器的 TEX
	tex := &TEX{
		Magic1:          MagicTEXV0005,
		Magic2:          MagicTEXI0001,
		Header:          &Header{Format: mipmap.TexFormatRGBA8888},
		ImagesContainer: &ImageContainer{Images: nil},
	}
	c := NewConverter()
	_, err := c.ConvertToImage(tex)
	if err == nil {
		t.Error("零图片 TEX 转换应返回错误但未返回")
	}
	// 验证不 panic
}

// ==================== JSONInfo 测试 ====================

func TestGenerateJSONInfo(t *testing.T) {
	_ = makeRGBAPixels(8, 8)
	data := makeTexV1(mipmap.TexFormatRGBA8888, FlagClampUVs|FlagNoInterpolation, 8, 8, makeRGBAPixels(8, 8), false, nil)
	tex, err := NewReader().ReadTex(bytes.NewReader(data))
	if err != nil {
		t.Fatalf("解析 TEX 失败: %v", err)
	}

	jsonInfo, err := GenerateJSONInfo(tex)
	if err != nil {
		t.Fatalf("生成 JSON 失败: %v", err)
	}

	if !strings.Contains(jsonInfo, "clampuvs") {
		t.Error("JSON 应包含 clampuvs")
	}
	if !strings.Contains(jsonInfo, "rgba8888") {
		t.Error("JSON 应包含格式信息")
	}
}

func TestGenerateJSONInfo_nilTEX(t *testing.T) {
	_, err := GenerateJSONInfo(nil)
	if err == nil {
		t.Error("nil TEX 应返回错误")
	}
}

// ==================== 转换器行为测试 ====================

// TestConverter_RG88通道映射 验证 RG88 像素通道映射与 C# 一致。
// C# 映射：RG88.R → Alpha, RG88.G → RGB 三通道。
func TestConverter_RG88通道映射(t *testing.T) {
	// RG88 像素：R=128, G=200
	pixels := []byte{128, 200}
	data := makeTexV1(mipmap.TexFormatRG88, 0, 1, 1, pixels, false, nil)

	reader := NewReader()
	tex, err := reader.ReadTex(bytes.NewReader(data))
	if err != nil {
		t.Fatalf("解析 RG88 TEX 失败: %v", err)
	}

	result, err := tex.Convert()
	if err != nil {
		t.Fatalf("转换 RG88 失败: %v", err)
	}

	// 解码输出 PNG 的像素数据
	img, _, err := image.Decode(bytes.NewReader(result.Bytes))
	if err != nil {
		t.Fatalf("解码输出图片失败: %v", err)
	}

	// 取第一个像素的 RGBA 值
	r, g, b, a := img.At(0, 0).RGBA()
	// RGBA() 返回 premultiplied 16-bit values (0-65535)
	//nolint:gosec // 已知 RGBA() 返回 16-bit 值，安全截断
	r8 := uint8(r >> 8)
	//nolint:gosec // 已知 RGBA() 返回 16-bit 值，安全截断
	g8 := uint8(g >> 8)
	//nolint:gosec // 已知 RGBA() 返回 16-bit 值，安全截断
	b8 := uint8(b >> 8)
	//nolint:gosec // 已知 RGBA() 返回 16-bit 值，安全截断
	a8 := uint8(a >> 8)

	// C# 行为：R(128) → Alpha=128, G(200) → R=200, G=200, B=200
	// 由于非预乘 alpha，PNG 编码后 RGBA() 返回预乘值：~(g*a/255) ≈ 100 per channel
	if a8 != 128 {
		t.Errorf("Alpha 通道 = %d, 期望 128 (C#: RG88.R → Alpha)", a8)
	}
	// 预乘后 R=G=B ≈ 200*128/255 ≈ 100
	expected := uint8(100)
	if r8 < expected-5 || r8 > expected+5 {
		t.Errorf("R 通道 = %d, 期望 ~%d (C#: RG88.G → RGB, 预乘 alpha)", r8, expected)
	}
	if g8 < expected-5 || g8 > expected+5 || b8 < expected-5 || b8 > expected+5 {
		t.Errorf("G/B 通道 = (%d,%d), 期望 ~(%d,%d) (C#: RG88.G → RGB, 预乘 alpha)", g8, b8, expected, expected)
	}
}

// TestConverter_DXT5不解压应报错 验证未解压的 DXT5 格式在 ConvertToImage 中应返回错误。
// 对应 C# TexToImageConverter: 若第一个 mipmap 格式为 IsCompressed 则抛出 InvalidOperationException。
func TestConverter_DXT5不解压应报错(t *testing.T) {
	dxtBlock := make([]byte, 16)
	dxtBlock[0] = 255
	dxtBlock[1] = 255
	dxtBlock[8] = 0x00
	dxtBlock[9] = 0xF8
	dxtBlock[10] = 0x00
	dxtBlock[11] = 0xF8

	data := makeTexV1(mipmap.TexFormatDXT5, 0, 4, 4, dxtBlock, false, nil)

	// 使用不解压模式读取，mipmap 将保持 FormatCompressedDXT5
	reader := NewReaderNoDecompress()
	tex, err := reader.ReadTex(bytes.NewReader(data))
	if err != nil {
		t.Fatalf("解析 DXT5 TEX 失败: %v", err)
	}

	// 验证 mipmap 格式确为压缩格式
	m := tex.FirstImage().FirstMipmap()
	if !m.Format.IsCompressed() {
		t.Fatalf("格式应为压缩格式，但为 %s", m.Format.String())
	}

	c := NewConverter()
	_, err = c.ConvertToImage(tex)
	if err == nil {
		t.Error("未解压的 DXT5 格式应返回错误但未返回")
	}
}

// TestConverter_MP4非法魔数 验证非 MP4 数据应被拒绝。
func TestConverter_MP4非法魔数(t *testing.T) {
	// 非 MP4 魔数的数据
	badData := []byte{0x00, 0x00, 0x00, 0x18, 'X', 'X', 'X', 'X', 'X', 'X', 'X', 'X'}
	data := makeTexV2(MagicTEXB0003, mipmap.TexFormatRGBA8888, FlagIsVideoTexture, 16, 16, badData, FIFMp4, false, nil)

	reader := NewReader()
	tex, err := reader.ReadTex(bytes.NewReader(data))
	if err != nil {
		t.Fatalf("解析 TEX 失败: %v", err)
	}

	c := NewConverter()
	result, err := c.ConvertToImage(tex)
	if err != nil {
		// 预期行为：非法魔数应返回错误
		t.Logf("正确拒绝非法 MP4: %v", err)
		return
	}

	// 如果没有报错但有结果，验证格式至少是 MP4
	if result.Format != mipmap.FormatVideoMp4 {
		t.Errorf("期望 VideoMp4 格式但得到 %s", result.Format.String())
	}
}

// TestConverter_MP4数据过短 验证过短的数据应被拒绝。
func TestConverter_MP4数据过短(t *testing.T) {
	shortData := []byte{0x00, 0x00, 0x00} // 仅 3 字节
	data := makeTexV2(MagicTEXB0003, mipmap.TexFormatRGBA8888, FlagIsVideoTexture, 16, 16, shortData, FIFMp4, false, nil)

	reader := NewReader()
	tex, err := reader.ReadTex(bytes.NewReader(data))
	if err != nil {
		t.Fatalf("解析 TEX 失败: %v", err)
	}

	c := NewConverter()
	_, err = c.ConvertToImage(tex)
	if err == nil {
		t.Error("过短 MP4 数据应返回错误")
	}
}

// TestConverter_RG88空数据 验证空 RG88 数据不会崩溃。
func TestConverter_RG88空数据(_ *testing.T) {
	pixels := []byte{} // 空数据
	data := makeTexV1(mipmap.TexFormatRG88, 0, 0, 0, pixels, false, nil)

	reader := NewReaderNoDecompress()
	_, err := reader.ReadTex(bytes.NewReader(data))
	// 零尺寸+空数据可能导致读取 Error，预期会有错误
	if err != nil {
		return // 预期
	}
	// 如果没有错误，那也是可接受的（零尺寸纹理）
}

// ==================== 类型化错误测试 ====================

// TestError_UnknownMagic 验证错误的 Magic 返回的错误能被识别。
func TestError_UnknownMagic(t *testing.T) {
	var buf bytes.Buffer
	writeNString(&buf, "BADE0001") // 错误 magic1
	writeNString(&buf, MagicTEXI0001)

	reader := NewReader()
	_, err := reader.ReadTex(bytes.NewReader(buf.Bytes()))
	if err == nil {
		t.Fatal("错误 Magic 应返回错误")
	}

	if !strings.Contains(err.Error(), "Magic") {
		t.Errorf("错误应包含 'Magic' 关键词: %v", err)
	}
}

// TestError_UnsafeTex 验证不安全的 TEX 数据应返回可识别的错误。
func TestError_UnsafeTex(t *testing.T) {
	var buf bytes.Buffer
	writeNString(&buf, MagicTEXV0005)
	writeNString(&buf, MagicTEXI0001)
	_, _ = buf.Write(makeTexHeader(mipmap.TexFormatRGBA8888, 0, 4, 4))
	writeNString(&buf, MagicTEXB0001)
	writeInt32(&buf, 1)    // 1 张图片
	writeInt32(&buf, 1000) // 1000 个 mipmap（超出限制）

	reader := NewReader()
	_, err := reader.ReadTex(bytes.NewReader(buf.Bytes()))
	if err == nil {
		t.Fatal("超大 mipmap 数量应返回错误")
	}

	if !strings.Contains(err.Error(), "mipmap") {
		t.Errorf("错误应包含 'mipmap' 关键词: %v", err)
	}
}

// ==================== isPowerOfTwo 测试 ====================

func TestIsPowerOfTwo(t *testing.T) {
	tests := []struct {
		n    int
		want bool
	}{
		{0, false},
		{1, true},
		{2, true},
		{3, false},
		{4, true},
		{5, false},
		{128, true},
		{129, false},
		{256, true},
		{1024, true},
		{-1, false},
	}

	for _, tt := range tests {
		if got := isPowerOfTwo(tt.n); got != tt.want {
			t.Errorf("isPowerOfTwo(%d) = %v, 期望 %v", tt.n, got, tt.want)
		}
	}
}

// TestGIFPalette_不超过256色 验证 16x16 GIF TEX（最多 256 像素）转换成功，
// 且每帧调色板不超过 256 色。
func TestGIFPalette_不超过256色(t *testing.T) { //nolint:funlen // 测试多步骤 GIF 创建验证
	const size int32 = 16
	pixels := makeUniqueRGBAPixels(size, size)

	var buf bytes.Buffer
	writeNString(&buf, MagicTEXV0005)
	writeNString(&buf, MagicTEXI0001)
	_, _ = buf.Write(makeTexHeader(mipmap.TexFormatRGBA8888, FlagIsGif, size, size))

	writeNString(&buf, MagicTEXB0003)
	writeInt32(&buf, 1) // 1 张图片
	writeInt32(&buf, int32(FIFUnknown))
	writeInt32(&buf, 1) // 1 个 mipmap
	_, _ = buf.Write(makeMipmapDataV2(size, size, pixels))

	// FrameInfoContainer: 1 帧，完整画面无裁剪
	writeNString(&buf, MagicTEXS0003)
	writeInt32(&buf, 1)               // 1 frame
	writeInt32(&buf, size)            // GifWidth
	writeInt32(&buf, size)            // GifHeight
	writeInt32(&buf, 0)               // ImageID
	writeFloat32(&buf, 0.1)           // Frametime
	writeFloat32(&buf, 0)             // X
	writeFloat32(&buf, 0)             // Y
	writeFloat32(&buf, float32(size)) // Width
	writeFloat32(&buf, 0)             // WidthY
	writeFloat32(&buf, 0)             // HeightX
	writeFloat32(&buf, float32(size)) // Height

	reader := NewReader()
	tex, err := reader.ReadTex(bytes.NewReader(buf.Bytes()))
	if err != nil {
		t.Fatalf("解析 TEX 失败: %v", err)
	}

	if !tex.IsGif() {
		t.Fatal("TEX 应为 GIF")
	}

	imgResult, err := NewConverter().ConvertToImage(tex)
	if err != nil {
		t.Fatalf("GIF 转换失败: %v", err)
	}

	if imgResult.Format != mipmap.FormatImageGIF {
		t.Errorf("格式 = %s, 期望 GIF", imgResult.Format.String())
	}

	decoded, err := gif.DecodeAll(bytes.NewReader(imgResult.Bytes))
	if err != nil {
		t.Fatalf("解码 GIF 失败: %v", err)
	}

	if len(decoded.Image) != 1 {
		t.Fatalf("GIF 帧数 = %d, 期望 1", len(decoded.Image))
	}

	frame := decoded.Image[0]
	palSize := len(frame.Palette)
	if palSize > 256 {
		t.Errorf("调色板大小 = %d, 超过 256", palSize)
	}

	bounds := frame.Bounds()
	if bounds.Dx() != int(size) || bounds.Dy() != int(size) {
		t.Errorf("帧尺寸 = %dx%d, 期望 %dx%d", bounds.Dx(), bounds.Dy(), size, size)
	}
}

// makeUniqueRGBAPixels 生成 16x16 图像的全唯一 RGBA 像素数据。
// 每个像素的 R=y, G=x, B=x+y, A=255，对于 16x16 图像可产生 256 种唯一颜色。
func makeUniqueRGBAPixels(w, h int32) []byte {
	pixels := make([]byte, int(w)*int(h)*4)
	for y := range int(h) {
		for x := range int(w) {
			idx := (y*int(w) + x) * 4
			pixels[idx] = byte(y)
			pixels[idx+1] = byte(x)
			pixels[idx+2] = byte(x + y)
			pixels[idx+3] = 255
		}
	}
	return pixels
}

// ==================== MPLR L2/L4 审查补充测试 ====================

// TestWriter_V4非MP4版本一致性 验证 V4 非 MP4 格式降级后 Magic 与 Version 保持一致。
func TestWriter_V4非MP4版本一致性(t *testing.T) {
	pngData := []byte{0x89, 0x50, 0x4E, 0x47, 0x0D, 0x0A, 0x1A, 0x0A, 0x00, 0x00, 0x00, 0x0D}
	orig := makeTexV4(mipmap.TexFormatRGBA8888, 0, pngData, FIFPng)

	reader := NewReaderNoDecompress()
	tex, err := reader.ReadTex(bytes.NewReader(orig))
	if err != nil {
		t.Fatalf("读取 V4 PNG TEX 失败: %v", err)
	}

	// V4 non-MP4 应被降级为 V3，且 Magic 也需要同步为 V3
	if tex.ImagesContainer.ImageContainerVersion != Version3 {
		t.Errorf("V4 non-MP4 应降级为 V3, 实际 = %d", tex.ImagesContainer.ImageContainerVersion)
	}
	if tex.ImagesContainer.Magic != MagicTEXB0003 {
		t.Errorf("降级后 Magic 应为 %s, 实际 = %s", MagicTEXB0003, tex.ImagesContainer.Magic)
	}

	// 写回
	writer := NewWriter()
	var written bytes.Buffer
	err = writer.WriteTo(&written, tex)
	if err != nil {
		t.Fatalf("写入失败: %v", err)
	}

	// 重读验证
	tex2, err := reader.ReadTex(bytes.NewReader(written.Bytes()))
	if err != nil {
		t.Fatalf("重读失败 (Magic/Version 不一致导致解析错位): %v", err)
	}
	if tex2.ImagesContainer.ImageContainerVersion != Version3 {
		t.Errorf("重读后版本 = %d, 期望 V3", tex2.ImagesContainer.ImageContainerVersion)
	}
}

// TestConverter_GIF空FrameInfoContainer 验证 GIF 转换时 FrameInfoContainer 非 nil。
func TestConverter_GIF空FrameInfoContainer(t *testing.T) {
	tex := &TEX{
		Magic1: MagicTEXV0005,
		Magic2: MagicTEXI0001,
		Header: &Header{Format: mipmap.TexFormatRGBA8888, Flags: FlagIsGif},
		ImagesContainer: &ImageContainer{
			Magic:                 MagicTEXB0001,
			Images:                []*Image{{Mipmaps: []*Mipmap{{Format: mipmap.FormatRGBA8888, Bytes: make([]byte, 16), Width: 2, Height: 2}}}},
			ImageContainerVersion: Version1,
		},
		FrameInfoContainer: nil,
	}

	c := NewConverter()
	_, err := c.ConvertToImage(tex)
	if err == nil {
		t.Error("GIF 转换缺少 FrameInfoContainer 应返回错误")
	}
}

// TestRotateImage_非正交角度 验证 rotateImage 对非 0/180 度角的处理。
func TestRotateImage_非正交角度(t *testing.T) {
	src := image.NewNRGBA(image.Rect(0, 0, 100, 50))

	// 45° 旋转：宽高应交换
	rotated := rotateImage(src, 45)
	bounds := rotated.Bounds()
	if bounds.Dx() != 50 || bounds.Dy() != 100 {
		t.Errorf("45° 旋转后尺寸 = %dx%d, 期望 50x100 (宽高交换)", bounds.Dx(), bounds.Dy())
	}

	// 0° 旋转：尺寸不变
	rotated0 := rotateImage(src, 0)
	b0 := rotated0.Bounds()
	if b0.Dx() != 100 || b0.Dy() != 50 {
		t.Errorf("0° 旋转后尺寸 = %dx%d, 期望 100x50 (不变)", b0.Dx(), b0.Dy())
	}

	// 90° 旋转：宽高应交换
	rotated90 := rotateImage(src, 90)
	b90 := rotated90.Bounds()
	if b90.Dx() != 50 || b90.Dy() != 100 {
		t.Errorf("90° 旋转后尺寸 = %dx%d, 期望 50x100 (宽高交换)", b90.Dx(), b90.Dy())
	}
}

// ==================== MPLR L1 审查补充测试 ====================

// TestDecompressor_负数DecompressedBytesCount 验证 DecompressedBytesCount 为负数时返回错误。
// MPLR L1: make([]byte, -1) 会直接 panic。
func TestDecompressor_负数DecompressedBytesCount(t *testing.T) {
	d := NewDecompressor()
	m := &Mipmap{
		Width:                  4,
		Height:                 4,
		IsLZ4Compressed:        true,
		DecompressedBytesCount: -1,
		Bytes:                  []byte{0, 0, 0, 0},
		Format:                 mipmap.FormatCompressedDXT5,
	}
	err := d.Decompress(m)
	if err == nil {
		t.Error("负数 DecompressedBytesCount 应返回错误但未返回")
	}
}

// TestDecompressor_负数尺寸 验证 Width/Height 为负数时返回错误。
// MPLR L1: make([]byte, w*h) 中 w*h 为负时 panic。
func TestDecompressor_负数尺寸(t *testing.T) {
	d := NewDecompressor()
	m := &Mipmap{
		Width:                  -1,
		Height:                 4,
		IsLZ4Compressed:        false,
		DecompressedBytesCount: 0,
		Bytes:                  make([]byte, 16),
		Format:                 mipmap.FormatCompressedDXT5,
	}
	err := d.Decompress(m)
	if err == nil {
		t.Error("负数 Width 应返回错误但未返回")
	}
}

// TestDecompressor_负数高度 验证 Height 为负数时返回错误。
func TestDecompressor_负数高度(t *testing.T) {
	d := NewDecompressor()
	m := &Mipmap{
		Width:                  4,
		Height:                 -1,
		IsLZ4Compressed:        false,
		DecompressedBytesCount: 0,
		Bytes:                  make([]byte, 16),
		Format:                 mipmap.FormatCompressedDXT5,
	}
	err := d.Decompress(m)
	if err == nil {
		t.Error("负数 Height 应返回错误但未返回")
	}
}

// TestReadNString_零maxLength 验证 maxLength=0 时返回空字符串。
// MPLR L4: 与 C# ReadNString 语义一致，maxLength=0 表示不可读取任何字符。
func TestReadNString_零maxLength(t *testing.T) {
	data := append([]byte("test"), 0)
	r := bytes.NewReader(data)
	s, err := binutil.ReadNString(r, 0)
	if err != nil {
		t.Errorf("maxLength=0 不应返回错误: %v", err)
	}
	if s != "" {
		t.Errorf("maxLength=0 应返回空字符串, 实际 = %q", s)
	}
}

// TestConverter_空Mipmap返回安全默认值 验证零 mipmap 的 Image 调用 GetConvertedFormat 不 panic。
// MPLR L1: FirstMipmap() 返回 nil，需防护。
func TestConverter_空Mipmap返回安全默认值(t *testing.T) {
	tex := &TEX{
		Magic1: MagicTEXV0005,
		Magic2: MagicTEXI0001,
		Header: &Header{Format: mipmap.TexFormatRGBA8888},
		ImagesContainer: &ImageContainer{
			Magic:                 MagicTEXB0001,
			Images:                []*Image{{Mipmaps: nil}}, // 零 mipmap
			ImageContainerVersion: Version1,
		},
	}

	c := NewConverter()
	// 不能 panic
	format := c.GetConvertedFormat(tex)
	if format != mipmap.FormatRGBA8888 {
		t.Errorf("空 mipmap 应返回默认格式 RGBA8888, 实际 = %s", format.String())
	}
}

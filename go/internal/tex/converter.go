// Package tex 提供 Wallpaper Engine TEX 纹理文件的读写、压缩解压和格式转换功能。
//
// 本文件提供 TEX 纹理到标准图片格式（PNG/GIF/MP4）的转换功能。
package tex

import (
	"bytes"
	"errors"
	"fmt"
	"image"
	"image/color"
	"image/gif"
	"image/png"
	"math"

	"github.com/1m188/repkg-go/pkg/mipmap"
)

// ImageResult 表示图片转换结果。
// 对应 C# ImageResult。
type ImageResult struct {
	Bytes  []byte        // 图片字节数据
	Format mipmap.Format // 图片格式
}

// Converter 负责将 TEX 纹理转换为标准图片格式。
// 对应 C# TexToImageConverter。
type Converter struct{}

// NewConverter 创建默认的转换器。
func NewConverter() *Converter {
	return &Converter{}
}

// GetConvertedFormat 获取转换后的输出格式。
func (c *Converter) GetConvertedFormat(tex *TEX) mipmap.Format {
	_ = c
	if tex == nil {
		return mipmap.FormatRGBA8888
	}
	if tex.IsVideoTexture() {
		return mipmap.FormatVideoMp4
	}
	img := tex.FirstImage()
	if img == nil {
		return mipmap.FormatRGBA8888
	}
	m := img.FirstMipmap()
	if m == nil {
		return mipmap.FormatRGBA8888
	}
	format := m.Format
	if format.IsRawFormat() {
		return mipmap.FormatImagePNG
	}
	return format
}

// ConvertToImage 将 TEX 转换为图片。
func (c *Converter) ConvertToImage(tex *TEX) (*ImageResult, error) {
	if tex == nil {
		return nil, errors.New("TEX 不能为 nil")
	}

	// GIF 动画
	if tex.IsGif() {
		return c.convertToGif(tex)
	}

	img := tex.FirstImage()
	if img == nil {
		return nil, errors.New("TEX 没有图片数据")
	}
	firstMipmap := img.FirstMipmap()
	if firstMipmap == nil {
		return nil, errors.New("TEX 图片没有 mipmap 数据")
	}

	// 视频纹理：验证 MP4 魔数
	if tex.IsVideoTexture() {
		err := validateMP4Magic(firstMipmap.Bytes)
		if err != nil {
			return nil, err
		}
		return &ImageResult{
			Bytes:  firstMipmap.Bytes,
			Format: mipmap.FormatVideoMp4,
		}, nil
	}

	format := firstMipmap.Format

	// 压缩格式（DXT1/3/5）必须先解压才能转换
	if format.IsCompressed() {
		return nil, errors.New("无法将压缩 mipmap 格式转换为图片，请先解压")
	}

	// JPEG/PNG 等已编码图片：直接返回
	if format.IsImage() {
		return &ImageResult{
			Bytes:  firstMipmap.Bytes,
			Format: format,
		}, nil
	}

	if !format.IsRawFormat() {
		return &ImageResult{
			Bytes:  firstMipmap.Bytes,
			Format: format,
		}, nil
	}

	result, err := convertRawToPNG(format, tex, firstMipmap)
	if err != nil {
		return nil, err
	}
	return result, nil
}

// convertRawToPNG 将原始像素格式的 mipmap 转换为 PNG 图片结果。
func convertRawToPNG(format mipmap.Format, tex *TEX, m *Mipmap) (*ImageResult, error) {
	img, err := imageFromRawFormat(format, m.Bytes, int(m.Width), int(m.Height))
	if err != nil {
		return nil, fmt.Errorf("创建图片失败: %w", err)
	}

	if m.Width != tex.Header.ImageWidth || m.Height != tex.Header.ImageHeight {
		if cropped, ok := img.(interface {
			SubImage(r image.Rectangle) image.Image
		}); ok {
			img = cropped.SubImage(image.Rect(0, 0, int(tex.Header.ImageWidth), int(tex.Header.ImageHeight)))
		}
	}

	var buf bytes.Buffer
	// 原始像素格式 → PNG
	err = png.Encode(&buf, img)
	if err != nil {
		return nil, fmt.Errorf("PNG 编码失败: %w", err)
	}
	return &ImageResult{
		Bytes:  buf.Bytes(),
		Format: mipmap.FormatImagePNG,
	}, nil
}

// convertToGif 将动画 TEX 转换为 GIF。
func (c *Converter) convertToGif(tex *TEX) (*ImageResult, error) { //nolint:funlen // GIF 转换逻辑步骤多，适度复杂度是必要的
	_ = c
	img := tex.FirstImage()
	if img == nil {
		return nil, errors.New("TEX 没有图片数据")
	}
	m := img.FirstMipmap()
	if m == nil {
		return nil, errors.New("TEX 图片没有 mipmap 数据")
	}
	frameFormat := m.Format
	if !frameFormat.IsRawFormat() {
		return nil, errors.New("GIF 转换仅支持原始像素格式")
	}

	// 验证帧信息容器
	if tex.FrameInfoContainer == nil {
		return nil, errors.New("GIF 纹理缺少帧信息容器")
	}

	// 创建基础 GIF 图片
	outGIF := &gif.GIF{
		Config: image.Config{
			Width:  int(tex.FrameInfoContainer.GifWidth),
			Height: int(tex.FrameInfoContainer.GifHeight),
		},
	}

	// 为每个帧创建子图片
	sequenceImages := make([]image.Image, 0, len(tex.ImagesContainer.Images))
	for _, img := range tex.ImagesContainer.Images {
		m := img.FirstMipmap()
		subImg, err := imageFromRawFormat(frameFormat, m.Bytes, int(m.Width), int(m.Height))
		if err != nil {
			return nil, fmt.Errorf("创建帧图片失败: %w", err)
		}
		sequenceImages = append(sequenceImages, subImg)
	}

	// 根据帧信息裁剪、旋转并加入 GIF
	for _, frameInfo := range tex.FrameInfoContainer.Frames {
		width := frameInfo.Width
		if width == 0 {
			width = frameInfo.HeightX
		}
		height := frameInfo.Height
		if height == 0 {
			height = frameInfo.WidthY
		}

		x := float64(min32(frameInfo.X, frameInfo.X+width))
		y := float64(min32(frameInfo.Y, frameInfo.Y+height))

		// 计算旋转角度
		rotationAngle := -(math.Atan2(float64(sign32(height)), float64(sign32(width))) - math.Pi/4)
		rotationAngle = math.Round(rotationAngle * 180 / math.Pi)

		if int(frameInfo.ImageID) >= len(sequenceImages) {
			return nil, fmt.Errorf("帧 ImageID %d 超出图片数量 %d", frameInfo.ImageID, len(sequenceImages))
		}
		src := sequenceImages[frameInfo.ImageID]

		// 裁剪
		cropRect := image.Rect(int(x), int(y), int(x+math.Abs(float64(width))), int(y+math.Abs(float64(height))))
		cropped := cropImage(src, cropRect)

		// 如果需要旋转
		if rotationAngle != 0 {
			cropped = rotateImage(cropped, rotationAngle)
		}

		// 转为 Paletted（GIF 需要调色板）
		paletted := imageToPaletted(cropped)

		delay := int(math.Round(float64(frameInfo.Frametime) * 100.0))
		outGIF.Image = append(outGIF.Image, paletted)
		outGIF.Delay = append(outGIF.Delay, delay)
	}

	var buf bytes.Buffer
	err := gif.EncodeAll(&buf, outGIF)
	if err != nil {
		return nil, fmt.Errorf("GIF 编码失败: %w", err)
	}
	return &ImageResult{
		Bytes:  buf.Bytes(),
		Format: mipmap.FormatImageGIF,
	}, nil
}

// imageFromRawFormat 从原始像素字节创建 image.Image。
//
//nolint:ireturn // 函数返回 image.Image 接口是工厂函数的合理设计
func imageFromRawFormat(format mipmap.Format, pixels []byte, width, height int) (image.Image, error) {
	switch format {
	case mipmap.FormatR8:
		return imageFromR8(pixels, width, height), nil
	case mipmap.FormatRG88:
		return imageFromRG88(pixels, width, height), nil
	case mipmap.FormatRGBA8888:
		return imageFromRGBA8888(pixels, width, height), nil
	default:
		return nil, fmt.Errorf("不支持的原始像素格式: %s", format.String())
	}
}

// imageFromR8 从 R8（灰度）像素字节创建 *image.Gray。
func imageFromR8(pixels []byte, width, height int) *image.Gray {
	img := image.NewGray(image.Rect(0, 0, width, height))
	for y := range height {
		for x := range width {
			idx := y*width + x
			if idx < len(pixels) {
				img.SetGray(x, y, color.Gray{Y: pixels[idx]})
			}
		}
	}
	return img
}

// imageFromRG88 从 RG88（双通道）像素字节创建 *image.NRGBA。
// C# 映射：RG88.R → Alpha, RG88.G → R,G,B。
func imageFromRG88(pixels []byte, width, height int) *image.NRGBA {
	img := image.NewNRGBA(image.Rect(0, 0, width, height))
	for y := range height {
		for x := range width {
			idx := (y*width + x) * 2
			if idx+1 < len(pixels) {
				r := pixels[idx]
				g := pixels[idx+1]
				img.SetNRGBA(x, y, color.NRGBA{R: g, G: g, B: g, A: r})
			}
		}
	}
	return img
}

// imageFromRGBA8888 从 RGBA8888 像素字节创建 *image.NRGBA。
func imageFromRGBA8888(pixels []byte, width, height int) *image.NRGBA {
	img := image.NewNRGBA(image.Rect(0, 0, width, height))
	for y := range height {
		for x := range width {
			idx := (y*width + x) * 4
			if idx+3 < len(pixels) {
				img.SetNRGBA(x, y, color.NRGBA{
					R: pixels[idx],
					G: pixels[idx+1],
					B: pixels[idx+2],
					A: pixels[idx+3],
				})
			}
		}
	}
	return img
}

// 辅助函数
func min32(a, b float32) float32 {
	if a < b {
		return a
	}
	return b
}

func sign32(v float32) float32 {
	if v < 0 {
		return -1
	}
	if v > 0 {
		return 1
	}
	return 0
}

func cropImage(img image.Image, rect image.Rectangle) *image.NRGBA {
	switch src := img.(type) {
	case *image.NRGBA:
		if nrgba, ok := src.SubImage(rect).(*image.NRGBA); ok {
			return nrgba
		}
		return image.NewNRGBA(rect)
	case *image.Gray:
		sub := src.SubImage(rect)
		bounds := sub.Bounds()
		nrgba := image.NewNRGBA(bounds)
		for y := bounds.Min.Y; y < bounds.Max.Y; y++ {
			for x := bounds.Min.X; x < bounds.Max.X; x++ {
				nrgba.Set(x, y, sub.At(x, y))
			}
		}
		return nrgba
	default:
		// 转为 NRGBA 再裁剪
		bounds := img.Bounds()
		newImg := image.NewNRGBA(bounds)
		for y := bounds.Min.Y; y < bounds.Max.Y; y++ {
			for x := bounds.Min.X; x < bounds.Max.X; x++ {
				newImg.Set(x, y, img.At(x, y))
			}
		}
		if nrgba, ok := newImg.SubImage(rect).(*image.NRGBA); ok {
			return nrgba
		}
		return image.NewNRGBA(rect)
	}
}

func rotateImage(img image.Image, angleDeg float64) *image.NRGBA {
	// 简单实现：只支持 0, 90, 180, 270 度旋转
	bounds := img.Bounds()
	w := bounds.Dx()
	h := bounds.Dy()

	var newW, newH int
	switch {
	case math.Abs(math.Mod(angleDeg, 180)) < 0.01:
		newW, newH = w, h
	default:
		newW, newH = h, w
	}

	rotated := image.NewNRGBA(image.Rect(0, 0, newW, newH))
	angle := angleDeg * math.Pi / 180.0

	for y := range newH {
		for x := range newW {
			// 反向旋转坐标
			cx := float64(x) - float64(newW)/2.0
			cy := float64(y) - float64(newH)/2.0

			srcX := cx*math.Cos(-angle) - cy*math.Sin(-angle) + float64(w)/2.0
			srcY := cx*math.Sin(-angle) + cy*math.Cos(-angle) + float64(h)/2.0

			if srcX >= 0 && srcX < float64(w) && srcY >= 0 && srcY < float64(h) {
				rotated.Set(x, y, img.At(int(srcX)+bounds.Min.X, int(srcY)+bounds.Min.Y))
			}
		}
	}
	return rotated
}

func imageToPaletted(img image.Image) *image.Paletted {
	bounds := img.Bounds()
	palette := generatePalette(img)
	paletted := image.NewPaletted(bounds, palette)
	for y := bounds.Min.Y; y < bounds.Max.Y; y++ {
		for x := bounds.Min.X; x < bounds.Max.X; x++ {
			paletted.Set(x, y, img.At(x, y))
		}
	}
	return paletted
}

// generatePalette 为 GIF 帧生成调色板。
// 遍历图像像素，收集遇到的唯一颜色，最多收集 256 种。
//
// 已知限制：当图像唯一颜色超过 256 种时，仅保留扫描顺序中前 256 种颜色
// （从左到右、从上到下），后续颜色会被 image.Paletted.Set 近似到调色板中最近的颜色。
// 这会导致图像底部和右侧的颜色偏差（top-left bias）。
// 后续可改用 median-cut 等量化算法以提升 >256 色图像的调色板质量。
// 对应 C# SixLabors.ImageSharp 的 GifColorTableMode.Local。
func generatePalette(img image.Image) color.Palette {
	colorMap := make(map[color.Color]bool)
	bounds := img.Bounds()
	for y := bounds.Min.Y; y < bounds.Max.Y; y++ {
		for x := bounds.Min.X; x < bounds.Max.X; x++ {
			colorMap[img.At(x, y)] = true
			if len(colorMap) >= 256 {
				break
			}
		}
		if len(colorMap) >= 256 {
			break
		}
	}

	palette := make(color.Palette, 0, len(colorMap))
	for c := range colorMap {
		palette = append(palette, c)
	}
	return palette
}

// validateMP4Magic 验证 MP4 文件的魔数字节。
// 检查 ftyp 容器类型是否为 isom、msnv 或 mp42。
func validateMP4Magic(data []byte) error {
	if len(data) < 12 {
		return &UnsafeTexError{Reason: fmt.Sprintf("MP4 数据过短: %d 字节 (最少需要 12)", len(data))}
	}
	magic := string(data[4:12])
	switch magic {
	case "ftypisom", "ftypmsnv", "ftypmp42":
		return nil
	default:
		return &UnknownMagicError{Source: SourceTexConverter, Property: "MP4Magic", Magic: magic}
	}
}

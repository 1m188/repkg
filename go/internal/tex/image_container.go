package tex

import (
	"fmt"
	"strconv"

	"github.com/1m188/repkg-go/internal/binutil"
	"github.com/1m188/repkg-go/pkg/mipmap"
)

// ImageContainer 表示 TEX 文件中的图片容器。
// 对应 C# TexImageContainer。
type ImageContainer struct {
	Magic                 string                // MagicTEXB0001 ~ MagicTEXB0004
	ImageFormat           FreeImageFormat       // FreeImage 格式（V3+）
	Images                []*Image              // 图片列表
	ImageContainerVersion ImageContainerVersion // 容器版本
}

// readImageContainer 从 reader 中读取 ImageContainer。
// 对应 C# TexImageContainerReader。
func readImageContainer(r ioRuneReader, texFormat mipmap.TexFormat, decompressor *Decompressor, readMipmapBytes, decompressMipmapBytes bool) (*ImageContainer, error) {
	magic, err := binutil.ReadNString(r, 16)
	if err != nil {
		return nil, fmt.Errorf("读取图片容器 magic 失败: %w", err)
	}

	imageCount, err := binutil.ReadInt32(r)
	if err != nil {
		return nil, fmt.Errorf("读取图片数量失败: %w", err)
	}
	if imageCount > maxImageCount {
		return nil, &UnsafeTexError{Reason: fmt.Sprintf("图片数量超出限制: %d / %d", imageCount, maxImageCount)}
	}

	container, err := parseImageContainerHeader(r, magic)
	if err != nil {
		return nil, err
	}

	format := mipmap.FormatForTex(int32(container.ImageFormat), texFormat)
	for i := range imageCount {
		img, err := readImage(r, container, format, decompressor, readMipmapBytes, decompressMipmapBytes)
		if err != nil {
			return nil, fmt.Errorf("读取图片 %d 失败: %w", i, err)
		}
		container.Images = append(container.Images, img)
	}

	return container, nil
}

// parseImageContainerHeader 解析图片容器头部（magic 后的格式字段和版本）。
func parseImageContainerHeader(r ioRuneReader, magic string) (*ImageContainer, error) {
	container := &ImageContainer{Magic: magic}

	switch magic {
	case MagicTEXB0001, MagicTEXB0002:
		// 无额外格式字段，默认为 FIF_UNKNOWN
		container.ImageFormat = FIFUnknown
	case MagicTEXB0003:
		fif, err := binutil.ReadInt32(r)
		if err != nil {
			return nil, fmt.Errorf("读取 ImageFormat 失败: %w", err)
		}
		container.ImageFormat = FreeImageFormat(fif)
	case MagicTEXB0004:
		fif, err := binutil.ReadInt32(r)
		if err != nil {
			return nil, fmt.Errorf("读取 ImageFormat 失败: %w", err)
		}
		isVideoMp4, err := binutil.ReadInt32(r)
		if err != nil {
			return nil, fmt.Errorf("读取视频标志失败: %w", err)
		}
		format := FreeImageFormat(fif)
		if format == FIFUnknown && isVideoMp4 == 1 {
			format = FIFMp4
		}
		container.ImageFormat = format
	default:
		return nil, &UnknownMagicError{Source: SourceTexReader, Property: "ImageContainerMagic", Magic: magic}
	}

	versionStr := magic[4:]
	version, err := strconv.Atoi(versionStr)
	if err != nil {
		return nil, fmt.Errorf("解析容器版本失败: %s", versionStr)
	}
	container.ImageContainerVersion = ImageContainerVersion(version) //nolint:gosec // version 来自二进制流读取，范围已验证

	if container.ImageContainerVersion == Version4 && container.ImageFormat != FIFMp4 {
		container.ImageContainerVersion = Version3
		container.Magic = MagicTEXB0003 // 同步更新 Magic 字符串，避免写入 V4 魔数 + V3 格式数据
	}

	if !isValidFreeImageFormat(container.ImageFormat) {
		return nil, &EnumNotValidError{Value: int32(container.ImageFormat), Source: "FreeImageFormat"}
	}

	return container, nil
}

// readImage 读取单张图片及其 mipmap 链。
func readImage(r ioRuneReader, container *ImageContainer, format mipmap.Format, decompressor *Decompressor, readMipmapBytes, decompressMipmapBytes bool) (*Image, error) { //nolint:revive // 与 C# 接口保持一致的控制标志
	mipmapCount, err := binutil.ReadInt32(r)
	if err != nil {
		return nil, fmt.Errorf("读取 mipmap 数量失败: %w", err)
	}
	if mipmapCount > maxMipmapCount {
		return nil, &UnsafeTexError{Reason: fmt.Sprintf("mipmap 数量超出限制: %d / %d", mipmapCount, maxMipmapCount)}
	}

	// 根据容器版本选择 mipmap 读取函数
	var readFn func(r ioRuneReader) (*Mipmap, error)
	skipBytes := !readMipmapBytes
	switch container.ImageContainerVersion {
	case Version1:
		readFn = func(r ioRuneReader) (*Mipmap, error) {
			return readMipmapV1(r, skipBytes)
		}
	case Version2, Version3:
		readFn = func(r ioRuneReader) (*Mipmap, error) {
			return readMipmapV2And3(r, skipBytes)
		}
	case Version4:
		readFn = func(r ioRuneReader) (*Mipmap, error) {
			return readMipmapV4(r, skipBytes)
		}
	default:
		return nil, &EnumNotValidError{Value: int32(container.ImageContainerVersion), Source: "ImageContainerVersion"}
	}

	img := &Image{}
	for i := range mipmapCount {
		m, err := readFn(r)
		if err != nil {
			return nil, fmt.Errorf("读取 mipmap %d 失败: %w", i, err)
		}
		m.Format = format

		// 解压（如果启用）
		if decompressMipmapBytes && !readMipmapBytes {
			continue
		}
		if decompressMipmapBytes && decompressor != nil {
			err = decompressor.Decompress(m)
			if err != nil {
				return nil, fmt.Errorf("解压 mipmap %d 失败: %w", i, err)
			}
		}
		img.Mipmaps = append(img.Mipmaps, m)
	}

	return img, nil
}

// ==================== ImageContainer 写入 ====================

// writeImageContainer 将 ImageContainer 写入 writer。
func (c *ImageContainer) write(w byteWriter) error {
	err := binutil.WriteNString(w, c.Magic)
	if err != nil {
		return fmt.Errorf("写入图片容器magic失败: %w", err)
	}
	err = binutil.WriteInt32(w, int32(len(c.Images))) //nolint:gosec // 图片数量受限于上游数据
	if err != nil {
		return fmt.Errorf("写入图片数量失败: %w", err)
	}

	// V3 写入 ImageFormat，V4 写入 ImageFormat + 视频标志
	if c.ImageContainerVersion == Version3 || c.ImageContainerVersion == Version4 {
		err = binutil.WriteInt32(w, int32(c.ImageFormat))
		if err != nil {
			return fmt.Errorf("写入ImageFormat失败: %w", err)
		}
	}
	if c.ImageContainerVersion == Version4 {
		isVideo := int32(0)
		if c.ImageFormat == FIFMp4 {
			isVideo = 1
		}
		err = binutil.WriteInt32(w, isVideo)
		if err != nil {
			return fmt.Errorf("写入视频标志失败: %w", err)
		}
	}

	for _, img := range c.Images {
		err = writeImage(w, img, c.ImageContainerVersion)
		if err != nil {
			return fmt.Errorf("写入图片失败: %w", err)
		}
	}
	return nil
}

// writeImage 写入一张图片及其 mipmap 链。
func writeImage(w byteWriter, img *Image, version ImageContainerVersion) error {
	err := binutil.WriteInt32(w, int32(len(img.Mipmaps))) //nolint:gosec // mipmap 数量受限于上游数据
	if err != nil {
		return fmt.Errorf("写入mipmap数量失败: %w", err)
	}

	for _, m := range img.Mipmaps {
		switch version {
		case Version1:
			err = m.writeV1(w)
		case Version4:
			// V4 每个 mipmap 前写入固定的四字段前导
			err = binutil.WriteInt32(w, 1) // param1: 始终为 1
			if err != nil {
				return fmt.Errorf("写入 V4 param1 失败: %w", err)
			}
			err = binutil.WriteInt32(w, 2) // param2: 始终为 2
			if err != nil {
				return fmt.Errorf("写入 V4 param2 失败: %w", err)
			}
			err = binutil.WriteNString(w, "") // conditionJson: 空（C# 未解析）
			if err != nil {
				return fmt.Errorf("写入 V4 conditionJson 失败: %w", err)
			}
			err = binutil.WriteInt32(w, 1) // param3: 始终为 1
			if err != nil {
				return fmt.Errorf("写入 V4 param3 失败: %w", err)
			}
			err = m.writeV2(w) // 前导后为标准 V2 格式 mipmap 数据
		default:
			err = m.writeV2(w)
		}
		if err != nil {
			return fmt.Errorf("写入mipmap失败: %w", err)
		}
	}
	return nil
}

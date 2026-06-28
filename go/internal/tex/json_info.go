// Package tex 提供 Wallpaper Engine TEX 纹理文件的读写、压缩解压和格式转换功能。
//
// 本文件提供 .tex-json 元数据生成功能。
package tex

import (
	"encoding/json"
	"errors"
	"fmt"
	"strconv"
	"strings"

	"github.com/1m188/repkg-go/pkg/mipmap"
)

// JSONInfo 表示 .tex-json 文件的元数据。
type JSONInfo struct {
	BleedTransparentColors bool                  `json:"bleedtransparentcolors"`         // BleedTransparentColors 是否渗透透明色。
	ClampUVs               bool                  `json:"clampuvs"`                       // ClampUVs 是否钳制 UV 坐标。
	Format                 string                `json:"format"`                         // Format 纹理格式名称。
	NoMip                  string                `json:"nomip"`                          // NoMip 是否禁用 mipmap。
	NoInterpolation        string                `json:"nointerpolation"`                // NoInterpolation 是否禁用插值。
	NonPowerOfTwo          string                `json:"nonpoweroftwo"`                  // NonPowerOfTwo 是否为非 2 的幂尺寸。
	SpritesheetSequences   []SpritesheetSequence `json:"spritesheetsequences,omitempty"` // SpritesheetSequences 精灵表序列信息。
}

// SpritesheetSequence 表示 GIF 动画的序列信息。
type SpritesheetSequence struct {
	Duration int `json:"duration"` // Duration 持续时间。
	Frames   int `json:"frames"`   // Frames 帧数。
	Width    int `json:"width"`    // Width 宽度。
	Height   int `json:"height"`   // Height 高度。
}

// GenerateJSONInfo 从 TEX 对象生成 .tex-json 元数据。
// 对应 C# TexJsonInfoGenerator。
func GenerateJSONInfo(tex *TEX) (string, error) {
	if tex == nil {
		return "", errors.New("TEX 不能为 nil")
	}

	firstImg := tex.FirstImage()
	if firstImg == nil {
		return "", errors.New("TEX 没有图片数据")
	}
	info := JSONInfo{
		BleedTransparentColors: true,
		ClampUVs:               tex.HasFlag(FlagClampUVs),
		Format:                 strings.ToLower(texFormatString(tex.Header.Format)),
		NoMip:                  strconv.FormatBool(len(firstImg.Mipmaps) == 1),
		NoInterpolation:        strconv.FormatBool(tex.HasFlag(FlagNoInterpolation)),
		NonPowerOfTwo: strconv.FormatBool(
			!isPowerOfTwo(int(tex.Header.ImageWidth)) ||
				!isPowerOfTwo(int(tex.Header.ImageHeight))),
	}

	if tex.IsGif() {
		if tex.FrameInfoContainer == nil {
			return "", errors.New("TEX 是动画但没有帧信息容器")
		}
		info.SpritesheetSequences = []SpritesheetSequence{{
			Duration: 1,
			Frames:   len(tex.FrameInfoContainer.Frames),
			Width:    int(tex.FrameInfoContainer.GifWidth),
			Height:   int(tex.FrameInfoContainer.GifHeight),
		}}
	}

	result, err := json.MarshalIndent(info, "", "    ")
	if err != nil {
		return "", fmt.Errorf("JSON 序列化失败: %w", err)
	}
	return string(result), nil
}

// texFormatString 返回 TexFormat 的可读名称。
func texFormatString(f mipmap.TexFormat) string {
	switch f {
	case mipmap.TexFormatRGBA8888:
		return "RGBA8888"
	case mipmap.TexFormatDXT5:
		return "DXT5"
	case mipmap.TexFormatDXT3:
		return "DXT3"
	case mipmap.TexFormatDXT1:
		return "DXT1"
	case mipmap.TexFormatRG88:
		return "RG88"
	case mipmap.TexFormatR8:
		return "R8"
	default:
		return "未知"
	}
}

// isPowerOfTwo 判断整数是否为 2 的幂。
func isPowerOfTwo(n int) bool {
	if n <= 0 {
		return false
	}
	return (n & (n - 1)) == 0
}

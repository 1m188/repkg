// Package tex 提供 Wallpaper Engine TEX 纹理文件的读写、压缩解压和格式转换功能。
//
// 本文件定义 GIF 动画帧信息的数据模型和读写逻辑。
package tex

import (
	"fmt"

	"github.com/1m188/repkg-go/internal/binutil"
)

// 最大帧数限制
const maxFrameCount = 100_000

// FrameInfo 表示 GIF 动画中的一帧信息。
// 对应 C# TexFrameInfo。
type FrameInfo struct {
	ImageID   int32   // ImageID 图片索引。
	Frametime float32 // Frametime 帧持续时间（秒）。
	X         float32 // X 帧在画布上的 X 坐标。
	Y         float32 // Y 帧在画布上的 Y 坐标。
	Width     float32 // Width 帧宽度。
	WidthY    float32 // WidthY 宽度 Y 分量（用于旋转计算）。
	HeightX   float32 // HeightX 高度 X 分量（用于旋转计算）。
	Height    float32 // Height 帧高度。
}

// FrameInfoContainer 表示 TEX 动画帧信息容器。
// 对应 C# TexFrameInfoContainer。
type FrameInfoContainer struct {
	Magic     string       // MagicTEXS0001 ~ MagicTEXS0003
	Frames    []*FrameInfo // 帧列表
	GifWidth  int32        // GIF 宽度
	GifHeight int32        // GIF 高度
}

// readFrameInfoContainer 从 reader 中读取 FrameInfoContainer。
func readFrameInfoContainer(r ioRuneReader) (*FrameInfoContainer, error) {
	magic, err := binutil.ReadNString(r, 16)
	if err != nil {
		return nil, fmt.Errorf("读取帧容器 magic 失败: %w", err)
	}

	frameCount, err := binutil.ReadInt32(r)
	if err != nil {
		return nil, fmt.Errorf("读取帧数量失败: %w", err)
	}
	if frameCount < 0 {
		return nil, &UnsafeTexError{Reason: fmt.Sprintf("帧数量无效: %d", frameCount)}
	}
	if frameCount > maxFrameCount {
		return nil, &UnsafeTexError{Reason: fmt.Sprintf("帧数量超出限制: %d / %d", frameCount, maxFrameCount)}
	}

	container := &FrameInfoContainer{Magic: magic}

	err = readGifSize(r, magic, container)
	if err != nil {
		return nil, err
	}

	isFloatCoord := magic != MagicTEXS0001
	for i := range frameCount {
		frame, err := readFrameCoord(r, isFloatCoord)
		if err != nil {
			return nil, fmt.Errorf("读取帧 %d 坐标失败: %w", i, err)
		}
		container.Frames = append(container.Frames, frame)
	}

	if container.GifWidth == 0 || container.GifHeight == 0 {
		if len(container.Frames) > 0 {
			container.GifWidth = int32(container.Frames[0].Width)
			container.GifHeight = int32(container.Frames[0].Height)
		}
	}

	return container, nil
}

// readGifSize 读取 GIF 尺寸字段（仅 TEXS0003 有）。
func readGifSize(r ioRuneReader, magic string, container *FrameInfoContainer) error {
	if magic != MagicTEXS0003 {
		return nil
	}
	gifW, err := binutil.ReadInt32(r)
	if err != nil {
		return fmt.Errorf("读取 GIF 宽度失败: %w", err)
	}
	gifH, err := binutil.ReadInt32(r)
	if err != nil {
		return fmt.Errorf("读取 GIF 高度失败: %w", err)
	}
	container.GifWidth = gifW
	container.GifHeight = gifH
	return nil
}

// readFrameCoord 读取单帧的坐标数据。
func readFrameCoord(r ioRuneReader, isFloat bool) (*FrameInfo, error) { //nolint:revive // 函数名保持与 C# 原版一致
	frame := &FrameInfo{}

	imageID, err := binutil.ReadInt32(r)
	if err != nil {
		return nil, fmt.Errorf("读取帧 ImageId 失败: %w", err)
	}
	frame.ImageID = imageID

	frametime, err := binutil.ReadFloat32(r)
	if err != nil {
		return nil, fmt.Errorf("读取帧 Frametime 失败: %w", err)
	}
	frame.Frametime = frametime

	if isFloat {
		return readFloat32Coord(r, frame)
	}
	return readInt32Coord(r, frame)
}

// readFloat32Coord 读取 float32 格式的帧坐标。
func readFloat32Coord(r ioRuneReader, frame *FrameInfo) (*FrameInfo, error) {
	x, err := binutil.ReadFloat32(r)
	if err != nil {
		return nil, fmt.Errorf("读取帧X坐标失败: %w", err)
	}
	frame.X = x
	y, err := binutil.ReadFloat32(r)
	if err != nil {
		return nil, fmt.Errorf("读取帧Y坐标失败: %w", err)
	}
	frame.Y = y
	w, err := binutil.ReadFloat32(r)
	if err != nil {
		return nil, fmt.Errorf("读取帧宽度失败: %w", err)
	}
	frame.Width = w
	wy, err := binutil.ReadFloat32(r)
	if err != nil {
		return nil, fmt.Errorf("读取帧WidthY失败: %w", err)
	}
	frame.WidthY = wy
	hx, err := binutil.ReadFloat32(r)
	if err != nil {
		return nil, fmt.Errorf("读取帧HeightX失败: %w", err)
	}
	frame.HeightX = hx
	h, err := binutil.ReadFloat32(r)
	if err != nil {
		return nil, fmt.Errorf("读取帧高度失败: %w", err)
	}
	frame.Height = h
	return frame, nil
}

// readInt32Coord 读取 int32 格式的帧坐标并转为 float32。
func readInt32Coord(r ioRuneReader, frame *FrameInfo) (*FrameInfo, error) {
	x, err := binutil.ReadInt32(r)
	if err != nil {
		return nil, fmt.Errorf("读取帧X坐标失败: %w", err)
	}
	frame.X = float32(x)
	y, err := binutil.ReadInt32(r)
	if err != nil {
		return nil, fmt.Errorf("读取帧Y坐标失败: %w", err)
	}
	frame.Y = float32(y)
	w, err := binutil.ReadInt32(r)
	if err != nil {
		return nil, fmt.Errorf("读取帧宽度失败: %w", err)
	}
	frame.Width = float32(w)
	wy, err := binutil.ReadInt32(r)
	if err != nil {
		return nil, fmt.Errorf("读取帧WidthY失败: %w", err)
	}
	frame.WidthY = float32(wy)
	hx, err := binutil.ReadInt32(r)
	if err != nil {
		return nil, fmt.Errorf("读取帧HeightX失败: %w", err)
	}
	frame.HeightX = float32(hx)
	h, err := binutil.ReadInt32(r)
	if err != nil {
		return nil, fmt.Errorf("读取帧高度失败: %w", err)
	}
	frame.Height = float32(h)
	return frame, nil
}

// ==================== FrameInfoContainer 写入 ====================

// writeFrameInfoContainer 将 FrameInfoContainer 写入 writer。
func (c *FrameInfoContainer) write(w byteWriter) error {
	err := binutil.WriteNString(w, c.Magic)
	if err != nil {
		return fmt.Errorf("写入帧容器 magic 失败: %w", err)
	}
	err = binutil.WriteInt32(w, int32(len(c.Frames))) //nolint:gosec // 帧数量受限于数据文件本身
	if err != nil {
		return fmt.Errorf("写入帧数量失败: %w", err)
	}

	if c.Magic == MagicTEXS0003 {
		err = binutil.WriteInt32(w, c.GifWidth)
		if err != nil {
			return fmt.Errorf("写入GIF宽度失败: %w", err)
		}
		err = binutil.WriteInt32(w, c.GifHeight)
		if err != nil {
			return fmt.Errorf("写入GIF高度失败: %w", err)
		}
	}

	isFloatCoord := c.Magic != MagicTEXS0001
	for _, f := range c.Frames {
		err = writeFrameCoord(w, f, isFloatCoord)
		if err != nil {
			return fmt.Errorf("写入帧坐标失败: %w", err)
		}
	}
	return nil
}

// writeFrameCoord 写入单帧的坐标数据。
func writeFrameCoord(w byteWriter, f *FrameInfo, isFloat bool) error { //nolint:revive // 控制标志 isFloat 是二进制格式的内在要求
	err := binutil.WriteInt32(w, f.ImageID)
	if err != nil {
		return fmt.Errorf("写入帧ImageID失败: %w", err)
	}
	err = binutil.WriteFloat32(w, f.Frametime)
	if err != nil {
		return fmt.Errorf("写入帧Frametime失败: %w", err)
	}

	if isFloat {
		return writeFloat32Coord(w, f)
	}
	return writeInt32Coord(w, f)
}

// writeFloat32Coord 写入 float32 格式的帧坐标。
func writeFloat32Coord(w byteWriter, f *FrameInfo) error {
	err := binutil.WriteFloat32(w, f.X)
	if err != nil {
		return fmt.Errorf("写入帧X坐标失败: %w", err)
	}
	err = binutil.WriteFloat32(w, f.Y)
	if err != nil {
		return fmt.Errorf("写入帧Y坐标失败: %w", err)
	}
	err = binutil.WriteFloat32(w, f.Width)
	if err != nil {
		return fmt.Errorf("写入帧宽度失败: %w", err)
	}
	err = binutil.WriteFloat32(w, f.WidthY)
	if err != nil {
		return fmt.Errorf("写入帧WidthY失败: %w", err)
	}
	err = binutil.WriteFloat32(w, f.HeightX)
	if err != nil {
		return fmt.Errorf("写入帧HeightX失败: %w", err)
	}
	err = binutil.WriteFloat32(w, f.Height)
	if err != nil {
		return fmt.Errorf("写入帧高度失败: %w", err)
	}
	return nil
}

// writeInt32Coord 写入 int32 格式的帧坐标。
func writeInt32Coord(w byteWriter, f *FrameInfo) error {
	err := binutil.WriteInt32(w, int32(f.X))
	if err != nil {
		return fmt.Errorf("写入帧X坐标失败: %w", err)
	}
	err = binutil.WriteInt32(w, int32(f.Y))
	if err != nil {
		return fmt.Errorf("写入帧Y坐标失败: %w", err)
	}
	err = binutil.WriteInt32(w, int32(f.Width))
	if err != nil {
		return fmt.Errorf("写入帧宽度失败: %w", err)
	}
	err = binutil.WriteInt32(w, int32(f.WidthY))
	if err != nil {
		return fmt.Errorf("写入帧WidthY失败: %w", err)
	}
	err = binutil.WriteInt32(w, int32(f.HeightX))
	if err != nil {
		return fmt.Errorf("写入帧HeightX失败: %w", err)
	}
	err = binutil.WriteInt32(w, int32(f.Height))
	if err != nil {
		return fmt.Errorf("写入帧高度失败: %w", err)
	}
	return nil
}

package tex

import (
	"errors"
	"fmt"
	"io"
	"strconv"

	"github.com/1m188/repkg-go/internal/binutil"
	"github.com/1m188/repkg-go/pkg/mipmap"
)

// 安全限制常量
const maxMipmapCount = 32
const maxMipmapByteCount = 250_000_000 // 250 MB
const maxImageCount = 100

// Mipmap 表示纹理的一个 mipmap 级别。
// 对应 C# TexMipmap。
type Mipmap struct {
	Bytes                  []byte        // 像素数据（可能压缩）
	Width                  int32         // 宽度
	Height                 int32         // 高度
	DecompressedBytesCount int32         // LZ4 解压后的预期字节数
	IsLZ4Compressed        bool          // 是否 LZ4 压缩
	Format                 mipmap.Format // 像素格式
}

// Image 表示 TEX 中的一张图片（含 mipmap 链）。
// 对应 C# TexImage。
type Image struct {
	Mipmaps []*Mipmap // Mipmaps mipmap 级别链。
}

// FirstMipmap 返回第一个 mipmap 级别，如果没有 mipmap 则返回 nil。
func (img *Image) FirstMipmap() *Mipmap {
	if len(img.Mipmaps) == 0 {
		return nil
	}
	return img.Mipmaps[0]
}

// ==================== Mipmap 读取 ====================

// readMipmapV1 读取版本 1 的 mipmap（无 LZ4 压缩字段）。
// skipBytes 为 true 时跳过字节数据（用于 ReadMipmapBytes=false 模式）。
func readMipmapV1(r byteReader, skipBytes bool) (*Mipmap, error) {
	w, err := binutil.ReadInt32(r)
	if err != nil {
		return nil, fmt.Errorf("读取mipmap宽度失败: %w", err)
	}
	h, err := binutil.ReadInt32(r)
	if err != nil {
		return nil, fmt.Errorf("读取mipmap高度失败: %w", err)
	}
	bytes, err := readMipmapBytes(r, skipBytes)
	if err != nil {
		return nil, err
	}
	return &Mipmap{Width: w, Height: h, Bytes: bytes}, nil
}

// readMipmapV2And3 读取版本 2/3 的 mipmap（含 LZ4 压缩字段）。
// skipBytes 为 true 时跳过字节数据（用于 ReadMipmapBytes=false 模式）。
func readMipmapV2And3(r byteReader, skipBytes bool) (*Mipmap, error) {
	w, err := binutil.ReadInt32(r)
	if err != nil {
		return nil, fmt.Errorf("读取mipmap宽度失败: %w", err)
	}
	h, err := binutil.ReadInt32(r)
	if err != nil {
		return nil, fmt.Errorf("读取mipmap高度失败: %w", err)
	}
	isLZ4, err := binutil.ReadInt32(r)
	if err != nil {
		return nil, fmt.Errorf("读取LZ4标志失败: %w", err)
	}
	decompCount, err := binutil.ReadInt32(r)
	if err != nil {
		return nil, fmt.Errorf("读取解压后字节数失败: %w", err)
	}
	bytes, err := readMipmapBytes(r, skipBytes)
	if err != nil {
		return nil, err
	}
	return &Mipmap{
		Width:                  w,
		Height:                 h,
		IsLZ4Compressed:        isLZ4 == 1,
		DecompressedBytesCount: decompCount,
		Bytes:                  bytes,
	}, nil
}

// readMipmapV4 读取版本 4 的 mipmap（含 conditionJson）。
func readMipmapV4(r ioRuneReader, skipBytes bool) (*Mipmap, error) {
	param1, err := binutil.ReadInt32(r)
	if err != nil {
		return nil, fmt.Errorf("读取param1失败: %w", err)
	}
	if param1 != 1 {
		return nil, &UnknownMagicError{Source: SourceReadMipmapV4, Property: "param1", Magic: strconv.Itoa(int(param1))}
	}
	param2, err := binutil.ReadInt32(r)
	if err != nil {
		return nil, fmt.Errorf("读取param2失败: %w", err)
	}
	if param2 != 2 {
		return nil, &UnknownMagicError{Source: SourceReadMipmapV4, Property: "param2", Magic: strconv.Itoa(int(param2))}
	}

	// 读取 conditionJson（null 结尾字符串）
	_, err = binutil.ReadNString(r, -1)
	if err != nil {
		return nil, fmt.Errorf("读取 conditionJson 失败: %w", err)
	}

	param3, err := binutil.ReadInt32(r)
	if err != nil {
		return nil, fmt.Errorf("读取param3失败: %w", err)
	}
	if param3 != 1 {
		return nil, &UnknownMagicError{Source: SourceReadMipmapV4, Property: "param3", Magic: strconv.Itoa(int(param3))}
	}

	return readMipmapV2And3(r, skipBytes)
}

// readMipmapBytes 读取 mipmap 的字节数据（int32 长度前缀 + 数据体）。
// 当 skipBytes 为 true 时跳过字节而不返回数据（此情况返回 nil）。
func readMipmapBytes(r byteReader, skipBytes bool) ([]byte, error) { //nolint:revive // skipBytes 是性能优化标志，由调用方决定
	byteCount, err := binutil.ReadInt32(r)
	if err != nil {
		return nil, fmt.Errorf("读取 mipmap 字节数失败: %w", err)
	}

	if byteCount < 0 {
		return nil, &UnsafeTexError{Reason: fmt.Sprintf("mipmap 字节数无效: %d", byteCount)}
	}
	if byteCount > maxMipmapByteCount {
		return nil, &UnsafeTexError{Reason: fmt.Sprintf("mipmap 字节数超出限制: %d / %d", byteCount, maxMipmapByteCount)}
	}

	if skipBytes {
		// 跳过字节但不分配内存
		skipBuf := make([]byte, 4096)
		remaining := int(byteCount)
		for remaining > 0 {
			chunk := skipBuf
			if remaining < len(chunk) {
				chunk = skipBuf[:remaining]
			}
			n, readErr := r.Read(chunk)
			if readErr != nil {
				return nil, fmt.Errorf("跳过 mipmap 数据失败: %w", readErr)
			}
			if n == 0 {
				return nil, &UnsafeTexError{Reason: "reader 返回 0 字节但无错误，无法继续跳过 mipmap 数据"}
			}
			remaining -= n
		}
		return nil, nil
	}

	buf := make([]byte, byteCount)
	_, err = io.ReadFull(r, buf)
	if err != nil {
		return nil, fmt.Errorf("读取 mipmap 数据失败: %w", err)
	}
	return buf, nil
}

// ==================== Mipmap 写入 ====================

// writeMipmapV1 写入版本 1 的 mipmap。V1 容器不支持 LZ4 压缩数据。
func (m *Mipmap) writeV1(w byteWriter) error {
	if m.IsLZ4Compressed {
		return errors.New("V1 容器不支持 LZ4 压缩的 mipmap")
	}
	err := binutil.WriteInt32(w, m.Width)
	if err != nil {
		return fmt.Errorf("写入mipmap宽度失败: %w", err)
	}
	err = binutil.WriteInt32(w, m.Height)
	if err != nil {
		return fmt.Errorf("写入mipmap高度失败: %w", err)
	}
	return writeMipmapBytes(w, m.Bytes)
}

// writeMipmapV2 写入版本 2+ 的 mipmap（含 LZ4 字段）。
func (m *Mipmap) writeV2(w byteWriter) error {
	err := binutil.WriteInt32(w, m.Width)
	if err != nil {
		return fmt.Errorf("写入mipmap宽度失败: %w", err)
	}
	err = binutil.WriteInt32(w, m.Height)
	if err != nil {
		return fmt.Errorf("写入mipmap高度失败: %w", err)
	}

	lz4Flag := int32(0)
	if m.IsLZ4Compressed {
		lz4Flag = 1
	}
	err = binutil.WriteInt32(w, lz4Flag)
	if err != nil {
		return fmt.Errorf("写入LZ4标志失败: %w", err)
	}
	err = binutil.WriteInt32(w, m.DecompressedBytesCount)
	if err != nil {
		return fmt.Errorf("写入解压后字节数失败: %w", err)
	}
	return writeMipmapBytes(w, m.Bytes)
}

// writeMipmapBytes 写入 mipmap 的字节数据。
func writeMipmapBytes(w byteWriter, bytes []byte) error {
	err := binutil.WriteInt32(w, int32(len(bytes))) //nolint:gosec // bytes 长度受限于上游数据大小
	if err != nil {
		return fmt.Errorf("写入mipmap字节数失败: %w", err)
	}
	_, err = w.Write(bytes)
	if err != nil {
		return fmt.Errorf("写入mipmap数据失败: %w", err)
	}
	return nil
}

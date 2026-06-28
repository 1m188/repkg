// Package binutil 提供二进制流的扩展读写方法。
// 对应 C# RePKG.Application.Extensions.cs 中的 BinaryReader/BinaryWriter 扩展方法。
package binutil

import (
	"encoding/binary"
	"fmt"
	"io"
	"math"
)

// ReadNString 从 reader 中读取一个 null 结尾的字符串。
// maxLength 为最大字符数，-1 表示无限制。
// 对应 C# 的 ReadNString。
func ReadNString(r io.Reader, maxLength int) (string, error) {
	if maxLength == 0 {
		return "", nil // C# 行为：maxLength=0 立即返回空字符串
	}
	buf := make([]byte, 0, 64)
	b := make([]byte, 1)

	for {
		_, err := io.ReadFull(r, b)
		if err != nil {
			if err == io.EOF && len(buf) > 0 {
				return string(buf), io.ErrUnexpectedEOF
			}
			return "", fmt.Errorf("读取 null 结尾字符串失败: %w", err)
		}
		if b[0] == 0 {
			break
		}
		buf = append(buf, b[0])
		if maxLength > 0 && len(buf) >= maxLength {
			break
		}
	}
	return string(buf), nil
}

// ReadStringI32Size 从 reader 中读取一个 int32 长度前缀的字符串。
// maxLength 为最大字符数，-1 表示无限制。
// 对应 C# 的 ReadStringI32Size。
func ReadStringI32Size(r io.Reader, maxLength int) (string, error) {
	var size int32
	err := binary.Read(r, binary.LittleEndian, &size)
	if err != nil {
		return "", fmt.Errorf("读取字符串长度失败: %w", err)
	}
	if size < 0 {
		return "", fmt.Errorf("字符串长度不能为负数: %d", size)
	}
	if maxLength > 0 && maxLength > math.MaxInt32 {
		maxLength = math.MaxInt32
	}
	origSize := size
	if maxLength > 0 && size > int32(maxLength) {
		size = int32(maxLength)
	}

	buf := make([]byte, size)
	_, err = io.ReadFull(r, buf)
	if err != nil {
		return "", fmt.Errorf("读取字符串内容失败: %w", err)
	}
	if origSize > size {
		_, err = io.CopyN(io.Discard, r, int64(origSize-size))
		if err != nil {
			return string(buf), fmt.Errorf("丢弃超长字符串尾部字节失败: %w", err)
		}
	}
	return string(buf), nil
}

// WriteNString 向 writer 中写入一个 null 结尾的字符串。
// 对应 C# 的 WriteNString。
func WriteNString(w io.Writer, s string) error {
	_, err := w.Write([]byte(s))
	if err != nil {
		return fmt.Errorf("写入字符串失败: %w", err)
	}
	_, err = w.Write([]byte{0})
	if err != nil {
		return fmt.Errorf("写入 null 终止符失败: %w", err)
	}
	return nil
}

// WriteStringI32Size 向 writer 中写入一个 int32 长度前缀的字符串。
// 对应 C# 的 WriteStringI32Size。
func WriteStringI32Size(w io.Writer, s string) error {
	if len(s) > math.MaxInt32 {
		return fmt.Errorf("字符串过长: %d 字节", len(s))
	}
	length := int32(len(s)) // #nosec G115 -- len(s) 已由上游 MaxInt32 检查限制
	err := binary.Write(w, binary.LittleEndian, length)
	if err != nil {
		return fmt.Errorf("写入字符串长度失败: %w", err)
	}
	_, err = w.Write([]byte(s))
	if err != nil {
		return fmt.Errorf("写入字符串内容失败: %w", err)
	}
	return nil
}

// ReadInt32 从 reader 中读取一个小端序 int32。
func ReadInt32(r io.Reader) (int32, error) {
	var v int32
	err := binary.Read(r, binary.LittleEndian, &v)
	if err != nil {
		return 0, fmt.Errorf("读取int32失败: %w", err)
	}
	return v, nil
}

// ReadUInt32 从 reader 中读取一个小端序 uint32。
func ReadUInt32(r io.Reader) (uint32, error) {
	var v uint32
	err := binary.Read(r, binary.LittleEndian, &v)
	if err != nil {
		return 0, fmt.Errorf("读取uint32失败: %w", err)
	}
	return v, nil
}

// ReadFloat32 从 reader 中读取一个小端序 float32。
func ReadFloat32(r io.Reader) (float32, error) {
	var v float32
	err := binary.Read(r, binary.LittleEndian, &v)
	if err != nil {
		return 0, fmt.Errorf("读取float32失败: %w", err)
	}
	return v, nil
}

// WriteInt32 向 writer 中写入一个小端序 int32。
func WriteInt32(w io.Writer, v int32) error {
	err := binary.Write(w, binary.LittleEndian, v)
	if err != nil {
		return fmt.Errorf("写入int32失败: %w", err)
	}
	return nil
}

// WriteUInt32 向 writer 中写入一个小端序 uint32。
func WriteUInt32(w io.Writer, v uint32) error {
	err := binary.Write(w, binary.LittleEndian, v)
	if err != nil {
		return fmt.Errorf("写入uint32失败: %w", err)
	}
	return nil
}

// WriteFloat32 向 writer 中写入一个小端序 float32。
func WriteFloat32(w io.Writer, v float32) error {
	err := binary.Write(w, binary.LittleEndian, v)
	if err != nil {
		return fmt.Errorf("写入float32失败: %w", err)
	}
	return nil
}

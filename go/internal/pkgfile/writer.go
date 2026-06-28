package pkgfile

import (
	"errors"
	"fmt"
	"io"
	"math"
	"strings"

	"github.com/1m188/repkg-go/internal/binutil"
)

// Writer 用于写入 PKG 文件。
// 对应 C# PackageWriter。
type Writer struct{}

// NewWriter 创建默认的 PKG 写入器。
func NewWriter() *Writer {
	return &Writer{}
}

// WriteTo 将 Package 写入 writer。
func (w *Writer) WriteTo(writer io.Writer, pkg *Package) error {
	_ = w
	if pkg == nil {
		return errors.New("package 不能为 nil")
	}
	if len(pkg.Entries) == 0 {
		return errors.New("package 条目列表为空")
	}

	// 写入 magic
	err := binutil.WriteStringI32Size(writer, pkg.Magic)
	if err != nil {
		return fmt.Errorf("写入 PKG magic 失败: %w", err)
	}

	// 写入条目数量和条目头部信息
	err = writeEntriesHeader(writer, pkg.Entries)
	if err != nil {
		return err
	}

	// 写入数据体
	return writeBody(writer, pkg.Entries)
}

// writeEntriesHeader 写入条目头部信息。
func writeEntriesHeader(writer io.Writer, entries []*Entry) error {
	// 写入条目数量
	if len(entries) > math.MaxInt32 {
		return fmt.Errorf("条目数量过多: %d", len(entries))
	}
	err := binutil.WriteInt32(writer, int32(len(entries))) //nolint:gosec // 条目数量已在上方检查 math.MaxInt32 上限
	if err != nil {
		return fmt.Errorf("写入条目数量失败: %w", err)
	}

	currentOffset := int32(0)
	for _, entry := range entries {
		if entry == nil {
			return errors.New("条目不能为 nil")
		}
		if strings.TrimSpace(entry.FullPath) == "" {
			return errors.New("条目的 FullPath 不能为空或全空白")
		}
		if entry.Bytes == nil {
			return fmt.Errorf("条目 %s 的 Bytes 不能为 nil", entry.FullPath)
		}

		// 写入完整路径
		err = binutil.WriteStringI32Size(writer, entry.FullPath)
		if err != nil {
			return fmt.Errorf("写入条目路径失败 (%s): %w", entry.FullPath, err)
		}

		// 写入偏移
		err = binutil.WriteInt32(writer, currentOffset)
		if err != nil {
			return fmt.Errorf("写入条目偏移失败 (%s): %w", entry.FullPath, err)
		}

		// 写入长度
		if len(entry.Bytes) > math.MaxInt32 {
			return fmt.Errorf("条目 %s 数据过大", entry.FullPath)
		}
		length := int32(len(entry.Bytes)) //nolint:gosec // 数据大小已在上方检查 math.MaxInt32 上限
		err = binutil.WriteInt32(writer, length)
		if err != nil {
			return fmt.Errorf("写入条目长度失败 (%s): %w", entry.FullPath, err)
		}

		// 更新条目的 Offset 和 Length
		entry.Offset = currentOffset
		entry.Length = length
		currentOffset += length
	}

	return nil
}

// writeBody 写入数据体（所有条目的字节数据）。
func writeBody(writer io.Writer, entries []*Entry) error {
	for _, entry := range entries {
		_, err := writer.Write(entry.Bytes)
		if err != nil {
			return fmt.Errorf("写入条目数据失败 (%s): %w", entry.FullPath, err)
		}
	}
	return nil
}

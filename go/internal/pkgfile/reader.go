package pkgfile

import (
	"encoding/binary"
	"fmt"
	"io"

	"github.com/1m188/repkg-go/internal/binutil"
)

// maxMagicLength PKG magic 最大长度。
const maxMagicLength = 32

// maxFilePathLength 条目路径最大长度。
const maxFilePathLength = 255

// maxEntryCount PKG 条目数量上限（防止恶意文件导致 OOM）。
const maxEntryCount = 100_000

// Reader 用于读取 PKG 文件。
// 对应 C# PackageReader。
type Reader struct {
	// ReadEntryBytes 控制是否读取条目的字节数据。默认 true。
	ReadEntryBytes bool
}

// NewReader 创建默认的 PKG 读取器。
func NewReader() *Reader {
	return &Reader{ReadEntryBytes: true}
}

// ReadPackage 从 reader 中读取并解析 PKG 数据。
//
//nolint:funlen // PKG 头部解析需要多步骤
func (r *Reader) ReadPackage(reader io.Reader) (*Package, error) { //nolint:gocyclo,cyclop // PKG 二进制解析需要多步骤处理
	pkg := &Package{}

	// 读取 magic 字符串
	magic, err := binutil.ReadStringI32Size(reader, maxMagicLength)
	if err != nil {
		return nil, fmt.Errorf("读取 PKG magic 失败: %w", err)
	}
	pkg.Magic = magic

	// 读取条目数量
	var entryCount int32
	err = binary.Read(reader, binary.LittleEndian, &entryCount)
	if err != nil {
		return nil, fmt.Errorf("读取条目数量失败: %w", err)
	}
	if entryCount < 0 {
		return nil, fmt.Errorf("条目数量无效: %d", entryCount)
	}
	if entryCount > maxEntryCount {
		return nil, fmt.Errorf("条目数量超出限制: %d / %d", entryCount, maxEntryCount)
	}

	// 读取条目列表（头部信息）
	entries := make([]*Entry, entryCount)
	for i := range entryCount {
		// 读取完整路径
		fullPath, err := binutil.ReadStringI32Size(reader, maxFilePathLength)
		if err != nil {
			return nil, fmt.Errorf("读取条目路径失败 (索引 %d): %w", i, err)
		}

		// 读取偏移
		var offset int32
		err = binary.Read(reader, binary.LittleEndian, &offset)
		if err != nil {
			return nil, fmt.Errorf("读取条目偏移失败 (索引 %d): %w", i, err)
		}

		// 读取长度
		var length int32
		err = binary.Read(reader, binary.LittleEndian, &length)
		if err != nil {
			return nil, fmt.Errorf("读取条目长度失败 (索引 %d): %w", i, err)
		}

		entry := &Entry{
			FullPath: fullPath,
			Offset:   offset,
			Length:   length,
			Type:     EntryTypeFromFileName(fullPath),
		}
		entries[i] = entry
	}

	pkg.Entries = entries

	// 计算头部大小（magic、entryCount、各条目元数据的总字节数）
	headerSize := int32(4 + len(pkg.Magic) + 4) //nolint:gosec // 头部大小受限于入口数据量
	for _, entry := range entries {
		headerSize += 4 + int32(len(entry.FullPath)) + 4 + 4 //nolint:gosec // 头部大小受限于入口数据量
	}
	pkg.HeaderSize = headerSize

	// 不读取字节数据时直接返回
	if !r.ReadEntryBytes {
		return pkg, nil
	}

	// 读取每个条目的字节数据（通过 Offset 字段定位，而非顺序读取）
	bodyBytes, err := io.ReadAll(reader)
	if err != nil {
		return nil, fmt.Errorf("读取 PKG 数据体失败: %w", err)
	}

	for _, entry := range entries {
		off := entry.Offset
		bodyLen := int64(len(bodyBytes))
		if off < 0 || entry.Length < 0 || int64(off)+int64(entry.Length) > bodyLen {
			return nil, fmt.Errorf("条目数据偏移越界 (%s): offset=%d length=%d bodyLen=%d",
				entry.FullPath, off, entry.Length, len(bodyBytes))
		}
		entry.Bytes = make([]byte, entry.Length)
		copy(entry.Bytes, bodyBytes[off:off+entry.Length])
	}

	return pkg, nil
}

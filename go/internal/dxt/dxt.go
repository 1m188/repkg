// Package dxt 提供 DXT1/3/5 纹理解压算法。
// 从 C# RePKG.Application.Texture.Helpers.DXT.cs 移植而来，
// 原始算法移植自 LibSquish (http://code.google.com/p/libsquish/)。
//
// 原始版权声明：
// Copyright (c) 2011 by Xalcon @ mmowned.com-Forum
// MIT License
package dxt

// Flags 表示 DXT 压缩标志位。
// 值与 C# DXTFlags 枚举保持一致。
type Flags int

const (
	// FlagDXT1 表示 DXT1 (BC1) 格式，每块 8 字节，可选 1 位 alpha。
	FlagDXT1 Flags = 1 << iota // 1
	// FlagDXT3 表示 DXT3 (BC2) 格式，每块 16 字节，显式 4 位 alpha。
	FlagDXT3 // 2
	// FlagDXT5 表示 DXT5 (BC3) 格式，每块 16 字节，插值 alpha。
	FlagDXT5 // 4
)

// Has 判断 flags 是否包含指定的标志位集合。
func (f Flags) Has(test Flags) bool {
	return f&test != 0
}

// DecompressImage 解压 DXT 压缩的纹理数据为 RGBA8888 格式。
// width/height 为图像尺寸，data 为压缩数据，flags 为 DXT 格式标志。
// 返回 RGBA 像素数据，每像素 4 字节，按行排列。
func DecompressImage(width, height int, data []byte, flags Flags) []byte {
	rgba := make([]byte, width*height*4)

	sourceBlockPos := 0
	bytesPerBlock := 8
	if !flags.Has(FlagDXT1) {
		bytesPerBlock = 16
	}
	targetRGBA := make([]byte, 4*16)

	// 按 4x4 块遍历图像
	for y := 0; y < height; y += 4 {
		for x := 0; x < width; x += 4 {
			if sourceBlockPos+bytesPerBlock > len(data) {
				continue
			}

			// 解压当前块
			decompressBlock(targetRGBA, data, sourceBlockPos, flags)

			// 将解压后的像素写入正确位置
			targetRGBAPos := 0
			for py := range 4 {
				for px := range 4 {
					sx := x + px
					sy := y + py
					if sx < width && sy < height {
						targetPixel := 4 * (width*sy + sx)
						rgba[targetPixel+0] = targetRGBA[targetRGBAPos+0]
						rgba[targetPixel+1] = targetRGBA[targetRGBAPos+1]
						rgba[targetPixel+2] = targetRGBA[targetRGBAPos+2]
						rgba[targetPixel+3] = targetRGBA[targetRGBAPos+3]
						targetRGBAPos += 4
					} else {
						// 超出图像边界的像素跳过
						targetRGBAPos += 4
					}
				}
			}
			sourceBlockPos += bytesPerBlock
		}
	}

	return rgba
}

// decompressBlock 解压单个 4x4 像素块。
// rgba 为输出缓冲区（至少 64 字节），block 为压缩块数据，
// blockIndex 为块数据在 block 中的起始偏移，flags 为 DXT 格式标志。
func decompressBlock(rgba, block []byte, blockIndex int, flags Flags) {
	colorBlockIndex := blockIndex

	// DXT3 和 DXT5 的 alpha 数据在颜色数据之前（前 8 字节）
	if flags.Has(FlagDXT3 | FlagDXT5) {
		colorBlockIndex += 8
	}

	// 解压颜色
	decompressColor(rgba, block, colorBlockIndex, flags.Has(FlagDXT1))

	// 根据格式解压 alpha
	if flags.Has(FlagDXT3) {
		decompressAlphaDxt3(rgba, block, blockIndex)
	} else if flags.Has(FlagDXT5) {
		decompressAlphaDxt5(rgba, block, blockIndex)
	}
}

// decompressAlphaDxt3 解压 DXT3 格式的 alpha 通道。
// DXT3 使用显式 4 位 alpha 值存储。
func decompressAlphaDxt3(rgba, block []byte, blockIndex int) {
	for i := range 8 {
		// 获取一个字节中的两个 4 位 alpha 值
		quant := block[blockIndex+i]

		lo := quant & 0x0F
		hi := quant & 0xF0

		// 将 4 位值扩展回 8 位
		rgba[8*i+3] = lo | (lo << 4)
		rgba[8*i+7] = hi | (hi >> 4)
	}
}

// decompressAlphaDxt5 解压 DXT5 格式的 alpha 通道。
// DXT5 使用两个端点值 + 3 位插值索引。
func decompressAlphaDxt5(rgba, block []byte, blockIndex int) {
	alpha0 := block[blockIndex+0]
	alpha1 := block[blockIndex+1]

	// 构建 8 色 alpha 调色板
	codes := make([]byte, 8)
	codes[0] = alpha0
	codes[1] = alpha1

	if alpha0 <= alpha1 {
		// 5 级 alpha 色板（含 0 和 255 极值）
		for i := 1; i < 5; i++ {
			codes[1+i] = byte((uint32(5-i)*uint32(alpha0) + uint32(i)*uint32(alpha1)) / 5) // #nosec G115 -- alpha 值范围受限
		}
		codes[6] = 0
		codes[7] = 255
	} else {
		// 7 级 alpha 色板（渐变色）
		for i := 1; i < 7; i++ {
			codes[i+1] = byte((uint32(7-i)*uint32(alpha0) + uint32(i)*uint32(alpha1)) / 7) // #nosec G115 -- alpha 值范围受限
		}
	}

	// 解码 3 位索引（共 16 个索引，从 6 字节中提取）
	indices := make([]byte, 16)
	blockSrcPos := 2
	indicesPos := 0
	for range 2 {
		// 从 3 字节中构建 24 位值
		value := 0
		for j := range 3 {
			b := int(block[blockIndex+blockSrcPos])
			blockSrcPos++
			value |= b << (8 * j)
		}

		// 提取 8 个 3 位索引
		for j := range 8 {
			index := (value >> (3 * j)) & 0x07
			indices[indicesPos] = byte(index)
			indicesPos++
		}
	}

	// 将调色板值写入输出
	for i := range 16 {
		rgba[4*i+3] = codes[indices[i]]
	}
}

// decompressColor 解压颜色块。
// isDxt1 为 true 时使用 DXT1 的颜色解码逻辑（含 1 位 alpha）。
func decompressColor(rgba, block []byte, blockIndex int, isDxt1 bool) { //nolint:revive // 函数内变量命名风格与算法原文保持一致
	codes := make([]byte, 16)

	// 解包两个 RGB565 端点颜色
	a := unpack565(block, blockIndex, 0, codes, 0)
	b := unpack565(block, blockIndex, 2, codes, 4)

	// 生成中间颜色值
	for i := range 3 {
		c := int(codes[i])
		d := int(codes[4+i])

		if isDxt1 && a <= b {
			// DXT1 3 色模式 + 透明
			codes[8+i] = byte(uint32(c+d) / 2) //nolint:gosec // 颜色值范围在 0-255 之间，无溢出风险
			codes[12+i] = 0
		} else {
			// 标准 4 色模式
			//nolint:gosec // c 和 d 已在 [0,255] 范围内
			codes[8+i] = byte((2*c + d) / 3)
			//nolint:gosec // c 和 d 已在 [0,255] 范围内
			codes[12+i] = byte((c + 2*d) / 3)
		}
	}

	// 填充中间值的 alpha 通道
	codes[8+3] = 255
	if isDxt1 && a <= b {
		codes[12+3] = 0 // 透明
	} else {
		codes[12+3] = 255 // 不透明
	}

	// 提取 2 位索引（每个块 16 个索引，存储在 4 字节中）
	indices := make([]byte, 16)
	for i := range 4 {
		packed := block[blockIndex+4+i]
		indices[0+i*4] = packed & 0x3
		indices[1+i*4] = (packed >> 2) & 0x3
		indices[2+i*4] = (packed >> 4) & 0x3
		indices[3+i*4] = (packed >> 6) & 0x3
	}

	// 将颜色写入输出
	for i := range 16 {
		offset := 4 * int(indices[i])
		rgba[4*i+0] = codes[offset+0]
		rgba[4*i+1] = codes[offset+1]
		rgba[4*i+2] = codes[offset+2]
		rgba[4*i+3] = codes[offset+3]
	}
}

// unpack565 解包一个 RGB565 颜色值（2 字节）为 4 字节 RGBA。
// 返回原始的 16 位 packed 值（用于 DXT1 的比较判断）。
func unpack565(block []byte, blockIndex, packedOffset int, color []byte, colorOffset int) int {
	// 构建 16 位 packed 值
	value := int(block[blockIndex+packedOffset]) | (int(block[blockIndex+1+packedOffset]) << 8)

	// 提取 RGB 分量
	red := byte((value >> 11) & 0x1F)
	green := byte((value >> 5) & 0x3F)
	blue := byte(value & 0x1F)

	// 将 5/6/5 位扩展到 8 位
	color[colorOffset+0] = (red << 3) | (red >> 2)
	color[colorOffset+1] = (green << 2) | (green >> 4)
	color[colorOffset+2] = (blue << 3) | (blue >> 2)
	color[colorOffset+3] = 255

	return value
}

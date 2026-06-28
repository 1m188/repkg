package dxt

import (
	"testing"
)

func TestFlags位运算(t *testing.T) {
	tests := []struct {
		name string
		f    Flags
		test Flags
		want bool
	}{
		{"DXT1包含DXT1", FlagDXT1, FlagDXT1, true},
		{"DXT3不包含DXT1", FlagDXT3, FlagDXT1, false},
		{"DXT5不包含DXT1", FlagDXT5, FlagDXT1, false},
		{"组合标志包含DXT3", FlagDXT3 | FlagDXT5, FlagDXT3, true},
		{"空标志不包含任何", 0, FlagDXT1, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.f.Has(tt.test); got != tt.want {
				t.Errorf("%v.Has(%v) = %v, 期望 %v", tt.f, tt.test, got, tt.want)
			}
		})
	}
}

// ==================== DXT1 测试 ====================

func TestDXT1解压_全红色块(t *testing.T) {
	// 构造 DXT1 块：两个端点均为纯红色 #FF0000
	// RGB565: R=31, G=0, B=0 → packed = (31<<11) | (0<<5) | 0 = 0xF800
	// 所有索引指向 color0（索引 0）
	block := []byte{
		0x00, 0xF8, // color0: R=31, G=0, B=0 → packed = 63488
		0x00, 0xF8, // color1: 同 color0
		0x00, 0x00, 0x00, 0x00, // 所有索引 = 0
	}

	rgba := DecompressImage(4, 4, block, FlagDXT1)

	// 验证第一个像素为红色
	if rgba[0] < 240 || rgba[1] != 0 || rgba[2] != 0 || rgba[3] != 255 {
		t.Errorf("期望红色像素 (#F80000, alpha 255), 实际得到 RGBA(%d,%d,%d,%d)",
			rgba[0], rgba[1], rgba[2], rgba[3])
	}

	// 验证所有像素 alpha 均为 255（opaque）
	for i := 3; i < 64; i += 4 {
		if rgba[i] != 255 {
			t.Errorf("像素 %d: 期望 alpha 255, 实际 %d", i/4, rgba[i])
		}
	}
}

func TestDXT1解压_透明模式(t *testing.T) {
	// DXT1 透明模式：color0 <= color1
	// color0: RGB565(0,0,0)=0, color1: RGB565(1,1,1)=0x0421
	// color0 <= color1 → 3色+透明模式
	// 所有索引指向 code 3（透明）
	block := []byte{
		0x00, 0x00, // color0: black (0)
		0x21, 0x04, // color1: 略亮 (0x0421), color0=0 <= color1=0x0421 → 透明模式
		0xFF, 0xFF, 0xFF, 0xFF, // 所有索引 = 3 (透明)
	}

	rgba := DecompressImage(4, 4, block, FlagDXT1)

	// 验证所有像素 alpha 均为 0（透明）
	for i := 3; i < 64; i += 4 {
		if rgba[i] != 0 {
			t.Errorf("像素 %d: 期望透明 alpha 0, 实际 %d", i/4, rgba[i])
		}
	}
}

func TestDXT1解压_边界尺寸(t *testing.T) {
	// 测试非 4 对齐的尺寸
	tests := []struct {
		name   string
		width  int
		height int
	}{
		{"1x1像素", 1, 1},
		{"3x3像素", 3, 3},
		{"5x5像素", 5, 5},
		{"7x1像素", 7, 1},
		{"1x7像素", 1, 7},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// 构造足够多的 DXT1 块 (每块 8 字节)
			blocksX := (tt.width + 3) / 4
			blocksY := (tt.height + 3) / 4
			totalBlocks := blocksX * blocksY
			block := make([]byte, totalBlocks*8)

			// 用全红色填充所有块
			for i := range totalBlocks {
				copy(block[i*8:], []byte{0x00, 0xF8, 0x00, 0xF8, 0x00, 0x00, 0x00, 0x00})
			}

			rgba := DecompressImage(tt.width, tt.height, block, FlagDXT1)

			expectedSize := tt.width * tt.height * 4
			if len(rgba) != expectedSize {
				t.Errorf("输出大小 = %d, 期望 %d", len(rgba), expectedSize)
			}
		})
	}
}

// ==================== DXT3 测试 ====================

func TestDXT3解压_alpha半透明(t *testing.T) {
	// DXT3 block: 前 8 字节 alpha, 后 8 字节颜色
	block := make([]byte, 16)
	// Alpha: 全设为 0x88 (半透明)
	for i := range 8 {
		block[i] = 0x88
	}
	// 颜色: 纯红色 (color0 > color1, DXT3 颜色使用 DXT1 逻辑但不触发透明)
	block[8] = 0x00
	block[9] = 0xF8
	block[10] = 0x00
	block[11] = 0xF8
	// indices: 全为 0

	rgba := DecompressImage(4, 4, block, FlagDXT3)

	// DXT3 alpha 扩展：0x8 → (0x8 | (0x8 << 4)) = 0x88 = 136
	expectedAlpha := byte(136)
	for i := 3; i < 64; i += 4 {
		if rgba[i] != expectedAlpha {
			t.Errorf("像素 %d: 期望 alpha %d, 实际 %d", i/4, expectedAlpha, rgba[i])
		}
	}
}

func TestDXT3解压_alpha极端值(t *testing.T) {
	tests := []struct {
		name       string
		alphaBytes [8]byte
		alpha0     byte
		alpha7     byte
	}{
		{"全透明", [8]byte{0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00}, 0, 0},
		{"全不透明", [8]byte{0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF}, 0xFF, 0xFF},
		{"混合alpha", [8]byte{0xF0, 0x00, 0xF0, 0x00, 0xF0, 0x00, 0xF0, 0x00}, 0x00, 0xFF},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			block := make([]byte, 16)
			copy(block, tt.alphaBytes[:])
			// 纯红色颜色块
			block[8] = 0x00
			block[9] = 0xF8
			block[10] = 0x00
			block[11] = 0xF8

			rgba := DecompressImage(4, 4, block, FlagDXT3)

			if rgba[3] != tt.alpha0 {
				t.Errorf("像素0 alpha = %d, 期望 %d", rgba[3], tt.alpha0)
			}
			if rgba[7] != tt.alpha7 {
				t.Errorf("像素1 alpha = %d, 期望 %d", rgba[7], tt.alpha7)
			}
		})
	}
}

// ==================== DXT5 测试 ====================

func TestDXT5解压_alpha渐变(t *testing.T) {
	// DXT5: alpha0=0, alpha1=255 → 7色渐变 (alpha0 > alpha1)
	block := make([]byte, 16)
	block[0] = 255 // alpha0
	block[1] = 0   // alpha1 (alpha0 > alpha1 → 7色渐变)
	// alpha 索引: 全部 0 (全选 alpha0)
	block[2] = 0
	block[3] = 0
	block[4] = 0
	block[5] = 0
	block[6] = 0
	block[7] = 0
	// 颜色: 纯红色
	block[8] = 0x00
	block[9] = 0xF8
	block[10] = 0x00
	block[11] = 0xF8

	rgba := DecompressImage(4, 4, block, FlagDXT5)

	// 全部索引为 0 → 选 codes[0] = alpha0 = 255
	for i := 3; i < 64; i += 4 {
		if rgba[i] != 255 {
			t.Errorf("像素 %d: 期望 alpha 255, 实际 %d", i/4, rgba[i])
		}
	}
}

func TestDXT5解压_alpha5色板(t *testing.T) {
	// DXT5: alpha0=0, alpha1=255 → 但 alpha0 <= alpha1 → 5色色板
	block := make([]byte, 16)
	block[0] = 0   // alpha0
	block[1] = 255 // alpha1 (alpha0 <= alpha1 → 5色色板，含 0 和 255)
	// alpha 索引: 全部 0 (全选 alpha0)
	block[2] = 0
	block[3] = 0
	block[4] = 0
	block[5] = 0
	block[6] = 0
	block[7] = 0
	// 颜色: 纯红色
	block[8] = 0x00
	block[9] = 0xF8
	block[10] = 0x00
	block[11] = 0xF8

	rgba := DecompressImage(4, 4, block, FlagDXT5)

	// 全部索引为 0 → 选 codes[0] = alpha0 = 0
	for i := 3; i < 64; i += 4 {
		if rgba[i] != 0 {
			t.Errorf("像素 %d: 期望透明 alpha 0, 实际 %d", i/4, rgba[i])
		}
	}
}

func TestDXT5解压_alpha极端值(t *testing.T) {
	tests := []struct {
		name   string
		alpha0 byte
		alpha1 byte
	}{
		{"全透明两端点", 0, 0},
		{"全不透明两端点", 255, 255},
		{"端点相同", 128, 128},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(_ *testing.T) {
			block := make([]byte, 16)
			block[0] = tt.alpha0
			block[1] = tt.alpha1
			// 颜色: 纯红色
			block[8] = 0x00
			block[9] = 0xF8
			block[10] = 0x00
			block[11] = 0xF8

			// 不应 panic
			_ = DecompressImage(4, 4, block, FlagDXT5)
		})
	}
}

// ==================== 大图像测试 ====================

func TestDXT1解压_大图像(t *testing.T) {
	const w, h = 256, 256
	blocksX := w / 4
	blocksY := h / 4
	totalBlocks := blocksX * blocksY
	data := make([]byte, totalBlocks*8)

	// 用纯红色填充
	for i := range totalBlocks {
		copy(data[i*8:], []byte{0x00, 0xF8, 0x00, 0xF8, 0x00, 0x00, 0x00, 0x00})
	}

	rgba := DecompressImage(w, h, data, FlagDXT1)

	expectedSize := w * h * 4
	if len(rgba) != expectedSize {
		t.Errorf("输出大小 = %d, 期望 %d", len(rgba), expectedSize)
	}
}

// ==================== 空数据测试 ====================

func TestDecompressImage_空数据不崩溃(t *testing.T) {
	// 空数据不应 panic
	rgba := DecompressImage(4, 4, []byte{}, FlagDXT1)
	// 应该得到全零的 RGBA 数据
	if len(rgba) != 64 {
		t.Errorf("期望 64 字节输出, 实际 %d", len(rgba))
	}
}

func TestDXT1解压_截断数据不崩溃(t *testing.T) {
	// 构造 DXT1 数据仅 5 字节（少于每块 8 字节），验证不 panic 且输出大小正确
	data := make([]byte, 5)
	rgba := DecompressImage(4, 4, data, FlagDXT1)
	expectedSize := 4 * 4 * 4 // 64 bytes for 4x4 RGBA
	if len(rgba) != expectedSize {
		t.Errorf("输出大小 = %d, 期望 %d", len(rgba), expectedSize)
	}
}

func TestDecompressImage_PNG对照已知RGBA(t *testing.T) {
	// DXT1 块，颜色 0 = 纯红 (0xF800), 颜色 1 = 纯蓝 (0x001F)
	// 颜色 0 > 颜色 1 → 4 色模式，第 3 色透明（alpha=0 仅 DXT1 伪透明）
	// 但这是 DXT1 的透明模式判定... 不对，color0 > color1 是 4色模式。
	// 此处测试 DXT1 基本颜色解码。
	block := []byte{
		0x00, 0xF8, // color0
		0x1F, 0x00, // color1
		0x00, 0x00, 0x00, 0x00, // 所有索引 = 0（color0）
	}

	rgba := DecompressImage(4, 4, block, FlagDXT1)

	// 所有像素应该是纯红色（因索引全为 0，指向 color0）
	for i := 0; i < 64; i += 4 {
		if rgba[i] < 240 || rgba[i+1] != 0 || rgba[i+2] != 0 {
			t.Errorf("像素 %d: 期望纯红色, 实际 RGBA(%d,%d,%d,%d)",
				i/4, rgba[i], rgba[i+1], rgba[i+2], rgba[i+3])
			break
		}
	}
}

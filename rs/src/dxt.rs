// DXT1/3/5 块解码算法，从 LibSquish 经 C# 移植。
// 与 C# 原版 RePKG.Application/Texture/Helpers/DXT.cs 对应。

/// 将 DXT 压缩图像解码为 RGBA8888 像素数组。
/// width、height 为图像尺寸，data 为压缩数据，flags 指定 DXT 格式。
pub fn decompress_image(
    width: u32,
    height: u32,
    data: &[u8],
    flags: u32,
) -> Result<Vec<u8>, Box<dyn std::error::Error>> {
    if width > 65536 || height > 65536 || width == 0 || height == 0 {
        return Err(format!("DXT 图像尺寸无效：{}x{}", width, height).into());
    }
    let total_bytes = (width as usize) * (height as usize) * 4;
    let mut rgba = vec![0u8; total_bytes];

    let block_size = if flags & 1 != 0 { 8 } else { 16 }; // DXT1=8, DXT3/5=16

    let blocks_x = width.div_ceil(4) as usize;
    let blocks_y = height.div_ceil(4) as usize;

    for by in 0..blocks_y {
        for bx in 0..blocks_x {
            let block_index = (by * blocks_x + bx) * block_size;
            if block_index + block_size > data.len() {
                return Err("DXT 压缩数据截断：块偏移超出数据长度".into());
            }
            decompress_block(
                &mut rgba,
                data,
                block_index,
                width as usize,
                height as usize,
                bx * 4,
                by * 4,
                flags,
            );
        }
    }

    Ok(rgba)
}

/// 解码单个 4×4 DXT 块。
#[allow(clippy::too_many_arguments)]
fn decompress_block(
    rgba: &mut [u8],
    block: &[u8],
    block_index: usize,
    image_width: usize,
    _image_height: usize,
    block_x: usize,
    block_y: usize,
    flags: u32,
) {
    let is_dxt1 = (flags & 1) != 0;
    let color_block_index = if is_dxt1 {
        block_index
    } else {
        block_index + 8
    };

    decompress_color(
        rgba,
        block,
        color_block_index,
        image_width,
        block_x,
        block_y,
        is_dxt1,
    );

    if (flags & 2) != 0 {
        decompress_alpha_dxt3(rgba, block, block_index, image_width, block_x, block_y);
    } else if (flags & 4) != 0 {
        decompress_alpha_dxt5(rgba, block, block_index, image_width, block_x, block_y);
    }
}

/// DXT3 alpha 解码：每个像素使用显式 4-bit alpha。
fn decompress_alpha_dxt3(
    rgba: &mut [u8],
    block: &[u8],
    block_index: usize,
    image_width: usize,
    block_x: usize,
    block_y: usize,
) {
    for i in 0..16 {
        let quant = block[block_index + i / 2];
        let lo = quant & 0x0F;
        let hi = quant & 0xF0;

        // 交替写入 lo 和 hi 的 alpha 值
        let a = if i % 2 == 0 {
            lo | (lo << 4)
        } else {
            hi | (hi >> 4)
        };

        let pixel_x = block_x + i % 4;
        let pixel_y = block_y + i / 4;
        if pixel_x < image_width && pixel_y < rgba.len() / (image_width * 4) {
            let pixel_index = (pixel_y * image_width + pixel_x) * 4;
            rgba[pixel_index + 3] = a;
        }
    }
}

/// DXT5 alpha 解码：使用插值 alpha 表。
fn decompress_alpha_dxt5(
    rgba: &mut [u8],
    block: &[u8],
    block_index: usize,
    image_width: usize,
    block_x: usize,
    block_y: usize,
) {
    let alpha0 = block[block_index] as i32;
    let alpha1 = block[block_index + 1] as i32;

    let mut codes = [0u8; 8];

    if alpha0 > alpha1 {
        // 7 插值模式
        codes[0] = alpha0 as u8;
        codes[1] = alpha1 as u8;
        for i in 1..7 {
            codes[i + 1] = (((7 - i as i32) * alpha0 + i as i32 * alpha1) / 7) as u8;
        }
    } else {
        // 5 插值 + 极值模式
        codes[0] = alpha0 as u8;
        codes[1] = alpha1 as u8;
        for i in 1..5 {
            codes[i + 1] = (((5 - i as i32) * alpha0 + i as i32 * alpha1) / 5) as u8;
        }
        codes[6] = 0;
        codes[7] = 255;
    }

    // 解码 3-bit 索引（48 位 = 16 个 3-bit 值）
    // 字节 2-4 和 5-7 各包含 8 个索引
    let mut indices = [0u8; 16];
    let read_index = |byte_pos: usize| -> u8 { block[block_index + byte_pos] };

    // 前 8 个索引来自字节 [2], [3], [4]
    let mut packed: u32 =
        (read_index(2) as u32) | ((read_index(3) as u32) << 8) | ((read_index(4) as u32) << 16);
    for item in indices.iter_mut().take(8) {
        *item = (packed & 0x07) as u8;
        packed >>= 3;
    }

    // 后 8 个索引来自字节 [5], [6], [7]
    let mut packed =
        (read_index(5) as u32) | ((read_index(6) as u32) << 8) | ((read_index(7) as u32) << 16);
    for item in indices.iter_mut().skip(8) {
        *item = (packed & 0x07) as u8;
        packed >>= 3;
    }

    for i in 0..16 {
        let pixel_x = block_x + i % 4;
        let pixel_y = block_y + i / 4;
        if pixel_x < image_width {
            let pixel_index = (pixel_y * image_width + pixel_x) * 4;
            if pixel_index + 3 < rgba.len() {
                rgba[pixel_index + 3] = codes[indices[i] as usize];
            }
        }
    }
}

/// DXT 颜色块解码（RGB565 端点 + 2-bit 索引）。
fn decompress_color(
    rgba: &mut [u8],
    block: &[u8],
    block_index: usize,
    image_width: usize,
    block_x: usize,
    block_y: usize,
    is_dxt1: bool,
) {
    let mut codes = [0u8; 16]; // 4 个颜色 × 4 通道 (RGBA)

    // 解包两个 RGB565 端点
    unpack565(block, block_index, 0, &mut codes, 0);
    unpack565(block, block_index + 2, 0, &mut codes, 4);

    // 插值计算中间颜色
    for channel in 0..3 {
        // 2/3 * c0 + 1/3 * c1
        codes[8 + channel] = ((2 * codes[channel] as u32 + codes[4 + channel] as u32) / 3) as u8;
        // 1/3 * c0 + 2/3 * c1
        codes[12 + channel] = ((codes[channel] as u32 + 2 * codes[4 + channel] as u32) / 3) as u8;
    }

    // DXT1 特殊处理
    let c0 = ((block[block_index + 1] as u16) << 8) | (block[block_index] as u16);
    let c1 = ((block[block_index + 3] as u16) << 8) | (block[block_index + 2] as u16);
    if is_dxt1 && c0 <= c1 {
        // 透明模式：code[2] 为平均值，code[3] 透明
        for channel in 0..3 {
            codes[8 + channel] = ((codes[channel] as u32 + codes[4 + channel] as u32) / 2) as u8;
            codes[12 + channel] = 0;
        }
        codes[11] = 255; // code[2] alpha = 255
        codes[15] = 0; // code[3] alpha = 0（透明）
    } else {
        // 非透明模式或非 DXT1：所有颜色 alpha = 255
        codes[11] = 255;
        codes[15] = 255;
    }
    // code[0] 和 code[1] 的 alpha 由 unpack565 已设为 255

    // 读取 4 字节的 2-bit 索引
    for i in 0..16 {
        let code = ((block[block_index + 4 + i / 4] >> (2 * (i % 4))) & 0x03) as usize;
        let pixel_x = block_x + i % 4;
        let pixel_y = block_y + i / 4;
        if pixel_x < image_width {
            let pixel_index = (pixel_y * image_width + pixel_x) * 4;
            if pixel_index + 3 < rgba.len() {
                for channel in 0..4 {
                    rgba[pixel_index + channel] = codes[code * 4 + channel];
                }
            }
        }
    }
}

/// 解包一个 RGB565 值到 RGB888 通道（R、G、B，第四个通道 alpha 设为 255）。
fn unpack565(
    block: &[u8],
    block_index: usize,
    _packed_offset: i32,
    colour: &mut [u8],
    colour_offset: usize,
) {
    let value = ((block[block_index + 1] as u16) << 8) | (block[block_index] as u16);

    let r5 = (value >> 11) & 0x1F;
    let g6 = (value >> 5) & 0x3F;
    let b5 = value & 0x1F;

    colour[colour_offset] = ((r5 << 3) | (r5 >> 2)) as u8;
    colour[colour_offset + 1] = ((g6 << 2) | (g6 >> 4)) as u8;
    colour[colour_offset + 2] = ((b5 << 3) | (b5 >> 2)) as u8;
    colour[colour_offset + 3] = 255;
}

#[cfg(test)]
#[allow(non_snake_case)]
mod tests {
    use super::*;

    // ========== unpack565 测试 ==========

    #[test]
    fn 测试_RGB565_解包到最大值() {
        // 0xFFFF = all 1s → R=0x1F, G=0x3F, B=0x1F → R8=0xFF, G8=0xFF, B8=0xFF
        let block = [0xFFu8, 0xFF];
        let mut colour = [0u8; 4];
        unpack565(&block, 0, 0, &mut colour, 0);
        assert_eq!(colour[0], 0xFF); // R
        assert_eq!(colour[1], 0xFF); // G
        assert_eq!(colour[2], 0xFF); // B
        assert_eq!(colour[3], 255); // A
    }

    #[test]
    fn 测试_RGB565_解包到最小值() {
        let block = [0x00u8, 0x00];
        let mut colour = [0u8; 4];
        unpack565(&block, 0, 0, &mut colour, 0);
        assert_eq!(colour[0], 0);
        assert_eq!(colour[1], 0);
        assert_eq!(colour[2], 0);
        assert_eq!(colour[3], 255);
    }

    // ========== DXT1 测试 ==========

    #[test]
    fn 测试_DXT1_4x4_单色块() {
        // 构造一个两端点颜色相同的 DXT1 块（8 字节）
        // 颜色 0xFF80 → R5=0x1F, G6=0x3C, B5=0x00
        // R8=(0x1F<<3)|(0x1F>>2)=0xFF, G8=(0x3C<<2)|(0x3C>>4)=0xF3, B8=0x00
        let mut block = [0u8; 8];
        block[0] = 0x80;
        block[1] = 0xFF;
        block[2] = 0x80;
        block[3] = 0xFF;
        // 索引字节全 0（所有像素使用 color0）
        block[4] = 0x00;
        block[5] = 0x00;
        block[6] = 0x00;
        block[7] = 0x00;

        let rgba = decompress_image(4, 4, &block, 1).unwrap();
        assert_eq!(rgba.len(), 64);

        // color0 = RGB(0xFF, 0xF3, 0x00)
        for i in 0..16 {
            assert_eq!(rgba[i * 4], 0xFF, "pixel {} R mismatch", i);
            assert_eq!(rgba[i * 4 + 1], 0xF3, "pixel {} G mismatch", i);
            assert_eq!(rgba[i * 4 + 2], 0x00, "pixel {} B mismatch", i);
            assert_eq!(rgba[i * 4 + 3], 255, "pixel {} A mismatch", i);
        }
    }

    #[test]
    fn 测试_DXT1_非透明模式_code3_alpha_应为_255() {
        // 非透明模式 (c0 > c1)，验证 code[3] (第 4 色) alpha = 255
        let mut block = [0u8; 8];
        // c0 = 0xFFFF (white), c1 = 0x0000 (black), c0 > c1 → 非透明
        block[0] = 0xFF;
        block[1] = 0xFF;
        block[2] = 0x00;
        block[3] = 0x00;
        // 所有像素使用颜色索引 3 (2-bit = 11)
        block[4] = 0xFF;
        block[5] = 0xFF;
        block[6] = 0xFF;
        block[7] = 0xFF;

        let rgba = decompress_image(4, 4, &block, 1).unwrap();
        for i in 0..16 {
            assert_eq!(
                rgba[i * 4 + 3],
                255,
                "pixel {} code3 alpha should be 255",
                i
            );
        }
    }

    #[test]
    fn 测试_DXT1_3x3_非对齐尺寸() {
        let mut block = [0u8; 8];
        // 红色编码
        block[0] = 0x80;
        block[1] = 0xFF;
        block[2] = 0x80;
        block[3] = 0xFF;
        block[4] = 0x00;
        block[5] = 0x00;
        block[6] = 0x00;
        block[7] = 0x00;

        let rgba = decompress_image(3, 3, &block, 1).unwrap();
        // 3×3 只有一个块，但只应输出 9 个像素
        assert_eq!(rgba.len(), 36);
    }

    #[test]
    fn 测试_DXT1_透明模式() {
        // c0 <= c1: 透明模式
        let mut block = [0u8; 8];
        // c0 = 0x0000 (black), c1 = 0xFFFF (white)
        block[0] = 0x00;
        block[1] = 0x00;
        block[2] = 0xFF;
        block[3] = 0xFF;
        // 索引: 将所有像素设为 code[3] (transparent)
        block[4] = 0xFF;
        block[5] = 0xFF;
        block[6] = 0xFF;
        block[7] = 0xFF;

        let rgba = decompress_image(4, 4, &block, 1).unwrap();
        // c0 <= c1 所以 code[3] 是透明的
        for i in 0..16 {
            assert_eq!(rgba[i * 4 + 3], 0, "pixel {} alpha should be 0", i);
        }
    }

    // ========== DXT3 测试 ==========

    #[test]
    fn 测试_DXT3_全透明() {
        let mut block = [0u8; 16];
        // Alpha 字节全为 0x00
        for i in 0..8 {
            block[i] = 0x00;
        }
        // 颜色块（偏移 +8）：用红色
        block[8] = 0x80;
        block[9] = 0xFF;
        block[10] = 0x80;
        block[11] = 0xFF;
        // 索引全 0
        for i in 12..16 {
            block[i] = 0x00;
        }

        let rgba = decompress_image(4, 4, &block, 2).unwrap(); // DXT3 flag
        for i in 0..16 {
            assert_eq!(rgba[i * 4 + 3], 0, "pixel {} alpha should be 0", i);
        }
    }

    #[test]
    fn 测试_DXT3_全不透明() {
        let mut block = [0u8; 16];
        // Alpha 字节全为 0xFF
        for i in 0..8 {
            block[i] = 0xFF;
        }
        // 颜色块
        block[8] = 0x80;
        block[9] = 0xFF;
        block[10] = 0x80;
        block[11] = 0xFF;
        for i in 12..16 {
            block[i] = 0x00;
        }

        let rgba = decompress_image(4, 4, &block, 2).unwrap(); // DXT3 flag
        for i in 0..16 {
            assert_eq!(rgba[i * 4 + 3], 255, "pixel {} alpha should be 255", i);
        }
    }

    // ========== DXT5 测试 ==========

    #[test]
    fn 测试_DXT5_alpha0_大于_alpha1() {
        let mut block = [0u8; 16];
        // alpha0 = 0xFF, alpha1 = 0x00 (alpha0 > alpha1 → 7 插值模式)
        block[0] = 0xFF;
        block[1] = 0x00;
        // alpha 索引全 0（使用 alpha0）
        for i in 2..8 {
            block[i] = 0x00;
        }
        // 颜色块
        block[8] = 0x80;
        block[9] = 0xFF;
        block[10] = 0x80;
        block[11] = 0xFF;
        for i in 12..16 {
            block[i] = 0x00;
        }

        let rgba = decompress_image(4, 4, &block, 4).unwrap(); // DXT5 flag
        for i in 0..16 {
            assert_eq!(rgba[i * 4 + 3], 255, "pixel {} alpha should be 255", i);
        }
    }

    #[test]
    fn 测试_DXT5_alpha0_不大于_alpha1() {
        let mut block = [0u8; 16];
        // alpha0 = 0x00, alpha1 = 0xFF (alpha0 <= alpha1 → 5 插值 + 极值模式)
        block[0] = 0x00;
        block[1] = 0xFF;
        // alpha 索引全 0（使用 code[0] = alpha0 = 0）
        for i in 2..8 {
            block[i] = 0x00;
        }
        // 颜色块
        block[8] = 0x80;
        block[9] = 0xFF;
        block[10] = 0x80;
        block[11] = 0xFF;
        for i in 12..16 {
            block[i] = 0x00;
        }

        let rgba = decompress_image(4, 4, &block, 4).unwrap(); // DXT5 flag
        for i in 0..16 {
            assert_eq!(rgba[i * 4 + 3], 0, "pixel {} alpha should be 0", i);
        }
    }

    #[test]
    fn 测试_DXT5_alpha_插值渐变色() {
        let mut block = [0u8; 16];
        // alpha0 = 0x80, alpha1 = 0x00 → alpha0 > alpha1 → 7 插值模式
        block[0] = 0x80;
        block[1] = 0x00;
        // 所有 alpha 索引设为 0（code[0] = 0x80）
        for i in 2..8 {
            block[i] = 0x00;
        }
        // 颜色块
        block[8] = 0x80;
        block[9] = 0xFF;
        block[10] = 0x80;
        block[11] = 0xFF;
        for i in 12..16 {
            block[i] = 0x00;
        }

        let rgba = decompress_image(4, 4, &block, 4).unwrap();
        // 所有像素 alpha 应为 0x80
        for i in 0..16 {
            assert_eq!(
                rgba[i * 4 + 3],
                0x80,
                "pixel {} alpha should be 0x80, got {}",
                i,
                rgba[i * 4 + 3]
            );
        }
    }
}

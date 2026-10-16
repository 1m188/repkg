/**
 * DXT1/DXT3/DXT5 块解码。
 *
 * 由 C# RePKG.Application.Texture.Helpers.DXT 逐行移植，
 * 该 C# 实现源自 LibSquish（Xalcon 的移植版），算法与内部数组布局保持一致，
 * 以保证解码结果与 C#/Go/Rust 三个桌面实现逐位相同。
 */

/** DXT 格式标志位，对应 C# DXTFlags（DXT1=1, DXT3=2, DXT5=4） */
export const DXTFlags = {
  DXT1: 1,
  DXT3: 2,
  DXT5: 4,
} as const;
export type DXTFlagsValue = 1 | 2 | 4;

/** 解码单个块（16 像素 → rgba 64 字节），对应 C# DXT.Decompress */
function decompress(rgba: Uint8Array, block: Uint8Array, blockIndex: number, flags: number): void {
  // 块内子块位置：DXT3/5 先存 8 字节 alpha，再存 8 字节颜色
  let colorBlockIndex = blockIndex;
  if ((flags & (DXTFlags.DXT3 | DXTFlags.DXT5)) !== 0) colorBlockIndex += 8;

  // 解码颜色
  decompressColor(rgba, block, colorBlockIndex, (flags & DXTFlags.DXT1) !== 0);

  // DXT3 / DXT5 单独解码 alpha
  if ((flags & DXTFlags.DXT3) !== 0) {
    decompressAlphaDxt3(rgba, block, blockIndex);
  } else if ((flags & DXTFlags.DXT5) !== 0) {
    decompressAlphaDxt5(rgba, block, blockIndex);
  }
}

/** DXT3 alpha（每像素 4 位，成对展开），对应 C# DecompressAlphaDxt3 */
function decompressAlphaDxt3(rgba: Uint8Array, block: Uint8Array, blockIndex: number): void {
  for (let i = 0; i < 8; i++) {
    const quant = block[blockIndex + i];

    const lo = quant & 0x0f;
    const hi = quant & 0xf0;

    // 4 位还原成 8 位
    rgba[8 * i + 3] = lo | (lo << 4);
    rgba[8 * i + 7] = hi | (hi >> 4);
  }
}

/** DXT5 alpha（8 级插值码本 + 3 位索引），对应 C# DecompressAlphaDxt5 */
function decompressAlphaDxt5(rgba: Uint8Array, block: Uint8Array, blockIndex: number): void {
  const alpha0 = block[blockIndex + 0];
  const alpha1 = block[blockIndex + 1];

  const codes = new Uint8Array(8);
  codes[0] = alpha0;
  codes[1] = alpha1;
  if (alpha0 <= alpha1) {
    // 5 级插值 + 0 / 255（与 C# 一致：整数除法截断）
    for (let i = 1; i < 5; i++) {
      codes[1 + i] = Math.floor(((5 - i) * alpha0 + i * alpha1) / 5);
    }
    codes[6] = 0;
    codes[7] = 255;
  } else {
    // 7 级插值
    for (let i = 1; i < 7; i++) {
      codes[i + 1] = Math.floor(((7 - i) * alpha0 + i * alpha1) / 7);
    }
  }

  // 解码 3 位索引：2 × 3 字节展开 16 个索引
  const indices = new Uint8Array(16);
  let blockSrcPos = 2;
  let indicesPos = 0;
  for (let i = 0; i < 2; i++) {
    let value = 0;
    for (let j = 0; j < 3; j++) {
      const b = block[blockIndex + blockSrcPos++];
      value |= b << (8 * j);
    }
    for (let j = 0; j < 8; j++) {
      const index = (value >> (3 * j)) & 0x07;
      indices[indicesPos++] = index;
    }
  }

  for (let i = 0; i < 16; i++) {
    rgba[4 * i + 3] = codes[indices[i]];
  }
}

/** 颜色块解码（565 端点 + 2 位索引），对应 C# DecompressColor */
function decompressColor(
  rgba: Uint8Array,
  block: Uint8Array,
  blockIndex: number,
  isDxt1: boolean,
): void {
  const codes = new Uint8Array(16);
  const a = unpack565(block, blockIndex, 0, codes, 0);
  const b = unpack565(block, blockIndex, 2, codes, 4);

  // 中间色
  for (let i = 0; i < 3; i++) {
    const c = codes[i];
    const d = codes[4 + i];

    if (isDxt1 && a <= b) {
      codes[8 + i] = Math.floor((c + d) / 2);
      codes[12 + i] = 0;
    } else {
      codes[8 + i] = Math.floor((2 * c + d) / 3);
      codes[12 + i] = Math.floor((c + 2 * d) / 3);
    }
  }

  // 中间色的 alpha 位
  codes[8 + 3] = 255;
  codes[12 + 3] = isDxt1 && a <= b ? 0 : 255;

  // 解码 2 位索引（每字节 4 像素）
  const indices = new Uint8Array(16);
  for (let i = 0; i < 4; i++) {
    const packed = block[blockIndex + 4 + i];

    indices[0 + i * 4] = packed & 0x3;
    indices[1 + i * 4] = (packed >> 2) & 0x3;
    indices[2 + i * 4] = (packed >> 4) & 0x3;
    indices[3 + i * 4] = (packed >> 6) & 0x3;
  }

  // 写出像素
  for (let i = 0; i < 16; i++) {
    const offset = 4 * indices[i];

    rgba[4 * i + 0] = codes[offset + 0];
    rgba[4 * i + 1] = codes[offset + 1];
    rgba[4 * i + 2] = codes[offset + 2];
    rgba[4 * i + 3] = codes[offset + 3];
  }
}

/** RGB565 → 8bit 端点 + alpha=255，对应 C# Unpack565；返回打包值用于端点比较 */
function unpack565(
  block: Uint8Array,
  blockIndex: number,
  packedOffset: number,
  colour: Uint8Array,
  colourOffset: number,
): number {
  const value = block[blockIndex + packedOffset] | (block[blockIndex + 1 + packedOffset] << 8);

  const red = (value >> 11) & 0x1f;
  const green = (value >> 5) & 0x3f;
  const blue = value & 0x1f;

  colour[0 + colourOffset] = (red << 3) | (red >> 2);
  colour[1 + colourOffset] = (green << 2) | (green >> 4);
  colour[2 + colourOffset] = (blue << 3) | (blue >> 2);
  colour[3 + colourOffset] = 255;

  return value;
}

/**
 * 解码整幅 DXT 压缩图像为 RGBA8888，对应 C# DXT.DecompressImage。
 *
 * 尺寸不必是 4 的倍数：按块循环，超出边界的像素被忽略（与 C# 一致）。
 */
export function dxtDecompressImage(
  width: number,
  height: number,
  data: Uint8Array,
  flags: DXTFlagsValue,
): Uint8Array {
  const rgba = new Uint8Array(width * height * 4);

  let sourceBlockPos = 0;
  const bytesPerBlock = (flags & DXTFlags.DXT1) !== 0 ? 8 : 16;
  const targetRGBA = new Uint8Array(4 * 16);

  for (let y = 0; y < height; y += 4) {
    for (let x = 0; x < width; x += 4) {
      if (data.length === sourceBlockPos) continue;

      decompress(targetRGBA, data, sourceBlockPos, flags);

      // 把 4x4 块像素写回正确位置
      let targetRGBAPos = 0;
      for (let py = 0; py < 4; py++) {
        for (let px = 0; px < 4; px++) {
          const sx = x + px;
          const sy = y + py;
          if (sx < width && sy < height) {
            const targetPixel = 4 * (width * sy + sx);

            rgba[targetPixel + 0] = targetRGBA[targetRGBAPos + 0];
            rgba[targetPixel + 1] = targetRGBA[targetRGBAPos + 1];
            rgba[targetPixel + 2] = targetRGBA[targetRGBAPos + 2];
            rgba[targetPixel + 3] = targetRGBA[targetRGBAPos + 3];

            targetRGBAPos += 4;
          } else {
            // 边界外像素忽略
            targetRGBAPos += 4;
          }
        }
      }
      sourceBlockPos += bytesPerBlock;
    }
  }

  return rgba;
}

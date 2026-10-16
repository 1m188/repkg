/**
 * LZ4 原始块解码器（block-format，无帧头）。
 *
 * 对应 C# K4os.Compression.LZ4 LZ4Codec.Decode(src, dst)，
 * 与 Go pierrec/lz4 UncompressBlock、Rust lz4_flex block_decompress 语义一致。
 *
 * LZ4 block 格式：token(1B) → literals 规模(0/1/..nB) → literals → offset(2B LE, 匹配存在时)
 *                → matchLen 扩展(255 续延)。流以“最后一个 token + literals”结尾，
 *                不存在末尾 match 的 offset 字段。
 */

export class Lz4FormatError extends Error {
  constructor(message: string) {
    super(`LZ4 解压失败: ${message}`);
    this.name = "Lz4FormatError";
  }
}

/**
 * 解压一个 LZ4 压缩块。
 *
 * @param src 压缩数据
 * @param dstSize 期望解压后的字节数（来自 mip 头 DecompressedBytesCount）
 * @returns 恰好 dstSize 字节的解压数据；格式非法/长度不符抛 Lz4FormatError
 */
export function lz4Decompress(src: Uint8Array, dstSize: number): Uint8Array {
  if (dstSize < 0) throw new Lz4FormatError("目标大小无效: 为负数");
  const dst = new Uint8Array(dstSize);
  const n = src.length;
  let s = 0;
  let d = 0;

  for (;;) {
    if (s >= n) throw new Lz4FormatError("数据提前结束（缺少 token）");
    const token = src[s++];

    // --- literals ---
    let litLen = token >> 4;
    if (litLen === 15) {
      for (;;) {
        if (s >= n) throw new Lz4FormatError("literals 扩展长度截断");
        const b = src[s++];
        litLen += b;
        if (b !== 255) break;
      }
    }
    if (litLen > 0) {
      if (s + litLen > n) throw new Lz4FormatError("literals 溢出输入");
      if (d + litLen > dstSize) throw new Lz4FormatError("literals 溢出输出");
      dst.set(src.subarray(s, s + litLen), d);
      d += litLen;
      s += litLen;
    }

    // 最后一个 token 可能只包含 literals —— 输入耗尽即流结束
    if (s >= n) break;

    // --- match ---
    if (s + 2 > n) throw new Lz4FormatError("offset 截断");
    const offset = src[s] | (src[s + 1] << 8);
    s += 2;
    if (offset === 0) throw new Lz4FormatError("offset 为 0");
    if (offset > d) throw new Lz4FormatError(`offset 越界: ${offset} > ${d}`);

    let matchLen = (token & 15) + 4;
    if ((token & 15) === 15) {
      for (;;) {
        if (s >= n) throw new Lz4FormatError("matchLen 扩展长度截断");
        const b = src[s++];
        matchLen += b;
        if (b !== 255) break;
      }
    }
    if (d + matchLen > dstSize) throw new Lz4FormatError("match 溢出输出");

    // LZ77 滑窗复制：允许与输出重叠，必须逐字节
    let m = d - offset;
    for (let k = 0; k < matchLen; k++) {
      dst[d++] = dst[m++];
    }
  }

  if (d !== dstSize) {
    throw new Lz4FormatError(`解压后大小不匹配: 期望 ${dstSize}, 实际 ${d}`);
  }
  return dst;
}

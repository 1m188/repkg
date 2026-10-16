/**
 * Worker 侧图像编码（依赖 DOM OffscreenCanvas / gifenc）。
 *
 * PNG：OffscreenCanvas → convertToBlob（现代浏览器原生，无损）。
 * GIF：gifenc 逐帧量化（rgba4444 + 单位 alpha）+ 逐帧局部调色板；
 *      gifenc 每帧在 (0,0) 以自身尺寸绘制 —— 与 C# ImageSharp 逐帧独立尺寸一致。
 *      delay 单位 ms = C# centiseconds × 10（gifenc 内部 /10 还原。
 */
import { GIFEncoder, quantize, applyPalette } from "gifenc";
import type { GifFrame } from "./convert";

/** RGBA 像素 → PNG 字节 */
export async function encodePng(pixels: Uint8Array, width: number, height: number): Promise<Uint8Array> {
  const canvas = new OffscreenCanvas(width, height);
  const ctx = canvas.getContext("2d");
  if (ctx === null) throw new Error("OffscreenCanvas 2d context 不可用");
  const data = new Uint8ClampedArray(pixels.length);
  data.set(pixels);
  ctx.putImageData(new ImageData(data, width, height), 0, 0);
  const blob = await canvas.convertToBlob({ type: "image/png" });
  return new Uint8Array(await blob.arrayBuffer());
}

/** GIF 帧序列 → GIF 字节（透明像素自动暴露为透明色索引） */
export async function encodeGif(frames: GifFrame[]): Promise<Uint8Array> {
  if (frames.length === 0) throw new Error("GIF 没有任何帧");

  const gif = GIFEncoder();
  for (const frame of frames) {
    const palette = quantize(frame.pixels, 256, { format: "rgba4444", oneBitAlpha: true });

    // 透明色索引：优先使用全透明调色板项；没有则不开透明
    let transparentIndex = 0;
    let hasTransparent = false;
    for (let i = 0; i < palette.length; i++) {
      const a = palette[i].length > 3 ? palette[i][3] : 255;
      if (a <= 0) {
        transparentIndex = i;
        hasTransparent = true;
        break;
      }
    }

    const index = applyPalette(frame.pixels, palette, "rgba4444");

    gif.writeFrame(index, frame.width, frame.height, {
      palette,
      delay: Math.max(0, frame.delayCs) * 10, // ms；gifenc 内部 /10 → centiseconds
      transparent: hasTransparent,
      transparentIndex,
      dispose: hasTransparent ? 2 : -1,
      repeat: 0, // 无限循环（与 WE 播放器行为一致）
    });
  }
  gif.finish();
  return gif.bytes();
}

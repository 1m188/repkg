/**
 * TEX → PNG / GIF / MP4 纯像素级转换逻辑。
 *
 * 对应 C# RePKG.Application.Texture.TexToImageConverter。
 * 本文件只负责像素数据的展开 / 裁剪 / 旋转 / 分类，不依赖 DOM ——
 * 可直接在 Node（vitest）与 Worker 中共同使用。
 * 具体编码（PNG/GIF）由宿主环境完成：Worker 用 OffscreenCanvas 调色/PNG，
 * GIF 由 gifenc 完成量化与编码。
 */
import {
  MipmapFormat,
  mipmapFileExtension,
  mipmapIsCompressed,
  mipmapIsRawFormat,
} from "./enums";
import {
  texFirstMipmap,
  texIsGif,
  texIsVideoTexture,
  type Tex,
  type TexMipmap,
} from "./tex";

/** 转换结果：PNG 原始像素 / GIF 帧序列 / 透传格式字节 */
export type ConvertResult =
  | { kind: "png"; width: number; height: number; pixels: Uint8Array }
  | { kind: "gif"; width: number; height: number; frames: GifFrame[] }
  | { kind: "passthrough"; format: number; bytes: Uint8Array };

/** 一帧 GIF：RGBA 像素 + 米顿延迟（C# Frametime，单位秒） */
export interface GifFrame {
  width: number;
  height: number;
  /** RGBA8888 像素（w*h*4） */
  pixels: Uint8Array;
  /** C# 帧延迟（centiseconds），与 ImageSharp FrameDelay 一致 */
  delayCs: number;
}

/** 分类输出格式（与 C# GetConvertedFormat 一致）。返回扩展名；raw → "png" */
export function getConvertedFormat(tex: Tex): string {
  if (texIsVideoTexture(tex)) return mipmapFileExtension(MipmapFormat.VideoMp4);

  const m = texFirstMipmap(tex);
  if (m === null) throw new Error("TEX 没有任何 mipmap");

  if (mipmapIsCompressed(m.format)) {
    throw new Error("Raw mipmap format must be uncompressed");
  }

  return mipmapIsRawFormat(m.format) ? "png" : mipmapFileExtension(m.format);
}

/** 校验 MP4 魔数（ftypisom / ftypmsnv / ftypmp42，大小写不敏感），对应 C# 分支 */
function validateMp4Magic(bytes: Uint8Array): boolean {
  if (bytes.length < 12) {
    throw new Error("Expected mp4 magic header");
  }
  const magic = asciiSlice(bytes, 4, 12);
  return (
    magic === "ftypisom" || magic === "ftypmsnv" || magic === "ftypmp42"
  );
}

function asciiSlice(bytes: Uint8Array, start: number, end: number): string {
  let out = "";
  for (let i = start; i < end; i++) {
    out += String.fromCharCode(bytes[i]);
  }
  return out;
}

/**
 * 把 mipmap 展开为 RGBA8888 像素数组。
 * R8 → 灰度；RG88 → (g,g,g,r)（对应 C# RG88 的 ToVector4: Vector4(G,G,G,R)）。
 */
export function expandMipmapToRGBA(mip: TexMipmap): { pixels: Uint8Array; width: number; height: number } {
  const w = mip.width;
  const h = mip.height;
  const src = mip.bytes;

  switch (mip.format) {
    case MipmapFormat.RGBA8888: {
      const need = w * h * 4;
      if (src.length < need) {
        const padded = new Uint8Array(need);
        padded.set(src.subarray(0, Math.min(src.length, need)));
        return { pixels: padded, width: w, height: h };
      }
      return { pixels: src.subarray(0, need), width: w, height: h };
    }
    case MipmapFormat.R8: {
      const out = new Uint8Array(w * h * 4);
      for (let i = 0; i < w * h; i++) {
        const v = i < src.length ? src[i] : 0;
        out[i * 4 + 0] = v;
        out[i * 4 + 1] = v;
        out[i * 4 + 2] = v;
        out[i * 4 + 3] = 255;
      }
      return { pixels: out, width: w, height: h };
    }
    case MipmapFormat.RG88: {
      const out = new Uint8Array(w * h * 4);
      for (let i = 0; i < w * h; i++) {
        const r = i * 2 < src.length ? src[i * 2] : 0;
        const g = i * 2 + 1 < src.length ? src[i * 2 + 1] : 0;
        out[i * 4 + 0] = g;
        out[i * 4 + 1] = g;
        out[i * 4 + 2] = g;
        out[i * 4 + 3] = r; // C# 映射：RG88.R → Alpha, RG88.G → R/G/B
      }
      return { pixels: out, width: w, height: h };
    }
    default:
      throw new Error(`Mipmap format: ${mip.format} is not supported`);
  }
}

/** 顶部裁剪到 (w, h)，对应 C# image.Mutate(Crop(ImageWidth, ImageHeight)) */
export function cropRgbaTopLeft(pixels: Uint8Array, srcW: number, dstW: number, dstH: number): Uint8Array {
  const out = new Uint8Array(dstW * dstH * 4);
  const rowBytes = Math.min(dstW, srcW) * 4;
  for (let y = 0; y < dstH; y++) {
    const srcRow = y * srcW * 4;
    const dstRow = y * dstW * 4;
    out.set(pixels.subarray(srcRow, srcRow + rowBytes), dstRow);
  }
  return out;
}

/* ------------------------------------------------------------------ */
/* 高层转换                                                            */
/* ------------------------------------------------------------------ */

/**
 * 整体转换，对应 C# TexToImageConverter.ConvertToImage。
 * 输出固定为首层 mipmap。
 */
export function convertToImage(tex: Tex): ConvertResult {
  if (texIsGif(tex)) return convertToGif(tex);

  const m = texFirstMipmap(tex);
  if (m === null) throw new Error("TEX 没有任何 mipmap");

  if (texIsVideoTexture(tex)) {
    if (!validateMp4Magic(m.bytes)) throw new Error("Expected mp4 magic header");
    return { kind: "passthrough", format: MipmapFormat.VideoMp4, bytes: m.bytes };
  }

  if (mipmapIsCompressed(m.format)) {
    throw new Error("Raw mipmap format must be uncompressed");
  }

  if (mipmapIsRawFormat(m.format)) {
    const { pixels, width, height } = expandMipmapToRGBA(m);

    let out = pixels;
    let outW = width;
    let outH = height;
    if (width !== tex.header.imageWidth || height !== tex.header.imageHeight) {
      outW = Math.min(tex.header.imageWidth, width);
      outH = Math.min(tex.header.imageHeight, height);
      out = cropRgbaTopLeft(pixels, width, outW, outH);
    }
    return { kind: "png", width: outW, height: outH, pixels: out };
  }

  // 其余（JPEG/PNG/BMP…）透传
  return { kind: "passthrough", format: m.format, bytes: m.bytes };
}

/**
 * GIF 动画合成，对应 C# ConvertToGif。
 *
 * 每帧 = 在 sprite 源图 (ImageId) 上裁剪 (x, y, |w|, |h|)，再按
 * -(atan2(sign(h), sign(w)) - π/4) 计算角度作正交旋转（C# 语义）。
 * 旋转角在 WE 数据中恒为 90° 的倍数，本实现做精确转置（与 ImageSharp 的快速路径一致）；
 * 若出现非正交角则抛错（防御性，真实数据不存在）。
 */
function convertToGif(tex: Tex): ConvertResult {
  const container = tex.frameInfoContainer;
  if (container === null) {
    throw new Error("TEX is animated but doesn't have frame info container");
  }

  const first = texFirstMipmap(tex);
  if (first === null) throw new Error("TEX is animated but doesn't have frame info container");
  const frameFormat = first.format;

  if (!mipmapIsRawFormat(frameFormat)) {
    throw new Error("Only raw mipmap formats are supported right now while converting gif");
  }

  // 展开所有 sprite 源图（每张图首层 mipmap）
  const sources = tex.imagesContainer.images.map((image) => {
    const mip = image.mipmaps[0];
    if (!mip) throw new Error("GIF sprite image has no mipmaps");
    return expandMipmapToRGBA(mip);
  });

  const frames: GifFrame[] = [];
  for (const fi of container.frames) {
    // 帧几何：width/height 为 0 时退回 HeightX / WidthY（C# 语义）
    const w0 = fi.width !== 0 ? fi.width : fi.heightX;
    const h0 = fi.height !== 0 ? fi.height : fi.widthY;

    const xInt = Math.trunc(Math.min(fi.x, fi.x + w0));
    const yInt = Math.trunc(Math.min(fi.y, fi.y + h0));
    const wAbs = Math.trunc(Math.abs(w0));
    const hAbs = Math.trunc(Math.abs(h0));

    if (fi.imageId < 0 || fi.imageId >= sources.length) {
      throw new Error(`帧 ImageId ${fi.imageId} 超出图片数量 ${sources.length}`);
    }

    const src = sources[fi.imageId];
    const srcW = src.width;

    if (wAbs === 0 || hAbs === 0) {
      throw new Error(`GIF 帧面积为 0: imageId=${fi.imageId} rect=(${xInt},${yInt},${wAbs},${hAbs})`);
    }

    // C# ImageSharp Crop 语义：矩形可以越过源图边界，越界区域保持透明
    const cropped = cropRect(src.pixels, srcW, xInt, yInt, wAbs, hAbs);

    // 旋转角度（度）；atan2 只取 sign(h0)/sign(w0)
    const sh = Math.sign(h0);
    const sw = Math.sign(w0);
    const deg = Math.round(-((Math.atan2(sh, sw) - Math.PI / 4) * 180) / Math.PI);
    const normDeg = ((deg % 360) + 360) % 360;

    let framePixels: Uint8Array;
    let frameW: number;
    let frameH: number;

    switch (normDeg) {
      case 0:
        framePixels = cropped;
        frameW = wAbs;
        frameH = hAbs;
        break;
      case 90: {
        // 顺时针 90°：dest[r][c] = cropped[hAbs-1-c][r]（源裁剪宽 wAbs、高 hAbs）
        frameW = hAbs;
        frameH = wAbs;
        framePixels = new Uint8Array(frameW * frameH * 4);
        for (let r = 0; r < frameH; r++) {
          for (let c = 0; c < frameW; c++) {
            const dst = (r * frameW + c) * 4;
            const s = ((hAbs - 1 - c) * wAbs + r) * 4;
            copyPixel(framePixels, dst, cropped, s);
          }
        }
        break;
      }
      case 180: {
        frameW = wAbs;
        frameH = hAbs;
        framePixels = new Uint8Array(frameW * frameH * 4);
        for (let r = 0; r < frameH; r++) {
          for (let c = 0; c < frameW; c++) {
            const dst = (r * frameW + c) * 4;
            const s = ((hAbs - 1 - r) * wAbs + (wAbs - 1 - c)) * 4;
            copyPixel(framePixels, dst, cropped, s);
          }
        }
        break;
      }
      case 270: {
        // 顺时针 270°（= 逆时针 90°）：dest[r][c] = cropped[c][wAbs-1-r]
        frameW = hAbs;
        frameH = wAbs;
        framePixels = new Uint8Array(frameW * frameH * 4);
        for (let r = 0; r < frameH; r++) {
          for (let c = 0; c < frameW; c++) {
            const dst = (r * frameW + c) * 4;
            const s = (c * wAbs + (wAbs - 1 - r)) * 4;
            copyPixel(framePixels, dst, cropped, s);
          }
        }
        break;
      }
      default:
        throw new Error(`不支持非正交旋转角: ${deg}°`);
    }

    frames.push({
      width: frameW,
      height: frameH,
      pixels: framePixels,
      delayCs: Math.round(fi.frametime * 100),
    });
  }

  const canvasW = container.gifWidth || (frames[0]?.width ?? 0);
  const canvasH = container.gifHeight || (frames[0]?.height ?? 0);
  return { kind: "gif", width: canvasW, height: canvasH, frames };
}

/** RGBA 4 字节像素拷贝 */
function copyPixel(dst: Uint8Array, dstIdx: number, src: Uint8Array, srcIdx: number): void {
  dst[dstIdx] = src[srcIdx];
  dst[dstIdx + 1] = src[srcIdx + 1];
  dst[dstIdx + 2] = src[srcIdx + 2];
  dst[dstIdx + 3] = src[srcIdx + 3];
}

/** 从 RGBA 源图裁剪矩形（越界像素填透明） */
function cropRect(
  pixels: Uint8Array,
  srcW: number,
  x: number,
  y: number,
  w: number,
  h: number,
): Uint8Array {
  const srcH = pixels.length / (srcW * 4);
  const out = new Uint8Array(w * h * 4);
  for (let r = 0; r < h; r++) {
    const sy = y + r;
    if (sy < 0 || sy >= srcH) continue;
    for (let c = 0; c < w; c++) {
      const sx = x + c;
      if (sx < 0 || sx >= srcW) continue;
      const s = (sy * srcW + sx) * 4;
      const d = (r * w + c) * 4;
      copyPixel(out, d, pixels, s);
    }
  }
  return out;
}

/**
 * 与桌面实现对齐的集成校验（tests/parity/baseline.json，由 tools/gen-baseline.mjs 生成）。
 *
 * 校验面：
 *  - PKG 条目表：路径 / 长度 / 每条目原始字节 CRC
 *  - TEX 转换：PNG 输出的像素级 sha256（解码后像素，与编码器无关）
 *  - 透传格式：输出字节 = 原始条目字节
 *  - .tex-json：与桌面版生成的 JSON 语义相等
 *
 * 基准缺失时自动跳过（CI 提交了 baseline.json，因此通常会运行）。
 */
import { createHash } from "node:crypto";
import { readFileSync } from "node:fs";
import { join, dirname, resolve } from "node:path";
import { fileURLToPath } from "node:url";
import { describe, expect, it } from "vitest";

import { parsePackage, type PackageEntry } from "../src/lib/pkg";
import { parseTex } from "../src/lib/tex";
import { convertToImage, getConvertedFormat } from "../src/lib/convert";
import { generateTexJson, generateTexJsonText } from "../src/lib/texjson";
import { mipmapIsRawFormat } from "../src/lib/enums";

interface Baseline {
  source: string;
  pkg: { magic: string; entryCount: number; size: number; shaHex: string };
  entries: { path: string; length: number; crcHex: number | string }[];
  texOutputs: {
    base: string;
    tex?: {
      format: number;
      flags: number;
      textureWidth: number;
      textureHeight: number;
      imageWidth: number;
      imageHeight: number;
      containerMagic: string;
      version: number;
      imageFormat: number;
      images: { width: number; height: number }[][];
    };
    output?: {
      kind: string;
      ext?: string;
      width?: number;
      height?: number;
      frames?: { left: number; top: number; width: number; height: number; delayCs: number }[];
    };
    pixelShaHex?: string;
    json?: Record<string, unknown>;
    rawCrcHex: string | null;
    mip0CrcHex?: string;
  }[];
}

const here = dirname(fileURLToPath(import.meta.url));
const webRoot = resolve(here, "..");
const repoRoot = resolve(webRoot, "..");
const pkgPath = resolve(repoRoot, "go", "testdata", "test.pkg");
const baselinePath = join(here, "parity", "baseline.json");

const hasBaseline = ((): boolean => {
  try {
    readFileSync(baselinePath);
    return true;
  } catch {
    return false;
  }
})();

function crc32(bytes: Uint8Array): number {
  const table = new Uint32Array(256);
  for (let n = 0; n < 256; n++) {
    let c = n;
    for (let k = 0; k < 8; k++) c = c & 1 ? 0xedb88320 ^ (c >>> 1) : c >>> 1;
    table[n] = c >>> 0;
  }
  let c = 0xffffffff;
  for (let i = 0; i < bytes.length; i++) c = table[(c ^ bytes[i]) & 0xff] ^ (c >>> 8);
  return (c ^ 0xffffffff) >>> 0;
}

function sha256Hex(bytes: Uint8Array): string {
  return createHash("sha256").update(bytes).digest("hex");
}

const pkgBytes = hasBaseline ? new Uint8Array(readFileSync(pkgPath)) : new Uint8Array(0);

describe.skipIf(!hasBaseline)("test.pkg 与桌面实现对齐（基准: gen-baseline）", () => {
  const baseline = JSON.parse(readFileSync(baselinePath, "utf8")) as Baseline;

  const pkg = parsePackage(pkgBytes);
  const dataStart = pkg.headerSize;
  const entryBytes = (entry: PackageEntry): Uint8Array =>
    pkgBytes.subarray(dataStart + entry.offset, dataStart + entry.offset + entry.length);

  it("PKG 头：魔数 / 条目数与基准一致", () => {
    expect(pkg.magic).toBe(baseline.pkg.magic);
    expect(pkg.entries.length).toBe(baseline.pkg.entryCount);
  });

  it("条目路径 / 长度 / CRC 与桌面提取产物一致", () => {
    const mine = new Map(pkg.entries.map((e) => [e.fullPath, e]));
    for (const ref of baseline.entries) {
      const entry = mine.get(ref.path);
      if (!entry) throw new Error(`缺失条目: ${ref.path}`);
      const bytes = entryBytes(entry);
      expect(entry.length).toBe(ref.length);
      expect(crc32(bytes).toString(16).padStart(8, "0")).toBe(String(ref.crcHex));
    }
  });

  it("offset 连续性（数据区按条目顺序拼接）", () => {
    const entries = pkg.entries;
    expect(entries.length).toBeGreaterThan(0);
    let offset = 0;
    for (const entry of entries) {
      expect(entry.offset).toBe(offset);
      offset += entry.length;
    }
    expect(dataStart + offset).toBe(pkgBytes.length);
  });

  for (const out of baseline.texOutputs) {
    const rawPath = `${out.base}.tex`;
    const entry = pkg.entries.find((e) => e.fullPath === rawPath);
    if (!entry) {
      it(`[基准错] ${out.base}`, () => {
        throw new Error(`基准指向不存在的条目: ${rawPath}`);
      });
      continue;
    }

    if (out.output?.kind === "png") {
      it(`像素一致（PNG）: ${out.base}`, () => {
        const tex = parseTex(entryBytes(entry), { decompress: true });
        const result = convertToImage(tex);
        if (result.kind !== "png") throw new Error(`期望 png，得到 ${result.kind}`);
        expect(result.width).toBe(out.output!.width);
        expect(result.height).toBe(out.output!.height);
        // 与基准 PNG 解码后的像素逐一相等 —— 与编码器无关
        expect(sha256Hex(result.pixels)).toBe(out.pixelShaHex);
      });
    } else if (out.output?.kind === "gif") {
      it(`GIF 帧结构一致: ${out.base}`, () => {
        const tex = parseTex(entryBytes(entry), { decompress: true });
        const result = convertToImage(tex);
        if (result.kind !== "gif") throw new Error(`期望 gif，得到 ${result.kind}`);
        expect(result.frames.length).toBe(out.output!.frames!.length);
        result.frames.forEach((f, i) => {
          expect(f.delayCs).toBe(out.output!.frames![i].delayCs);
        });
      });
    } else if (out.output?.kind === "passthrough") {
      it(`透传字节一致: ${out.base}（.${out.output!.ext}）`, () => {
        const tex = parseTex(entryBytes(entry), { decompress: true });
        const format = tex.imagesContainer.images[0].mipmaps[0].format;
        expect(mipmapIsRawFormat(format)).toBe(false);
        expect(getConvertedFormat(tex)).toBe(out.output!.ext);
        // 透传输出 == 首层 mipmap 字节
        if (out.mip0CrcHex !== undefined) {
          expect(crc32(tex.imagesContainer.images[0].mipmaps[0].bytes).toString(16).padStart(8, "0")).toBe(
            String(out.mip0CrcHex),
          );
        }
      });
    }

    if (out.tex && out.output) {
      it(`TEX 头/容器/mipmap 元数据一致: ${out.base}`, () => {
        const tex = parseTex(entryBytes(entry), { decompress: true });
        const info = out.tex!;
        expect(tex.header.format).toBe(info.format);
        expect(tex.header.flags).toBe(info.flags);
        expect(tex.header.textureWidth).toBe(info.textureWidth);
        expect(tex.header.textureHeight).toBe(info.textureHeight);
        expect(tex.header.imageWidth).toBe(info.imageWidth);
        expect(tex.header.imageHeight).toBe(info.imageHeight);
        expect(tex.imagesContainer.magic).toBe(info.containerMagic);
        expect(tex.imagesContainer.version).toBe(info.version);
        expect(tex.imagesContainer.imageFormat).toBe(info.imageFormat);
        expect(tex.imagesContainer.images.length).toBe(info.images.length);
        tex.imagesContainer.images.forEach((img, i) => {
          expect(img.mipmaps.map((m) => ({ width: m.width, height: m.height }))).toEqual(
            info.images[i],
          );
        });
      });
    }

    if (out.json !== undefined) {
      it(`.tex-json 语义一致: ${out.base}`, () => {
        const tex = parseTex(entryBytes(entry), { decompress: true });
        expect(generateTexJson(tex)).toEqual(out.json);
        expect(JSON.parse(generateTexJsonText(tex))).toEqual(out.json);
      });
    }
  }
});

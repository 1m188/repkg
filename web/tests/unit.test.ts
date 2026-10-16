/**
 * 单元测试：binutil / lz4 / dxt（含 Go 实现交叉定义源）/ pkg / tex / convert / zip / sanitize。
 * 所有数据在内存构造；webfixtures.json 由仓库 Go 实现（已删除的临时生成器）产生一次并入库。
 */
import { readFileSync } from "node:fs";
import { join, dirname } from "node:path";
import { fileURLToPath } from "node:url";
import { describe, expect, it } from "vitest";

import { ByteReader, EnumNotValidError, TruncatedDataError, UnknownMagicError } from "../src/lib/binutil";
import { lz4Decompress, Lz4FormatError } from "../src/lib/lz4";
import { dxtDecompressImage, DXTFlags } from "../src/lib/dxt";
import { parsePackage, entryName, entryExtension, entryDirectory, packageEntryTypeFromPath, type PackageEntry } from "../src/lib/pkg";
import { parseTex, texIsGif, texIsVideoTexture } from "../src/lib/tex";
import { convertToImage, expandMipmapToRGBA, cropRgbaTopLeft } from "../src/lib/convert";
import { MipmapFormat, TexFlags, TexFormat, Constants } from "../src/lib/enums";
import { generateTexJson, generateTexJsonText } from "../src/lib/texjson";
import { ZipBuilder, crc32, sanitizeZipPath } from "../src/lib/zip";

const here = dirname(fileURLToPath(import.meta.url));
const fixtures = JSON.parse(readFileSync(join(here, "fixtures", "webfixtures.json"), "utf8")) as {
  lz4: { name: string; rawHex: string; compHex: string }[];
  dxt: { name: string; w: number; h: number; flag: number; srcHex: string; outHex: string }[];
};

const hex = (s: string) => Uint8Array.from(Buffer.from(s, "hex"));
const toHex = (b: Uint8Array) => Buffer.from(b).toString("hex");

/* ------------------------------------------------------------------ */
/* binutil                                                            */
/* ------------------------------------------------------------------ */

describe("ByteReader", () => {
  it("readNString：空串 / 混合 / maxLength 截断", () => {
    // "\0abc" → 空串（首个 \0 即结束）
    const r1 = new ByteReader(new Uint8Array([0, 97, 98, 99]));
    expect(r1.readNString()).toBe("");
    expect(r1.pos).toBe(1);

    // "hi\0rest" → hi，并且跳过 \0
    const r2 = new ByteReader(new Uint8Array([104, 105, 0, 114]));
    expect(r2.readNString()).toBe("hi");
    expect(r2.pos).toBe(3);
    expect(r2.readBytes(1)[0]).toBe(114);

    // maxLength 截断：读满 3 字节停止，且未跳过 \0
    const r3 = new ByteReader(new Uint8Array([97, 98, 99, 100, 0]));
    expect(r3.readNString(3)).toBe("abc");
    expect(r3.pos).toBe(3);
  });

  it("readStringI32Size：负数抛错 / maxLength 钳制", () => {
    // -1 + "abc" → 负数抛错
    const r1 = new ByteReader(new Uint8Array([0xff, 0xff, 0xff, 0xff, 97, 98, 99]));
    expect(() => r1.readStringI32Size()).toThrow("Size cannot be negative");

    // size=10 被钳到 3，只读 abc
    const r2 = new ByteReader(new Uint8Array([10, 0, 0, 0, 97, 98, 99]));
    expect(r2.readStringI32Size(3)).toBe("abc");
  });

  it("int32/u32/f32 小端读取", () => {
    const bytes = new Uint8Array([0x01, 0x02, 0x03, 0x04, 0x05, 0x06, 0x07, 0x08]);
    const r = new ByteReader(bytes);
    expect(r.readInt32()).toBe(0x04030201);
    expect(r.readUInt32()).toBe(0x08070605);
    expect(r.eof).toBe(true);
  });

  it("截断抛 TruncatedDataError", () => {
    const r = new ByteReader(new Uint8Array([1, 2, 3, 4]));
    r.readBytes(3);
    expect(() => r.readInt32()).toThrow(TruncatedDataError);
  });
});

/* ------------------------------------------------------------------ */
/* LZ4（Go 版实现产生的交叉定义源）                                      */
/* ------------------------------------------------------------------ */

describe("LZ4 块解码", () => {
  for (const c of fixtures.lz4) {
    it(`与 Go pierrec/lz4 逐位一致: ${c.name}`, () => {
      const raw = lz4Decompress(hex(c.compHex), c.rawHex.length / 2);
      expect(toHex(raw)).toBe(c.rawHex);
    });
  }

  it("错误防护：offset 为 0 / 输出尺寸不符 / 截断", () => {
    // 手工构造：token=0xFF?（全字面量 15+255），只有字面量，无 match —— 没问题
    // 手工构造含 offset 0 的块
    //   token = 0x00 (0 literal, match len 4)
    //   offset = 0x0000 → 非法
    expect(() => lz4Decompress(new Uint8Array([0x00, 0x00, 0x00]), 4)).toThrow(Lz4FormatError);

    // 输出尺寸大于实际可解
    //   token = 0x40（literal=4）+ 4 个字面量
    expect(() => lz4Decompress(new Uint8Array([0x40, 1, 2, 3, 4]), 10)).toThrow(Lz4FormatError);

    // 输入截断（token 声称有 4 字节字面量但只有 2 字节）
    expect(() => lz4Decompress(new Uint8Array([0x40, 1, 2]), 4)).toThrow(Lz4FormatError);

    // 正常的 "仅字面量" 结尾块
    const out = lz4Decompress(new Uint8Array([0x40, 1, 2, 3, 4]), 4);
    expect([...out]).toEqual([1, 2, 3, 4]);
  });
});

/* ------------------------------------------------------------------ */
/* DXT（Go internal/dxt 逐位参考）                                       */
/* ------------------------------------------------------------------ */

describe("DXT 解码与 Go 实现逐位一致", () => {
  it.each(fixtures.dxt.map((c) => [c.name, c] as const))("%s", (_name, c) => {
    const out = dxtDecompressImage(c.w, c.h, hex(c.srcHex), c.flag as 1 | 2 | 4);
    expect(toHex(out)).toBe(c.outHex);
  });

  it("DXTFlags 常量与 C#/Go 一致", () => {
    expect(DXTFlags.DXT1).toBe(1);
    expect(DXTFlags.DXT3).toBe(2);
    expect(DXTFlags.DXT5).toBe(4);
  });
});

/* ------------------------------------------------------------------ */
/* PKG 解析                                                            */
/* ------------------------------------------------------------------ */

/** 构造 PKG 字节：魔数 + 条目表 + 数据区 */
function buildPkg(magic: string, items: { path: string; data: Uint8Array }[]): Uint8Array {
  const enc = new TextEncoder();
  const parts: Uint8Array[] = [];
  const magicBytes = enc.encode(magic);

  const head = new Uint8Array(4 + magicBytes.length + 4);
  const dv = new DataView(head.buffer);
  dv.setUint32(0, magicBytes.length, true);
  head.set(magicBytes, 4);
  dv.setInt32(4 + magicBytes.length, items.length, true);
  parts.push(head);

  let offset = 0;
  const offsets: number[] = [];
  const lens = items.map((x) => x.data.length);
  for (const len of lens) {
    offsets.push(offset);
    offset += len;
  }

  for (let i = 0; i < items.length; i++) {
    const pathBytes = enc.encode(items[i].path);
    const entryBuf = new Uint8Array(4 + pathBytes.length + 8);
    const edv = new DataView(entryBuf.buffer);
    edv.setInt32(0, pathBytes.length, true);
    entryBuf.set(pathBytes, 4);
    edv.setInt32(4 + pathBytes.length, offsets[i], true);
    edv.setInt32(4 + pathBytes.length + 4, lens[i], true);
    parts.push(entryBuf);
  }

  for (const item of items) parts.push(item.data);
  return concatAll(parts);
}

function concatAll(parts: Uint8Array[]): Uint8Array {
  const total = parts.reduce((a, p) => a + p.length, 0);
  const out = new Uint8Array(total);
  let o = 0;
  for (const p of parts) {
    out.set(p, o);
    o += p.length;
  }
  return out;
}

describe("PKG 解析", () => {
  it("读回魔数与条目（相对条目数据正确）", () => {
    const pkg = parsePackage(
      buildPkg("PKGV0009", [
        { path: "scene.json", data: new Uint8Array([1, 2, 3, 4, 5]) },
        { path: "materials/a.tex", data: new Uint8Array([9, 9, 9]) },
      ]),
    );
    expect(pkg.magic).toBe("PKGV0009");
    expect(pkg.entries.length).toBe(2);

    // headerSize = 4 + len(magic) + 4 + Σ(4 + len(path) + 8)
    const expectedHeaderSize =
      4 + 8 + 4 + (4 + 10 + 8) + (4 + "materials/a.tex".length + 8);
    expect(pkg.headerSize).toBe(expectedHeaderSize);

    const [a, b] = pkg.entries;
    expect(a.fullPath).toBe("scene.json");
    expect(a.offset).toBe(0);
    expect(a.length).toBe(5);
    expect(a.type).toBe(0);
    expect(b.fullPath).toBe("materials/a.tex");
    expect(b.offset).toBe(5);
    expect(b.type).toBe(1);
  });

  it("条目数量为负报警", () => {
    const enc = new TextEncoder();
    const magicBytes = enc.encode("PKG");
    const head = new Uint8Array(4 + magicBytes.length + 4);
    const dv = new DataView(head.buffer);
    dv.setUint32(0, magicBytes.length, true);
    head.set(magicBytes, 4);
    dv.setInt32(4 + magicBytes.length, -3, true);
    expect(() => parsePackage(head)).toThrow("条目数量为负数");
  });

  it("路径辅助与 C# Path 语义对齐", () => {
    expect(entryName("materials/deeptest/name.png")).toBe("name");
    expect(entryName("plain")).toBe("plain");
    expect(entryName("a.b.c.tex")).toBe("a.b.c");
    expect(entryExtension("a.b.c.tex")).toBe(".tex");
    expect(entryExtension("readme")).toBe("");
    expect(entryDirectory("materials/deeptest/name.png")).toBe("materials/deeptest");
    expect(entryDirectory("root.json")).toBe("");

    expect(packageEntryTypeFromPath("x.tex")).toBe(1);
    expect(packageEntryTypeFromPath("x.dat")).toBe(0);
  });
});

/* ------------------------------------------------------------------ */
/* TEX 构造辅助                                                        */
/* ------------------------------------------------------------------ */

function buildTex(parts: Uint8Array[]): Uint8Array {
  return concatAll(parts);
}

function nStr(s: string): Uint8Array {
  const bytes = new TextEncoder().encode(s);
  const out = new Uint8Array(bytes.length + 1);
  out.set(bytes);
  return out;
}

function i32(v: number): Uint8Array {
  const b = new Uint8Array(4);
  new DataView(b.buffer).setInt32(0, v, true);
  return b;
}

function u32(v: number): Uint8Array {
  const b = new Uint8Array(4);
  new DataView(b.buffer).setUint32(0, v, true);
  return b;
}

function f32(v: number): Uint8Array {
  const b = new Uint8Array(4);
  new DataView(b.buffer).setFloat32(0, v, true);
  return b;
}

describe("TEX 解析（合成数据 V1/V2/V3 + GIF）", () => {
  it("V1：RGBA8888 无压缩字面量像素", () => {
    // 2×2 RGBA
    const pixels = new Uint8Array([
      255, 0, 0, 255, 0, 255, 0, 255, 0, 0, 255, 255, 255, 255, 255, 255,
    ]);
    const mip = buildTex([i32(2), i32(2), i32(pixels.length), pixels]);

    const tex = parseTex(
      buildTex([
        nStr("TEXV0005"),
        nStr("TEXI0001"),
        i32(TexFormat.RGBA8888),
        i32(0), // flags
        i32(2), i32(2), i32(2), i32(2),
        u32(0),
        nStr("TEXB0001"),
        i32(1), // one image
        i32(1), // one mipmap
        mip,
      ]),
    );

    expect(tex.header.imageWidth).toBe(2);
    expect(tex.imagesContainer.magic).toBe("TEXB0001");
    expect(tex.imagesContainer.version).toBe(1);

    const result = convertToImage(tex);
    if (result.kind !== "png") throw new Error("expected png");
    expect(result.width).toBe(2);
    expect(result.height).toBe(2);
    expect([...result.pixels]).toEqual([...pixels]);
  });

  it("V2：LZ4 压缩 + DXT5 → RGBA", () => {
    // 4×4 DXT5 块：alpha 块（alpha0=255>alpha1=0 → 7 级码本，索引全 0 → 全不透明）
    const dxt5Block = new Uint8Array(16);
    dxt5Block[0] = 255; // alpha0
    dxt5Block[1] = 0; // alpha1

    // 颜色块：端点 0 = 0x07E0（绿），端点 1 = 0x001F（蓝）
    dxt5Block[8] = 0xe0; // 0x07E0 low byte
    dxt5Block[9] = 0x07; // 0x07E0 high byte
    dxt5Block[10] = 0x1f; // 0x001F low byte
    dxt5Block[11] = 0x00;
    // 2 位索引：0xE4 = 11-10-01-00 → 像素0=绿, 像素1=蓝, 像素2=(2c+d)/3, 像素3=(c+2d)/3
    dxt5Block[12] = 0xe4;

    // 原始压缩态（isLZ4=0）→ 让 parseTex(decompress=true) 走 DXT5 解码路径
    const compMip = buildTex([
      i32(4), i32(4),        // w/h
      i32(0), i32(0),        // 未 LZ4；decompressed count 无效
      i32(dxt5Block.length),
      dxt5Block,
    ]);

    const tex = parseTex(
      buildTex([
        nStr("TEXV0005"),
        nStr("TEXI0001"),
        i32(TexFormat.DXT5),
        i32(0),
        i32(4), i32(4), i32(4), i32(4),
        u32(0),
        nStr("TEXB0002"),
        i32(1),
        i32(1),
        compMip,
      ]),
    );

    const mip = tex.imagesContainer.images[0].mipmaps[0];
    expect(mip.format).toBe(MipmapFormat.RGBA8888); // 解压后升级
    const result = convertToImage(tex);
    if (result.kind !== "png") throw new Error("expected png");
    expect(result.pixels.length).toBe(4 * 4 * 4);

    // 逐像素期望：行 0 = [绿, 蓝, (0,170,85), (0,85,170)]；行 1-3 索引字节为 0 → 全绿
    const g = [0, 255, 0, 255];
    const b = [0, 0, 255, 255];
    const mid1 = [0, 170, 85, 255];
    const mid2 = [0, 85, 170, 255];
    for (let i = 0; i < 16; i++) {
      const expectPx = i === 0 ? g : i === 1 ? b : i === 2 ? mid1 : i === 3 ? mid2 : g;
      const got = [...result.pixels.subarray(i * 4, i * 4 + 4)];
      expect(got).toEqual(expectPx);
    }
  });

  it("V3：嵌入 JPEG 透传（FIF_JPEG）+ tex-json", () => {
    const fakeJpeg = new Uint8Array([0xff, 0xd8, 0xff, 0xe0, 1, 2, 3]);
    // 容器布局：TEXB0003 → [magic][imageCount][imageFormat][每张 mipmapCount + mip…]
    const mip = buildTex([i32(1), i32(4), i32(4), i32(0), i32(0), i32(fakeJpeg.length), fakeJpeg]);

    const tex = parseTex(
      buildTex([
        nStr("TEXV0005"),
        nStr("TEXI0001"),
        i32(TexFormat.RGBA8888),
        i32(TexFlags.ClampUVs),
        i32(8), i32(8), i32(4), i32(4),
        u32(0),
        nStr("TEXB0003"),
        i32(1), // imageCount
        i32(2), // FIF_JPEG
        mip,
      ]),
    );

    expect(texIsVideoTexture(tex)).toBe(false);
    expect(tex.imagesContainer.imageFormat).toBe(2);

    const result = convertToImage(tex);
    if (result.kind !== "passthrough") throw new Error("expected passthrough");
    expect([...result.bytes]).toEqual([...fakeJpeg]);

    const info = generateTexJson(tex);
    expect(info["format"]).toBe("rgba8888");
    expect(info["clampuvs"]).toBe(true);
    expect(info["nonpoweroftwo"]).toBe("false");
    expect("spritesheetsequences" in info).toBe(false);
  });

  it("GIF：TEXS0003 + 若干帧（旋转与延迟解析）", () => {
    const px = new Uint8Array(4 * 4 * 4).fill(128);
    const img = () =>
      buildTex([
        i32(1),
        i32(1),
        i32(4),
        i32(4),
        i32(0),
        i32(0),
        i32(px.length),
        px,
      ]);

    const frame = buildTex([
      i32(0),                 // imageId
      f32(0.5),               // frametime → 50 cs
      f32(0), f32(0),         // x, y（TEXS0002/0003 坐标为 float）
      f32(4), f32(0), f32(0), f32(4), // width=4, widthY=0, heightX=0, height=4 → 角度 0
    ]);
    const frameRot = buildTex([
      i32(1),
      f32(0.25),
      f32(0), f32(0),
      f32(-4), f32(0), f32(0), f32(4), // width=-4 → 90°（C# 公式的逆时针语义）
    ]);

    const tex = parseTex(
      buildTex([
        nStr("TEXV0005"),
        nStr("TEXI0001"),
        i32(TexFormat.RGBA8888),
        i32(TexFlags.IsGif),
        i32(8), i32(8), i32(4), i32(4),
        u32(0),
        nStr("TEXB0001"),
        i32(2), // 2 images
        i32(1), // image1.mipmapCount
        buildTex([i32(4), i32(4), i32(px.length), px]), // V1：[w][h][byteCount][bytes]
        i32(1), // image2.mipmapCount
        buildTex([i32(4), i32(4), i32(px.length), px]),
        nStr("TEXS0003"),
        i32(2), // 2 frames
        i32(8), i32(4), // gifW/gifH
        frame,
        frameRot,
      ]),
    );

    expect(texIsGif(tex)).toBe(true);
    expect(tex.frameInfoContainer?.frames.length).toBe(2);
    expect(tex.frameInfoContainer?.gifWidth).toBe(8);

    const result = convertToImage(tex);
    if (result.kind !== "gif") throw new Error("expected gif");
    expect(result.frames.length).toBe(2);
    expect(result.frames[0].width).toBe(4);
    expect(result.frames[0].height).toBe(4);
    expect(result.frames[0].delayCs).toBe(50);
    expect(result.frames[1].width).toBe(4); // 旋转 90 → 宽高交换（4×4 方阵下仍 4）
    expect(result.frames[1].delayCs).toBe(25);
  });

  it("非法输入：魔数错误 / imageCount 超限 / mipmap 字节数超限", () => {
    const badMagic = buildTex([nStr("TEXV0006")]);
    expect(() => parseTex(badMagic)).toThrow(UnknownMagicError);

    const base = (fmt: number, flags = 0) => [
      nStr("TEXV0005"),
      nStr("TEXI0001"),
      i32(fmt),
      i32(flags),
      i32(1), i32(1), i32(1), i32(1),
      u32(0),
    ];

    const tooManyImages = buildTex([
      ...base(TexFormat.RGBA8888),
      nStr("TEXB0001"),
      i32(Constants.MaximumImageCount + 1),
    ]);
    expect(() => parseTex(tooManyImages)).toThrow(/Image count exceeds/);

    const tooBigMipmap = buildTex([
      ...base(TexFormat.RGBA8888),
      nStr("TEXB0001"),
      i32(1), // imageCount
      i32(1), // mipmapCount
      i32(1), // width
      i32(1), // height
      i32(Constants.MaximumMipmapByteCount + 1), // byteCount（V1 布局）
    ]);
    expect(() => parseTex(tooBigMipmap)).toThrow(/exceeds stream length|byte count exceeds/);

    const unknownTexFormat = buildTex([
      ...base(42),
      nStr("TEXB0001"),
      i32(0),
    ]);
    expect(() => parseTex(unknownTexFormat)).toThrow(EnumNotValidError);
  });
});

/* ------------------------------------------------------------------ */
/* convert 纯像素工具                                                  */
/* ------------------------------------------------------------------ */

describe("expandMipmapToRGBA / crop", () => {
  it("R8 → 灰度 RGBA", () => {
    const mip = {
      width: 2,
      height: 1,
      isLZ4Compressed: false,
      decompressedBytesCount: 0,
      bytes: new Uint8Array([10, 200]),
      format: MipmapFormat.R8,
    };
    const out = expandMipmapToRGBA(mip);
    expect([...out.pixels]).toEqual([10, 10, 10, 255, 200, 200, 200, 255]);
  });

  it("RG88：(r,g) → (g,g,g,r)", () => {
    const mip = {
      width: 1,
      height: 1,
      isLZ4Compressed: false,
      decompressedBytesCount: 0,
      bytes: new Uint8Array([77, 250]), // r=77, g=250
      format: MipmapFormat.RG88,
    };
    const out = expandMipmapToRGBA(mip);
    expect([...out.pixels]).toEqual([250, 250, 250, 77]);
  });

  it("cropRgbaTopLeft 行拷贝", () => {
    const src = new Uint8Array(8 * 8 * 4).fill(9);
    const crop = cropRgbaTopLeft(src, 8, 4, 2);
    expect(crop.length).toBe(4 * 2 * 4);
    expect(crop[0]).toBe(9);
    expect(crop[4 * 2 * 4 - 1]).toBe(9);
  });
});

/* ------------------------------------------------------------------ */
/* ZIP / sanitize                                                      */
/* ------------------------------------------------------------------ */

describe("sanitizeZipPath（zip-slip 防护）", () => {
  it("规范化与防御", () => {
    expect(sanitizeZipPath("a/b/c.png")).toBe("a/b/c.png");
    expect(sanitizeZipPath("\\a\\b")).toBe("a/b");
    expect(sanitizeZipPath("C:\\evil\\x")).toBe("evil/x");
    expect(sanitizeZipPath("../a/../b/./c")).toBe("b/c");
    expect(sanitizeZipPath("/abs//double///")).toBe("abs/double");
    expect(sanitizeZipPath("a\u0000b")).toBe("ab");
    expect(sanitizeZipPath("..")).toBe("");
    expect(sanitizeZipPath("")).toBe("");
    expect(sanitizeZipPath("a/.hidden/dot")).toBe("a/.hidden/dot");
  });

  it("CRC32 对应参考向量", () => {
    // 123456789 CRC should be CBF43926 (IEEE)
    expect(crc32(new TextEncoder().encode("123456789"))).toBe(0xcbf43926);
  });

  it("ZipBuilder：store 模式可被手动解包且重名去重", () => {
    const zip = new ZipBuilder();
    zip.add("dir/a.png", new Uint8Array([1, 2, 3]));
    zip.add("dir/a.png", new Uint8Array([4, 5]));
    zip.add("dir/../evil", new Uint8Array([6])); // sanitize 后为 "evil"
    zip.add("x/y.z/w", new Uint8Array([7]));

    // 手工校验 EOCD：central count = 4
    const parse = parseMinimalZip(zip.finish());
    expect(parse.entries.length).toBe(4);
    expect(parse.entries.map((e) => e.name).sort()).toEqual(
      ["dir/a.png", "dir/a_2.png", "evil", "x/y.z/w"].sort(),
    );

    const a1 = parse.entries.find((e) => e.name === "dir/a.png")!;
    const a2 = parse.entries.find((e) => e.name === "dir/a_2.png")!;
    expect([...a1.data]).toEqual([1, 2, 3]);
    expect([...a2.data]).toEqual([4, 5]);
  });
});

/** 手工解析最简 ZIP（store 模式），验证我们写入的结构合法 */
function parseMinimalZip(bytes: Uint8Array): { entries: { name: string; data: Uint8Array }[] } {
  const dv = new DataView(bytes.buffer, bytes.byteOffset, bytes.byteLength);

  // 找 EOCD（从尾部）
  let eocd = -1;
  for (let i = bytes.length - 22; i >= 0; i--) {
    if (dv.getUint32(i, true) === 0x06054b50) {
      eocd = i;
      break;
    }
  }
  expect(eocd).toBeGreaterThanOrEqual(0);

  const count = dv.getUint16(eocd + 10, true);
  const cdOffset = dv.getUint32(eocd + 16, true);

  const out: { name: string; data: Uint8Array }[] = [];
  let p = cdOffset;
  for (let i = 0; i < count; i++) {
    expect(dv.getUint32(p, true)).toBe(0x02014b50);
    const localOffset = dv.getUint32(p + 42, true);
    const nameLen = dv.getUint16(p + 28, true);
    const dataLen = dv.getUint32(p + 24, true);
    const crc = dv.getUint32(p + 16, true);
    const name = new TextDecoder().decode(bytes.subarray(p + 46, p + 46 + nameLen));
    const localNameLen = dv.getUint16(localOffset + 26, true);
    const dataStart = localOffset + 30 + localNameLen;
    const data = bytes.subarray(dataStart, dataStart + dataLen);
    expect(crc32(data)).toBe(crc);
    out.push({ name, data });
    p += 46 + nameLen;
  }
  return { entries: out };
}

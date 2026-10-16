#!/usr/bin/env node
/**
 * gen-baseline.mjs —— 生成桌面实现基准（tests/parity/baseline.json）。
 *
 * 原理（零循环依赖）：
 *   1) 用仓库中已有的桌面实现（优先 Go，其次 C#/.NET）把 go/testdata/test.pkg 提取两次：
 *      A) --no-tex-convert       → 得到「纯原始」文件树（每个文件 = PKG 条目，路径/大小/CRC）
 *      B) 默认（含 TEX→图片转换） → 得到 A 的超集
 *      两树差集 = 转换产物（图片/GIF/透传）与 .tex-json。
 *   2) PNG 通过 pngjs 解码为 RGBA 像素并取 sha256 —— 与编码器无关的像素级校验。
 *      GIF 解析帧尺寸/延迟做结构级校验；其余透传格式记录扩展名（输出字节数与原始 CRC 相等）。
 *   3) vitest 用 baseline.json 校验 TS 版本的解析与像素输出。
 *
 * 运行： node tools/gen-baseline.mjs        （产物写入 web/tests/parity/baseline.json）
 */
import { execFileSync } from "node:child_process";
import { createHash } from "node:crypto";
import { existsSync, mkdirSync, readdirSync, readFileSync, rmSync, statSync, writeFileSync } from "node:fs";
import { createRequire } from "node:module";
import { join, dirname, relative, resolve } from "node:path";
import { fileURLToPath } from "node:url";

const require = createRequire(import.meta.url);
const { PNG } = require("pngjs");

const here = dirname(fileURLToPath(import.meta.url)); // web/tools
const webDir = resolve(here, "..");
const repoRoot = resolve(webDir, "..");
const pkgPath = resolve(repoRoot, "go/testdata/test.pkg");
const outDir = join(webDir, "tests", "parity");
const tmpDir = join(outDir, "_tmp");

/* ---------------- 基础工具 ---------------- */

function crc32(bytes) {
  const table = new Uint32Array(256).map((_, n) => {
    let c = n;
    for (let k = 0; k < 8; k++) c = c & 1 ? 0xedb88320 ^ (c >>> 1) : c >>> 1;
    return c >>> 0;
  });
  let c = 0xffffffff;
  for (let i = 0; i < bytes.length; i++) c = table[(c ^ bytes[i]) & 0xff] ^ (c >>> 8);
  return (c ^ 0xffffffff) >>> 0;
}

function sha256(bytes) {
  return createHash("sha256").update(bytes).digest("hex");
}

function walk(dir, baseOut = dir, acc = []) {
  for (const name of readdirSync(dir)) {
    const full = join(dir, name);
    const st = statSync(full);
    if (st.isDirectory()) walk(full, baseOut, acc);
    else acc.push(relative(baseOut, full).split("\\").join("/"));
  }
  return acc;
}

function run(command, args, cwd) {
  execFileSync(command, args, { cwd, stdio: ["ignore", "pipe", "pipe"], encoding: "utf8" });
}

/* ---------------- CLI 探测 ---------------- */

function tryGo() {
  try {
    run("go", ["version"], repoRoot);
    const goDir = resolve(repoRoot, "go");
    return {
      id: "go",
      extract(pkg, out) {
        run("go", ["run", "./cmd/repkg", "extract", "--overwrite", "-o", out, pkg], goDir);
      },
      extractNoConvert(pkg, out) {
        run(
          "go",
          ["run", "./cmd/repkg", "extract", "--overwrite", "--no-tex-convert", "-o", out, pkg],
          goDir,
        );
      },
    };
  } catch {
    return null;
  }
}

function tryDotnet() {
  try {
    run("dotnet", ["--version"], repoRoot);
    return {
      id: "csharp",
      extract(pkg, out) {
        run(
          "dotnet",
          ["run", "--project", join(repoRoot, "RePKG"), "-c", "Release", "--", "extract", "--overwrite", "-o", out, pkg],
          repoRoot,
        );
      },
      extractNoConvert(pkg, out) {
        run(
          "dotnet",
          ["run", "--project", join(repoRoot, "RePKG"), "-c", "Release", "--", "extract", "--overwrite", "--no-tex-convert", "-o", out, pkg],
          repoRoot,
        );
      },
    };
  } catch {
    return null;
  }
}

/* ---------------- PKG 头直读（仅原始字节观察，非解析实现） ---------------- */

function readPkgHeaderObservations(bytes) {
  const dv = new DataView(bytes.buffer, bytes.byteOffset, bytes.byteLength);
  const magicLen = dv.getUint32(0, true);
  const magic = new TextDecoder("ascii").decode(bytes.subarray(4, 4 + magicLen));
  const entryCount = dv.getInt32(4 + magicLen, true);
  return { magic, entryCount };
}

/* ---------------- 独立迷你 TEX 结构解析（依据仓库文档记录的格式布局） ----------------
 * 用途：为基准保存 TexHeader / 容器 / mipmap 元数据，并独立判定“透传 or 真转码”。
 * 这里不依赖 web/ 的任何 TS 实现（也不依赖三个桌面实现），只按逆向格式说明读取。
 */

const FIF_EXT = {
  0: "bmp", 1: "ico", 2: "jpg", 3: "jng", 4: "koa", 5: "lbm", 6: "mng",
  7: "pbm", 8: "pbm", 9: "pcd", 10: "pcx", 11: "pgm", 12: "pgm", 13: "png",
  14: "ppm", 15: "ppm", 16: "ras", 17: "tga", 18: "tif", 19: "wbmp",
  20: "psd", 21: "cut", 22: "xbm", 23: "xpm", 24: "dds", 25: "gif",
  26: "hdr", 27: "g3", 28: "sgi", 29: "exr", 30: "j2k", 31: "jp2",
  32: "pfm", 33: "pict", 34: "raw", 35: "mp4",
};

function readNStringMini(dv, pos, maxLen = 16) {
  let end = pos;
  let count = 0;
  while (end < dv.byteLength && dv.getUint8(end) !== 0 && (maxLen < 0 || count < maxLen)) {
    end++;
    count++;
  }
  const str = new TextDecoder().decode(new DataView(dv.buffer, dv.byteOffset + pos, count));
  const consumed = Math.min(count, maxLen < 0 ? Infinity : maxLen);
  return { str, end: pos + consumed + 1 };
}

function readTexStructure(bytes) {
  const dv = new DataView(bytes.buffer, bytes.byteOffset, bytes.byteLength);
  let p = 0;

  const magic1 = readNStringMini(dv, p, 16);
  if (magic1.str !== "TEXV0005") throw new Error(`TEX magic1 不符: ${magic1.str}`);
  p = magic1.end;

  const magic2 = readNStringMini(dv, p, 16);
  if (magic2.str !== "TEXI0001") throw new Error(`TEX magic2 不符: ${magic2.str}`);
  p = magic2.end;

  const header = {
    format: dv.getInt32(p, true),
    flags: dv.getInt32(p + 4, true),
    textureWidth: dv.getInt32(p + 8, true),
    textureHeight: dv.getInt32(p + 12, true),
    imageWidth: dv.getInt32(p + 16, true),
    imageHeight: dv.getInt32(p + 20, true),
  };
  p += 28;

  const containerMagic = readNStringMini(dv, p, 16);
  if (!/^TEXB000[1-4]$/.test(containerMagic.str)) {
    throw new Error(`TEX 容器魔数不符: ${containerMagic.str}`);
  }
  p = containerMagic.end;

  let version = Number(containerMagic.str.slice(4));
  const imageCount = dv.getInt32(p, true);
  p += 4;

  let imageFormat = -1;
  if (version >= 3) {
    imageFormat = dv.getInt32(p, true);
    p += 4;
    if (version === 4) {
      const isVideoMp4 = dv.getInt32(p, true);
      p += 4;
      if (imageFormat === -1 && isVideoMp4 === 1) imageFormat = 35;
    }
  }
  if (version === 4 && imageFormat !== 35) {
    // C#/Go 的 V4 非 MP4 降级语义：mipmap 布局按 V2/V3 读
    version = 3;
  }

  const images = [];
  for (let i = 0; i < imageCount; i++) {
    const mipmapCount = dv.getInt32(p, true);
    p += 4;
    const mips = [];
    for (let j = 0; j < mipmapCount; j++) {
      if (version === 1) {
        const width = dv.getInt32(p, true);
        const height = dv.getInt32(p + 4, true);
        p += 8;
        const byteCount = dv.getInt32(p, true);
        p += 4;
        mips.push({ width, height, isLZ4: false, decompressedBytesCount: 0, byteOffset: p, byteCount });
        p += byteCount;
      } else if (version === 4) {
        const param1 = dv.getInt32(p, true);
        const param2 = dv.getInt32(p + 4, true);
        p += 8;
        const cond = readNStringMini(dv, p, -1);
        p = cond.end;
        const param3 = dv.getInt32(p, true);
        p += 4;
        if (param1 !== 1 || param2 !== 2 || param3 !== 1) throw new Error("TEX V4 参数不符");
        const width = dv.getInt32(p, true);
        const height = dv.getInt32(p + 4, true);
        const isLZ4 = dv.getInt32(p + 8, true) === 1;
        const decomp = dv.getInt32(p + 12, true);
        p += 16;
        const byteCount = dv.getInt32(p, true);
        p += 4;
        mips.push({ width, height, isLZ4, decompressedBytesCount: decomp, byteOffset: p, byteCount });
        p += byteCount;
      } else {
        const width = dv.getInt32(p, true);
        const height = dv.getInt32(p + 4, true);
        const isLZ4 = dv.getInt32(p + 8, true) === 1;
        const decomp = dv.getInt32(p + 12, true);
        p += 16;
        const byteCount = dv.getInt32(p, true);
        p += 4;
        mips.push({ width, height, isLZ4, decompressedBytesCount: decomp, byteOffset: p, byteCount });
        p += byteCount;
      }
    }
    images.push({ mipmapCount, mips });
  }

  return { header, containerMagic: containerMagic.str, version, imageFormat, images };
}

/* ---------------- GIF 结构解析（帧尺寸 + 延迟） ---------------- */

function parseGifStructure(bytes) {
  const b = bytes;
  let p = 6 + 7; // header + LSD
  let gctSize = 0;
  {
    const packed = b[10];
    if (packed & 0x80) {
      gctSize = 2 ** (((packed & 7) + 1) >>> 0) * 3;
      p = 13 + gctSize;
    }
  }
  const frames = [];
  let pendingDelay = 0;

  while (p < b.length) {
    const block = b[p++];
    if (block === 0x3b) break; // trailer
    if (block === 0x21) {
      const label = b[p++];
      if (label === 0xf9) {
        // Graphic Control Extension
        const size = b[p++];
        pendingDelay = b[p + 1] | (b[p + 2] << 8); // centiseconds
        p += size;
        p++; // terminator
      } else {
        while (p < b.length) {
          const size = b[p++];
          if (size === 0) break;
          p += size;
        }
      }
      continue;
    }
    if (block === 0x2c) {
      // Image Descriptor
      const left = b[p] | (b[p + 1] << 8);
      const top = b[p + 2] | (b[p + 3] << 8);
      const width = b[p + 4] | (b[p + 5] << 8);
      const height = b[p + 6] | (b[p + 7] << 8);
      p += 8;
      const packed = b[p++];
      if (packed & 0x80) {
        p += 2 ** ((packed & 7) + 1) * 3;
      }
      // LZW 数据子块
      while (p < b.length) {
        const size = b[p++];
        if (size === 0) break;
        p += size;
      }
      frames.push({ left, top, width, height, delayCs: pendingDelay });
      pendingDelay = 0;
      continue;
    }
    // 未知块：尽力跳出
    break;
  }
  return frames;
}

/* ---------------- 主流程 ---------------- */

function main() {
  if (!existsSync(pkgPath)) {
    console.error(`未找到 test.pkg: ${pkgPath}`);
    process.exit(1);
  }

  const toolchain = tryGo() ?? tryDotnet();
  if (!toolchain) {
    console.error("未找到 go 或 dotnet，无法生成基准。请在有桌面试剂的机器上运行一次。");
    process.exit(2);
  }
  console.log(`基准工具链: ${toolchain.id}`);

  rmSync(tmpDir, { recursive: true, force: true });
  mkdirSync(join(tmpDir, "raw"), { recursive: true });
  mkdirSync(join(tmpDir, "full"), { recursive: true });

  toolchain.extractNoConvert(pkgPath, join(tmpDir, "raw"));
  toolchain.extract(pkgPath, join(tmpDir, "full"));

  const rawFiles = new Map(); // path -> {size, crcHex, bytes}
  for (const rel of walk(join(tmpDir, "raw"))) {
    const bytes = readFileSync(join(tmpDir, "raw", rel));
    rawFiles.set(rel, { size: bytes.length, crcHex: crc32(bytes).toString(16).padStart(8, "0"), bytes });
  }

  const fullFiles = new Map();
  for (const rel of walk(join(tmpDir, "full"))) {
    fullFiles.set(rel, readFileSync(join(tmpDir, "full", rel)));
  }

  // 差集 = 转换产物 / tex-json
  const converted = [...fullFiles.keys()].filter((k) => !rawFiles.has(k));

  const entries = [];
  for (const [rel, info] of rawFiles) {
    entries.push({ path: rel, length: info.size, crcHex: info.crcHex });
  }
  entries.sort((a, b) => (a.path < b.path ? -1 : a.path > b.path ? 1 : 0));

  const texOutputs = [];
  for (const rel of converted.sort()) {
    const bytes = fullFiles.get(rel);

    if (rel.endsWith(".tex-json")) {
      const base = rel.slice(0, -".tex-json".length);
      const rawName = `${base}.tex`;
      const jsonText = new TextDecoder().decode(bytes);
      texOutputs.push({
        base,
        json: JSON.parse(jsonText),
        rawCrcHex: rawFiles.get(rawName)?.crcHex ?? null,
      });
      continue;
    }

    // 转换产物：base = 去掉最后扩展名
    const dot = rel.lastIndexOf(".");
    const slash = rel.lastIndexOf("/");
    const base = dot > slash ? rel.slice(0, dot) : rel;
    const ext = dot > slash ? rel.slice(dot + 1) : "";

    const rawName = `${base}.tex`;
    const raw = rawFiles.get(rawName);
    if (!raw) throw new Error(`转换产物 ${rel} 没有对应的原始 tex: ${rawName}`);
    const rawBytes = raw.bytes;
    const structure = readTexStructure(rawBytes);

    const headerInfo = {
      format: structure.header.format,
      flags: structure.header.flags,
      textureWidth: structure.header.textureWidth,
      textureHeight: structure.header.textureHeight,
      imageWidth: structure.header.imageWidth,
      imageHeight: structure.header.imageHeight,
      containerMagic: structure.containerMagic,
      version: structure.version,
      imageFormat: structure.imageFormat,
      images: structure.images.map((img) =>
        img.mips.map((m) => ({ width: m.width, height: m.height })),
      ),
    };

    const mip0 = structure.images[0].mips[0];
    const outCrcHex = crc32(bytes).toString(16).padStart(8, "0");
    const mip0CrcHex = crc32(rawBytes.subarray(mip0.byteOffset, mip0.byteOffset + mip0.byteCount))
      .toString(16)
      .padStart(8, "0");

    // 分类：
    //  - 容器的 mip 格式 >= 1000（嵌入图片/视频） → 透传（输出 = 首层 mipmap 字节）
    //  - flags 含 isGif → GIF 结构
    //  - 其余（raw / DXT） → 重新编码 PNG（像素哈希）
    function mipFormatOf(structure) {
      // 独立实现 C# TexMipmapFormatGetter 的语义
      if (structure.imageFormat >= 0 && FIF_EXT[structure.imageFormat]) {
        return 1000 + structure.imageFormat; // 图像/视频族（FreeImageFormat 语义）
      }
      switch (structure.header.format) {
        case 0:
          return 1; // RGBA8888
        case 4:
          return 4; // DXT5
        case 6:
          return 5; // DXT3
        case 7:
          return 6; // DXT1
        case 8:
          return 3; // RG88
        case 9:
          return 2; // R8
        default:
          throw new Error(`未知 TexFormat: ${structure.header.format}`);
      }
    }

    const mipFmt = mipFormatOf(structure);
    const isGifTex = (structure.header.flags & 4) !== 0;
    const isVideoTex = (structure.imageFormat === 35) || (structure.header.flags & 32) !== 0;

    if (isGifTex) {
      const frames = parseGifStructure(bytes);
      texOutputs.push({ base, tex: headerInfo, output: { kind: "gif", frames }, rawCrcHex: raw.crcHex });
    } else if (isVideoTex || mipFmt >= 1000) {
      if (outCrcHex !== mip0CrcHex) {
        console.warn(`警告: ${rel} 被判定为透传，但输出字节 != 首层 mipmap 字节`);
      }
      texOutputs.push({
        base,
        tex: headerInfo,
        output: { kind: "passthrough", ext },
        rawCrcHex: raw.crcHex,
        mip0CrcHex,
      });
    } else if (ext === "png") {
      const png = PNG.sync.read(bytes);
      texOutputs.push({
        base,
        tex: headerInfo,
        output: { kind: "png", width: png.width, height: png.height },
        pixelShaHex: createHash("sha256").update(png.data).digest("hex"),
        rawCrcHex: raw.crcHex,
      });
    } else {
      texOutputs.push({ base, tex: headerInfo, output: { kind: "other", ext }, rawCrcHex: raw.crcHex });
    }
  }

  const pkgBytes = readFileSync(pkgPath);
  const obs = readPkgHeaderObservations(pkgBytes);

  const baseline = {
    source: toolchain.id,
    pkgPath: relative(repoRoot, pkgPath).split("\\").join("/"),
    pkg: {
      magic: obs.magic,
      entryCount: obs.entryCount,
      size: pkgBytes.length,
      shaHex: sha256(pkgBytes),
    },
    entries,
    texOutputs,
  };

  mkdirSync(outDir, { recursive: true });
  writeFileSync(join(outDir, "baseline.json"), JSON.stringify(baseline, null, 2));
  console.log(
    `完成: ${entries.length} 条目, ${texOutputs.length} 个转换产物 → ${join(outDir, "baseline.json")}`,
  );

  rmSync(tmpDir, { recursive: true, force: true });
}

main();

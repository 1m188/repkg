/**
 * Pipeline 集成测试（Node 端、注入伪编码器）：
 * 完整回放 worker 词法 —— open（pkg/tex 文档）→ convert（converted/raw/passthrough）→
 * zip（转换/原始/tex-json 开关组合），并手工解包 ZIP 验证内部结构。
 */
import { readFileSync } from "node:fs";
import { resolve, dirname, join } from "node:path";
import { fileURLToPath } from "node:url";
import { describe, expect, it } from "vitest";

import { Pipeline } from "../src/lib/pipeline";
import { parsePackage } from "../src/lib/pkg";
import type { WorkerRequest, WorkerResponse } from "../src/lib/protocol";
import { crc32 } from "../src/lib/zip";

const here = dirname(fileURLToPath(import.meta.url));
const pkgBytes = readFileSync(resolve(here, "../../go/testdata/test.pkg"));

/** 按路径定位 pkg 条目的索引（避免依赖文件内的顺序） */
function entryIndexByPath(pkg: ReturnType<typeof parsePackage>, fullPath: string): number {
  const idx = pkg.entries.findIndex((e) => e.fullPath === fullPath);
  if (idx < 0) throw new Error(`test.pkg 缺少条目: ${fullPath}`);
  return idx;
}

const PKG = () => parsePackage(pkgBytes);

/** 收集 worker 回传的消息 */
function collect() {
  const messages: WorkerResponse[] = [];
  const pipeline = new Pipeline(
    (msg) => {
      messages.push(msg);
    },
    {
      // 伪编码器：把输入包一层可识别头，便于断言"产物出自编码管线"
      encodePng: (pixels, w, h) => {
        const head = new TextEncoder().encode(`FAKEPNG:${w}x${h}:`);
        const out = new Uint8Array(head.length + pixels.length);
        out.set(head);
        out.set(pixels, head.length);
        return out;
      },
      encodeGif: () => new TextEncoder().encode("FAKEGIF"),
    },
  );
  return { messages, pipeline };
}

async function dispatch(pipeline: Pipeline, req: { type: string } & Partial<WorkerRequest>) {
  await pipeline.dispatch(req as unknown as WorkerRequest);
}

describe("Pipeline：PKG 文档", () => {
  it("open：完整消息与条目表", async () => {
    const { messages, pipeline } = collect();
    const file = new File([pkgBytes], "test.pkg");
    await dispatch(pipeline, { type: "open", docId: 1, file, filename: "test.pkg" });

    expect(messages.length).toBe(1);
    const opened = messages[0];
    expect(opened.type).toBe("opened");
    if (opened.type !== "opened") return;
    expect(opened.kind).toBe("pkg");
    expect(opened.magic).toBe("PKGV0009");
    expect(opened.entryCount).toBe(10);
    expect(opened.totalBytes).toBe(pkgBytes.length);
    expect(opened.entries?.length).toBe(10);
    expect(opened.entries?.some((e) => e.type === 1 && e.fullPath.endsWith(".tex"))).toBe(true);
  });

  it("convert：TEX 条目 → converted 产物（png/透传）+ texJson", async () => {
    const { messages, pipeline } = collect();
    await dispatch(pipeline, {
      type: "open",
      docId: 1,
      file: new File([pkgBytes], "test.pkg"),
      filename: "test.pkg",
    });

    messages.length = 0;
    // 普通可转码 tex：waterripplenormal（RGBA → PNG）
    await dispatch(pipeline, {
      type: "convert",
      docId: 1,
      entryIndex: entryIndexByPath(PKG(), "materials/effects/waterripplenormal.tex"),
    });
    const art = messages.find((m) => m.type === "artifact");
    expect(art).toBeDefined();
    if (art?.type !== "artifact") return;
    // 输出名带扩展名（与 C# ConvertToImageAndSave 的 {base}.{ext} 一致）
    expect(art.outPath).toBe("materials/effects/waterripplenormal.png");
    expect(art.ext).toBe("png");
    expect(art.mime).toBe("image/png");
    expect(art.source).toBe("converted");
    const text = new TextDecoder().decode(new Uint8Array(art.bytes.slice(0, 8)));
    expect(text.startsWith("FAKEPNG:")).toBe(true);
    expect(art.texJson).toBeDefined();
    expect(art.info?.imageWidth).toBe(256);
  });

  it("convert：嵌入 JPEG 的 tex → passthrough（jpg，字节=首层 mipmap）", async () => {
    const { messages, pipeline } = collect();
    await dispatch(pipeline, {
      type: "open",
      docId: 1,
      file: new File([pkgBytes], "test.pkg"),
      filename: "test.pkg",
    });

    messages.length = 0;
    const idx = entryIndexByPath(PKG(), "materials/116.tex");
    await dispatch(pipeline, { type: "convert", docId: 1, entryIndex: idx });
    const art = messages.find((m) => m.type === "artifact");
    if (art?.type !== "artifact") throw new Error("no artifact");
    expect(art.ext).toBe("jpg");
    expect(art.mime).toBe("image/jpeg");
    expect(art.outPath).toBe("materials/116.jpg");

    // 透传字节 = 首层 mipmap（解析出的字节长一致）
    const { parseTex } = await import("../src/lib/tex");
    const pkg = PKG();
    const entry = pkg.entries[idx];
    const raw = pkgBytes.subarray(pkg.headerSize + entry.offset, pkg.headerSize + entry.offset + entry.length);
    const tex = parseTex(raw, { decompress: true });
    const mipLen = tex.imagesContainer.images[0].mipmaps[0].bytes.length;
    expect(art.bytes.byteLength).toBe(mipLen);
    expect(crc32(new Uint8Array(art.bytes)).toString(16).padStart(8, "0")).toBe(
      crc32(tex.imagesContainer.images[0].mipmaps[0].bytes).toString(16).padStart(8, "0"),
    );
  });

  it("convert：二进制条目 → raw 直传", async () => {
    const { messages, pipeline } = collect();
    await dispatch(pipeline, {
      type: "open",
      docId: 1,
      file: new File([pkgBytes], "test.pkg"),
      filename: "test.pkg",
    });

    messages.length = 0;
    await dispatch(pipeline, {
      type: "convert",
      docId: 1,
      entryIndex: entryIndexByPath(PKG(), "effects/waterripple/effect.json"),
    });
    const art = messages.find((m) => m.type === "artifact");
    if (art?.type !== "artifact") throw new Error("no artifact");
    expect(art.source).toBe("raw");
    expect(art.outPath).toBe("effects/waterripple/effect.json");
    expect(art.mime).toBe("application/octet-stream");
  });

  it("convert：raw=true → 不解析直接返回原始字节", async () => {
    const { messages, pipeline } = collect();
    await dispatch(pipeline, {
      type: "open",
      docId: 1,
      file: new File([pkgBytes], "test.pkg"),
      filename: "test.pkg",
    });

    messages.length = 0;
    const idx = entryIndexByPath(PKG(), "materials/effects/waterripplenormal.tex");
    await dispatch(pipeline, { type: "convert", docId: 1, entryIndex: idx, raw: true });
    const art = messages.find((m) => m.type === "artifact");
    if (art?.type !== "artifact") throw new Error("no artifact");
    expect(art.source).toBe("raw");
    expect(art.outPath).toBe("materials/effects/waterripplenormal.tex");
    expect(art.texJson).toBeUndefined();
  });

  it("convert：非法条目 index → error 消息", async () => {
    const { messages, pipeline } = collect();
    await dispatch(pipeline, {
      type: "open",
      docId: 1,
      file: new File([pkgBytes], "test.pkg"),
      filename: "test.pkg",
    });
    messages.length = 0;
    await dispatch(pipeline, { type: "convert", docId: 1, entryIndex: 999 });
    expect(messages[0].type).toBe("error");
    if (messages[0].type === "error") {
      expect(messages[0].message).toContain("条目不存在");
    }
  });

  it("zip：转换产物 + tex-json + 可选原始，ZIP 可手工解包", async () => {
    const { messages, pipeline } = collect();
    await dispatch(pipeline, {
      type: "open",
      docId: 1,
      file: new File([pkgBytes], "test.pkg"),
      filename: "test.pkg",
    });

    messages.length = 0;
    await dispatch(pipeline, {
      type: "zip",
      docId: 1,
      params: { convertTex: true, includeRawTex: true, includeTexJson: true },
    });

    const done = messages.find((m) => m.type === "zipDone");
    if (done?.type !== "zipDone") throw new Error("no zipDone");
    // 7 二进制 + 3 tex 转换 + 3 tex-json + 3 原始 tex（includeRawTex=true）
    expect(done.fileCount).toBe(16);
    // 手工解包（store 结构），确认关键名称
    const zipBuf = new Uint8Array(done.bytes);
    const names = collectZipNames(zipBuf);
    expect(names.has("scene.json")).toBe(true);
    expect(names.has("materials/116.jpg")).toBe(true); // 嵌入 JPEG 透传
    expect(names.has("materials/effects/waterripplenormal.png")).toBe(true);
    expect(names.has("materials/effects/waterripplenormal.tex-json")).toBe(true);
    expect(names.has("materials/effects/waterripplenormal.tex")).toBe(true);
    // 进度消息贯通
    const progress = messages.filter((m) => m.type === "zipProgress");
    expect(progress.length).toBe(10);
    expect(progress.at(-1)).toMatchObject({ done: 10, total: 10 });
  });

  it("zip：convertTex=false 时全部原样", async () => {
    const { messages, pipeline } = collect();
    await dispatch(pipeline, {
      type: "open",
      docId: 1,
      file: new File([pkgBytes], "test.pkg"),
      filename: "test.pkg",
    });
    messages.length = 0;
    await dispatch(pipeline, {
      type: "zip",
      docId: 1,
      params: { convertTex: false, includeRawTex: false, includeTexJson: false },
    });
    const done = messages.find((m) => m.type === "zipDone");
    if (done?.type !== "zipDone") throw new Error("no zipDone");
    expect(done.fileCount).toBe(10);
    const names = collectZipNames(new Uint8Array(done.bytes));
    expect(names.has("materials/116.tex")).toBe(true);
    expect(names.has("materials/116.jpg")).toBe(false);
  });
});

describe("Pipeline：单 TEX 文档", () => {
  it("open(kind=tex)：类型判定 + 摘要", async () => {
    const { messages, pipeline } = collect();
    const pkg = PKG();
    const entry = pkg.entries[entryIndexByPath(pkg, "materials/effects/waterripplenormal.tex")];
    const texBytes = pkgBytes.subarray(
      pkg.headerSize + entry.offset,
      pkg.headerSize + entry.offset + entry.length,
    );
    await dispatch(pipeline, {
      type: "open",
      docId: 2,
      file: new File([texBytes], "normal.tex"),
      filename: "normal.tex",
    });

    const opened = messages[0];
    expect(opened.type).toBe("opened");
    if (opened.type !== "opened") return;
    expect(opened.kind).toBe("tex");
    expect(opened.texSummary?.imageWidth).toBe(256);
  });

  it("TEX 文档 convert：基名来自 filename + 输出扩展名", async () => {
    const { messages, pipeline } = collect();
    const pkg = PKG();
    const entry = pkg.entries[entryIndexByPath(pkg, "materials/effects/waterripplenormal.tex")];
    const texBytes = pkgBytes.subarray(
      pkg.headerSize + entry.offset,
      pkg.headerSize + entry.offset + entry.length,
    );
    await dispatch(pipeline, {
      type: "open",
      docId: 3,
      file: new File([texBytes], "bake.tex"),
      filename: "bake.tex",
    });

    messages.length = 0;
    await dispatch(pipeline, { type: "convert", docId: 3, entryIndex: null });
    const art = messages.find((m) => m.type === "artifact");
    if (art?.type !== "artifact") throw new Error("no artifact");
    expect(art.outPath).toBe("bake.png");
    expect(art.ext).toBe("png");
  });
});

/* ------------------------------------------------------------------ */
/* 手工 ZIP 名称提取（store 模式，测试专用）                             */
/* ------------------------------------------------------------------ */

function collectZipNames(bytes: Uint8Array): Set<string> {
  const dv = new DataView(bytes.buffer, bytes.byteOffset, bytes.byteLength);
  let eocd = -1;
  for (let i = bytes.length - 22; i >= 0; i--) {
    if (dv.getUint32(i, true) === 0x06054b50) {
      eocd = i;
      break;
    }
  }
  if (eocd < 0) throw new Error("no EOCD");
  const count = dv.getUint16(eocd + 10, true);
  const cdOffset = dv.getUint32(eocd + 16, true);

  const names = new Set<string>();
  let p = cdOffset;
  for (let i = 0; i < count; i++) {
    expect(dv.getUint32(p, true)).toBe(0x02014b50);
    const nameLen = dv.getUint16(p + 28, true);
    const dataLen = dv.getUint32(p + 24, true);
    const crc = dv.getUint32(p + 16, true);
    const localOffset = dv.getUint32(p + 42, true);
    const name = new TextDecoder().decode(bytes.subarray(p + 46, p + 46 + nameLen));
    const localNameLen = dv.getUint16(localOffset + 26, true);
    const dataStart = localOffset + 30 + localNameLen;
    const data = bytes.subarray(dataStart, dataStart + dataLen);
    expect(crc32(data)).toBe(crc);
    names.add(name);
    p += 46 + nameLen;
  }
  return names;
}

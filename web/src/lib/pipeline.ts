/**
 * 解码流水线核心（消息协议的执行体）。
 *
 * worker.ts 只是 `self.onmessage → Pipeline` 的薄壳；这样管线可以在 Node
 * （vitest）与浏览器 Worker中共用，还允许测试注入伪编码器（OffscreenCanvas
 * 不可用的环境里验证完整 open/convert/zip 流程）。
 */
import { TexFlags, type MipmapFormatValue } from "./enums";
import { mipmapMime } from "./enums";
import { parsePackage, entryExtension, type PackageEntry } from "./pkg";
import { TruncatedDataError } from "./binutil";
import { parseTex, texIsGif, texIsVideoTexture, type Tex } from "./tex";
import { convertToImage, getConvertedFormat, type ConvertResult, type GifFrame } from "./convert";
import { generateTexJsonText } from "./texjson";
import { ZipBuilder, sanitizeZipPath } from "./zip";
import type { EntryMeta, TexSummary, WorkerRequest, WorkerResponse, ZipParams } from "./protocol";

/** PNG / GIF 编码注入接口（默认实现依赖 OffscreenCanvas，见 imaging.ts） */
export interface Encoders {
  encodePng(pixels: Uint8Array, width: number, height: number): Promise<Uint8Array> | Uint8Array;
  encodeGif(frames: GifFrame[]): Promise<Uint8Array> | Uint8Array;
}

interface DocCtx {
  docId: number;
  file: File | Blob;
  filename: string;
  kind: "pkg" | "tex";
  magic: string;
  /** PKG 数据区起点；TEX 文档为 0 */
  headerSize: number;
  entries?: PackageEntry[];
}

/** 条目表最大尝试长度（255B 路径 × 百万条 ≈ 30MB，足够） */
export const HEADER_MAX = 32 * 1024 * 1024;

const ENTRY_TYPE_TEX = 1;

export class Pipeline {
  private readonly docs = new Map<number, DocCtx>();
  private readonly encoders: Encoders;

  constructor(
    /** 回传消息（浏览器 Worker 里是 postMessage；测试里是收集器） */
    private readonly post: (msg: WorkerResponse) => void,
    encoders: Encoders,
  ) {
    this.encoders = encoders;
  }

  /** 处理一条请求（异常被捕获并以 error 消息回报，与 Worker 顶层一致） */
  async dispatch(req: WorkerRequest): Promise<void> {
    try {
      switch (req.type) {
        case "open":
          await this.handleOpen(req.docId, req.file, req.filename);
          break;
        case "convert":
          await this.handleConvert(req.docId, req.entryIndex, req.raw === true);
          break;
        case "zip":
          await this.handleZip(req.docId, req.params);
          break;
      }
    } catch (err) {
      this.post({ type: "error", docId: req.docId, entryIndex: null, message: errorMessage(err) });
    }
  }

  /* ---------------------------------------------------------------- */
  /* 判别 PKG / TEX                                                   */
  /* ---------------------------------------------------------------- */

  async detectKind(file: File | Blob): Promise<"pkg" | "tex"> {
    const head = new Uint8Array(await file.slice(0, 16).arrayBuffer());
    if (head.length >= 4) {
      const firstInt = new DataView(head.buffer).getUint32(0, true);
      // PKG 以 int32 长度前缀魔数开始（"PKGV0005" → 首四字节 08 00 00 00）
      if (firstInt >= 1 && firstInt <= 36) return "pkg";
    }
    if (head.length >= 8) {
      const magic = String.fromCharCode(...head.subarray(0, 8));
      // TEX 以 null 结尾魔数 "TEXV0005" 开始
      if (magic === "TEXV0005") return "tex";
    }
    // 兜底：按扩展名猜；解析错误再报
    const lower = (file as File).name?.toLowerCase() ?? "";
    return lower.endsWith(".tex") ? "tex" : "pkg";
  }

  /* ---------------------------------------------------------------- */
  /* open                                                            */
  /* ---------------------------------------------------------------- */

  async handleOpen(docId: number, file: File | Blob, filename: string): Promise<void> {
    const kind = await this.detectKind(file);

    if (kind === "pkg") {
      const pkg = await readPkgHeader(file);

      const ctx: DocCtx = {
        docId,
        file,
        filename,
        kind,
        magic: pkg.magic,
        headerSize: pkg.headerSize,
        entries: pkg.entries,
      };
      this.docs.set(docId, ctx);

      const entries: EntryMeta[] = pkg.entries.map((e, index) => ({
        index,
        fullPath: e.fullPath,
        offset: e.offset,
        length: e.length,
        type: e.type,
      }));

      this.post({
        type: "opened",
        docId,
        kind,
        filename,
        magic: pkg.magic,
        headerSize: pkg.headerSize,
        entryCount: pkg.entries.length,
        totalBytes: file.size,
        entries,
      });
      return;
    }

    // TEX 文档：整体读入（仅解析头部，mipmap 不预解压 —— 转换时再解压）
    const bytes = new Uint8Array(await file.arrayBuffer());
    const tex = parseTex(bytes, { decompress: false });
    const ctx: DocCtx = { docId, file, filename, kind, magic: tex.magic1, headerSize: 0 };
    this.docs.set(docId, ctx);

    this.post({
      type: "opened",
      docId,
      kind,
      filename,
      magic: tex.magic1,
      headerSize: 0,
      entryCount: 1,
      totalBytes: file.size,
      texSummary: summarizeTex(tex),
    });
  }

  /* ---------------------------------------------------------------- */
  /* convert（单条目 / 单 tex 文档）                                   */
  /* ---------------------------------------------------------------- */

  async handleConvert(docId: number, entryIndex: number | null, raw: boolean): Promise<void> {
    const ctx = this.docs.get(docId);
    if (!ctx) {
      this.post({ type: "error", docId, entryIndex, message: "文档尚未打开" });
      return;
    }

    try {
      if (ctx.kind === "tex") {
        if (raw) {
          const bytes = new Uint8Array(await ctx.file.arrayBuffer());
          this.postRawArtifact(docId, entryIndex, ctx.filename, bytes);
          return;
        }
        const bytes = new Uint8Array(await ctx.file.arrayBuffer());
        const tex = parseTex(bytes, { decompress: true });
        await this.postTexArtifact(docId, entryIndex, entryPathWithoutExt(ctx.filename), tex);
        return;
      }

      const entry = ctx.entries?.[entryIndex ?? -1];
      if (!entry) {
        this.post({ type: "error", docId, entryIndex, message: `条目不存在: ${entryIndex}` });
        return;
      }

      const entryBytes = await this.readEntryBytes(ctx, entry);

      if (raw || entry.type !== ENTRY_TYPE_TEX) {
        this.postRawArtifact(docId, entryIndex, entry.fullPath, entryBytes);
        return;
      }

      // TEX 条目：解析（含解压）+ 转换编码
      const tex = parseTex(entryBytes, { decompress: true });
      await this.postTexArtifact(docId, entryIndex, entryPathWithoutExt(entry.fullPath), tex);
    } catch (err) {
      this.post({ type: "error", docId, entryIndex, message: errorMessage(err) });
    }
  }

  async readEntryBytes(ctx: DocCtx, entry: PackageEntry): Promise<Uint8Array> {
    const start = ctx.headerSize + entry.offset;
    return new Uint8Array(await ctx.file.slice(start, start + entry.length).arrayBuffer());
  }

  /** 解析/转换/编码并回传 converted 产物 */
  async postTexArtifact(
    docId: number,
    entryIndex: number | null,
    outPath: string,
    tex: Tex,
  ): Promise<void> {
    const ext = getConvertedFormat(tex);
    const result = convertToImage(tex);
    const bytes = await this.encodeResult(result);

    this.post({
      type: "artifact",
      docId,
      entryIndex,
      // 输出名与 C# ConvertToImageAndSave 一致：{去扩展名路径}.{输出扩展名}
      // outPath 必须带扩展名 —— 下载文件名直接取它（回归：曾丢失扩展名被当 txt 保存）
      outPath: sanitizeZipPath(`${outPath}.${ext}`),
      ext,
      mime:
        result.kind === "passthrough"
          ? mipmapMime(result.format as MipmapFormatValue)
          : result.kind === "png"
            ? "image/png"
            : "image/gif",
      source: "converted",
      bytes: toTransferOf(bytes),
      texJson: generateTexJsonText(tex),
      info: summarizeTex(tex),
    });
  }

  /** 原始字节产物（二进制条目 / 兜底） */
  postRawArtifact(docId: number, entryIndex: number | null, outPath: string, bytes: Uint8Array): void {
    this.post({
      type: "artifact",
      docId,
      entryIndex,
      outPath: sanitizeZipPath(outPath),
      ext: entryExtension(outPath).trim().toLowerCase(),
      mime: "application/octet-stream",
      source: "raw",
      bytes: toTransferOf(bytes),
    });
  }

  async encodeResult(result: ConvertResult): Promise<Uint8Array> {
    switch (result.kind) {
      case "png":
        return await this.encoders.encodePng(result.pixels, result.width, result.height);
      case "gif":
        return await this.encoders.encodeGif(result.frames);
      case "passthrough":
        return result.bytes;
    }
  }

  /* ---------------------------------------------------------------- */
  /* ZIP                                                             */
  /* ---------------------------------------------------------------- */

  async handleZip(docId: number, params: ZipParams): Promise<void> {
    const ctx = this.docs.get(docId);
    if (!ctx) {
      this.post({ type: "error", docId, entryIndex: null, message: "文档尚未打开" });
      return;
    }

    const builder = new ZipBuilder();
    let failed = 0;

    try {
      if (ctx.kind === "tex") {
        try {
          await this.zipTexDoc(ctx, builder, params, (done, total) =>
            this.post({ type: "zipProgress", docId, done, total, failed }),
          );
        } catch (err) {
          failed++;
          this.post({
            type: "error",
            docId,
            entryIndex: null,
            message: `TEX 转换失败（已打包原始文件）: ${errorMessage(err)}`,
          });
        }
      } else {
        const entries = ctx.entries ?? [];
        const total = entries.length;

        for (let i = 0; i < total; i++) {
          const entry = entries[i];
          try {
            const entryBytes = await this.readEntryBytes(ctx, entry);

            if (entry.type !== ENTRY_TYPE_TEX || !params.convertTex) {
              // 二进制条目恒原样；tex 关闭转换时原样
              const path = sanitizeZipPath(entry.fullPath);
              if (path !== "") builder.add(path, entryBytes);
            } else {
              // tex 条目：转换输出（含 .tex-json）；失败则回退原文件
              try {
                const tex = parseTex(entryBytes, { decompress: true });
                const base = entryPathWithoutExt(entry.fullPath);
                const ext = getConvertedFormat(tex);
                const result = convertToImage(tex);
                builder.add(`${base}.${ext}`, await this.encodeResult(result));
                if (params.includeTexJson) {
                  builder.add(
                    `${base}.tex-json`,
                    new TextEncoder().encode(generateTexJsonText(tex)),
                  );
                }
              } catch (convErr) {
                failed++;
                this.post({
                  type: "error",
                  docId,
                  entryIndex: i,
                  message: `条目转换失败（已回退原始）: ${entry.fullPath}: ${errorMessage(convErr)}`,
                });
              }
              if (params.includeRawTex) {
                builder.add(sanitizeZipPath(entry.fullPath), entryBytes);
              }
            }
          } catch (err) {
            failed++;
            this.post({
              type: "error",
              docId,
              entryIndex: i,
              message: `条目打包失败: ${entry.fullPath}: ${errorMessage(err)}`,
            });
          }

          this.post({ type: "zipProgress", docId, done: i + 1, total, failed });
        }
      }

      const zip = builder.finish();
      this.post({
        type: "zipDone",
        docId,
        fileCount: builder.fileCount,
        size: zip.byteLength,
        bytes: toTransferOf(zip),
      });
    } catch (err) {
      this.post({
        type: "error",
        docId,
        entryIndex: null,
        message: `打包失败: ${errorMessage(err)}`,
      });
    }
  }

  async zipTexDoc(
    ctx: DocCtx,
    builder: ZipBuilder,
    params: ZipParams,
    onProgress: (done: number, total: number) => void,
  ): Promise<void> {
    onProgress(0, 1);
    const bytes = new Uint8Array(await ctx.file.arrayBuffer());
    try {
      const tex = parseTex(bytes, { decompress: true });
      const base = entryPathWithoutExt(ctx.filename);
      const ext = getConvertedFormat(tex);
      const result = convertToImage(tex);
      builder.add(`${base}.${ext}`, await this.encodeResult(result));
      if (params.includeTexJson) {
        builder.add(`${base}.tex-json`, new TextEncoder().encode(generateTexJsonText(tex)));
      }
      if (params.includeRawTex) {
        builder.add(sanitizeZipPath(ctx.filename), bytes);
      }
    } catch (err) {
      // 转换失败回退原始文件，保证数据不丢
      builder.add(sanitizeZipPath(ctx.filename), bytes);
      throw err;
    }
    onProgress(1, 1);
  }
}

/* ------------------------------------------------------------------ */
/* 顶层自由函数（单元可测）                                             */
/* ------------------------------------------------------------------ */

function errorMessage(err: unknown): string {
  return err instanceof Error ? err.message : String(err);
}

/** 输出文件基名 = 去扩展名的完整路径，与 C# 提取行为一致 */
export function entryPathWithoutExt(fullPath: string): string {
  const ext = entryExtension(fullPath);
  return ext ? fullPath.slice(0, fullPath.length - ext.length) : fullPath;
}

/** 分片指数增长重试解析 PKG 条目表 */
export async function readPkgHeader(file: File | Blob) {
  let cap = 1 << 20;
  for (;;) {
    const buf = new Uint8Array(await file.slice(0, cap).arrayBuffer());
    try {
      return parsePackage(buf);
    } catch (err) {
      if (err instanceof TruncatedDataError && cap < HEADER_MAX && file.size > cap) {
        cap = Math.min(HEADER_MAX, cap * 4);
        continue;
      }
      throw err;
    }
  }
}

function toTransferOf(bytes: Uint8Array): ArrayBuffer {
  return bytes.buffer.slice(bytes.byteOffset, bytes.byteOffset + bytes.byteLength) as ArrayBuffer;
}

/* ------------------------------------------------------------------ */
/* summarize（info 面板摘要）                                          */
/* ------------------------------------------------------------------ */

const TEX_FORMAT_NAMES: Record<number, string> = {
  0: "rgba8888",
  4: "dxt5",
  6: "dxt3",
  7: "dxt1",
  8: "rg88",
  9: "r8",
};

const MIPMAP_FORMAT_NAMES: Record<number, string> = {
  1: "RGBA8888",
  2: "R8",
  3: "RG88",
  4: "CompressedDXT5",
  5: "CompressedDXT3",
  6: "CompressedDXT1",
  7: "VideoMp4",
};

function mipmapFormatName(f: number): string {
  return f >= 1000 ? `Image@${f}` : (MIPMAP_FORMAT_NAMES[f] ?? String(f));
}

export function summarizeTex(tex: Tex): TexSummary {
  const flags = tex.header.flags;
  const flagNames: string[] = [];
  if ((flags & TexFlags.NoInterpolation) !== 0) flagNames.push("nointerpolation");
  if ((flags & TexFlags.ClampUVs) !== 0) flagNames.push("clampuvs");
  if ((flags & TexFlags.IsGif) !== 0) flagNames.push("isgif");
  if ((flags & 8) !== 0) flagNames.push("unk:8");
  if ((flags & 16) !== 0) flagNames.push("unk:16");
  if ((flags & TexFlags.IsVideoTexture) !== 0) flagNames.push("isvideotexture");
  if ((flags & 64) !== 0) flagNames.push("unk:64");
  if ((flags & 128) !== 0) flagNames.push("unk:128");

  const summary: TexSummary = {
    formatName: TEX_FORMAT_NAMES[tex.header.format] ?? String(tex.header.format),
    flags: flagNames,
    textureWidth: tex.header.textureWidth,
    textureHeight: tex.header.textureHeight,
    imageWidth: tex.header.imageWidth,
    imageHeight: tex.header.imageHeight,
    containerMagic: tex.imagesContainer.magic,
    containerVersion: tex.imagesContainer.version,
    imageFormat: tex.imagesContainer.imageFormat,
    imageCount: tex.imagesContainer.images.length,
    images: tex.imagesContainer.images.map((img) => ({
      mipmaps: img.mipmaps.map((m) => ({
        width: m.width,
        height: m.height,
        formatName: mipmapFormatName(m.format),
      })),
    })),
    isGif: texIsGif(tex),
    isVideo: texIsVideoTexture(tex),
  };

  if (tex.frameInfoContainer) {
    summary.frameCount = tex.frameInfoContainer.frames.length;
    summary.gifWidth = tex.frameInfoContainer.gifWidth;
    summary.gifHeight = tex.frameInfoContainer.gifHeight;
  }

  return summary;
}

/**
 * 主线程 ↔ Worker 协议消息定义（仅类型，无运行时代码）。
 */

/** PKG 条目元信息（随 opened 下发） */
export interface EntryMeta {
  index: number;
  fullPath: string;
  offset: number;
  length: number;
  /** 0=binary, 1=tex */
  type: number;
}

/** TEX 信息摘要（info 面板 / 预览侧栏用） */
export interface TexSummary {
  formatName: string;
  flags: string[];
  textureWidth: number;
  textureHeight: number;
  imageWidth: number;
  imageHeight: number;
  containerMagic: string;
  containerVersion: number;
  imageFormat: number;
  imageCount: number;
  images: { mipmaps: { width: number; height: number; formatName: string }[] }[];
  isGif: boolean;
  isVideo: boolean;
  frameCount?: number;
  gifWidth?: number;
  gifHeight?: number;
}

export interface ZipParams {
  /** 把 .tex 转换为图片打包 */
  convertTex: boolean;
  /** 同时保留原始 .tex 文件 */
  includeRawTex: boolean;
  /** 同时生成 .tex-json */
  includeTexJson: boolean;
}

export type WorkerRequest =
  | { type: "open"; docId: number; file: File | Blob; filename: string }
  | { type: "convert"; docId: number; entryIndex: number | null; raw?: boolean }
  | { type: "zip"; docId: number; params: ZipParams };

export type WorkerResponse =
  | {
      type: "opened";
      docId: number;
      kind: "pkg" | "tex";
      filename: string;
      magic: string;
      headerSize: number;
      entryCount: number;
      totalBytes: number;
      entries?: EntryMeta[];
      texSummary?: TexSummary;
    }
  | {
      type: "artifact";
      docId: number;
      entryIndex: number | null;
      outPath: string;
      ext: string;
      mime: string;
      source: "converted" | "raw";
      bytes: ArrayBuffer;
      texJson?: string;
      info?: TexSummary;
    }
  | { type: "zipProgress"; docId: number; done: number; total: number; failed: number }
  | { type: "zipDone"; docId: number; fileCount: number; size: number; bytes: ArrayBuffer }
  | { type: "error"; docId: number; entryIndex: number | null; message: string };

/**
 * PKG 容器解析。
 *
 * 对应 C# RePKG.Application.Package.PackageReader / RePKG.Core.Package.PackageEntry。
 *
 * PKG 布局：
 *   [Magic: int32 长度前缀字符串，最长 32]
 *   [EntryCount: int32]
 *   Info 区（每条）: [FullPath: int32 长度前缀字符串，最长 255] [Offset: int32] [Length: int32]
 *   Data 区：按条目顺序原始字节（HeaderSize 结束后开始）
 *
 * Web 版读取策略：先只解析头部（条目表），条目数据通过 offset/length 惰性
 * 从 File/Blob 睦片读取，避免整包载入内存。
 */
import { ByteReader } from "./binutil";
import { EntryType, type EntryTypeValue } from "./enums";

/** PKG 条目元信息（未载入数据），对应 C# PackageEntry 的持久部分 */
export interface PackageEntry {
  fullPath: string;
  /** 数据区内偏移 */
  offset: number;
  length: number;
  type: EntryTypeValue;
}

/** 解析完成的包结构，对应 C# Package */
export interface Package {
  magic: string;
  /** 条目表长度，即数据区的起点 */
  headerSize: number;
  entries: PackageEntry[];
}

/** 根据完整路径识别条目类型，对应 C# PackageEntryTypeGetter.GetFromFileName */
export function packageEntryTypeFromPath(fullPath: string): EntryTypeValue {
  const dot = fullPath.lastIndexOf(".");
  if (dot <= 0) return EntryType.Binary;
  const ext = fullPath.slice(dot).toLowerCase();
  return ext === ".tex" ? EntryType.Tex : EntryType.Binary;
}

/**
 * 解析 PKG 头部并返回条目表。
 *
 * bytes 只需包含完整头部（条目表）。若输入截断会抛 TruncatedDataError。
 * 与提取目录批量入口时的判断无关 —— 该函数不校验 Magic（与 C# PackageReader 一致）。
 */
export function parsePackage(bytes: Uint8Array): Package {
  const reader = new ByteReader(bytes);
  const packageOffset = 0;

  const magic = reader.readStringI32Size(32);
  const entryCount = reader.readInt32();
  if (entryCount < 0) {
    throw new Error(`条目数量为负数: ${entryCount}`);
  }
  // Java/C# 循环 int 上限防御（> 100 万条在真实壁纸上不存在，直接判别为损坏）
  if (entryCount > 1_000_000) {
    throw new Error(`条目数量过大: ${entryCount}`);
  }

  const entries: PackageEntry[] = new Array(entryCount);
  for (let i = 0; i < entryCount; i++) {
    const fullPath = reader.readStringI32Size(255);
    const off = reader.readInt32();
    const length = reader.readInt32();
    if (off < 0 || length < 0) {
      throw new Error(`条目 offset/length 非法: ${fullPath} (off=${off}, len=${length})`);
    }
    entries[i] = {
      fullPath,
      offset: off,
      length,
      type: packageEntryTypeFromPath(fullPath),
    };
  }

  const dataStart = reader.pos;
  return {
    magic,
    headerSize: dataStart - packageOffset,
    entries,
  };
}

/* ------------------------------------------------------------------ */
/* 路径辅助：镜像 C# Path.GetFileName / GetExtension / GetDirectoryName   */
/* PKG 内部路径分隔符为 '/'，兼容防御 '\'                                */
/* ------------------------------------------------------------------ */

export function entryName(fullPath: string): string {
  return fileNameWithoutExtension(fullPath);
}

export function entryExtension(fullPath: string): string {
  const dot = fullPath.lastIndexOf(".");
  // C# Path.GetExtension：无扩展名返回 ""；"foo." 返回 "." 之前的场景不存在（返回 "."）
  return dot > 0 ? fullPath.slice(dot) : "";
}

export function entryDirectory(fullPath: string): string {
  const s = fullPath.indexOf("/") >= 0 ? "/" : "\\";
  const idx = fullPath.lastIndexOf(s);
  return idx === -1 ? "" : fullPath.slice(0, idx);
}

function fileName(fullPath: string): string {
  const idx = Math.max(fullPath.lastIndexOf("/"), fullPath.lastIndexOf("\\"));
  return idx === -1 ? fullPath : fullPath.slice(idx + 1);
}

function fileNameWithoutExtension(fullPath: string): string {
  const name = fileName(fullPath);
  const dot = name.lastIndexOf(".");
  // 与 C# GetFileNameWithoutExtension 一致：前导点的文件名（如 ".hidden"）不剥离
  return dot > 0 ? name.slice(0, dot) : name;
}

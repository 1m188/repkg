/**
 * 最小 ZIP（store 模式）打包器 —— 零依赖。
 *
 * 仅支持「不压缩直存」；图片/视频本身已是压缩数据，store 反而更快。
 * PKG 条目路径统一改 '/'，写入前必须先经 sanitizeZipPath 清洗（防 zip-slip）。
 */

/** CRC32 查表（IEEE 0xedb88320） */
const CRC_TABLE = (() => {
  const table = new Uint32Array(256);
  for (let n = 0; n < 256; n++) {
    let c = n;
    for (let k = 0; k < 8; k++) {
      c = c & 1 ? 0xedb88320 ^ (c >>> 1) : c >>> 1;
    }
    table[n] = c >>> 0;
  }
  return table;
})();

export function crc32(bytes: Uint8Array): number {
  let c = 0xffffffff;
  for (let i = 0; i < bytes.length; i++) {
    c = CRC_TABLE[(c ^ bytes[i]) & 0xff] ^ (c >>> 8);
  }
  return (c ^ 0xffffffff) >>> 0;
}

const utf8 = new TextEncoder();

/** 单个文件条目：清洗后的路径 + 数据 */
export interface ZipFile {
  path: string;
  data: Uint8Array;
}

/** 十六进制小端写入器 */
class ByteWriter {
  chunks: Uint8Array[] = [];
  private _len = 0;

  get length(): number {
    return this._len;
  }

  push(bytes: Uint8Array): void {
    this.chunks.push(bytes);
    this._len += bytes.byteLength;
  }

  u16(v: number): void {
    const b = new Uint8Array(2);
    b[0] = v & 0xff;
    b[1] = (v >> 8) & 0xff;
    this.push(b);
  }

  u32(v: number): void {
    const b = new Uint8Array(4);
    b[0] = v & 0xff;
    b[1] = (v >>> 8) & 0xff;
    b[2] = (v >>> 16) & 0xff;
    b[3] = (v >>> 24) & 0xff;
    this.push(b);
  }
}

/** DOS 时间（固定为打包时刻，用于展示） */
function dosDateTime(): { date: number; time: number } {
  const d = new Date();
  const year = Math.max(1981, d.getFullYear());
  const date = ((year - 1980) << 9) | ((d.getMonth() + 1) << 5) | d.getDate();
  const time = (d.getHours() << 11) | (d.getMinutes() << 5) | (Math.floor(d.getSeconds() / 2) & 0x1f);
  return { date, time };
}

export class ZipBuilder {
  private files: ZipFile[] = [];
  private usedNames = new Set<string>();

  /** 添加文件；路径先在调用方清理，这里再兜底去重（重名追加 _2/_3…，扩展名保留在后缀） */
  add(path: string, data: Uint8Array): void {
    const clean = sanitizeZipPath(path);
    if (clean === "") return;
    const slash = clean.lastIndexOf("/");
    const dot = clean.lastIndexOf(".");
    const isExt = dot > 0 && dot > slash;
    const base = isExt ? clean.slice(0, dot) : clean;
    const ext = isExt ? clean.slice(dot) : "";
    let name = clean;
    let n = 2;
    while (this.usedNames.has(name.toLowerCase())) {
      name = `${base}_${n}${ext}`;
      n++;
    }
    this.usedNames.add(name.toLowerCase());
    this.files.push({ path: name, data });
  }

  /** 是否包含名为 path 的文件 */
  hasEntry(path: string): boolean {
    return this.usedNames.has(sanitizeZipPath(path).toLowerCase());
  }

  /** 总数据字节数（未含 ZIP 结构开销） */
  get contentLength(): number {
    return this.files.reduce((a, f) => a + f.data.length, 0);
  }

  get fileCount(): number {
    return this.files.length;
  }

  /** 组装最终 ZIP 字节 */
  finish(): Uint8Array {
    const out = new ByteWriter();
    const { date, time } = dosDateTime();
    const central: {
      name: Uint8Array;
      crc: number;
      size: number;
      offset: number;
    }[] = [];

    for (const f of this.files) {
      const name = utf8.encode(f.path);
      const crc = crc32(f.data);
      const offset = out.length;

      out.u32(0x04034b50); // local file header
      out.u16(20); // version needed
      out.u16(0x0800); // flags: UTF-8
      out.u16(0); // method: store
      out.u16(time);
      out.u16(date);
      out.u32(crc);
      out.u32(f.data.length);
      out.u32(f.data.length);
      out.u16(name.length);
      out.u16(0);
      out.push(name);
      out.push(f.data);

      central.push({ name, crc, size: f.data.length, offset });
    }

    const cdStart = out.length;
    for (const c of central) {
      out.u32(0x02014b50);
      out.u16(20); // version made by
      out.u16(20); // version needed
      out.u16(0x0800);
      out.u16(0);
      out.u16(time);
      out.u16(date);
      out.u32(c.crc);
      out.u32(c.size);
      out.u32(c.size);
      out.u16(c.name.length);
      out.u16(0); // extra
      out.u16(0); // comment
      out.u16(0); // disk
      out.u16(0); // internal attrs
      out.u32(0); // external attrs
      out.u32(c.offset);
      out.push(c.name);
    }
    const cdSize = out.length - cdStart;

    out.u32(0x06054b50); // EOCD
    out.u16(0);
    out.u16(0);
    out.u16(central.length);
    out.u16(central.length);
    out.u32(cdSize);
    out.u32(cdStart);
    out.u16(0);

    return concat(out.chunks, out.length);
  }
}

function concat(chunks: Uint8Array[], total: number): Uint8Array {
  const outBuf = new Uint8Array(total);
  let o = 0;
  for (const c of chunks) {
    outBuf.set(c, o);
    o += c.byteLength;
  }
  return outBuf;
}

/**
 * 清洗 ZIP 内部路径，防 zip-slip / 非法字符。
 * '\' 改 '/'；去掉盘符与前导斜杠；折叠多斜杠；逐段过滤 '.'/'..' 与控制字符。
 * 清洗结果可能为空字符串（纯非法路径），调用方应跳过此类文件。
 */
export function sanitizeZipPath(p: string): string {
  const norm = p.replace(/\\/g, "/").replace(/^([a-zA-Z]:)/, "").replace(/\/{2,}/g, "/");
  const parts = norm.split("/");
  const out: string[] = [];
  for (const seg0 of parts) {
    const seg = seg0.replace(/[\u0000-\u001f<>:"|?*]/g, "");
    if (seg === "" || seg === ".") continue;
    if (seg === "..") {
      out.length = Math.max(0, out.length - 1);
      continue;
    }
    out.push(seg.trimEnd());
  }
  const cleaned = out.filter((s) => s !== "").join("/");
  return cleaned.length > 250 ? cleaned.slice(cleaned.length - 250) : cleaned;
}

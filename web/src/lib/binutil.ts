/**
 * 二进制流读取工具与错误类型。
 *
 * 对应 C# RePKG.Application.Extensions（ReadNString / ReadStringI32Size）、
 * BinaryReader 基础读取，以及 RePKG.Application.Exceptions（UnknownMagicException /
 * EnumNotValidException / UnsafeTexException）。
 *
 * 与 C# BinaryReader 一致：全部小端序；ReadNString 读取 null 结尾字符串。
 */

/** 未知魔数错误，对应 C# UnknownMagicException（两种构造形式：完整三元组或仅来源+魔数） */
export class UnknownMagicError extends Error {
  constructor(source: string, property: string, magic?: string) {
    if (magic === undefined) {
      super(`${source}: 未知魔数: ${JSON.stringify(property)}`);
    } else {
      super(`${source}: 未知魔数 (${property}): ${JSON.stringify(magic)}`);
    }
    this.name = "UnknownMagicError";
  }
}

/** 枚举值非法错误，对应 C# EnumNotValidException */
export class EnumNotValidError extends Error {
  constructor(enumName: string, value: number) {
    super(`${enumName} 枚举值非法: ${value}`);
    this.name = "EnumNotValidError";
  }
}

/** 数据不安全/超限错误，对应 C# UnsafeTexException */
export class UnsafeTexError extends Error {
  constructor(message: string) {
    super(message);
    this.name = "UnsafeTexError";
  }
}

/** 流提前结束/长度非法错误（对应 C# BinaryReader 抛出的 EndOfStreamException 等场合） */
export class TruncatedDataError extends Error {
  constructor(message: string) {
    super(message);
    this.name = "TruncatedDataError";
  }
}

const utf8Decoder = new TextDecoder("utf-8", { fatal: false });

/**
 * 基于 Uint8Array/DataView 的顺序二进制读取器（小端序）。
 * 语义与 C# BinaryReader 对齐，增加截断检测（越界抛 TruncatedDataError）。
 */
export class ByteReader {
  private readonly bytes: Uint8Array;
  private readonly view: DataView;
  private position: number;

  constructor(bytes: Uint8Array, offset = 0) {
    this.bytes = bytes;
    this.view = new DataView(bytes.buffer, bytes.byteOffset, bytes.byteLength);
    this.position = offset;
  }

  /** 总字节长度 */
  get length(): number {
    return this.bytes.byteLength;
  }

  /** 当前读取位置 */
  get pos(): number {
    return this.position;
  }

  /** 是否已到达末尾 */
  get eof(): boolean {
    return this.position >= this.bytes.byteLength;
  }

  /** 剩余可读字节数 */
  get remaining(): number {
    return this.bytes.byteLength - this.position;
  }

  /** 校验 n 字节可读，越界抛错 */
  private ensure(n: number): void {
    if (n < 0) throw new TruncatedDataError(`读取长度为负数: ${n}`);
    if (this.position + n > this.bytes.byteLength) {
      throw new TruncatedDataError(
        `数据提前结束: 需要 ${n} 字节 @${this.position}, 仅剩 ${this.remaining}`,
      );
    }
  }

  /** int32（小端） */
  readInt32(): number {
    this.ensure(4);
    const v = this.view.getInt32(this.position, true);
    this.position += 4;
    return v;
  }

  /** uint32（小端），对应 C# ReadUInt32 */
  readUInt32(): number {
    this.ensure(4);
    const v = this.view.getUint32(this.position, true);
    this.position += 4;
    return v;
  }

  /** float32（小端），对应 C# ReadSingle */
  readSingle(): number {
    this.ensure(4);
    const v = this.view.getFloat32(this.position, true);
    this.position += 4;
    return v;
  }

  /** 原始字节块，对应 C# ReadBytes（截断抛错） */
  readBytes(n: number): Uint8Array {
    this.ensure(n);
    const out = this.bytes.subarray(this.position, this.position + n);
    this.position += n;
    return out;
  }

  /**
   * 读取 null 结尾字符串，对应 C# ReadNString(maxLength)。
   * maxLength > 0 时最多读取 maxLength 个字符（不含 \0）。
   */
  readNString(maxLength = -1): string {
    const start = this.position;
    let len = 0;
    while (this.position < this.bytes.byteLength) {
      const c = this.bytes[this.position];
      if (c === 0) {
        this.position += 1; // 跳过 \0
        return utf8Decoder.decode(this.bytes.subarray(start, start + len));
      }
      if (maxLength > -1 && len >= maxLength) {
        return utf8Decoder.decode(this.bytes.subarray(start, start + len));
      }
      this.position += 1;
      len += 1;
    }
    // 流结束仍未看到 \0：C# 会返回已读内容
    return utf8Decoder.decode(this.bytes.subarray(start, this.position));
  }

  /**
   * 读取 int32 长度前缀字符串，对应 C# ReadStringI32Size(maxLength)。
   * maxLength > 0 时把声明的 size 钳到 maxLength（顺从 C# 行为，不做硬报错）。
   */
  readStringI32Size(maxLength = -1): string {
    let size = this.readInt32();
    if (maxLength > -1) size = Math.min(size, maxLength);
    if (size < 0) throw new Error("Size cannot be negative");
    const buf = this.readBytes(size);
    return utf8Decoder.decode(buf);
  }
}

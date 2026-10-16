/**
 * TEX 纹理解析（含 LZ4 / DXT 解压）。
 *
 * 对应 C# RePKG.Application.Texture：
 *   TexReader / TexHeaderReader / TexImageContainerReader / TexImageReader /
 *   TexFrameInfoContainerReader / TexMipmapDecompressor。
 *
 * TEX 布局：
 *   [Magic1: null 结尾字符串] "TEXV0005"
 *   [Magic2: null 结尾字符串] "TEXI0001"
 *   [Header: 7 × int32] format / flags / textureW / textureH / imageW / imageH / unkInt0
 *   [ImageContainer]
 *     [Magic: null 结尾] "TEXB0001"~"TEXB0004"
 *     [ImageCount: int32]
 *     V3: [ImageFormat: int32(FreeImageFormat)]
 *     V4: [ImageFormat: int32] [IsVideoMp4: int32]（仅 MP4 保持 V4，否则降级 V3）
 *     每张 Image: [MipmapCount: int32]
 *       每层 Mipmap（按容器版本）：
 *         V1: [W] [H] [ByteCount] [Bytes]
 *         V2/V3: [W] [H] [IsLZ4] [DecompressedCount] [ByteCount] [Bytes]
 *         V4: [p1=1] [p2=2] [ConditionJson: null 结尾] [p3=1] 后同 V2/V3
 *   [FrameInfoContainer]（仅 TexFlags.IsGif 时）
 *     [Magic: null 结尾] "TEXS0001"~"TEXS0003"
 *     [FrameCount: int32]
 *     V3: [GifWidth: int32] [GifHeight: int32]
 *     每帧：V1 全 int32（imageId, frametime:float32 同位, x, y, width, widthY, heightX, height）
 *          V2/V3 后六个坐标字段改 float32
 */
import { ByteReader, UnknownMagicError, EnumNotValidError, UnsafeTexError } from "./binutil";
import {
  Constants,
  FreeImageFormat,
  MipmapFormat,
  TexFlags,
  TexImageContainerVersion,
  freeImageFormatIsValid,
  getFormatForTex,
  mipmapIsImage,
  texFormatIsValid,
  type FreeImageFormatValue,
  type MipmapFormatValue,
  type TexFormatValue,
} from "./enums";
import { lz4Decompress } from "./lz4";
import { dxtDecompressImage, DXTFlags } from "./dxt";

/** TEX 头部，对应 C# TexHeader */
export interface TexHeader {
  format: TexFormatValue;
  flags: number;
  textureWidth: number;
  textureHeight: number;
  imageWidth: number;
  imageHeight: number;
  unkInt0: number;
}

/** 单层 mipmap，对应 C# TexMipmap */
export interface TexMipmap {
  width: number;
  height: number;
  isLZ4Compressed: boolean;
  decompressedBytesCount: number;
  bytes: Uint8Array;
  format: MipmapFormatValue;
}

/** 单张图像（mipmap 链），对应 C# TexImage */
export interface TexImage {
  mipmaps: TexMipmap[];
}

/** 图像容器，对应 C# TexImageContainer */
export interface TexImageContainer {
  magic: string;
  version: number;
  imageFormat: FreeImageFormatValue;
  images: TexImage[];
}

/** 单帧信息，对应 C# TexFrameInfo（坐标在 TEXS0001 中为整数、0002/0003 中为浮点） */
export interface TexFrameInfo {
  imageId: number;
  frametime: number;
  x: number;
  y: number;
  width: number;
  widthY: number;
  heightX: number;
  height: number;
}

/** 帧信息容器，对应 C# TexFrameInfoContainer */
export interface TexFrameInfoContainer {
  magic: string;
  gifWidth: number;
  gifHeight: number;
  frames: TexFrameInfo[];
}

/** 顶层 TEX 结构，对应 C# Tex */
export interface Tex {
  magic1: string;
  magic2: string;
  header: TexHeader;
  imagesContainer: TexImageContainer;
  frameInfoContainer: TexFrameInfoContainer | null;
}

/* ------------------------------------------------------------------ */
/* 拆分访问器                                                          */
/* ------------------------------------------------------------------ */

export function texFirstImage(tex: Tex): TexImage | null {
  return tex.imagesContainer.images.length > 0 ? tex.imagesContainer.images[0] : null;
}

export function texFirstMipmap(tex: Tex): TexMipmap | null {
  const img = texFirstImage(tex);
  if (img === null || img.mipmaps.length === 0) return null;
  return img.mipmaps[0];
}

export function texHasFlag(tex: Tex, flag: number): boolean {
  return (tex.header.flags & flag) === flag;
}

export function texIsGif(tex: Tex): boolean {
  return texHasFlag(tex, TexFlags.IsGif);
}

export function texIsVideoTexture(tex: Tex): boolean {
  return texHasFlag(tex, TexFlags.IsVideoTexture);
}

/* ------------------------------------------------------------------ */
/* 头部                                                                */
/* ------------------------------------------------------------------ */

/** 对应 C# TexHeaderReader.ReadFrom */
function readTexHeader(reader: ByteReader): TexHeader {
  const format = reader.readInt32();
  const flags = reader.readInt32();
  const header: TexHeader = {
    format: format as TexFormatValue,
    flags,
    textureWidth: reader.readInt32(),
    textureHeight: reader.readInt32(),
    imageWidth: reader.readInt32(),
    imageHeight: reader.readInt32(),
    unkInt0: reader.readUInt32(),
  };

  if (!texFormatIsValid(header.format)) {
    throw new EnumNotValidError("TexFormat", format);
  }

  return header;
}

/* ------------------------------------------------------------------ */
/* mipmap 级                                                           */
/* ------------------------------------------------------------------ */

/** 通用尾部读取：[W] [H] [IsLZ4] [Decompressed] [ByteCount] [Bytes]，对应 V2/V3/V4 后段 */
function readMipmapTail(reader: ByteReader): Omit<TexMipmap, "format"> {
  const width = reader.readInt32();
  const height = reader.readInt32();
  const isLZ4 = reader.readInt32() === 1;
  const decompressedBytesCount = reader.readInt32();
  const bytes = readMipmapBytes(reader);

  return { width, height, isLZ4Compressed: isLZ4, decompressedBytesCount, bytes };
}

/** 读 mipmap 字节块与长度护栏，对应 C# TexImageReader.ReadBytes */
function readMipmapBytes(reader: ByteReader): Uint8Array {
  const byteCount = reader.readInt32();

  if (reader.pos + byteCount > reader.length) {
    throw new UnsafeTexError("Detected invalid mipmap byte count - exceeds stream length");
  }
  if (byteCount > Constants.MaximumMipmapByteCount) {
    throw new UnsafeTexError(
      `Mipmap byte count exceeds maximum size: ${byteCount}/${Constants.MaximumMipmapByteCount}`,
    );
  }
  return reader.readBytes(byteCount);
}

/**
 * 按容器版本读取一整张 Image 的全部 mipmap，对应 C# TexImageReader.ReadFrom。
 * 返回值中的 mipmap 未经过解压（isLZ4Compressed / 压缩 format 保留原始态）。
 */
function readImageMipmaps(reader: ByteReader, containerVersion: number, format: MipmapFormatValue): TexImage {
  const mipmapCount = reader.readInt32();
  if (mipmapCount > Constants.MaximumMipmapCount) {
    throw new UnsafeTexError(
      `Mipmap count exceeds limit: ${mipmapCount}/${Constants.MaximumMipmapCount}`,
    );
  }

  const mipmaps: TexMipmap[] = new Array(mipmapCount);
  for (let i = 0; i < mipmapCount; i++) {
    let raw: Omit<TexMipmap, "format">;
    switch (containerVersion) {
      case TexImageContainerVersion.Version1: {
        const width = reader.readInt32();
        const height = reader.readInt32();
        raw = {
          width,
          height,
          isLZ4Compressed: false,
          decompressedBytesCount: 0,
          bytes: readMipmapBytes(reader),
        };
        break;
      }
      case TexImageContainerVersion.Version2:
      case TexImageContainerVersion.Version3: {
        raw = readMipmapTail(reader);
        break;
      }
      case TexImageContainerVersion.Version4: {
        // 前置参数语义未知（编辑器内置显示用），只接受已知取值，与 C# 一致
        const param1 = reader.readInt32();
        if (param1 !== 1) {
          throw new UnsafeTexError(`ReadMipmapV4 unknow param1 :${param1}`);
        }
        const param2 = reader.readInt32();
        if (param2 !== 2) {
          throw new UnsafeTexError(`ReadMipmapV4 unknow param2 :${param2}`);
        }
        const conditionJson = reader.readNString();
        void conditionJson;

        const param3 = reader.readInt32();
        if (param3 !== 1) {
          throw new UnsafeTexError(`ReadMipmapV4 unknow param3 :${param3}`);
        }
        raw = readMipmapTail(reader);
        break;
      }
      default:
        throw new Error(`Tex image container version: ${containerVersion} is not supported!`);
    }
    mipmaps[i] = { ...raw, format };
  }

  return { mipmaps };
}

/* ------------------------------------------------------------------ */
/* 图像容器                                                            */
/* ------------------------------------------------------------------ */

/** 对应 C# TexImageContainerReader.ReadFrom */
function readImageContainer(reader: ByteReader, texFormat: TexFormatValue): TexImageContainer {
  const magic = reader.readNString(16);

  const imageCount = reader.readInt32();
  if (imageCount > Constants.MaximumImageCount) {
    throw new UnsafeTexError(
      `Image count exceeds limit: ${imageCount}/${Constants.MaximumImageCount}`,
    );
  }

  let version: number;
  let imageFormat: FreeImageFormatValue;

  switch (magic) {
    case "TEXB0001":
    case "TEXB0002": {
      version = Number(magic.slice(4));
      imageFormat = FreeImageFormat.FIF_UNKNOWN;
      break;
    }
    case "TEXB0003": {
      version = 3;
      const fmt = reader.readInt32();
      if (!freeImageFormatIsValid(fmt)) throw new EnumNotValidError("FreeImageFormat", fmt);
      imageFormat = fmt as FreeImageFormatValue;
      break;
    }
    case "TEXB0004": {
      version = 4;
      const fmt = reader.readInt32();
      const isVideoMp4 = reader.readInt32() === 1;
      if (!freeImageFormatIsValid(fmt)) throw new EnumNotValidError("FreeImageFormat", fmt);
      if (fmt === FreeImageFormat.FIF_UNKNOWN && isVideoMp4) {
        imageFormat = FreeImageFormat.FIF_MP4;
      } else {
        imageFormat = fmt as FreeImageFormatValue;
      }
      break;
    }
    default:
      throw new UnknownMagicError("TexImageContainerReader", magic);
  }

  // C# 行为：V4 非 MP4 降级为 V3 处理（mipmap 按 V2/V3 布局读）
  if (version === TexImageContainerVersion.Version4 && imageFormat !== FreeImageFormat.FIF_MP4) {
    version = TexImageContainerVersion.Version3;
  }

  if (!freeImageFormatIsValid(imageFormat)) {
    throw new EnumNotValidError("FreeImageFormat", imageFormat);
  }

  const format = getFormatForTex(imageFormat, texFormat);
  const images: TexImage[] = new Array(imageCount);
  for (let i = 0; i < imageCount; i++) {
    images[i] = readImageMipmaps(reader, version, format);
  }

  return { magic, version, imageFormat, images };
}

/* ------------------------------------------------------------------ */
/* 帧信息容器（GIF）                                                    */
/* ------------------------------------------------------------------ */

/** 对应 C# TexFrameInfoContainerReader.ReadFrom */
function readFrameInfoContainer(reader: ByteReader): TexFrameInfoContainer {
  const container: TexFrameInfoContainer = { magic: "", gifWidth: 0, gifHeight: 0, frames: [] };
  container.magic = reader.readNString(16);

  const frameCount = reader.readInt32();
  if (frameCount > Constants.MaximumFrameCount) {
    throw new UnsafeTexError(
      `Frame count exceeds limit: ${frameCount}/${Constants.MaximumFrameCount}`,
    );
  }

  switch (container.magic) {
    case "TEXS0001": {
      for (let i = 0; i < frameCount; i++) {
        container.frames.push({
          imageId: reader.readInt32(),
          frametime: reader.readSingle(),
          x: reader.readInt32(),
          y: reader.readInt32(),
          width: reader.readInt32(),
          widthY: reader.readInt32(),
          heightX: reader.readInt32(),
          height: reader.readInt32(),
        });
      }
      break;
    }
    case "TEXS0002":
    case "TEXS0003": {
      if (container.magic === "TEXS0003") {
        container.gifWidth = reader.readInt32();
        container.gifHeight = reader.readInt32();
      }
      for (let i = 0; i < frameCount; i++) {
        container.frames.push({
          imageId: reader.readInt32(),
          frametime: reader.readSingle(),
          x: reader.readSingle(),
          y: reader.readSingle(),
          width: reader.readSingle(),
          widthY: reader.readSingle(),
          heightX: reader.readSingle(),
          height: reader.readSingle(),
        });
      }
      break;
    }
    default:
      throw new UnknownMagicError("TexFrameInfoContainerReader", container.magic);
  }

  // TEXS0001/0002 不存 gif 尺寸 —— 从第一帧取（供转换器使用），与 C# 一致
  if (container.gifWidth === 0 || container.gifHeight === 0) {
    const first = container.frames[0];
    if (first) {
      container.gifWidth = Math.trunc(first.width);
      container.gifHeight = Math.trunc(first.height);
    }
  }

  return container;
}

/* ------------------------------------------------------------------ */
/* 解压                                                                */
/* ------------------------------------------------------------------ */

/**
 * 解压单个 mipmap：LZ4 → DXT。
 * 对应 C# TexMipmapDecompressor.DecompressMipmap；处理后格式自动升级为 RGBA8888。
 */
export function decompressMipmap(mipmap: TexMipmap): void {
  if (mipmap.isLZ4Compressed) {
    mipmap.bytes = lz4Decompress(mipmap.bytes, mipmap.decompressedBytesCount);
    mipmap.isLZ4Compressed = false;
  }

  if (mipmapIsImage(mipmap.format)) return;

  switch (mipmap.format) {
    case MipmapFormat.CompressedDXT5:
      mipmap.bytes = dxtDecompressImage(mipmap.width, mipmap.height, mipmap.bytes, DXTFlags.DXT5);
      mipmap.format = MipmapFormat.RGBA8888;
      break;
    case MipmapFormat.CompressedDXT3:
      mipmap.bytes = dxtDecompressImage(mipmap.width, mipmap.height, mipmap.bytes, DXTFlags.DXT3);
      mipmap.format = MipmapFormat.RGBA8888;
      break;
    case MipmapFormat.CompressedDXT1:
      mipmap.bytes = dxtDecompressImage(mipmap.width, mipmap.height, mipmap.bytes, DXTFlags.DXT1);
      mipmap.format = MipmapFormat.RGBA8888;
      break;
  }
}

/* ------------------------------------------------------------------ */
/* 顶层                                                               */
/* ------------------------------------------------------------------ */

/** 解析选项：decompress=false 时保留 LZ4/DXT 压缩态（info 场景用） */
export interface TexParseOptions {
  decompress?: boolean;
}

/**
 * 解析完整 TEX 文件，对应 C# TexReader.ReadFrom（默认读取即解压全部 mipmap）。
 */
export function parseTex(bytes: Uint8Array, options: TexParseOptions = {}): Tex {
  const decompress = options.decompress ?? true;
  const reader = new ByteReader(bytes);

  const magic1 = reader.readNString(16);
  if (magic1 !== "TEXV0005") {
    throw new UnknownMagicError("TexReader", "Magic1", magic1);
  }

  const magic2 = reader.readNString(16);
  if (magic2 !== "TEXI0001") {
    throw new UnknownMagicError("TexReader", "Magic2", magic2);
  }

  const header = readTexHeader(reader);
  const imagesContainer = readImageContainer(reader, header.format);

  if (decompress) {
    for (const image of imagesContainer.images) {
      for (const mipmap of image.mipmaps) {
        decompressMipmap(mipmap);
      }
    }
  }

  let frameInfoContainer: TexFrameInfoContainer | null = null;
  if ((header.flags & TexFlags.IsGif) === TexFlags.IsGif) {
    frameInfoContainer = readFrameInfoContainer(reader);
  }

  return { magic1, magic2, header, imagesContainer, frameInfoContainer };
}

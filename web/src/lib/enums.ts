/**
 * 格式枚举定义。
 *
 * 对应 C# RePKG.Core.Texture.Enums（TexFormat / TexFlags / MipmapFormat /
 * FreeImageFormat / TexImageContainerVersion）与 RePKG.Core.Package.Enums（EntryType）。
 * 全部枚举值与 C# 原版逐一对应，不可改动。
 */

/** TEX 头部纹理格式，对应 C# TexFormat */
export const TexFormat = {
  RGBA8888: 0,
  DXT5: 4,
  DXT3: 6,
  DXT1: 7,
  RG88: 8,
  R8: 9,
} as const;
export type TexFormatValue = (typeof TexFormat)[keyof typeof TexFormat];

/** 判断 TexFormat 枚举值是否有效 */
export function texFormatIsValid(v: number): v is TexFormatValue {
  return (
    v === TexFormat.RGBA8888 ||
    v === TexFormat.DXT5 ||
    v === TexFormat.DXT3 ||
    v === TexFormat.DXT1 ||
    v === TexFormat.RG88 ||
    v === TexFormat.R8
  );
}

/** TEX 头部标志位，对应 C# TexFlags */
export const TexFlags = {
  None: 0,
  NoInterpolation: 1,
  ClampUVs: 2,
  IsGif: 4,
  Unk3: 8,
  Unk4: 16,
  IsVideoTexture: 32,
  Unk6: 64,
  Unk7: 128,
} as const;

/** mipmap 像素格式，对应 C# MipmapFormat */
export const MipmapFormat = {
  Invalid: 0,
  RGBA8888: 1,
  R8: 2,
  RG88: 3,
  CompressedDXT5: 4,
  CompressedDXT3: 5,
  CompressedDXT1: 6,
  VideoMp4: 7,
  ImageBMP: 1000,
  ImageICO: 1001,
  ImageJPEG: 1002,
  ImageJNG: 1003,
  ImageKOALA: 1004,
  ImageLBM: 1005,
  ImageIFF: 1006,
  ImageMNG: 1007,
  ImagePBM: 1008,
  ImagePBMRAW: 1009,
  ImagePCD: 1010,
  ImagePCX: 1011,
  ImagePGM: 1012,
  ImagePGMRAW: 1013,
  ImagePNG: 1014,
  ImagePPM: 1015,
  ImagePPMRAW: 1016,
  ImageRAS: 1017,
  ImageTARGA: 1018,
  ImageTIFF: 1019,
  ImageWBMP: 1020,
  ImagePSD: 1021,
  ImageCUT: 1022,
  ImageXBM: 1023,
  ImageXPM: 1024,
  ImageDDS: 1025,
  ImageGIF: 1026,
  ImageHDR: 1027,
  ImageFAXG3: 1028,
  ImageSGI: 1029,
  ImageEXR: 1030,
  ImageJ2K: 1031,
  ImageJP2: 1032,
  ImagePFM: 1033,
  ImagePICT: 1034,
  ImageRAW: 1035,
} as const;
export type MipmapFormatValue = (typeof MipmapFormat)[keyof typeof MipmapFormat];

/** 图像类格式（>= 1000），对应 C# MipmapFormatExtensions.IsImage */
export function mipmapIsImage(f: MipmapFormatValue | number): boolean {
  return f >= 1000;
}

/** 原始未压缩像素格式（1~3），对应 C# MipmapFormatExtensions.IsRawFormat */
export function mipmapIsRawFormat(f: MipmapFormatValue | number): boolean {
  return f >= 1 && f <= 3;
}

/** 原始 DXT 压缩格式（4~6），对应 C# MipmapFormatExtensions.IsCompressed */
export function mipmapIsCompressed(f: MipmapFormatValue | number): boolean {
  return (
    f === MipmapFormat.CompressedDXT5 ||
    f === MipmapFormat.CompressedDXT3 ||
    f === MipmapFormat.CompressedDXT1
  );
}

/** TEXB 图像容器版本（TEX 版本 1~4），对应 C# TexImageContainerVersion */
export const TexImageContainerVersion = {
  Version1: 1,
  Version2: 2,
  Version3: 3,
  Version4: 4,
} as const;

/** FreeImage 格式枚举（容器 V3/V4 使用），对应 C# FreeImageFormat */
export const FreeImageFormat = {
  FIF_UNKNOWN: -1,
  FIF_BMP: 0,
  FIF_ICO: 1,
  FIF_JPEG: 2,
  FIF_JNG: 3,
  FIF_KOALA: 4,
  FIF_LBM: 5,
  FIF_IFF: 5,
  FIF_MNG: 6,
  FIF_PBM: 7,
  FIF_PBMRAW: 8,
  FIF_PCD: 9,
  FIF_PCX: 10,
  FIF_PGM: 11,
  FIF_PGMRAW: 12,
  FIF_PNG: 13,
  FIF_PPM: 14,
  FIF_PPMRAW: 15,
  FIF_RAS: 16,
  FIF_TARGA: 17,
  FIF_TIFF: 18,
  FIF_WBMP: 19,
  FIF_PSD: 20,
  FIF_CUT: 21,
  FIF_XBM: 22,
  FIF_XPM: 23,
  FIF_DDS: 24,
  FIF_GIF: 25,
  FIF_HDR: 26,
  FIF_FAXG3: 27,
  FIF_SGI: 28,
  FIF_EXR: 29,
  FIF_J2K: 30,
  FIF_JP2: 31,
  FIF_PFM: 32,
  FIF_PICT: 33,
  FIF_RAW: 34,
  FIF_MP4: 35,
} as const;
export type FreeImageFormatValue = (typeof FreeImageFormat)[keyof typeof FreeImageFormat];

/** 判断 FreeImageFormat 枚举值是否有效（在 -1..35 内即合法） */
export function freeImageFormatIsValid(v: number): v is FreeImageFormatValue {
  return v >= -1 && v <= 35;
}

/** FreeImageFormat → MipmapFormat 映射，对应 C# TexMipmapFormatGetter.FreeImageFormatToMipmapFormat。
 *  注：C# 枚举里 FIF_LBM 与 FIF_IFF 同为 5（别名），映射表取首个有效分支 ImageLBM。 */
const FIF_TO_MIPMAP: Record<number, MipmapFormatValue> = {
  [FreeImageFormat.FIF_BMP]: MipmapFormat.ImageBMP,
  [FreeImageFormat.FIF_ICO]: MipmapFormat.ImageICO,
  [FreeImageFormat.FIF_JPEG]: MipmapFormat.ImageJPEG,
  [FreeImageFormat.FIF_JNG]: MipmapFormat.ImageJNG,
  [FreeImageFormat.FIF_KOALA]: MipmapFormat.ImageKOALA,
  [FreeImageFormat.FIF_LBM]: MipmapFormat.ImageLBM,
  [FreeImageFormat.FIF_MNG]: MipmapFormat.ImageMNG,
  [FreeImageFormat.FIF_PBM]: MipmapFormat.ImagePBM,
  [FreeImageFormat.FIF_PBMRAW]: MipmapFormat.ImagePBMRAW,
  [FreeImageFormat.FIF_PCD]: MipmapFormat.ImagePCD,
  [FreeImageFormat.FIF_PCX]: MipmapFormat.ImagePCX,
  [FreeImageFormat.FIF_PGM]: MipmapFormat.ImagePGM,
  [FreeImageFormat.FIF_PGMRAW]: MipmapFormat.ImagePGMRAW,
  [FreeImageFormat.FIF_PNG]: MipmapFormat.ImagePNG,
  [FreeImageFormat.FIF_PPM]: MipmapFormat.ImagePPM,
  [FreeImageFormat.FIF_PPMRAW]: MipmapFormat.ImagePPMRAW,
  [FreeImageFormat.FIF_RAS]: MipmapFormat.ImageRAS,
  [FreeImageFormat.FIF_TARGA]: MipmapFormat.ImageTARGA,
  [FreeImageFormat.FIF_TIFF]: MipmapFormat.ImageTIFF,
  [FreeImageFormat.FIF_WBMP]: MipmapFormat.ImageWBMP,
  [FreeImageFormat.FIF_PSD]: MipmapFormat.ImagePSD,
  [FreeImageFormat.FIF_CUT]: MipmapFormat.ImageCUT,
  [FreeImageFormat.FIF_XBM]: MipmapFormat.ImageXBM,
  [FreeImageFormat.FIF_XPM]: MipmapFormat.ImageXPM,
  [FreeImageFormat.FIF_DDS]: MipmapFormat.ImageDDS,
  [FreeImageFormat.FIF_GIF]: MipmapFormat.ImageGIF,
  [FreeImageFormat.FIF_HDR]: MipmapFormat.ImageHDR,
  [FreeImageFormat.FIF_FAXG3]: MipmapFormat.ImageFAXG3,
  [FreeImageFormat.FIF_SGI]: MipmapFormat.ImageSGI,
  [FreeImageFormat.FIF_EXR]: MipmapFormat.ImageEXR,
  [FreeImageFormat.FIF_J2K]: MipmapFormat.ImageJ2K,
  [FreeImageFormat.FIF_JP2]: MipmapFormat.ImageJP2,
  [FreeImageFormat.FIF_PFM]: MipmapFormat.ImagePFM,
  [FreeImageFormat.FIF_PICT]: MipmapFormat.ImagePICT,
  [FreeImageFormat.FIF_RAW]: MipmapFormat.ImageRAW,
  [FreeImageFormat.FIF_MP4]: MipmapFormat.VideoMp4,
};

/**
 * 计算某个 TEX 的 mipmap 格式，对应 C# TexMipmapFormatGetter.GetFormatForTex(imageFormat, texFormat)。
 * 容器 V3/V4 已带 ImageFormat 时优先使用；否则按头部 TexFormat 映射。
 */
export function getFormatForTex(
  imageFormat: FreeImageFormatValue,
  texFormat: TexFormatValue,
): MipmapFormatValue {
  if (imageFormat !== FreeImageFormat.FIF_UNKNOWN) {
    if (imageFormat < 0) {
      throw new Error(`Can't convert '${imageFormat}' to 'MipmapFormat'`);
    }
    const m = FIF_TO_MIPMAP[imageFormat];
    if (m === undefined) throw new Error(`FreeImageFormat 溢出: ${imageFormat}`);
    return m;
  }
  switch (texFormat) {
    case TexFormat.RGBA8888:
      return MipmapFormat.RGBA8888;
    case TexFormat.DXT5:
      return MipmapFormat.CompressedDXT5;
    case TexFormat.DXT3:
      return MipmapFormat.CompressedDXT3;
    case TexFormat.DXT1:
      return MipmapFormat.CompressedDXT1;
    case TexFormat.R8:
      return MipmapFormat.R8;
    case TexFormat.RG88:
      return MipmapFormat.RG88;
    default:
      throw new Error(`TexFormat 无效: ${texFormat}`);
  }
}

/** 图像类 / 视频格式的文件扩展名，对应 C# MipmapFormatExtensions.GetFileExtension */
const MIPMAP_EXT: Record<number, string> = {
  [MipmapFormat.ImageBMP]: "bmp",
  [MipmapFormat.ImageICO]: "ico",
  [MipmapFormat.ImageJPEG]: "jpg",
  [MipmapFormat.ImageJNG]: "jng",
  [MipmapFormat.ImageKOALA]: "koa",
  [MipmapFormat.ImageLBM]: "lbm",
  [MipmapFormat.ImageIFF]: "iff",
  [MipmapFormat.ImageMNG]: "mng",
  [MipmapFormat.ImagePBM]: "pbm",
  [MipmapFormat.ImagePBMRAW]: "pbm",
  [MipmapFormat.ImagePCD]: "pcd",
  [MipmapFormat.ImagePCX]: "pcx",
  [MipmapFormat.ImagePGM]: "pgm",
  [MipmapFormat.ImagePGMRAW]: "pgm",
  [MipmapFormat.ImagePNG]: "png",
  [MipmapFormat.ImagePPM]: "ppm",
  [MipmapFormat.ImagePPMRAW]: "ppm",
  [MipmapFormat.ImageRAS]: "ras",
  [MipmapFormat.ImageTARGA]: "tga",
  [MipmapFormat.ImageTIFF]: "tif",
  [MipmapFormat.ImageWBMP]: "wbmp",
  [MipmapFormat.ImagePSD]: "psd",
  [MipmapFormat.ImageCUT]: "cut",
  [MipmapFormat.ImageXBM]: "xbm",
  [MipmapFormat.ImageXPM]: "xpm",
  [MipmapFormat.ImageDDS]: "dds",
  [MipmapFormat.ImageGIF]: "gif",
  [MipmapFormat.ImageHDR]: "hdr",
  [MipmapFormat.ImageFAXG3]: "g3",
  [MipmapFormat.ImageSGI]: "sgi",
  [MipmapFormat.ImageEXR]: "exr",
  [MipmapFormat.ImageJ2K]: "j2k",
  [MipmapFormat.ImageJP2]: "jp2",
  [MipmapFormat.ImagePFM]: "pfm",
  [MipmapFormat.ImagePICT]: "pict",
  [MipmapFormat.ImageRAW]: "raw",
  [MipmapFormat.VideoMp4]: "mp4",
};

/** 返回图像/视频格式对应的扩展名（与 C# 一致，非法格式抛错） */
export function mipmapFileExtension(format: MipmapFormatValue | number): string {
  const ext = MIPMAP_EXT[format];
  if (ext === undefined) throw new Error(`mipmap 格式没有扩展名: ${format}`);
  return ext;
}

/** 常见图像/视频格式 → 浏览器 MIME 类型（仅用于展示与下载） */
const MIPMAP_MIME: Record<number, string> = {
  [MipmapFormat.ImageBMP]: "image/bmp",
  [MipmapFormat.ImageJPEG]: "image/jpeg",
  [MipmapFormat.ImagePNG]: "image/png",
  [MipmapFormat.ImageGIF]: "image/gif",
  [MipmapFormat.ImageTIFF]: "image/tiff",
  [MipmapFormat.ImageTARGA]: "image/x-tga",
  [MipmapFormat.ImageDDS]: "image/vnd-ms.dds",
  [MipmapFormat.VideoMp4]: "video/mp4",
  [MipmapFormat.ImageHDR]: "image/x-hdr",
  [MipmapFormat.ImageEXR]: "image/x-exr",
};

/** 浏览器展示用的 MIME；未知格式返回 application/octet-stream（仍可下载） */
export function mipmapMime(format: MipmapFormatValue | number): string {
  return MIPMAP_MIME[format] ?? "application/octet-stream";
}

/** PKG 条目类型，对应 C# EntryType */
export const EntryType = {
  Binary: 0,
  Tex: 1,
} as const;
export type EntryTypeValue = (typeof EntryType)[keyof typeof EntryType];

/** 安全护栏常量，对应 C# RePKG.Application.Constants */
export const Constants = {
  MaximumFrameCount: 100_000,
  MaximumImageCount: 100,
  MaximumMipmapCount: 32,
  MaximumMipmapByteCount: 250_000_000, // 250 MB
} as const;

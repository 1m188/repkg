// MipmapFormat、TexFormat、FreeImageFormat、TexFlags、DXTFlags 等枚举定义。
// 与 C# 原版 RePKG.Core/Texture/Enums/ 对应。

/// Mipmap 像素格式枚举，数值严格对齐 C# 原版。
/// 1-3 为原始像素格式，4-6 为压缩格式，7 为视频，1000+ 为图片格式。
#[derive(Debug, Clone, Copy, PartialEq, Eq)]
pub enum MipmapFormat {
    /// 无效格式。
    Invalid = 0,
    /// 32 位 RGBA 原始像素（4 字节/像素）。
    RGBA8888 = 1,
    /// 8 位灰度原始像素（1 字节/像素）。
    R8 = 2,
    /// 16 位双通道原始像素（2 字节/像素），R→Alpha, G→RGB。
    RG88 = 3,
    /// DXT5 压缩格式（BC3）。
    CompressedDXT5 = 4,
    /// DXT3 压缩格式（BC2）。
    CompressedDXT3 = 5,
    /// DXT1 压缩格式（BC1）。
    CompressedDXT1 = 6,
    /// MP4 视频格式。
    VideoMp4 = 7,
    /// BMP 图片格式。
    ImageBMP = 1000,
    /// ICO 图标格式。
    ImageICO = 1001,
    /// JPEG 图片格式。
    ImageJPEG = 1002,
    /// JNG 图片格式。
    ImageJNG = 1003,
    /// KOALA 图片格式。
    ImageKOALA = 1004,
    /// LBM 图片格式。
    ImageLBM = 1005,
    /// IFF 图片格式。
    ImageIFF = 1006,
    /// MNG 图片格式。
    ImageMNG = 1007,
    /// PBM 图片格式。
    ImagePBM = 1008,
    /// PBMRAW 图片格式。
    ImagePBMRAW = 1009,
    /// PCD 图片格式。
    ImagePCD = 1010,
    /// PCX 图片格式。
    ImagePCX = 1011,
    /// PGM 图片格式。
    ImagePGM = 1012,
    /// PGMRAW 图片格式。
    ImagePGMRAW = 1013,
    /// PNG 图片格式。
    ImagePNG = 1014,
    /// PPM 图片格式。
    ImagePPM = 1015,
    /// PPMRAW 图片格式。
    ImagePPMRAW = 1016,
    /// RAS 图片格式。
    ImageRAS = 1017,
    /// TARGA 图片格式。
    ImageTARGA = 1018,
    /// TIFF 图片格式。
    ImageTIFF = 1019,
    /// WBMP 图片格式。
    ImageWBMP = 1020,
    /// PSD 图片格式。
    ImagePSD = 1021,
    /// CUT 图片格式。
    ImageCUT = 1022,
    /// XBM 图片格式。
    ImageXBM = 1023,
    /// XPM 图片格式。
    ImageXPM = 1024,
    /// DDS 图片格式。
    ImageDDS = 1025,
    /// GIF 图片格式。
    ImageGIF = 1026,
    /// HDR 图片格式。
    ImageHDR = 1027,
    /// FAXG3 图片格式。
    ImageFAXG3 = 1028,
    /// SGI 图片格式。
    ImageSGI = 1029,
    /// EXR 图片格式。
    ImageEXR = 1030,
    /// J2K 图片格式。
    ImageJ2K = 1031,
    /// JP2 图片格式。
    ImageJP2 = 1032,
    /// PFM 图片格式。
    ImagePFM = 1033,
    /// PICT 图片格式。
    ImagePICT = 1034,
    /// RAW 图片格式。
    ImageRAW = 1035,
}

impl MipmapFormat {
    /// 判断是否为原始像素格式（RGBA8888、R8、RG88）。
    pub fn is_raw_format(self) -> bool {
        matches!(self, Self::RGBA8888 | Self::R8 | Self::RG88)
    }

    /// 判断是否为压缩格式（DXT1/3/5）。
    pub fn is_compressed(self) -> bool {
        matches!(
            self,
            Self::CompressedDXT1 | Self::CompressedDXT3 | Self::CompressedDXT5
        )
    }

    /// 判断是否为图片格式（Image* 系列）。
    pub fn is_image(self) -> bool {
        (self as i32) >= 1000
    }

    /// 判断是否为视频格式。
    pub fn is_video(self) -> bool {
        matches!(self, Self::VideoMp4)
    }

    /// 获取对应的文件扩展名（不含点号）。
    pub fn get_file_extension(self) -> &'static str {
        match self {
            Self::ImageBMP => "bmp",
            Self::ImageICO => "ico",
            Self::ImageJPEG => "jpg",
            Self::ImageJNG => "jng",
            Self::ImageKOALA => "koa",
            Self::ImageLBM | Self::ImageIFF => "iff",
            Self::ImageMNG => "mng",
            Self::ImagePBM | Self::ImagePBMRAW => "pbm",
            Self::ImagePCD => "pcd",
            Self::ImagePCX => "pcx",
            Self::ImagePGM | Self::ImagePGMRAW => "pgm",
            Self::ImagePNG => "png",
            Self::ImagePPM | Self::ImagePPMRAW => "ppm",
            Self::ImageRAS => "ras",
            Self::ImageTARGA => "tga",
            Self::ImageTIFF => "tiff",
            Self::ImageWBMP => "wbmp",
            Self::ImagePSD => "psd",
            Self::ImageCUT => "cut",
            Self::ImageXBM => "xbm",
            Self::ImageXPM => "xpm",
            Self::ImageDDS => "dds",
            Self::ImageGIF => "gif",
            Self::ImageHDR => "hdr",
            Self::ImageFAXG3 => "fax",
            Self::ImageSGI => "sgi",
            Self::ImageEXR => "exr",
            Self::ImageJ2K => "j2k",
            Self::ImageJP2 => "jp2",
            Self::ImagePFM => "pfm",
            Self::ImagePICT => "pict",
            Self::ImageRAW => "raw",
            Self::VideoMp4 => "mp4",
            Self::RGBA8888 | Self::R8 | Self::RG88 => "png",
            _ => "dat",
        }
    }
}

impl TryFrom<i32> for MipmapFormat {
    type Error = crate::error::EnumNotValidError;

    fn try_from(value: i32) -> Result<Self, Self::Error> {
        match value {
            0 => Ok(Self::Invalid),
            1 => Ok(Self::RGBA8888),
            2 => Ok(Self::R8),
            3 => Ok(Self::RG88),
            4 => Ok(Self::CompressedDXT5),
            5 => Ok(Self::CompressedDXT3),
            6 => Ok(Self::CompressedDXT1),
            7 => Ok(Self::VideoMp4),
            1000 => Ok(Self::ImageBMP),
            1001 => Ok(Self::ImageICO),
            1002 => Ok(Self::ImageJPEG),
            1003 => Ok(Self::ImageJNG),
            1004 => Ok(Self::ImageKOALA),
            1005 => Ok(Self::ImageLBM),
            1006 => Ok(Self::ImageIFF),
            1007 => Ok(Self::ImageMNG),
            1008 => Ok(Self::ImagePBM),
            1009 => Ok(Self::ImagePBMRAW),
            1010 => Ok(Self::ImagePCD),
            1011 => Ok(Self::ImagePCX),
            1012 => Ok(Self::ImagePGM),
            1013 => Ok(Self::ImagePGMRAW),
            1014 => Ok(Self::ImagePNG),
            1015 => Ok(Self::ImagePPM),
            1016 => Ok(Self::ImagePPMRAW),
            1017 => Ok(Self::ImageRAS),
            1018 => Ok(Self::ImageTARGA),
            1019 => Ok(Self::ImageTIFF),
            1020 => Ok(Self::ImageWBMP),
            1021 => Ok(Self::ImagePSD),
            1022 => Ok(Self::ImageCUT),
            1023 => Ok(Self::ImageXBM),
            1024 => Ok(Self::ImageXPM),
            1025 => Ok(Self::ImageDDS),
            1026 => Ok(Self::ImageGIF),
            1027 => Ok(Self::ImageHDR),
            1028 => Ok(Self::ImageFAXG3),
            1029 => Ok(Self::ImageSGI),
            1030 => Ok(Self::ImageEXR),
            1031 => Ok(Self::ImageJ2K),
            1032 => Ok(Self::ImageJP2),
            1033 => Ok(Self::ImagePFM),
            1034 => Ok(Self::ImagePICT),
            1035 => Ok(Self::ImageRAW),
            _ => Err(crate::error::EnumNotValidError {
                value,
                type_name: "MipmapFormat".to_string(),
            }),
        }
    }
}

/// TEX 头部纹理格式枚举，与 C# 原版 TexFormat 严格对齐。
#[derive(Debug, Clone, Copy, PartialEq, Eq)]
pub enum TexFormat {
    /// 32 位 RGBA8888。
    RGBA8888 = 0,
    /// BC3/DXT5 压缩。
    DXT5 = 4,
    /// BC2/DXT3 压缩。
    DXT3 = 6,
    /// BC1/DXT1 压缩。
    DXT1 = 7,
    /// 16 位双通道 RG88。
    RG88 = 8,
    /// 8 位单通道 R8。
    R8 = 9,
}

impl TexFormat {
    /// 判断给定整数是否为有效的 TexFormat 值。
    pub fn is_valid(value: i32) -> bool {
        matches!(value, 0 | 4 | 6 | 7 | 8 | 9)
    }

    /// 将 TexFormat 映射为 MipmapFormat（用于 V1/V2 容器回退）。
    pub fn to_mipmap_format(self) -> MipmapFormat {
        match self {
            Self::RGBA8888 => MipmapFormat::RGBA8888,
            Self::DXT5 => MipmapFormat::CompressedDXT5,
            Self::DXT3 => MipmapFormat::CompressedDXT3,
            Self::DXT1 => MipmapFormat::CompressedDXT1,
            Self::RG88 => MipmapFormat::RG88,
            Self::R8 => MipmapFormat::R8,
        }
    }
}

impl TryFrom<i32> for TexFormat {
    type Error = crate::error::EnumNotValidError;

    fn try_from(value: i32) -> Result<Self, Self::Error> {
        match value {
            0 => Ok(Self::RGBA8888),
            4 => Ok(Self::DXT5),
            6 => Ok(Self::DXT3),
            7 => Ok(Self::DXT1),
            8 => Ok(Self::RG88),
            9 => Ok(Self::R8),
            _ => Err(crate::error::EnumNotValidError {
                value,
                type_name: "TexFormat".to_string(),
            }),
        }
    }
}

/// FreeImage 格式枚举（全部 37 个值），与 C# 原版 FreeImageFormat 严格对齐。
/// 变体名称与 C# 保持一致，使用 `FIF_` 前缀。
#[derive(Debug, Clone, Copy, PartialEq, Eq)]
#[allow(non_camel_case_types)]
pub enum FreeImageFormat {
    /// 未知格式。
    FIF_UNKNOWN = -1,
    /// Windows BMP。
    FIF_BMP = 0,
    /// Windows 图标。
    FIF_ICO = 1,
    /// JPEG。
    FIF_JPEG = 2,
    /// JPEG Network Graphics。
    FIF_JNG = 3,
    /// Commodore KOALA。
    FIF_KOALA = 4,
    /// Amiga IFF（与 LBM 同值 5）。
    FIF_LBM = 5,
    /// Multiple Network Graphics。
    FIF_MNG = 6,
    /// Portable Bitmap (ASCII)。
    FIF_PBM = 7,
    /// Portable Bitmap (RAW)。
    FIF_PBMRAW = 8,
    /// Kodak PhotoCD。
    FIF_PCD = 9,
    /// Zsoft PCX。
    FIF_PCX = 10,
    /// Portable Graymap (ASCII)。
    FIF_PGM = 11,
    /// Portable Graymap (RAW)。
    FIF_PGMRAW = 12,
    /// Portable Network Graphics。
    FIF_PNG = 13,
    /// Portable Pixelmap (ASCII)。
    FIF_PPM = 14,
    /// Portable Pixelmap (RAW)。
    FIF_PPMRAW = 15,
    /// Sun Raster。
    FIF_RAS = 16,
    /// Truevision TARGA。
    FIF_TARGA = 17,
    /// Tagged Image File Format。
    FIF_TIFF = 18,
    /// Wireless Bitmap。
    FIF_WBMP = 19,
    /// Photoshop PSD。
    FIF_PSD = 20,
    /// Dr. Halo CUT。
    FIF_CUT = 21,
    /// X11 Bitmap。
    FIF_XBM = 22,
    /// X11 Pixmap。
    FIF_XPM = 23,
    /// DirectDraw Surface。
    FIF_DDS = 24,
    /// Graphics Interchange Format。
    FIF_GIF = 25,
    /// Radiance HDR。
    FIF_HDR = 26,
    /// Raw Fax G3。
    FIF_FAXG3 = 27,
    /// Silicon Graphics Image。
    FIF_SGI = 28,
    /// OpenEXR。
    FIF_EXR = 29,
    /// JPEG 2000 codestream。
    FIF_J2K = 30,
    /// JPEG 2000 file format。
    FIF_JP2 = 31,
    /// Portable Floatmap。
    FIF_PFM = 32,
    /// Macintosh PICT。
    FIF_PICT = 33,
    /// Camera RAW。
    FIF_RAW = 34,
    /// MP4 视频。
    FIF_MP4 = 35,
}

impl FreeImageFormat {
    /// 判断给定整数是否为有效的 FreeImageFormat 值。
    pub fn is_valid(value: i32) -> bool {
        (-1..=35).contains(&value)
    }
}

impl TryFrom<i32> for FreeImageFormat {
    type Error = crate::error::EnumNotValidError;

    fn try_from(value: i32) -> Result<Self, Self::Error> {
        match value {
            -1 => Ok(Self::FIF_UNKNOWN),
            0 => Ok(Self::FIF_BMP),
            1 => Ok(Self::FIF_ICO),
            2 => Ok(Self::FIF_JPEG),
            3 => Ok(Self::FIF_JNG),
            4 => Ok(Self::FIF_KOALA),
            5 => Ok(Self::FIF_LBM),
            6 => Ok(Self::FIF_MNG),
            7 => Ok(Self::FIF_PBM),
            8 => Ok(Self::FIF_PBMRAW),
            9 => Ok(Self::FIF_PCD),
            10 => Ok(Self::FIF_PCX),
            11 => Ok(Self::FIF_PGM),
            12 => Ok(Self::FIF_PGMRAW),
            13 => Ok(Self::FIF_PNG),
            14 => Ok(Self::FIF_PPM),
            15 => Ok(Self::FIF_PPMRAW),
            16 => Ok(Self::FIF_RAS),
            17 => Ok(Self::FIF_TARGA),
            18 => Ok(Self::FIF_TIFF),
            19 => Ok(Self::FIF_WBMP),
            20 => Ok(Self::FIF_PSD),
            21 => Ok(Self::FIF_CUT),
            22 => Ok(Self::FIF_XBM),
            23 => Ok(Self::FIF_XPM),
            24 => Ok(Self::FIF_DDS),
            25 => Ok(Self::FIF_GIF),
            26 => Ok(Self::FIF_HDR),
            27 => Ok(Self::FIF_FAXG3),
            28 => Ok(Self::FIF_SGI),
            29 => Ok(Self::FIF_EXR),
            30 => Ok(Self::FIF_J2K),
            31 => Ok(Self::FIF_JP2),
            32 => Ok(Self::FIF_PFM),
            33 => Ok(Self::FIF_PICT),
            34 => Ok(Self::FIF_RAW),
            35 => Ok(Self::FIF_MP4),
            _ => Err(crate::error::EnumNotValidError {
                value,
                type_name: "FreeImageFormat".to_string(),
            }),
        }
    }
}

/// DXT 压缩标志位，与 C# 原版 DXTFlags 对应。
#[derive(Debug, Clone, Copy, PartialEq, Eq)]
pub struct DxtFlags(pub u32);

impl DxtFlags {
    /// DXT1 格式标志。
    pub const DXT1: Self = Self(1);
    /// DXT3 格式标志。
    pub const DXT3: Self = Self(2);
    /// DXT5 格式标志。
    pub const DXT5: Self = Self(4);

    pub fn is_empty(self) -> bool {
        self.0 == 0
    }

    pub fn contains(self, other: Self) -> bool {
        (self.0 & other.0) != 0
    }
}

/// TEX 标志位常量，与 C# 原版 TexFlags 对应。
pub mod tex_flags {
    /// 无标志。
    pub const NONE: u32 = 0;
    /// 禁止纹理插值。
    pub const NO_INTERPOLATION: u32 = 1 << 0;
    /// 钳制 UV 坐标。
    pub const CLAMP_UVS: u32 = 1 << 1;
    /// 为 GIF 动画纹理。
    pub const IS_GIF: u32 = 1 << 2;
    /// 未知标志 3。
    pub const UNK3: u32 = 1 << 3;
    /// 未知标志 4。
    pub const UNK4: u32 = 1 << 4;
    /// 为视频纹理。
    pub const IS_VIDEO_TEXTURE: u32 = 1 << 5;
    /// 未知标志 6。
    pub const UNK6: u32 = 1 << 6;
    /// 未知标志 7。
    pub const UNK7: u32 = 1 << 7;
}

#[cfg(test)]
#[allow(non_snake_case)]
mod tests {
    use super::*;

    // ========== MipmapFormat 测试 ==========

    #[test]
    fn 测试枚举值对齐_CSharp() {
        assert_eq!(MipmapFormat::Invalid as i32, 0);
        assert_eq!(MipmapFormat::RGBA8888 as i32, 1);
        assert_eq!(MipmapFormat::R8 as i32, 2);
        assert_eq!(MipmapFormat::RG88 as i32, 3);
        assert_eq!(MipmapFormat::CompressedDXT5 as i32, 4);
        assert_eq!(MipmapFormat::CompressedDXT3 as i32, 5);
        assert_eq!(MipmapFormat::CompressedDXT1 as i32, 6);
        assert_eq!(MipmapFormat::VideoMp4 as i32, 7);
        assert_eq!(MipmapFormat::ImageBMP as i32, 1000);
        assert_eq!(MipmapFormat::ImageRAW as i32, 1035);
    }

    #[test]
    fn 测试原始格式判断() {
        assert!(MipmapFormat::RGBA8888.is_raw_format());
        assert!(MipmapFormat::R8.is_raw_format());
        assert!(MipmapFormat::RG88.is_raw_format());
        assert!(!MipmapFormat::CompressedDXT1.is_raw_format());
        assert!(!MipmapFormat::ImagePNG.is_raw_format());
    }

    #[test]
    fn 测试压缩格式判断() {
        assert!(MipmapFormat::CompressedDXT1.is_compressed());
        assert!(MipmapFormat::CompressedDXT3.is_compressed());
        assert!(MipmapFormat::CompressedDXT5.is_compressed());
        assert!(!MipmapFormat::RGBA8888.is_compressed());
    }

    #[test]
    fn 测试图片格式判断() {
        assert!(MipmapFormat::ImagePNG.is_image());
        assert!(MipmapFormat::ImageBMP.is_image());
        assert!(!MipmapFormat::RGBA8888.is_image());
        assert!(!MipmapFormat::VideoMp4.is_image());
    }

    #[test]
    fn 测试文件扩展名获取() {
        assert_eq!(MipmapFormat::ImagePNG.get_file_extension(), "png");
        assert_eq!(MipmapFormat::ImageBMP.get_file_extension(), "bmp");
        assert_eq!(MipmapFormat::RGBA8888.get_file_extension(), "png");
        assert_eq!(MipmapFormat::VideoMp4.get_file_extension(), "mp4");
    }

    #[test]
    fn 测试从_i32_转换_MipmapFormat() {
        assert_eq!(
            MipmapFormat::try_from(6).unwrap(),
            MipmapFormat::CompressedDXT1
        );
        assert_eq!(MipmapFormat::try_from(1).unwrap(), MipmapFormat::RGBA8888);
        assert!(MipmapFormat::try_from(999).is_err());
    }

    // ========== TexFormat 测试 ==========

    #[test]
    fn 测试_TexFormat_枚举值对齐_CSharp() {
        assert_eq!(TexFormat::RGBA8888 as i32, 0);
        assert_eq!(TexFormat::DXT5 as i32, 4);
        assert_eq!(TexFormat::DXT3 as i32, 6);
        assert_eq!(TexFormat::DXT1 as i32, 7);
        assert_eq!(TexFormat::RG88 as i32, 8);
        assert_eq!(TexFormat::R8 as i32, 9);
    }

    #[test]
    fn 测试_TexFormat_有效性验证() {
        assert!(TexFormat::is_valid(0));
        assert!(TexFormat::is_valid(4));
        assert!(TexFormat::is_valid(7));
        assert!(!TexFormat::is_valid(1));
        assert!(!TexFormat::is_valid(99));
    }

    #[test]
    fn 测试从_i32_转换_TexFormat() {
        assert_eq!(TexFormat::try_from(4).unwrap(), TexFormat::DXT5);
        assert!(TexFormat::try_from(5).is_err());
    }

    // ========== FreeImageFormat 测试 ==========

    #[test]
    fn 测试_FreeImageFormat_枚举值对齐_CSharp() {
        assert_eq!(FreeImageFormat::FIF_UNKNOWN as i32, -1);
        assert_eq!(FreeImageFormat::FIF_BMP as i32, 0);
        assert_eq!(FreeImageFormat::FIF_PNG as i32, 13);
        assert_eq!(FreeImageFormat::FIF_GIF as i32, 25);
        assert_eq!(FreeImageFormat::FIF_MP4 as i32, 35);
    }

    #[test]
    fn 测试_FreeImageFormat_有效性验证() {
        assert!(FreeImageFormat::is_valid(-1));
        assert!(FreeImageFormat::is_valid(0));
        assert!(FreeImageFormat::is_valid(35));
        assert!(!FreeImageFormat::is_valid(-2));
        assert!(!FreeImageFormat::is_valid(36));
    }

    // ========== DxtFlags 测试 ==========

    #[test]
    fn 测试_DxtFlags_位运算() {
        let flags = DxtFlags(3); // DXT1 | DXT3
        assert!(!flags.is_empty());
        assert!(flags.contains(DxtFlags::DXT1));
        assert!(flags.contains(DxtFlags::DXT3));
        assert!(!flags.contains(DxtFlags::DXT5));
    }

    // ========== TexFlags 测试 ==========

    #[test]
    fn 测试_TexFlags_位值正确() {
        assert_eq!(tex_flags::NO_INTERPOLATION, 1);
        assert_eq!(tex_flags::CLAMP_UVS, 2);
        assert_eq!(tex_flags::IS_GIF, 4);
        assert_eq!(tex_flags::UNK3, 8);
        assert_eq!(tex_flags::UNK4, 16);
        assert_eq!(tex_flags::IS_VIDEO_TEXTURE, 32);
        assert_eq!(tex_flags::UNK6, 64);
        assert_eq!(tex_flags::UNK7, 128);
    }
}

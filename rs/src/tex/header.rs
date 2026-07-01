// TEX Header 数据模型与读写，包含魔数常量和 TexFlags 位掩码定义。
// 与 C# 原版 RePKG.Core/Texture/TexHeader.cs 和 RePKG.Application/Texture/TexHeaderReader.cs 对应。

use std::io::{self, Read, Write};

use byteorder::{LittleEndian, ReadBytesExt, WriteBytesExt};

use crate::error::{UnknownMagicError, UnsafeTexError};
use crate::format::TexFormat;

/// TEX 文件顶层魔数 1。
pub const MAGIC_TEXV0005: &str = "TEXV0005";
/// TEX 文件顶层魔数 2。
pub const MAGIC_TEXI0001: &str = "TEXI0001";
/// ImageContainer V1 魔数（无 LZ4，无 FreeImageFormat）。
pub const MAGIC_TEXB0001: &str = "TEXB0001";
/// ImageContainer V2 魔数（支持 LZ4）。
pub const MAGIC_TEXB0002: &str = "TEXB0002";
/// ImageContainer V3 魔数（增加 FreeImageFormat 字段）。
pub const MAGIC_TEXB0003: &str = "TEXB0003";
/// ImageContainer V4 魔数（增加视频标志和 conditionJson）。
pub const MAGIC_TEXB0004: &str = "TEXB0004";
/// FrameInfoContainer V1 魔数（int32 坐标）。
pub const MAGIC_TEXS0001: &str = "TEXS0001";
/// FrameInfoContainer V2 魔数（float32 坐标）。
pub const MAGIC_TEXS0002: &str = "TEXS0002";
/// FrameInfoContainer V3 魔数（float32 坐标 + GifWidth/GifHeight）。
pub const MAGIC_TEXS0003: &str = "TEXS0003";

/// ImageContainer 版本枚举。
#[derive(Debug, Clone, Copy, PartialEq, Eq)]
pub enum ImageContainerVersion {
    /// V1：无 LZ4 压缩，无 FreeImageFormat。
    Version1 = 1,
    /// V2：支持 LZ4 压缩。
    Version2 = 2,
    /// V3：增加 FreeImageFormat 字段。
    Version3 = 3,
    /// V4：增加视频标志位和 conditionJson。
    Version4 = 4,
}

impl ImageContainerVersion {
    /// 根据魔数字符串获取对应的 ImageContainer 版本。
    pub fn from_magic(magic: &str) -> Result<Self, UnknownMagicError> {
        match magic {
            MAGIC_TEXB0001 => Ok(Self::Version1),
            MAGIC_TEXB0002 => Ok(Self::Version2),
            MAGIC_TEXB0003 => Ok(Self::Version3),
            MAGIC_TEXB0004 => Ok(Self::Version4),
            _ => Err(UnknownMagicError {
                magic: magic.to_string(),
            }),
        }
    }

    /// 获取版本对应的魔数字符串。
    pub fn magic_str(self) -> &'static str {
        match self {
            Self::Version1 => MAGIC_TEXB0001,
            Self::Version2 => MAGIC_TEXB0002,
            Self::Version3 => MAGIC_TEXB0003,
            Self::Version4 => MAGIC_TEXB0004,
        }
    }
}

/// TEX 文件头部（28 字节）。
#[derive(Debug, Clone)]
pub struct Header {
    /// 纹理像素格式。
    pub format: TexFormat,
    /// 标志位（TexFlags 位掩码）。
    pub flags: u32,
    /// 纹理宽度。
    pub texture_width: i32,
    /// 纹理高度。
    pub texture_height: i32,
    /// 实际图像宽度。
    pub image_width: i32,
    /// 实际图像高度。
    pub image_height: i32,
    /// 未知字段（保留回写）。
    pub unk_int0: u32,
}

impl Header {
    /// 从 reader 读取 Header。
    pub fn read<R: Read>(reader: &mut R) -> io::Result<Self> {
        let format_val = reader.read_i32::<LittleEndian>()?;
        let format = TexFormat::try_from(format_val)
            .map_err(|e| io::Error::new(io::ErrorKind::InvalidData, format!("{}", e)))?;
        let flags = reader.read_u32::<LittleEndian>()?;
        let texture_width = reader.read_i32::<LittleEndian>()?;
        let texture_height = reader.read_i32::<LittleEndian>()?;
        let image_width = reader.read_i32::<LittleEndian>()?;
        let image_height = reader.read_i32::<LittleEndian>()?;

        if texture_width < 0 || texture_height < 0 || image_width < 0 || image_height < 0 {
            return Err(io::Error::new(
                io::ErrorKind::InvalidData,
                UnsafeTexError {
                    message: format!(
                        "Header 尺寸无效：tx={}x{} img={}x{}",
                        texture_width, texture_height, image_width, image_height
                    ),
                }
                .to_string(),
            ));
        }
        let unk_int0 = reader.read_u32::<LittleEndian>()?;

        Ok(Self {
            format,
            flags,
            texture_width,
            texture_height,
            image_width,
            image_height,
            unk_int0,
        })
    }

    /// 向 writer 写入 Header。
    pub fn write<W: Write>(&self, writer: &mut W) -> io::Result<()> {
        writer.write_i32::<LittleEndian>(self.format as i32)?;
        writer.write_u32::<LittleEndian>(self.flags)?;
        writer.write_i32::<LittleEndian>(self.texture_width)?;
        writer.write_i32::<LittleEndian>(self.texture_height)?;
        writer.write_i32::<LittleEndian>(self.image_width)?;
        writer.write_i32::<LittleEndian>(self.image_height)?;
        writer.write_u32::<LittleEndian>(self.unk_int0)?;
        Ok(())
    }
}

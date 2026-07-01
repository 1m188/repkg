// Image 和 Mipmap 数据模型，以及 V1/V2/V3/V4 格式的 mipmap 读写。
// 与 C# 原版 RePKG.Core/Texture/TexImage.cs + TexMipmap.cs 和 RePKG.Application/Texture/TexImageReader.cs 对应。

use std::io::{self, Read, Write};

use byteorder::{LittleEndian, ReadBytesExt, WriteBytesExt};

use crate::binutil;
use crate::error::UnsafeTexError;
use crate::format::MipmapFormat;

/// 单个 mipmap 级别数据。
#[derive(Debug, Clone)]
pub struct Mipmap {
    /// 像素或压缩数据。
    pub bytes: Vec<u8>,
    /// 宽度。
    pub width: i32,
    /// 高度。
    pub height: i32,
    /// LZ4 解压后的预期字节数。
    pub decompressed_bytes_count: i32,
    /// 是否经过 LZ4 压缩。
    pub is_lz4_compressed: bool,
    /// mipmap 像素格式。
    pub format: MipmapFormat,
    /// V4 容器中的 conditionJson（保留回写）。
    pub condition_json: Option<String>,
}

/// 单张图像，包含一组 mipmap 链。
#[derive(Debug, Clone)]
pub struct Image {
    /// mipmap 链（[0] 为最高分辨率）。
    pub mipmaps: Vec<Mipmap>,
}

/// mipmap 安全常量。
const MAX_MIPMAP_COUNT: usize = 32;
const MAX_MIPMAP_BYTE_COUNT: usize = 250_000_000;

/// 读取 V1 格式的 mipmap（无 LZ4 支持）。
pub fn read_mipmap_v1<R: Read>(reader: &mut R, format: MipmapFormat) -> io::Result<Mipmap> {
    let width = reader.read_i32::<LittleEndian>()?;
    let height = reader.read_i32::<LittleEndian>()?;
    let byte_count = reader.read_i32::<LittleEndian>()?;

    if width < 0 || height < 0 {
        return Err(io::Error::new(
            io::ErrorKind::InvalidData,
            UnsafeTexError {
                message: format!("mipmap 尺寸无效：{}x{}", width, height),
            }
            .to_string(),
        ));
    }

    if byte_count < 0 || byte_count as usize > MAX_MIPMAP_BYTE_COUNT {
        return Err(io::Error::new(
            io::ErrorKind::InvalidData,
            UnsafeTexError {
                message: format!("mipmap 字节数超限：{}", byte_count),
            }
            .to_string(),
        ));
    }

    let mut bytes = vec![0u8; byte_count as usize];
    reader.read_exact(&mut bytes)?;

    Ok(Mipmap {
        bytes,
        width,
        height,
        decompressed_bytes_count: 0,
        is_lz4_compressed: false,
        format,
        condition_json: None,
    })
}

/// 读取 V2/V3 格式的 mipmap（含 LZ4 支持）。
pub fn read_mipmap_v2_v3<R: Read>(reader: &mut R, format: MipmapFormat) -> io::Result<Mipmap> {
    let width = reader.read_i32::<LittleEndian>()?;
    let height = reader.read_i32::<LittleEndian>()?;
    let is_lz4_val = reader.read_i32::<LittleEndian>()?;
    let is_lz4_compressed = is_lz4_val == 1;
    let decompressed_bytes_count = reader.read_i32::<LittleEndian>()?;
    let byte_count = reader.read_i32::<LittleEndian>()?;

    if width < 0 || height < 0 {
        return Err(io::Error::new(
            io::ErrorKind::InvalidData,
            UnsafeTexError {
                message: format!("mipmap 尺寸无效：{}x{}", width, height),
            }
            .to_string(),
        ));
    }
    if decompressed_bytes_count < 0 || decompressed_bytes_count as usize > MAX_MIPMAP_BYTE_COUNT {
        return Err(io::Error::new(
            io::ErrorKind::InvalidData,
            UnsafeTexError {
                message: format!(
                    "mipmap 解压后字节数超限：{}（最大 {}）",
                    decompressed_bytes_count, MAX_MIPMAP_BYTE_COUNT
                ),
            }
            .to_string(),
        ));
    }

    if byte_count < 0 || byte_count as usize > MAX_MIPMAP_BYTE_COUNT {
        return Err(io::Error::new(
            io::ErrorKind::InvalidData,
            UnsafeTexError {
                message: format!("mipmap 字节数超限：{}", byte_count),
            }
            .to_string(),
        ));
    }

    let mut bytes = vec![0u8; byte_count as usize];
    reader.read_exact(&mut bytes)?;

    Ok(Mipmap {
        bytes,
        width,
        height,
        decompressed_bytes_count,
        is_lz4_compressed,
        format,
        condition_json: None,
    })
}

/// 读取 V4 格式的 mipmap（含视频前置字段）。
pub fn read_mipmap_v4<R: Read>(reader: &mut R, format: MipmapFormat) -> io::Result<Mipmap> {
    let param1 = reader.read_i32::<LittleEndian>()?;
    let param2 = reader.read_i32::<LittleEndian>()?;
    let condition_json = binutil::read_n_string(reader, 256)?;
    let param3 = reader.read_i32::<LittleEndian>()?;

    if param1 != 1 || param2 != 2 || param3 != 1 {
        return Err(io::Error::new(
            io::ErrorKind::InvalidData,
            UnsafeTexError {
                message: format!(
                    "V4 mipmap 参数错误：param1={}, param2={}, param3={}",
                    param1, param2, param3
                ),
            }
            .to_string(),
        ));
    }

    let mut mipmap = read_mipmap_v2_v3(reader, format)?;
    mipmap.condition_json = if condition_json.is_empty() {
        None
    } else {
        Some(condition_json)
    };
    Ok(mipmap)
}

/// 写入 V1 格式的 mipmap（仅适用于未压缩数据）。
pub fn write_mipmap_v1<W: Write>(writer: &mut W, mipmap: &Mipmap) -> io::Result<()> {
    writer.write_i32::<LittleEndian>(mipmap.width)?;
    writer.write_i32::<LittleEndian>(mipmap.height)?;
    writer.write_i32::<LittleEndian>(mipmap.bytes.len() as i32)?;
    writer.write_all(&mipmap.bytes)?;
    Ok(())
}

/// 写入 V2/V3 格式的 mipmap（含 LZ4 信息）。
pub fn write_mipmap_v2_v3<W: Write>(writer: &mut W, mipmap: &Mipmap) -> io::Result<()> {
    writer.write_i32::<LittleEndian>(mipmap.width)?;
    writer.write_i32::<LittleEndian>(mipmap.height)?;
    writer.write_i32::<LittleEndian>(if mipmap.is_lz4_compressed { 1 } else { 0 })?;
    writer.write_i32::<LittleEndian>(mipmap.decompressed_bytes_count)?;
    writer.write_i32::<LittleEndian>(mipmap.bytes.len() as i32)?;
    writer.write_all(&mipmap.bytes)?;
    Ok(())
}

/// 写入 V4 格式的 mipmap（含视频前置字段）。
pub fn write_mipmap_v4<W: Write>(writer: &mut W, mipmap: &Mipmap) -> io::Result<()> {
    writer.write_i32::<LittleEndian>(1)?; // param1
    writer.write_i32::<LittleEndian>(2)?; // param2
    let cj = mipmap.condition_json.as_deref().unwrap_or("");
    binutil::write_n_string(writer, cj)?; // conditionJson
    writer.write_i32::<LittleEndian>(1)?; // param3
    write_mipmap_v2_v3(writer, mipmap)
}

/// 读取单张 Image（mipmapCount + mipmap 链）。
pub fn read_image<R: Read>(
    reader: &mut R,
    version: super::header::ImageContainerVersion,
    format: MipmapFormat,
) -> io::Result<Image> {
    let mipmap_count = reader.read_i32::<LittleEndian>()?;
    if mipmap_count < 0 || mipmap_count as usize > MAX_MIPMAP_COUNT {
        return Err(io::Error::new(
            io::ErrorKind::InvalidData,
            UnsafeTexError {
                message: format!("mipmap 数量超限：{}", mipmap_count),
            }
            .to_string(),
        ));
    }

    let mut mipmaps = Vec::with_capacity(mipmap_count as usize);
    for _ in 0..mipmap_count {
        let mipmap = match version {
            super::header::ImageContainerVersion::Version1 => read_mipmap_v1(reader, format)?,
            super::header::ImageContainerVersion::Version4 => read_mipmap_v4(reader, format)?,
            _ => read_mipmap_v2_v3(reader, format)?,
        };
        mipmaps.push(mipmap);
    }

    Ok(Image { mipmaps })
}

/// 写入单张 Image。
pub fn write_image<W: Write>(
    writer: &mut W,
    image: &Image,
    version: super::header::ImageContainerVersion,
) -> io::Result<()> {
    writer.write_i32::<LittleEndian>(image.mipmaps.len() as i32)?;

    for mipmap in &image.mipmaps {
        match version {
            super::header::ImageContainerVersion::Version1 => write_mipmap_v1(writer, mipmap)?,
            super::header::ImageContainerVersion::Version4 => write_mipmap_v4(writer, mipmap)?,
            _ => write_mipmap_v2_v3(writer, mipmap)?,
        };
    }

    Ok(())
}

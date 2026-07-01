// ImageContainer 的读写，包含 V4→V3 降级逻辑。
// 与 C# 原版 RePKG.Core/Texture/TexImageContainer.cs 和 RePKG.Application/Texture/TexImageContainerReader.cs 对应。

use std::io::{self, Read, Write};

use byteorder::{LittleEndian, ReadBytesExt, WriteBytesExt};

use crate::binutil;
use crate::error::UnsafeTexError;
use crate::format::{FreeImageFormat, MipmapFormat, TexFormat};

use super::header::ImageContainerVersion;
use super::image::{self, Image};

/// 图像容器安全常量。
const MAX_IMAGE_COUNT: usize = 100;

/// TEX 图像容器。
#[derive(Debug, Clone)]
pub struct ImageContainer {
    /// 魔数字符串。
    pub magic: String,
    /// FreeImage 格式。
    pub image_format: FreeImageFormat,
    /// 容器版本。
    pub version: ImageContainerVersion,
    /// 包含的图像列表。
    pub images: Vec<Image>,
}

/// 从 reader 读取 ImageContainer。
/// tex_format 用于 V1/V2 容器中 image_format 为 FIF_UNKNOWN 时的格式回退。
pub fn read_image_container<R: Read>(
    reader: &mut R,
    tex_format: TexFormat,
) -> io::Result<ImageContainer> {
    let magic = binutil::read_n_string(reader, 16)?;
    let mut version = ImageContainerVersion::from_magic(&magic)
        .map_err(|e| io::Error::new(io::ErrorKind::InvalidData, format!("{}", e)))?;

    let image_count = reader.read_i32::<LittleEndian>()?;
    if image_count < 0 || image_count as usize > MAX_IMAGE_COUNT {
        return Err(io::Error::new(
            io::ErrorKind::InvalidData,
            UnsafeTexError {
                message: format!("图像数量超限：{}", image_count),
            }
            .to_string(),
        ));
    }

    // V3/V4: 读取 ImageFormat
    let mut image_format = FreeImageFormat::FIF_UNKNOWN;
    if version == ImageContainerVersion::Version3 || version == ImageContainerVersion::Version4 {
        let fif_val = reader.read_i32::<LittleEndian>()?;
        image_format = FreeImageFormat::try_from(fif_val)
            .map_err(|e| io::Error::new(io::ErrorKind::InvalidData, format!("{}", e)))?;
    }

    // V4: 检查视频标志
    if version == ImageContainerVersion::Version4 {
        let is_video_mp4 = reader.read_i32::<LittleEndian>()?;
        if image_format == FreeImageFormat::FIF_UNKNOWN && is_video_mp4 == 1 {
            image_format = FreeImageFormat::FIF_MP4;
        }

        // V4 non-MP4 → 降级为 V3
        if image_format != FreeImageFormat::FIF_MP4 {
            version = ImageContainerVersion::Version3;
        }
    }

    // 读取各个 Image
    let format = if image_format == FreeImageFormat::FIF_UNKNOWN {
        tex_format.to_mipmap_format()
    } else {
        free_image_format_to_mipmap_format(image_format)
    };
    let mut images = Vec::with_capacity(image_count as usize);
    for _ in 0..image_count {
        let image = image::read_image(reader, version, format)?;
        images.push(image);
    }

    Ok(ImageContainer {
        magic,
        image_format,
        version,
        images,
    })
}

/// 写入 ImageContainer。
pub fn write_image_container<W: Write>(
    writer: &mut W,
    container: &ImageContainer,
) -> io::Result<()> {
    binutil::write_n_string(writer, container.version.magic_str())?;
    writer.write_i32::<LittleEndian>(container.images.len() as i32)?;

    // V3/V4: 写入 ImageFormat
    if container.version == ImageContainerVersion::Version3
        || container.version == ImageContainerVersion::Version4
    {
        writer.write_i32::<LittleEndian>(container.image_format as i32)?;
    }

    // V4: 写入视频标志
    if container.version == ImageContainerVersion::Version4 {
        let is_mp4 = if container.image_format == FreeImageFormat::FIF_MP4 {
            1
        } else {
            0
        };
        writer.write_i32::<LittleEndian>(is_mp4)?;
    }

    for image in &container.images {
        image::write_image(writer, image, container.version)?;
    }

    Ok(())
}

/// FreeImageFormat 转 MipmapFormat 映射表（对齐 C#）。
fn free_image_format_to_mipmap_format(fif: FreeImageFormat) -> MipmapFormat {
    match fif {
        FreeImageFormat::FIF_UNKNOWN => MipmapFormat::Invalid,
        FreeImageFormat::FIF_BMP => MipmapFormat::ImageBMP,
        FreeImageFormat::FIF_ICO => MipmapFormat::ImageICO,
        FreeImageFormat::FIF_JPEG => MipmapFormat::ImageJPEG,
        FreeImageFormat::FIF_JNG => MipmapFormat::ImageJNG,
        FreeImageFormat::FIF_KOALA => MipmapFormat::ImageKOALA,
        FreeImageFormat::FIF_LBM => MipmapFormat::ImageLBM,
        FreeImageFormat::FIF_MNG => MipmapFormat::ImageMNG,
        FreeImageFormat::FIF_PBM => MipmapFormat::ImagePBM,
        FreeImageFormat::FIF_PBMRAW => MipmapFormat::ImagePBMRAW,
        FreeImageFormat::FIF_PCD => MipmapFormat::ImagePCD,
        FreeImageFormat::FIF_PCX => MipmapFormat::ImagePCX,
        FreeImageFormat::FIF_PGM => MipmapFormat::ImagePGM,
        FreeImageFormat::FIF_PGMRAW => MipmapFormat::ImagePGMRAW,
        FreeImageFormat::FIF_PNG => MipmapFormat::ImagePNG,
        FreeImageFormat::FIF_PPM => MipmapFormat::ImagePPM,
        FreeImageFormat::FIF_PPMRAW => MipmapFormat::ImagePPMRAW,
        FreeImageFormat::FIF_RAS => MipmapFormat::ImageRAS,
        FreeImageFormat::FIF_TARGA => MipmapFormat::ImageTARGA,
        FreeImageFormat::FIF_TIFF => MipmapFormat::ImageTIFF,
        FreeImageFormat::FIF_WBMP => MipmapFormat::ImageWBMP,
        FreeImageFormat::FIF_PSD => MipmapFormat::ImagePSD,
        FreeImageFormat::FIF_CUT => MipmapFormat::ImageCUT,
        FreeImageFormat::FIF_XBM => MipmapFormat::ImageXBM,
        FreeImageFormat::FIF_XPM => MipmapFormat::ImageXPM,
        FreeImageFormat::FIF_DDS => MipmapFormat::ImageDDS,
        FreeImageFormat::FIF_GIF => MipmapFormat::ImageGIF,
        FreeImageFormat::FIF_HDR => MipmapFormat::ImageHDR,
        FreeImageFormat::FIF_FAXG3 => MipmapFormat::ImageFAXG3,
        FreeImageFormat::FIF_SGI => MipmapFormat::ImageSGI,
        FreeImageFormat::FIF_EXR => MipmapFormat::ImageEXR,
        FreeImageFormat::FIF_J2K => MipmapFormat::ImageJ2K,
        FreeImageFormat::FIF_JP2 => MipmapFormat::ImageJP2,
        FreeImageFormat::FIF_PFM => MipmapFormat::ImagePFM,
        FreeImageFormat::FIF_PICT => MipmapFormat::ImagePICT,
        FreeImageFormat::FIF_RAW => MipmapFormat::ImageRAW,
        FreeImageFormat::FIF_MP4 => MipmapFormat::VideoMp4,
    }
}

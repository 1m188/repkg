// TEX 顶层读取器，编排魔数→Header→ImageContainer→FrameInfoContainer 的解析流程。
// 与 C# 原版 RePKG.Application/Texture/TexReader.cs 对应。

use std::io::{self, Read};

use crate::binutil;
use crate::error::UnknownMagicError;
use crate::format;

use super::container::{self, ImageContainer};
use super::frame::{self, FrameInfoContainer};
use super::header::{Header, MAGIC_TEXI0001, MAGIC_TEXV0005};

/// TEX 文件顶层结构。
#[derive(Debug, Clone)]
pub struct Tex {
    /// 魔数 1（始终为 "TEXV0005"）。
    pub magic1: String,
    /// 魔数 2（始终为 "TEXI0001"）。
    pub magic2: String,
    /// 纹理头部。
    pub header: Header,
    /// 图像容器。
    pub images_container: ImageContainer,
    /// 帧信息容器（动画纹理非空）。
    pub frame_info_container: Option<FrameInfoContainer>,
}

/// 从 reader 读取 TEX 文件。
pub fn read_tex<R: Read>(reader: &mut R) -> io::Result<Tex> {
    // 读取魔数 1
    let magic1 = binutil::read_n_string(reader, 16)?;
    if magic1 != MAGIC_TEXV0005 {
        return Err(io::Error::new(
            io::ErrorKind::InvalidData,
            format!("{}", UnknownMagicError { magic: magic1 }),
        ));
    }

    // 读取魔数 2
    let magic2 = binutil::read_n_string(reader, 16)?;
    if magic2 != MAGIC_TEXI0001 {
        return Err(io::Error::new(
            io::ErrorKind::InvalidData,
            format!("{}", UnknownMagicError { magic: magic2 }),
        ));
    }

    // 读取 Header
    let header = Header::read(reader)?;

    // 读取 ImageContainer
    let images_container = container::read_image_container(reader, header.format)?;

    // 读取 FrameInfoContainer（仅当 IsGif 时）
    let frame_info_container = if (header.flags & format::tex_flags::IS_GIF) != 0 {
        frame::read_frame_info_container(reader)?
    } else {
        None
    };

    Ok(Tex {
        magic1,
        magic2,
        header,
        images_container,
        frame_info_container,
    })
}

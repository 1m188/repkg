// TEX 顶层写入器，按序序列化全部 TEX 组件。
// 与 C# 原版 RePKG.Application/Texture/Writer/TexWriter.cs 对应。

use std::io::{self, Write};

use crate::binutil;

use super::container;
use super::frame;
use super::reader::Tex;

/// 将 Tex 序列化写入 writer。
pub fn write_tex<W: Write>(writer: &mut W, tex: &Tex) -> io::Result<()> {
    binutil::write_n_string(writer, &tex.magic1)?;
    binutil::write_n_string(writer, &tex.magic2)?;

    tex.header.write(writer)?;

    container::write_image_container(writer, &tex.images_container)?;

    if let Some(ref container) = tex.frame_info_container {
        frame::write_frame_info_container(writer, container)?;
    }

    Ok(())
}

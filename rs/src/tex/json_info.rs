// .tex-json 元数据生成器。
// 与 C# 原版 RePKG.Application/Texture/TexJsonInfoGenerator.cs 对应。

use serde::Serialize;

use crate::format;
use crate::tex::reader::Tex;

/// TEX JSON 元数据。
#[derive(Debug, Serialize)]
pub struct TexJsonInfo {
    /// 纹理格式名称。
    pub format: String,
    /// 是否禁止插值。
    #[serde(skip_serializing_if = "std::ops::Not::not")]
    pub nointerpolation: bool,
    /// 是否钳制 UV。
    #[serde(skip_serializing_if = "std::ops::Not::not")]
    pub clamp: bool,
    /// 是否非 2 的幂。
    #[serde(skip_serializing_if = "Option::is_none")]
    pub nonpoweroftwo: Option<bool>,
    /// 出血标志（与 ClampUVs 相同含义）。
    #[serde(skip_serializing_if = "Option::is_none")]
    pub bleed: Option<bool>,
    /// 帧序列（精灵图）。
    #[serde(skip_serializing_if = "Option::is_none")]
    pub sequences: Option<Vec<SequenceFrame>>,
}

/// 精灵图序列中的单帧。
#[derive(Debug, Serialize)]
pub struct SequenceFrame {
    /// X 坐标。
    pub x: f32,
    /// Y 坐标。
    pub y: f32,
    /// 宽度。
    pub width: f32,
    /// 高度。
    pub height: f32,
    /// 帧延迟（百分之一秒）。
    pub delay: u16,
}

/// 从 TEX 数据生成 JSON 元数据。
pub fn generate_json_info(tex: &Tex) -> TexJsonInfo {
    let format_name = format!("{:?}", tex.header.format);

    let is_non_power_of_two = !super::converter::is_power_of_two(tex.header.texture_width)
        || !super::converter::is_power_of_two(tex.header.texture_height)
        || tex.header.texture_width != tex.header.image_width
        || tex.header.texture_height != tex.header.image_height;

    let sequences = tex.frame_info_container.as_ref().map(|container| {
        container
            .frames
            .iter()
            .map(|f| SequenceFrame {
                x: f.x,
                y: f.y,
                width: f.width,
                height: f.height,
                delay: (f.frametime * 100.0).round() as u16,
            })
            .collect()
    });

    TexJsonInfo {
        format: format_name,
        nointerpolation: (tex.header.flags & format::tex_flags::NO_INTERPOLATION) != 0,
        clamp: (tex.header.flags & format::tex_flags::CLAMP_UVS) != 0,
        nonpoweroftwo: if is_non_power_of_two {
            Some(true)
        } else {
            None
        },
        bleed: if (tex.header.flags & format::tex_flags::CLAMP_UVS) != 0 {
            Some(true)
        } else {
            None
        },
        sequences,
    }
}

// FrameInfo 和 FrameInfoContainer 的读写，支持 V1/V2/V3 三种版本的帧坐标格式。
// 与 C# 原版 RePKG.Core/Texture/TexFrameInfo.cs + TexFrameInfoContainer.cs 和 RePKG.Application/Texture/TexFrameInfoContainerReader.cs 对应。

use std::io::{self, Read, Write};

use byteorder::{LittleEndian, ReadBytesExt, WriteBytesExt};

use crate::binutil;
use crate::error::UnsafeTexError;

/// 单帧信息。
#[derive(Debug, Clone)]
pub struct FrameInfo {
    /// 关联的图像 ID（指向 ImageContainer 中的索引）。
    pub image_id: i32,
    /// 帧显示时长（秒）。
    pub frametime: f32,
    /// 帧左上角 X 坐标。
    pub x: f32,
    /// 帧左上角 Y 坐标。
    pub y: f32,
    /// 帧宽度。
    pub width: f32,
    /// 旋转矩阵 WidthY。
    pub width_y: f32,
    /// 旋转矩阵 HeightX。
    pub height_x: f32,
    /// 帧高度。
    pub height: f32,
}

/// 帧信息容器（仅动画 TEX 存在）。
#[derive(Debug, Clone)]
pub struct FrameInfoContainer {
    /// 魔数字符串。
    pub magic: String,
    /// 帧列表。
    pub frames: Vec<FrameInfo>,
    /// GIF 输出宽度（V3 显式存储，V1/V2 从首帧推导）。
    pub gif_width: i32,
    /// GIF 输出高度（V3 显式存储，V1/V2 从首帧推导）。
    pub gif_height: i32,
}

/// 帧数安全上限。
const MAX_FRAME_COUNT: usize = 100_000;

/// 从 reader 读取 FrameInfoContainer。若魔数不匹配则视为无动画。
pub fn read_frame_info_container<R: Read>(
    reader: &mut R,
) -> io::Result<Option<FrameInfoContainer>> {
    let magic = match binutil::read_n_string(reader, 16) {
        Ok(m) => m,
        Err(_) => return Ok(None),
    };

    let is_frame_section = magic == super::header::MAGIC_TEXS0001
        || magic == super::header::MAGIC_TEXS0002
        || magic == super::header::MAGIC_TEXS0003;

    if !is_frame_section {
        return Ok(None);
    }

    let frame_count = reader.read_i32::<LittleEndian>()?;
    if frame_count < 0 || frame_count as usize > MAX_FRAME_COUNT {
        return Err(io::Error::new(
            io::ErrorKind::InvalidData,
            UnsafeTexError {
                message: format!("帧数超限：{}", frame_count),
            }
            .to_string(),
        ));
    }

    let (gif_width, gif_height) = if magic == super::header::MAGIC_TEXS0003 {
        (
            reader.read_i32::<LittleEndian>()?,
            reader.read_i32::<LittleEndian>()?,
        )
    } else {
        (0, 0)
    };

    let is_version1 = magic == super::header::MAGIC_TEXS0001;
    let mut frames = Vec::with_capacity(frame_count as usize);

    for _ in 0..frame_count {
        let image_id = reader.read_i32::<LittleEndian>()?;
        if image_id < 0 {
            return Err(io::Error::new(
                io::ErrorKind::InvalidData,
                UnsafeTexError {
                    message: format!("帧 image_id 无效：{}", image_id),
                }
                .to_string(),
            ));
        }
        let frametime = reader.read_f32::<LittleEndian>()?;
        if frametime.is_nan() || frametime.is_infinite() || frametime < 0.0 {
            return Err(io::Error::new(
                io::ErrorKind::InvalidData,
                UnsafeTexError {
                    message: format!("帧 frametime 无效：{}", frametime),
                }
                .to_string(),
            ));
        }

        let (x, y, width, width_y, height_x, height) = if is_version1 {
            // V1: int32 坐标 → 转 float32
            (
                reader.read_i32::<LittleEndian>()? as f32,
                reader.read_i32::<LittleEndian>()? as f32,
                reader.read_i32::<LittleEndian>()? as f32,
                reader.read_i32::<LittleEndian>()? as f32,
                reader.read_i32::<LittleEndian>()? as f32,
                reader.read_i32::<LittleEndian>()? as f32,
            )
        } else {
            // V2/V3: float32 坐标
            (
                reader.read_f32::<LittleEndian>()?,
                reader.read_f32::<LittleEndian>()?,
                reader.read_f32::<LittleEndian>()?,
                reader.read_f32::<LittleEndian>()?,
                reader.read_f32::<LittleEndian>()?,
                reader.read_f32::<LittleEndian>()?,
            )
        };

        frames.push(FrameInfo {
            image_id,
            frametime,
            x,
            y,
            width,
            width_y,
            height_x,
            height,
        });
    }

    // V1/V2 无显式 GIF 尺寸时从首帧推导
    let (gif_width, gif_height) = if gif_width == 0 || gif_height == 0 {
        if let Some(first) = frames.first() {
            (first.width as i32, first.height as i32)
        } else {
            (0, 0)
        }
    } else {
        (gif_width, gif_height)
    };

    Ok(Some(FrameInfoContainer {
        magic,
        frames,
        gif_width,
        gif_height,
    }))
}

/// 写入 FrameInfoContainer。
pub fn write_frame_info_container<W: Write>(
    writer: &mut W,
    container: &FrameInfoContainer,
) -> io::Result<()> {
    binutil::write_n_string(writer, &container.magic)?;
    writer.write_i32::<LittleEndian>(container.frames.len() as i32)?;

    let is_v3 = container.magic == super::header::MAGIC_TEXS0003;
    if is_v3 {
        writer.write_i32::<LittleEndian>(container.gif_width)?;
        writer.write_i32::<LittleEndian>(container.gif_height)?;
    }

    let is_v1 = container.magic == super::header::MAGIC_TEXS0001;

    for frame in &container.frames {
        writer.write_i32::<LittleEndian>(frame.image_id)?;
        writer.write_f32::<LittleEndian>(frame.frametime)?;

        let (x, y, w, wy, hx, h) = (
            frame.x,
            frame.y,
            frame.width,
            frame.width_y,
            frame.height_x,
            frame.height,
        );

        if is_v1 {
            // 安全转换：NaN/Inf → 0，超出 i32 范围的值截断到边界
            let to_i32 = |v: f32| -> i32 {
                if v.is_nan() || v.is_infinite() {
                    0
                } else {
                    v as i32
                }
            };
            writer.write_i32::<LittleEndian>(to_i32(x))?;
            writer.write_i32::<LittleEndian>(to_i32(y))?;
            writer.write_i32::<LittleEndian>(to_i32(w))?;
            writer.write_i32::<LittleEndian>(to_i32(wy))?;
            writer.write_i32::<LittleEndian>(to_i32(hx))?;
            writer.write_i32::<LittleEndian>(to_i32(h))?;
        } else {
            writer.write_f32::<LittleEndian>(x)?;
            writer.write_f32::<LittleEndian>(y)?;
            writer.write_f32::<LittleEndian>(w)?;
            writer.write_f32::<LittleEndian>(wy)?;
            writer.write_f32::<LittleEndian>(hx)?;
            writer.write_f32::<LittleEndian>(h)?;
        }
    }

    Ok(())
}

// TEX 到 PNG/GIF/MP4 的格式转换器。
// 与 C# 原版 RePKG.Application/Texture/TexToImageConverter.cs 对应。

use image::ImageEncoder;

use crate::format::MipmapFormat;
use crate::tex::compressor;
use crate::tex::image::Mipmap;

/// 转换结果。
#[derive(Debug)]
pub struct ImageResult {
    /// 编码后的图片/视频字节。
    pub bytes: Vec<u8>,
    /// 输出格式。
    pub format: MipmapFormat,
}

/// 将 mipmap 转换为图片（先解压 LZ4/DXT，再按解码后格式分发）。
pub fn convert_to_image(mipmap: &Mipmap) -> Result<ImageResult, Box<dyn std::error::Error>> {
    let decoded = compressor::decompress(mipmap)?;

    match decoded.format {
        MipmapFormat::RGBA8888 | MipmapFormat::R8 | MipmapFormat::RG88 => {
            encode_raw_to_png(&decoded)
        }
        _ if decoded.format.is_image() || decoded.format.is_video() => {
            convert_to_raw_inner(&decoded)
        }
        _ => Err(format!("不支持的像素格式：{:?}", decoded.format).into()),
    }
}

/// 将原始像素编码为 PNG。
fn encode_raw_to_png(decoded: &Mipmap) -> Result<ImageResult, Box<dyn std::error::Error>> {
    let w = decoded.width;
    let h = decoded.height;
    if w <= 0 || h <= 0 || w > 32768 || h > 32768 {
        return Err(format!("无效的原始图像尺寸：{}x{}", w, h).into());
    }
    let wu = w as u32;
    let hu = h as u32;
    let size = (wu as usize) * (hu as usize) * 4;

    match decoded.format {
        MipmapFormat::RGBA8888 => {
            let img = image::RgbaImage::from_raw(wu, hu, decoded.bytes.clone())
                .ok_or("无效的 RGBA 图像数据")?;

            let mut buf = Vec::new();
            let encoder = image::codecs::png::PngEncoder::new(&mut buf);
            encoder.write_image(
                &img,
                img.width(),
                img.height(),
                image::ExtendedColorType::Rgba8,
            )?;

            Ok(ImageResult {
                bytes: buf,
                format: MipmapFormat::ImagePNG,
            })
        }
        MipmapFormat::R8 => {
            let mut rgba = vec![0u8; size];
            for i in 0..decoded.bytes.len() {
                let g = decoded.bytes[i];
                rgba[i * 4] = g;
                rgba[i * 4 + 1] = g;
                rgba[i * 4 + 2] = g;
                rgba[i * 4 + 3] = 255;
            }
            let img = image::RgbaImage::from_raw(wu, hu, rgba).ok_or("无效的灰度图像数据")?;

            let mut buf = Vec::new();
            let encoder = image::codecs::png::PngEncoder::new(&mut buf);
            encoder.write_image(
                &img,
                img.width(),
                img.height(),
                image::ExtendedColorType::Rgba8,
            )?;

            Ok(ImageResult {
                bytes: buf,
                format: MipmapFormat::ImagePNG,
            })
        }
        MipmapFormat::RG88 => {
            let mut rgba = vec![0u8; size];
            for i in 0..(decoded.bytes.len() / 2) {
                let r = decoded.bytes[i * 2];
                let g = decoded.bytes[i * 2 + 1];
                rgba[i * 4] = g;
                rgba[i * 4 + 1] = g;
                rgba[i * 4 + 2] = g;
                rgba[i * 4 + 3] = r;
            }
            let img = image::RgbaImage::from_raw(wu, hu, rgba).ok_or("无效的 RG88 图像数据")?;

            let mut buf = Vec::new();
            let encoder = image::codecs::png::PngEncoder::new(&mut buf);
            encoder.write_image(
                &img,
                img.width(),
                img.height(),
                image::ExtendedColorType::Rgba8,
            )?;

            Ok(ImageResult {
                bytes: buf,
                format: MipmapFormat::ImagePNG,
            })
        }
        _ => Err(format!("不支持的像素格式：{:?}", decoded.format).into()),
    }
}

/// 已编码格式的原始字节传递（含 MP4 魔数验证）。
fn convert_to_raw_inner(decoded: &Mipmap) -> Result<ImageResult, Box<dyn std::error::Error>> {
    if decoded.format == MipmapFormat::VideoMp4 && decoded.bytes.len() >= 12 {
        let ftyp = &decoded.bytes[4..12];
        if ftyp != b"ftypisom" && ftyp != b"ftypmsnv" && ftyp != b"ftypmp42" {
            return Err("非法 MP4 数据：ftyp 标识无效".into());
        }
    }

    Ok(ImageResult {
        bytes: decoded.bytes.clone(),
        format: decoded.format,
    })
}

/// 将 mipmap 直接传递为原始字节（已编码格式，先解压 LZ4）。
pub fn convert_to_raw(mipmap: &Mipmap) -> Result<ImageResult, Box<dyn std::error::Error>> {
    let decoded = compressor::decompress(mipmap)?;
    convert_to_raw_inner(&decoded)
}

/// 将多帧 TEX 转换为 GIF 动画。
/// images 是 ImageContainer 中的图像列表（每个 image.mipmaps[0] 是精灵图）。
/// frames 是 FrameInfoContainer 中的帧信息（含 x/y/width/height 裁剪坐标）。
pub fn convert_to_gif(
    images: &[crate::tex::image::Image],
    frame_container: &crate::tex::frame::FrameInfoContainer,
) -> Result<ImageResult, Box<dyn std::error::Error>> {
    use image::codecs::gif::GifEncoder;
    use image::Frame;

    let mut gif_buf = Vec::new();
    {
        let mut encoder = GifEncoder::new(&mut gif_buf);

        for (fi, frame_info) in frame_container.frames.iter().enumerate() {
            let image_idx = frame_info.image_id as usize;
            let image = images
                .get(image_idx)
                .ok_or(format!("帧引用的图像 #{} 不存在", image_idx))?;
            let mipmap = image
                .mipmaps
                .first()
                .ok_or(format!("图像 #{} 无 mipmap", image_idx))?;

            let decoded = compressor::decompress(mipmap)?;
            let sprite = image::RgbaImage::from_raw(
                decoded.width as u32,
                decoded.height as u32,
                decoded.bytes.clone(),
            )
            .ok_or("无效的 GIF 帧图像数据")?;

            // 帧坐标回退逻辑：width 或 height 为 0 时使用备选字段
            let w = if frame_info.width != 0.0 {
                frame_info.width
            } else {
                frame_info.height_x
            };
            let h = if frame_info.height != 0.0 {
                frame_info.height
            } else {
                frame_info.width_y
            };
            // 裁剪起点取 min(X, X+width) 处理负数宽高
            let x = (frame_info.x).min(frame_info.x + w);
            let y = (frame_info.y).min(frame_info.y + h);
            let crop_w = w.abs() as u32;
            let crop_h = h.abs() as u32;

            if crop_w == 0 || crop_h == 0 {
                return Err(format!("帧 #{} 裁剪尺寸无效：{}x{}", fi, crop_w, crop_h).into());
            }

            let x32 = if x >= 0.0 { x as u32 } else { 0 };
            let y32 = if y >= 0.0 { y as u32 } else { 0 };

            let cropped =
                if x32 == 0 && y32 == 0 && crop_w == sprite.width() && crop_h == sprite.height() {
                    sprite
                } else {
                    image::imageops::crop_imm(&sprite, x32, y32, crop_w, crop_h).to_image()
                };

            let ft = frame_info.frametime;
            if ft.is_nan() || ft.is_infinite() || ft < 0.0 {
                return Err(format!("帧 #{} 的 frametime 无效：{}", fi, ft).into());
            }
            let delay = image::Delay::from_numer_denom_ms((ft * 1000.0).round() as u32, 1);

            // 跳过首帧黑帧（GIF encoder 初始化时自动插入的），直接 encode 后续帧
            if fi == 0 {
                continue;
            }
            encoder.encode_frame(Frame::from_parts(cropped, 0, 0, delay))?;
        }
    } // encoder 在此 drop，gif_buf 完整

    Ok(ImageResult {
        bytes: gif_buf,
        format: MipmapFormat::ImageGIF,
    })
}

/// 判断 i32 是否为 2 的幂。
pub fn is_power_of_two(n: i32) -> bool {
    n > 0 && (n & (n - 1)) == 0
}

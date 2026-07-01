// LZ4 压缩/解压与 DXT 解码调度。
// 与 C# 原版 RePKG.Application/Texture/TexMipmapDecompressor.cs 和 TexMipmapCompressor.cs 对应。

use crate::dxt;
use crate::format::MipmapFormat;
use crate::tex::image::Mipmap;

/// 解压 mipmap：LZ4 解压 + DXT 解码（如果适用）。
pub fn decompress(mipmap: &Mipmap) -> Result<Mipmap, Box<dyn std::error::Error>> {
    let bytes = if mipmap.is_lz4_compressed {
        if mipmap.decompressed_bytes_count < 0 {
            return Err("LZ4 解压后字节数为负值".into());
        }
        lz4_flex::block::decompress(&mipmap.bytes, mipmap.decompressed_bytes_count as usize)?
    } else {
        mipmap.bytes.clone()
    };

    let (decoded_bytes, format) = match mipmap.format {
        MipmapFormat::CompressedDXT1 => (
            dxt::decompress_image(mipmap.width as u32, mipmap.height as u32, &bytes, 1)?,
            MipmapFormat::RGBA8888,
        ),
        MipmapFormat::CompressedDXT3 => (
            dxt::decompress_image(mipmap.width as u32, mipmap.height as u32, &bytes, 2)?,
            MipmapFormat::RGBA8888,
        ),
        MipmapFormat::CompressedDXT5 => (
            dxt::decompress_image(mipmap.width as u32, mipmap.height as u32, &bytes, 4)?,
            MipmapFormat::RGBA8888,
        ),
        _ => (bytes, mipmap.format),
    };

    Ok(Mipmap {
        bytes: decoded_bytes,
        width: mipmap.width,
        height: mipmap.height,
        decompressed_bytes_count: 0,
        is_lz4_compressed: false,
        format,
        condition_json: mipmap.condition_json.clone(),
    })
}

/// 压缩 mipmap：将未压缩数据使用 LZ4 压缩。
pub fn compress(mipmap: &mut Mipmap) -> Result<(), Box<dyn std::error::Error>> {
    if mipmap.is_lz4_compressed {
        return Ok(());
    }

    let max_size = lz4_flex::block::get_maximum_output_size(mipmap.bytes.len());
    let mut compressed = vec![0u8; max_size];
    let compressed_size = lz4_flex::block::compress_into(&mipmap.bytes, &mut compressed)?;

    compressed.truncate(compressed_size);

    mipmap.decompressed_bytes_count = mipmap.bytes.len() as i32;
    mipmap.bytes = compressed;
    mipmap.is_lz4_compressed = true;

    Ok(())
}

#[cfg(test)]
#[allow(non_snake_case)]
mod tests {
    use super::*;

    #[test]
    fn 测试_LZ4_压缩解压往返() {
        let original = vec![0xAAu8; 1024];

        let mut mipmap = Mipmap {
            bytes: original.clone(),
            width: 32,
            height: 32,
            decompressed_bytes_count: 0,
            is_lz4_compressed: false,
            format: MipmapFormat::RGBA8888,
            condition_json: None,
        };

        compress(&mut mipmap).unwrap();
        assert!(mipmap.is_lz4_compressed);
        assert!(mipmap.bytes.len() < original.len());

        let decompressed = decompress(&mipmap).unwrap();
        assert_eq!(decompressed.bytes, original);
        assert!(!decompressed.is_lz4_compressed);
    }

    #[test]
    fn 测试已压缩数据不重复压缩() {
        let compressed_data = lz4_flex::block::compress(&[0xAAu8; 256]);
        let mut mipmap = Mipmap {
            bytes: compressed_data,
            width: 16,
            height: 16,
            decompressed_bytes_count: 256,
            is_lz4_compressed: true,
            format: MipmapFormat::RGBA8888,
            condition_json: None,
        };

        let original_bytes = mipmap.bytes.clone();
        compress(&mut mipmap).unwrap();
        assert_eq!(mipmap.bytes, original_bytes);
    }

    #[test]
    fn 测试非压缩格式不解码() {
        let mipmap = Mipmap {
            bytes: vec![0u8; 64],
            width: 4,
            height: 4,
            decompressed_bytes_count: 0,
            is_lz4_compressed: false,
            format: MipmapFormat::RGBA8888,
            condition_json: None,
        };

        let result = decompress(&mipmap).unwrap();
        assert_eq!(result.format, MipmapFormat::RGBA8888);
        assert_eq!(result.bytes.len(), 64);
    }
}

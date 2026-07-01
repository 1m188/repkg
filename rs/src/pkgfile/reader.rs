// PKG 读取器，负责从二进制流中解析 PKG 文件。
// 与 C# 原版 RePKG.Application/Package/PackageReader.cs 对应。

use std::io::{self, Read};

use crate::binutil;

use super::{Entry, EntryType, Package};

/// PKG 文件安全常量。
const MAX_MAGIC_LENGTH: usize = 32;
const MAX_FILE_PATH_LENGTH: usize = 255;
const MAX_ENTRY_COUNT: usize = 100_000;

/// 从 reader 中读取并解析 PKG 文件。
pub fn read_package<R: Read>(reader: &mut R) -> io::Result<Package> {
    let magic = binutil::read_string_i32_size(reader, MAX_MAGIC_LENGTH)?;

    let mut buf = [0u8; 4];
    reader.read_exact(&mut buf)?;
    let entry_count_i32 = i32::from_le_bytes(buf);
    if entry_count_i32 < 0 {
        return Err(io::Error::new(
            io::ErrorKind::InvalidData,
            format!("条目数无效（负数）：{}", entry_count_i32),
        ));
    }
    let entry_count = entry_count_i32 as usize;
    if entry_count > MAX_ENTRY_COUNT {
        return Err(io::Error::new(
            io::ErrorKind::InvalidData,
            format!("条目数超限：{}（最大 {}）", entry_count, MAX_ENTRY_COUNT),
        ));
    }

    let mut entries = Vec::with_capacity(entry_count);
    for _ in 0..entry_count {
        let full_path = binutil::read_string_i32_size(reader, MAX_FILE_PATH_LENGTH)?;

        reader.read_exact(&mut buf)?;
        let offset = i32::from_le_bytes(buf);

        reader.read_exact(&mut buf)?;
        let length = i32::from_le_bytes(buf);

        if offset < 0 || length < 0 {
            return Err(io::Error::new(
                io::ErrorKind::InvalidData,
                format!(
                    "条目 \"{}\" 偏移或长度无效：offset={}, length={}",
                    full_path, offset, length
                ),
            ));
        }

        let entry_type = if full_path.to_lowercase().ends_with(".tex") {
            EntryType::Tex
        } else {
            EntryType::Binary
        };

        entries.push(Entry {
            full_path,
            offset,
            length,
            bytes: Vec::new(),
            entry_type,
        });
    }

    // 读取数据体
    let mut body = Vec::new();
    reader.read_to_end(&mut body)?;

    // 根据 offset 填充每条的 bytes
    for entry in entries.iter_mut() {
        let off = entry.offset as usize;
        let len = entry.length as usize;
        if off + len > body.len() {
            return Err(io::Error::new(
                io::ErrorKind::UnexpectedEof,
                format!(
                    "条目 \"{}\" 数据截断：偏移 {} + 长度 {} > 数据体长度 {}",
                    entry.full_path,
                    off,
                    len,
                    body.len()
                ),
            ));
        }
        entry.bytes = body[off..off + len].to_vec();
    }

    Ok(Package { magic, entries })
}

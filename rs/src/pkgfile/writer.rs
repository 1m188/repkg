// PKG 写入器，负责将 Package 结构序列化为 PKG 二进制数据。
// 与 C# 原版 RePKG.Application/Package/PackageWriter.cs 对应。

use std::io::{self, Write};

use byteorder::{LittleEndian, WriteBytesExt};

use crate::binutil;

use super::Package;

/// 将 Package 序列化写入 writer。
pub fn write_package<W: Write>(writer: &mut W, package: &Package) -> io::Result<()> {
    // 写入魔数字符串
    binutil::write_string_i32_size(writer, &package.magic)?;

    // 写入条目数
    writer.write_i32::<LittleEndian>(package.entries.len() as i32)?;

    // 写入条目头
    let mut running_offset: i32 = 0;
    for entry in &package.entries {
        binutil::write_string_i32_size(writer, &entry.full_path)?;
        writer.write_i32::<LittleEndian>(running_offset)?;
        writer.write_i32::<LittleEndian>(entry.length)?;
        running_offset = running_offset.checked_add(entry.length).ok_or_else(|| {
            io::Error::new(io::ErrorKind::InvalidData, "PKG 数据体总大小超过 i32 上限")
        })?;
    }

    // 写入数据体
    for entry in &package.entries {
        writer.write_all(&entry.bytes)?;
    }

    Ok(())
}

#[cfg(test)]
#[allow(non_snake_case)]
mod tests {
    use super::*;
    use crate::pkgfile::Entry;
    use std::io::Cursor;

    #[test]
    fn 测试写入空包() {
        let pkg = Package {
            magic: "PKGV0005".to_string(),
            entries: vec![],
        };
        let mut buf = Vec::new();
        write_package(&mut buf, &pkg).unwrap();
        assert!(buf.len() > 0);
    }

    #[test]
    fn 测试写入后读取往返() {
        let pkg = Package {
            magic: "PKGV0005".to_string(),
            entries: vec![
                Entry {
                    full_path: "test.txt".to_string(),
                    offset: 0,
                    length: 5,
                    bytes: b"hello".to_vec(),
                    entry_type: super::super::EntryType::Binary,
                },
                Entry {
                    full_path: "test.tex".to_string(),
                    offset: 5,
                    length: 4,
                    bytes: b"tex!".to_vec(),
                    entry_type: super::super::EntryType::Tex,
                },
            ],
        };

        let mut buf = Vec::new();
        write_package(&mut buf, &pkg).unwrap();

        let mut cursor = Cursor::new(buf);
        let read_pkg = super::super::reader::read_package(&mut cursor).unwrap();

        assert_eq!(read_pkg.magic, "PKGV0005");
        assert_eq!(read_pkg.entries.len(), 2);
        assert_eq!(read_pkg.entries[0].full_path, "test.txt");
        assert_eq!(read_pkg.entries[0].bytes, b"hello");
        assert_eq!(
            read_pkg.entries[0].entry_type,
            super::super::EntryType::Binary
        );
        assert_eq!(read_pkg.entries[1].full_path, "test.tex");
        assert_eq!(read_pkg.entries[1].bytes, b"tex!");
        assert_eq!(read_pkg.entries[1].entry_type, super::super::EntryType::Tex);
    }
}

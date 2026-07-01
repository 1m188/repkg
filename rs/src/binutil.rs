// 二进制 I/O 辅助函数，提供 null 结尾字符串和 int32 长度前缀字符串的读写。
// 与 C# 原版 Extensions.cs 对应。

use std::io::{self, Read, Write};

use byteorder::{LittleEndian, ReadBytesExt, WriteBytesExt};

/// 最大魔数字符串长度（用于 max_length=0 时的上限）。
const DEFAULT_MAX_STRING_LENGTH: usize = 256;

/// 从 reader 中读取一个 null 结尾的字符串。
/// max_length 为 0 时使用默认上限（256 字节）。
#[allow(clippy::unbuffered_bytes)]
pub fn read_n_string<R: Read>(reader: &mut R, max_length: usize) -> io::Result<String> {
    let limit = if max_length == 0 {
        DEFAULT_MAX_STRING_LENGTH
    } else {
        max_length
    };

    let bytes = reader
        .bytes()
        .take(limit)
        .take_while(|b| !matches!(b, Ok(0)))
        .collect::<Result<Vec<u8>, _>>()?;

    String::from_utf8(bytes).map_err(|e| io::Error::new(io::ErrorKind::InvalidData, e))
}

/// 向 writer 写入一个 null 结尾的字符串。
pub fn write_n_string<W: Write>(writer: &mut W, s: &str) -> io::Result<()> {
    writer.write_all(s.as_bytes())?;
    writer.write_all(&[0u8])?;
    Ok(())
}

/// 从 reader 中读取一个 int32 长度前缀的字符串。
/// max_length 为 0 时使用默认上限（256 字节）。
pub fn read_string_i32_size<R: Read>(reader: &mut R, max_length: usize) -> io::Result<String> {
    let size = reader.read_i32::<LittleEndian>()?;
    if size < 0 {
        return Err(io::Error::new(
            io::ErrorKind::InvalidData,
            "字符串长度不能为负数",
        ));
    }

    let size = size as usize;
    let limit = if max_length == 0 {
        DEFAULT_MAX_STRING_LENGTH
    } else {
        max_length
    };
    let effective_size = if size > limit { limit } else { size };

    let mut buf = vec![0u8; effective_size];
    reader.read_exact(&mut buf)?;

    // 跳过剩余字节（如果 size > effective_size）
    if size > effective_size {
        let skip = size - effective_size;
        let mut sink = std::io::sink();
        let mut limited = reader.take(skip as u64);
        std::io::copy(&mut limited, &mut sink)?;
    }

    String::from_utf8(buf).map_err(|e| io::Error::new(io::ErrorKind::InvalidData, e))
}

/// 向 writer 写入一个 int32 长度前缀的字符串。
pub fn write_string_i32_size<W: Write>(writer: &mut W, s: &str) -> io::Result<()> {
    let len: i32 = s.len().try_into().map_err(|_| {
        io::Error::new(
            io::ErrorKind::InvalidData,
            "字符串过长，无法以 i32 长度前缀写入",
        )
    })?;
    writer.write_i32::<LittleEndian>(len)?;
    writer.write_all(s.as_bytes())?;
    Ok(())
}

#[cfg(test)]
#[allow(non_snake_case)]
mod tests {
    use super::*;
    use std::io::Cursor;

    // ========== read_n_string 测试 ==========

    #[test]
    fn 测试读取正常_null_结尾字符串() {
        let data = b"hello\0world";
        let mut cursor = Cursor::new(data);
        let result = read_n_string(&mut cursor, 10).unwrap();
        assert_eq!(result, "hello");
    }

    #[test]
    fn 测试读取空_null_结尾字符串() {
        let data = b"\0extra";
        let mut cursor = Cursor::new(data);
        let result = read_n_string(&mut cursor, 10).unwrap();
        assert_eq!(result, "");
    }

    #[test]
    fn 测试读取字符串超过_max_length() {
        let data = b"helloworld\0";
        let mut cursor = Cursor::new(data);
        let result = read_n_string(&mut cursor, 5).unwrap();
        assert_eq!(result, "hello");
    }

    #[test]
    fn 测试不包含_null_时达到_max_length_截断() {
        let data = b"helloworld";
        let mut cursor = Cursor::new(data);
        let result = read_n_string(&mut cursor, 5).unwrap();
        assert_eq!(result, "hello");
    }

    #[test]
    fn 测试_max_length_为_0_时使用默认上限() {
        let mut long_str = vec![b'a'; 300];
        long_str.push(0);
        let mut cursor = Cursor::new(long_str);
        let result = read_n_string(&mut cursor, 0).unwrap();
        assert_eq!(result.len(), 256);
    }

    // ========== write_n_string 测试 ==========

    #[test]
    fn 测试写入并读取往返() {
        let mut buf = Vec::new();
        write_n_string(&mut buf, "hello").unwrap();
        assert_eq!(&buf, b"hello\0");

        let mut cursor = Cursor::new(buf);
        let result = read_n_string(&mut cursor, 10).unwrap();
        assert_eq!(result, "hello");
    }

    // ========== read_string_i32_size 测试 ==========

    #[test]
    fn 测试读取正常长度前缀字符串() {
        let mut buf = Vec::new();
        buf.write_i32::<LittleEndian>(5).unwrap();
        buf.extend(b"hello");
        let mut cursor = Cursor::new(buf);
        let result = read_string_i32_size(&mut cursor, 10).unwrap();
        assert_eq!(result, "hello");
    }

    #[test]
    fn 测试读取零长度字符串() {
        let buf = [0u8; 4].to_vec(); // i32 size = 0
        let mut cursor = Cursor::new(buf);
        let result = read_string_i32_size(&mut cursor, 10).unwrap();
        assert_eq!(result, "");
    }

    #[test]
    fn 测试负长度字符串返回错误() {
        let mut buf = Vec::new();
        buf.write_i32::<LittleEndian>(-1).unwrap();
        let mut cursor = Cursor::new(buf);
        let result = read_string_i32_size(&mut cursor, 10);
        assert!(result.is_err());
    }

    #[test]
    fn 测试字符串超过_max_length_时截断() {
        let mut buf = Vec::new();
        buf.write_i32::<LittleEndian>(10).unwrap();
        buf.extend(b"helloworld");
        let mut cursor = Cursor::new(buf);
        let result = read_string_i32_size(&mut cursor, 5).unwrap();
        assert_eq!(result, "hello");
    }

    #[test]
    fn 测试_max_length_为_0_时不限制长度() {
        let mut buf = Vec::new();
        buf.write_i32::<LittleEndian>(10).unwrap();
        buf.extend(b"helloworld");
        let mut cursor = Cursor::new(buf);
        let result = read_string_i32_size(&mut cursor, 0).unwrap();
        assert_eq!(result, "helloworld");
    }

    // ========== write_string_i32_size 测试 ==========

    #[test]
    fn 测试写入长度前缀字符串并读取往返() {
        let mut buf = Vec::new();
        write_string_i32_size(&mut buf, "hello").unwrap();

        let mut cursor = Cursor::new(buf);
        let result = read_string_i32_size(&mut cursor, 10).unwrap();
        assert_eq!(result, "hello");
    }

    #[test]
    fn 测试写入空字符串往返() {
        let mut buf = Vec::new();
        write_string_i32_size(&mut buf, "").unwrap();

        let mut cursor = Cursor::new(buf);
        let result = read_string_i32_size(&mut cursor, 10).unwrap();
        assert_eq!(result, "");
    }
}

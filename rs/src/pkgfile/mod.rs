// PKG 容器格式的读写与数据模型。
// 与 C# 原版 RePKG.Core/Package/ 和 RePKG.Application/Package/ 对应。

pub mod reader;
pub mod writer;

/// 条目类型枚举。
#[derive(Debug, Clone, Copy, PartialEq, Eq)]
pub enum EntryType {
    /// 二进制文件（.mat、.json 等）。
    Binary = 0,
    /// TEX 纹理文件。
    Tex = 1,
}

/// PKG 中的单个条目。
#[derive(Debug, Clone)]
pub struct Entry {
    /// 文件在 PKG 内的完整路径，如 "materials/sky.tex"。
    pub full_path: String,
    /// 数据体中的字节偏移。
    pub offset: i32,
    /// 数据长度（字节数）。
    pub length: i32,
    /// 原始数据字节。
    pub bytes: Vec<u8>,
    /// 条目类型。
    pub entry_type: EntryType,
}

/// PKG 文件顶层结构。
#[derive(Debug, Clone)]
pub struct Package {
    /// PKG 格式魔数，如 "PKGV0005"。
    pub magic: String,
    /// 所有条目。
    pub entries: Vec<Entry>,
}

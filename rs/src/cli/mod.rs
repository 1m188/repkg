// CLI 命令行处理（extract、info 子命令和交互模式）。
// 与 C# 原版 RePKG/Command/ 对应。

use std::path::Path;

use clap::Parser;

pub mod extract;
pub mod info;
pub mod interactive;

/// 检查文件是否为 PKG 格式（按扩展名）。
pub fn is_pkg_file(path: &Path) -> bool {
    path.extension()
        .map(|e| e.eq_ignore_ascii_case("pkg"))
        .unwrap_or(false)
}

/// 检查文件是否为 TEX 格式（按扩展名）。
pub fn is_tex_file(path: &Path) -> bool {
    path.extension()
        .map(|e| e.eq_ignore_ascii_case("tex"))
        .unwrap_or(false)
}

/// 顶层 CLI 命令枚举。
#[derive(Parser, Debug)]
#[command(
    name = "repkg",
    version = "0.1.0",
    about = "Wallpaper Engine PKG/TEX 解包与转换工具（Rust 移植）",
    long_about = "RePKG 是 Wallpaper Engine 自定义二进制格式（.pkg 和 .tex）的解包和转换工具。支持：
  - 解压 .pkg 打包文件，提取内部的纹理、模型、脚本等资源
  - 将 .tex 纹理转换为 PNG、GIF、MP4 等通用格式
  - 查看 .pkg 和 .tex 文件的元数据信息",
    after_help = "示例：
  repkg extract scene.pkg -o ./output
  repkg extract scene.pkg --copyproject --usename
  repkg info scene.pkg -e --sort -b size
  repkg interactive"
)]
pub enum Cli {
    /// 解压 PKG 文件或转换 TEX 文件。
    #[command(
        long_about = "从 .pkg 文件中提取所有资源，自动将 .tex 纹理转换为 PNG 图片格式。支持批量处理目录、扩展名过滤、复制项目信息等功能。"
    )]
    Extract(extract::ExtractArgs),
    /// 查看 PKG/TEX 文件的元数据信息。
    #[command(
        long_about = "显示 .pkg 文件的条目列表、.tex 文件的纹理格式和 mipmap 信息。支持条目排序、项目信息提取和标题过滤。"
    )]
    Info(info::InfoArgs),
}

/// 使用命令行参数运行 CLI（供 main 和 interactive 复用）。
pub fn run_cli(args: Vec<String>) -> Result<(), Box<dyn std::error::Error>> {
    match Cli::try_parse_from(args) {
        Ok(Cli::Extract(args)) => extract::run(args),
        Ok(Cli::Info(args)) => info::run(args),
        Err(e) => {
            if e.kind() == clap::error::ErrorKind::DisplayHelp
                || e.kind() == clap::error::ErrorKind::DisplayVersion
            {
                e.exit();
            }
            Err(e.into())
        }
    }
}

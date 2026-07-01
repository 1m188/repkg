// info 子命令，负责查看 PKG/TEX 文件的元数据信息。
// 与 C# 原版 RePKG/Command/Info.cs 对应。

use std::fs;
use std::io::BufReader;
use std::path::Path;

use crate::pkgfile;
use crate::tex;

use serde_json;
/// info 子命令参数。
#[derive(clap::Parser, Debug)]
pub struct InfoArgs {
    /// 输入文件或目录路径（.pkg 文件或 .tex 文件）。
    pub input: String,

    /// 按名称排序条目列表。
    #[arg(short = 's', long = "sort", help = "对条目列表排序")]
    pub sort: bool,

    /// 排序依据：name（名称）、extension（扩展名）或 size（大小）。
    #[arg(
        short = 'b',
        long = "sortby",
        default_value = "name",
        help = "排序字段：name/extension/size"
    )]
    pub sortby: String,

    /// 将输入目录视为 TEX 文件集合，显示每个 .tex 的信息。
    #[arg(short = 't', long = "tex", help = "将目录视为 TEX 文件集合")]
    pub tex: bool,

    /// 提取 project.json 中的字段（"*" 表示全部字段）。
    #[arg(
        short = 'p',
        long = "projectinfo",
        default_value = "",
        help = "提取 project.json 字段"
    )]
    pub projectinfo: String,

    /// 打印 PKG 文件中所有条目的列表。
    #[arg(short = 'e', long = "printentries", help = "打印条目列表")]
    pub printentries: bool,

    /// 按标题子串过滤（仅限 PKG 批量模式）。
    #[arg(long = "title-filter", default_value = "", help = "按标题子串过滤结果")]
    pub title_filter: String,
}

/// 执行 info 操作。
pub fn run(args: InfoArgs) -> Result<(), Box<dyn std::error::Error>> {
    let input_path = Path::new(&args.input);
    if !input_path.exists() {
        return Err(format!("输入路径不存在：{}", args.input).into());
    }

    if input_path.is_file() {
        if super::is_pkg_file(input_path) {
            info_pkg_file(input_path, &args)?;
        } else if super::is_tex_file(input_path) {
            info_tex_file(input_path)?;
        }
    } else if input_path.is_dir() {
        for entry in fs::read_dir(input_path)? {
            let entry = entry?;
            let path = entry.path();
            if path.is_file() {
                if super::is_pkg_file(&path) {
                    info_pkg_file(&path, &args)?;
                } else if args.tex && super::is_tex_file(&path) {
                    info_tex_file(&path)?;
                }
            }
        }
    }

    Ok(())
}

/// 显示 PKG 文件信息。
fn info_pkg_file(path: &Path, args: &InfoArgs) -> Result<(), Box<dyn std::error::Error>> {
    let file = fs::File::open(path)?;
    let mut reader = BufReader::new(file);
    let package = pkgfile::reader::read_package(&mut reader)?;

    println!("文件：{}", path.display());
    println!("  PKG 魔数：{}", package.magic);
    println!("  条目总数：{}", package.entries.len());

    // --projectinfo：提取 project.json 字段
    if !args.projectinfo.is_empty() {
        if let Some(proj_entry) = package
            .entries
            .iter()
            .find(|e| e.full_path.to_lowercase() == "project.json")
        {
            if let Ok(json) = serde_json::from_slice::<serde_json::Value>(&proj_entry.bytes) {
                if args.projectinfo == "*" {
                    if let Some(obj) = json.as_object() {
                        for (k, v) in obj {
                            println!("  [project.json] {} = {}", k, v);
                        }
                    }
                } else if let Some(val) = json.get(&args.projectinfo) {
                    println!("  [project.json] {} = {}", args.projectinfo, val);
                }
            }
        }
    }

    // --title-filter：按标题子串过滤
    if !args.title_filter.is_empty() {
        let matches = package
            .entries
            .iter()
            .find(|e| e.full_path.to_lowercase() == "project.json")
            .and_then(|e| serde_json::from_slice::<serde_json::Value>(&e.bytes).ok())
            .and_then(|json| {
                json.get("title")
                    .and_then(|v| v.as_str())
                    .map(|t| t.to_lowercase().contains(&args.title_filter.to_lowercase()))
            })
            .unwrap_or(false);
        if !matches {
            return Ok(());
        }
    }

    if args.printentries {
        let mut entries: Vec<_> = package.entries.iter().collect();

        if args.sort {
            match args.sortby.as_str() {
                "extension" => entries.sort_by_key(|e| {
                    Path::new(&e.full_path)
                        .extension()
                        .map(|x| x.to_string_lossy().to_string())
                        .unwrap_or_default()
                }),
                "size" => entries.sort_by_key(|e| e.length),
                _ => entries.sort_by_key(|e| e.full_path.clone()),
            }
        }

        for entry in &entries {
            let entry_type = match entry.entry_type {
                pkgfile::EntryType::Tex => "TEX",
                pkgfile::EntryType::Binary => "BIN",
            };
            println!(
                "  [{entry_type}] {path} ({size} B)",
                entry_type = entry_type,
                path = entry.full_path,
                size = entry.length
            );
        }
    }

    Ok(())
}

/// 显示 TEX 文件信息。
fn info_tex_file(path: &Path) -> Result<(), Box<dyn std::error::Error>> {
    let file = fs::File::open(path)?;
    let mut reader = BufReader::new(file);
    let tex = tex::reader::read_tex(&mut reader)?;

    println!("文件：{}", path.display());
    println!("  格式：{:?}", tex.header.format);
    println!(
        "  纹理尺寸：{}x{}",
        tex.header.texture_width, tex.header.texture_height
    );
    println!(
        "  图像尺寸：{}x{}",
        tex.header.image_width, tex.header.image_height
    );
    println!("  标志位：0x{:08X}", tex.header.flags);
    println!("  未知字段：{}", tex.header.unk_int0);
    println!(
        "  容器魔数：{}（版本 {:?}，图像格式 {:?}）",
        tex.images_container.magic, tex.images_container.version, tex.images_container.image_format
    );

    let total_images = tex.images_container.images.len();
    println!("  图像数量：{}", total_images);

    let total_mipmaps: usize = tex
        .images_container
        .images
        .iter()
        .map(|i| i.mipmaps.len())
        .sum();
    println!("  Mipmap 总数：{}", total_mipmaps);

    for (img_idx, image) in tex.images_container.images.iter().enumerate() {
        println!("  图像 #{}：{} 级 mipmap", img_idx, image.mipmaps.len());
        for (mip_idx, mipmap) in image.mipmaps.iter().enumerate() {
            let lz4_str = if mipmap.is_lz4_compressed {
                "LZ4"
            } else {
                "raw"
            };
            println!(
                "    Mip #{}: {}x{} ({}, {} B)",
                mip_idx,
                mipmap.width,
                mipmap.height,
                lz4_str,
                mipmap.bytes.len()
            );
        }
    }

    if let Some(ref frame_container) = tex.frame_info_container {
        println!("  GIF 帧数：{}", frame_container.frames.len());
        println!(
            "  GIF 尺寸：{}x{}",
            frame_container.gif_width, frame_container.gif_height
        );
    }

    Ok(())
}

// extract 子命令，负责解压 PKG 文件和转换 TEX 文件。
// 与 C# 原版 RePKG/Command/Extract.cs 对应。

use std::fs;
use std::io::{self, BufReader};
use std::path::{Path, PathBuf};

use crate::pkgfile;
use crate::tex;

/// extract 子命令参数。
#[derive(clap::Parser, Debug)]
pub struct ExtractArgs {
    /// 输入文件或目录路径（.pkg 文件或 .tex 文件，或包含这些文件的目录）。
    pub input: String,

    /// 输出目录。所有提取的文件将写入此目录。
    #[arg(
        short = 'o',
        long = "output",
        default_value = "./output",
        help = "输出目录路径，默认为 ./output"
    )]
    pub output: String,

    /// 忽略的文件扩展名（逗号分隔，不含点号）。如 "tex,mat"。
    #[arg(
        short = 'i',
        long = "ignoreexts",
        default_value = "",
        help = "跳过指定扩展名的文件（逗号分隔）"
    )]
    pub ignoreexts: String,

    /// 仅提取指定的扩展名（逗号分隔，不含点号）。如 "tex,json"。
    #[arg(
        short = 'e',
        long = "onlyexts",
        default_value = "",
        help = "仅提取指定扩展名的文件（逗号分隔）"
    )]
    pub onlyexts: String,

    /// 将输入目录视为 TEX 文件集合，批量转换为图片。
    #[arg(short = 't', long = "tex", help = "将目录中的 .tex 文件批量转换为图片")]
    pub tex: bool,

    /// 所有文件输出到同一目录，不保留子目录结构。
    #[arg(short = 's', long = "singledir", help = "平铺输出，不保留子目录结构")]
    pub singledir: bool,

    /// 递归搜索子目录中的所有 .pkg 文件。
    #[arg(short = 'r', long = "recursive", help = "递归搜索子目录")]
    pub recursive: bool,

    /// 同时复制 project.json 和预览图片到输出目录。
    #[arg(short = 'c', long = "copyproject", help = "复制项目配置文件到输出目录")]
    pub copyproject: bool,

    /// 使用 project.json 的 title 字段作为输出子目录名。
    #[arg(short = 'n', long = "usename", help = "以项目标题创建子目录")]
    pub usename: bool,

    /// 跳过 TEX 纹理转换，以原始 .tex 格式输出。
    #[arg(long = "no-tex-convert", help = "不转换 TEX 文件，保留原始格式")]
    pub no_tex_convert: bool,

    /// 覆盖已存在的输出文件。
    #[arg(long = "overwrite", help = "覆盖已存在的文件")]
    pub overwrite: bool,
}

/// 执行 extract 操作。
pub fn run(args: ExtractArgs) -> Result<(), Box<dyn std::error::Error>> {
    let input_path = Path::new(&args.input);
    if !input_path.exists() {
        return Err(format!("输入路径不存在：{}", args.input).into());
    }

    let output_dir = Path::new(&args.output);
    fs::create_dir_all(output_dir)?;

    if input_path.is_dir() && args.tex {
        extract_tex_directory(input_path, output_dir, &args)?;
    } else if input_path.is_dir() {
        extract_pkg_directory(input_path, output_dir, &args)?;
    } else if super::is_pkg_file(input_path) {
        extract_pkg_file(input_path, output_dir, &args)?;
    } else if super::is_tex_file(input_path) {
        extract_tex_file(input_path, output_dir)?;
    } else {
        return Err(format!("不支持的文件类型：{}", args.input).into());
    }

    Ok(())
}

/// 执行 extract 操作。
fn extract_pkg_file(
    path: &Path,
    output_dir: &Path,
    args: &ExtractArgs,
) -> Result<(), Box<dyn std::error::Error>> {
    let file = fs::File::open(path)?;
    let mut reader = BufReader::new(file);
    let package = pkgfile::reader::read_package(&mut reader)?;

    let mut out_dir = output_dir.to_path_buf();

    // --usename：读取 project.json 中的 title 并创建子目录
    if args.usename {
        if let Some(project_entry) = package
            .entries
            .iter()
            .find(|e| e.full_path.to_lowercase() == "project.json")
        {
            if let Ok(json) = serde_json::from_slice::<serde_json::Value>(&project_entry.bytes) {
                if let Some(title) = json.get("title").and_then(|v| v.as_str()) {
                    let safe = title
                        .chars()
                        .map(|c| {
                            if c.is_alphanumeric() || c == '_' || c == '-' || c == ' ' {
                                c
                            } else {
                                '_'
                            }
                        })
                        .collect::<String>();
                    out_dir = out_dir.join(safe);
                }
            }
        }
    }

    // --copyproject：提取 project.json 到输出目录
    if args.copyproject {
        if let Some(project_entry) = package
            .entries
            .iter()
            .find(|e| e.full_path.to_lowercase() == "project.json")
        {
            let proj_path = out_dir.join("project.json");
            fs::create_dir_all(&out_dir)?;
            fs::write(&proj_path, &project_entry.bytes)?;
            println!("  项目信息：{}", proj_path.display());
        }
    }

    fs::create_dir_all(&out_dir)?;

    println!(
        "PKG 魔数：{}，条目数：{}",
        package.magic,
        package.entries.len()
    );

    for entry in &package.entries {
        if skip_entry(entry, args) {
            continue;
        }

        let relative_path = if args.singledir {
            Path::new(&entry.full_path)
                .file_name()
                .map(PathBuf::from)
                .unwrap_or_else(|| PathBuf::from(&entry.full_path))
        } else {
            PathBuf::from(&entry.full_path)
        };

        let out_path = out_dir.join(&relative_path);
        if let Some(parent) = out_path.parent() {
            fs::create_dir_all(parent)?;
        }

        if args.overwrite || !out_path.exists() {
            fs::write(&out_path, &entry.bytes)?;

            // TEX 转换
            if entry.entry_type == pkgfile::EntryType::Tex && !args.no_tex_convert {
                match tex::reader::read_tex(&mut io::Cursor::new(&entry.bytes)) {
                    Ok(tex_data) => {
                        // GIF 动画：使用多帧转换
                        if let Some(container) = &tex_data.frame_info_container {
                            match tex::converter::convert_to_gif(
                                &tex_data.images_container.images,
                                container,
                            ) {
                                Ok(result) => {
                                    let img_path = out_path.with_extension("gif");
                                    fs::write(&img_path, &result.bytes)?;
                                    println!(
                                        "  转换：{} → {}（GIF {} 帧）",
                                        entry.full_path,
                                        img_path.display(),
                                        container.frames.len()
                                    );
                                }
                                Err(e) => {
                                    eprintln!("  警告：转换 GIF {} 失败：{}", entry.full_path, e);
                                }
                            }
                        } else if let Some(image) = tex_data.images_container.images.first() {
                            if let Some(mipmap) = image.mipmaps.first() {
                                match tex::converter::convert_to_image(mipmap) {
                                    Ok(result) => {
                                        let ext = match result.format {
                                            crate::format::MipmapFormat::ImageJPEG => "jpg",
                                            crate::format::MipmapFormat::ImagePNG => "png",
                                            crate::format::MipmapFormat::ImageGIF => "gif",
                                            crate::format::MipmapFormat::VideoMp4 => "mp4",
                                            _ => "png",
                                        };
                                        let img_path = out_path.with_extension(ext);
                                        fs::write(&img_path, &result.bytes)?;
                                        println!(
                                            "  转换：{} → {}",
                                            entry.full_path,
                                            img_path.display()
                                        );
                                    }
                                    Err(e) => {
                                        eprintln!("  警告：转换 {} 失败：{}", entry.full_path, e);
                                    }
                                }
                            }
                        }
                    }
                    Err(e) => {
                        eprintln!("  警告：读取 {} 失败：{}", entry.full_path, e);
                    }
                }
            }

            println!("  提取：{}", entry.full_path);
        }
    }

    Ok(())
}

/// 跳过应忽略的条目。
fn skip_entry(entry: &pkgfile::Entry, args: &ExtractArgs) -> bool {
    let ext = Path::new(&entry.full_path)
        .extension()
        .map(|e| e.to_string_lossy().to_string())
        .unwrap_or_default();

    if !args.ignoreexts.is_empty() {
        let ignored: Vec<&str> = args.ignoreexts.split(',').map(|s| s.trim()).collect();
        if ignored.iter().any(|i| *i == ext) {
            return true;
        }
    }

    if !args.onlyexts.is_empty() {
        let allowed: Vec<&str> = args.onlyexts.split(',').map(|s| s.trim()).collect();
        if !allowed.iter().any(|a| *a == ext) {
            return true;
        }
    }

    false
}

/// 提取单个 TEX 文件（转换）。
fn extract_tex_file(path: &Path, output_dir: &Path) -> Result<(), Box<dyn std::error::Error>> {
    let file = fs::File::open(path)?;
    let mut reader = BufReader::new(file);
    let tex = tex::reader::read_tex(&mut reader)?;

    println!(
        "TEX 格式: {:?}, 尺寸: {}x{}",
        tex.header.format, tex.header.texture_width, tex.header.texture_height
    );

    let out_name = path.file_stem().unwrap_or_default().to_string_lossy();

    // GIF 动画：使用多帧转换
    if let Some(container) = &tex.frame_info_container {
        match tex::converter::convert_to_gif(&tex.images_container.images, container) {
            Ok(result) => {
                let out_path = output_dir.join(format!("{}.gif", out_name));
                fs::write(&out_path, &result.bytes)?;
                println!(
                    "  转换：{} → {}（GIF {} 帧）",
                    path.display(),
                    out_path.display(),
                    container.frames.len()
                );
            }
            Err(e) => {
                return Err(format!("转换 GIF {} 失败：{}", path.display(), e).into());
            }
        }
        return Ok(());
    }

    if let Some(image) = tex.images_container.images.first() {
        if let Some(mipmap) = image.mipmaps.first() {
            let result = tex::converter::convert_to_image(mipmap)?;
            let ext = match result.format {
                crate::format::MipmapFormat::ImageJPEG => "jpg",
                crate::format::MipmapFormat::ImagePNG => "png",
                crate::format::MipmapFormat::ImageGIF => "gif",
                crate::format::MipmapFormat::VideoMp4 => "mp4",
                _ => "png",
            };
            let out_path = output_dir.join(format!("{}.{}", out_name, ext));
            fs::write(&out_path, &result.bytes)?;
            println!("  转换：{} → {}", path.display(), out_path.display());
        }
    }

    Ok(())
}

/// 批量处理目录中的 TEX 文件。
fn extract_tex_directory(
    dir: &Path,
    output_dir: &Path,
    args: &ExtractArgs,
) -> Result<(), Box<dyn std::error::Error>> {
    for entry in fs::read_dir(dir)? {
        let entry = entry?;
        let path = entry.path();
        if path.is_file() && super::is_tex_file(&path) {
            extract_tex_file(&path, output_dir)?;
        } else if args.recursive && path.is_dir() {
            let dir_name = path.file_name().unwrap_or_default().to_string_lossy();
            let sub_out = output_dir.join(dir_name.as_ref());
            fs::create_dir_all(&sub_out)?;
            extract_tex_directory(&path, &sub_out, args)?;
        }
    }
    Ok(())
}

/// 批量处理目录中的 PKG 文件。
fn extract_pkg_directory(
    dir: &Path,
    output_dir: &Path,
    args: &ExtractArgs,
) -> Result<(), Box<dyn std::error::Error>> {
    for entry in fs::read_dir(dir)? {
        let entry = entry?;
        let path = entry.path();
        if path.is_file() && super::is_pkg_file(&path) {
            let pkg_name = path.file_stem().unwrap_or_default().to_string_lossy();
            let out = output_dir.join(pkg_name.as_ref());
            fs::create_dir_all(&out)?;
            println!("解压：{}", path.display());
            extract_pkg_file(&path, &out, args)?;
        } else if args.recursive && path.is_dir() {
            let dir_name = path.file_name().unwrap_or_default().to_string_lossy();
            let sub_out = output_dir.join(dir_name.as_ref());
            extract_pkg_directory(&path, &sub_out, args)?;
        }
    }
    Ok(())
}

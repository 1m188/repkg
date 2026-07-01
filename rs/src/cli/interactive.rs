// 交互模式 REPL，循环读取标准输入并复用 clap 解析命令。
// 与 C# 原版 RePKG/Program.cs 的 InteractiveConsole 方法对应。

use std::io::{self, BufRead, Write};

/// 将带引号的参数字符串分割为 Vec<String>。
fn split_arguments(input: &str) -> Vec<String> {
    let mut args = Vec::new();
    let mut current = String::new();
    let mut in_single = false;
    let mut in_double = false;

    for ch in input.chars() {
        match ch {
            '\'' if !in_double => in_single = !in_single,
            '"' if !in_single => in_double = !in_double,
            ' ' if !in_single && !in_double => {
                if !current.is_empty() {
                    args.push(std::mem::take(&mut current));
                }
            }
            _ => current.push(ch),
        }
    }
    if !current.is_empty() {
        args.push(current);
    }

    args
}

/// 运行交互模式 REPL。
pub fn run() -> Result<(), Box<dyn std::error::Error>> {
    println!("RePKG 交互模式（输入 help 查看命令，exit 退出）");

    let stdin = io::stdin();
    let mut stdout = io::stdout();

    loop {
        write!(stdout, "> ")?;
        stdout.flush()?;

        let mut line = String::new();
        if stdin.lock().read_line(&mut line)? == 0 {
            break; // EOF
        }

        let line = line.trim();
        if line.is_empty() {
            continue;
        }
        if line == "exit" || line == "quit" {
            break;
        }

        let mut args = vec!["repkg".to_string()];
        args.extend(split_arguments(line));

        // 解析并执行命令
        match super::run_cli(args) {
            Ok(()) => {}
            Err(e) => eprintln!("错误：{}", e),
        }
    }

    println!("再见！");
    Ok(())
}

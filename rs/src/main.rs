// main 是 repkg CLI 工具的入口，支持 extract/info 子命令和交互模式。
// 与 C# 原版 RePKG/Program.cs 对应。

use repkg::cli;

fn main() {
    // 注册 Ctrl+C 信号处理
    let _ = ctrlc::set_handler(|| {
        eprintln!("\n收到中断信号，正在退出...");
        std::process::exit(0);
    });

    let args: Vec<String> = std::env::args().collect();

    // 无参数或 interactive → 交互模式
    if args.len() <= 1 || args.get(1).is_some_and(|a| a == "interactive") {
        if let Err(e) = cli::interactive::run() {
            eprintln!("交互模式错误：{}", e);
            std::process::exit(1);
        }
        return;
    }

    match cli::run_cli(args) {
        Ok(()) => {}
        Err(e) => {
            eprintln!("错误：{}", e);
            std::process::exit(1);
        }
    }
}

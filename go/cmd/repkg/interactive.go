// 本文件提供交互模式，从标准输入逐行读取命令并执行。
package main

import (
	"bufio"
	"fmt"
	"os"
	"strings"
)

// interactiveMode 启动交互模式。
// 从标准输入逐行读取命令，通过 Cobra parser 解析所有 flags 后执行。
func interactiveMode() {
	_, _ = fmt.Println("RePKG 交互模式已启动。输入命令操作：")
	_, _ = fmt.Println("输入 \"help\" 查看帮助，输入 \"exit\" 退出")

	scanner := bufio.NewScanner(os.Stdin)
	for !closing.Load() {
		_, _ = fmt.Print("> ")
		if !scanner.Scan() {
			break
		}
		line := scanner.Text()
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		if line == "exit" || line == "quit" {
			break
		}

		args := splitArgs(line)
		if len(args) == 0 {
			continue
		}

		// 通过 Cobra parser 完整解析 flags（与 C# 行为一致）
		rootCmd := newRootCommand()
		rootCmd.SetArgs(args)
		_ = rootCmd.Execute() //nolint:errcheck // 交互模式下忽略执行错误

		// 检查 Ctrl+C 中断
		if closing.Load() {
			break
		}
	}

	err := scanner.Err()
	if err != nil {
		_, _ = fmt.Fprintf(os.Stderr, "读取输入失败: %v\n", err)
	}
	_, _ = fmt.Println("退出交互模式")
}

// splitArgs 将命令行字符串分割为参数列表（支持引号）。
func splitArgs(line string) []string {
	var args []string
	var current strings.Builder
	inQuote := false
	quoteChar := byte(0)

	for i := range len(line) {
		ch := line[i]
		if inQuote {
			if ch == quoteChar {
				inQuote = false
			} else {
				_ = current.WriteByte(ch)
			}
		} else {
			switch ch {
			case '"', '\'':
				inQuote = true
				quoteChar = ch
			case ' ':
				if current.Len() > 0 {
					args = append(args, current.String())
					current.Reset()
				}
			default:
				_ = current.WriteByte(ch)
			}
		}
	}
	if current.Len() > 0 {
		args = append(args, current.String())
	}
	return args
}

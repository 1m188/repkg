// Package main 是 RePKG Go 移植版本的 CLI 入口。
// 提供 extract（提取/转换）和 info（查看信息）两个子命令。
package main

import (
	"fmt"
	"os"
	"os/signal"
	"sync/atomic"
	"syscall"

	"github.com/spf13/cobra"
)

// closing 为全局终止标志。Ctrl+C 时被设置为 true，各处理函数检查此标志以提前退出。
var closing atomic.Bool

func init() {
	// 注册 SIGINT/SIGTERM 信号处理器
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, os.Interrupt, syscall.SIGTERM)
	go func() {
		<-sigCh
		closing.Store(true)
		_, _ = fmt.Fprintln(os.Stderr, "\n正在终止...")
	}()
}

var (
	// extract 命令选项
	outputDir    string
	ignoreExts   string
	onlyExts     string
	texDir       bool
	singleDir    bool
	recursive    bool
	copyProject  bool
	useName      bool
	noTexConvert bool
	overwrite    bool

	// info 命令选项
	sortEntries  bool
	sortBy       string
	texInfo      bool
	projectInfo  string
	printEntries bool
	titleFilter  string
)

func main() {
	rootCmd := newRootCommand()
	err := rootCmd.Execute()
	if err != nil {
		_, _ = fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

// newRootCommand 创建 Cobra 根命令。
// 供交互模式和测试复用。
func newRootCommand() *cobra.Command {
	rootCmd := &cobra.Command{
		Use:   "repkg",
		Short: "RePKG - Wallpaper Engine PKG 解包 / TEX 转换工具 (Go 移植版)",
		Long: `RePKG 是一个用于 Wallpaper Engine .pkg 文件解包和 .tex 纹理文件转换的命令行工具。

支持功能：
  - 提取 PKG 文件中的所有条目
  - 将 TEX 纹理转换为 PNG/GIF/MP4
  - 查看 PKG/TEX 文件的元数据信息
  - 交互模式`,
		Run: func(c *cobra.Command, args []string) {
			if len(args) > 0 && args[0] == "interactive" {
				interactiveMode()
				return
			}
			_ = c.Help() //nolint:errcheck // CLI 入口调用 Help() 无需检查错误
		},
	}

	// extract 子命令
	extractCmd := &cobra.Command{
		Use:   "extract <文件或目录>",
		Short: "提取 PKG 文件或转换 TEX 纹理为图片",
		Long: `提取 PKG 文件中的所有条目，并将 TEX 纹理转换为标准图片格式。

示例：
  repkg extract scene.pkg
  repkg extract -c ~/wallpapers/workshop/123/
  repkg extract -e tex -s -o ./output ~/wallpapers/`,
		Args: cobra.ExactArgs(1),
		Run: func(_ *cobra.Command, args []string) {
			actionExtract(args[0])
		},
	}
	extractCmd.Flags().StringVarP(&outputDir, "output", "o", "./output", "输出目录")
	extractCmd.Flags().StringVarP(&ignoreExts, "ignoreexts", "i", "", "不提取指定扩展名的文件（逗号分隔）")
	extractCmd.Flags().StringVarP(&onlyExts, "onlyexts", "e", "", "仅提取指定扩展名的文件（逗号分隔）")
	extractCmd.Flags().BoolVarP(&texDir, "tex", "t", false, "将指定目录下所有 TEX 文件转换为图片")
	extractCmd.Flags().BoolVarP(&singleDir, "singledir", "s", false, "所有文件放入同一目录而非按条目路径存放")
	extractCmd.Flags().BoolVarP(&recursive, "recursive", "r", false, "递归搜索所有子目录")
	extractCmd.Flags().BoolVarP(&copyProject, "copyproject", "c", false, "将 project.json 和预览图复制到输出目录")
	extractCmd.Flags().BoolVarP(&useName, "usename", "n", false, "使用 project.json 中的名称作为子目录名")
	extractCmd.Flags().BoolVar(&noTexConvert, "no-tex-convert", false, "解压 PKG 时不转换 TEX 文件")
	extractCmd.Flags().BoolVar(&overwrite, "overwrite", false, "覆盖所有已存在的文件")

	// info 子命令
	infoCmd := &cobra.Command{
		Use:   "info <文件或目录>",
		Short: "查看 PKG/TEX 文件信息",
		Long: `查看 PKG 包的条目列表或 TEX 文件的元数据信息。

示例：
  repkg info -e scene.pkg
  repkg info -p "*" scene.pkg`,
		Args: cobra.ExactArgs(1),
		Run: func(_ *cobra.Command, args []string) {
			actionInfo(args[0])
		},
	}
	infoCmd.Flags().BoolVarP(&sortEntries, "sort", "s", false, "按名称排序条目")
	infoCmd.Flags().StringVarP(&sortBy, "sortby", "b", "name", "排序方式（name/extension/size）")
	infoCmd.Flags().BoolVarP(&texInfo, "tex", "t", false, "输出目录下所有 TEX 文件的信息")
	infoCmd.Flags().StringVarP(&projectInfo, "projectinfo", "p", "", "要输出的 project.json 字段（逗号分隔，* 表示全部）")
	infoCmd.Flags().BoolVarP(&printEntries, "printentries", "e", false, "输出 PKG 包的条目列表")
	infoCmd.Flags().StringVar(&titleFilter, "title-filter", "", "标题过滤（仅显示匹配的）")

	rootCmd.AddCommand(extractCmd, infoCmd)
	return rootCmd
}

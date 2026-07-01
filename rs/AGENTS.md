# RePKG Rust 移植项目

## 项目概述

本项目将 C# 实现的 [RePKG](https://github.com/1m188/repkg.git)（Wallpaper Engine PKG 解包 / TEX 纹理转换工具）完整移植到 Rust 语言。

### 功能

- **extract** — 解压 `.pkg` 文件，提取其中的所有条目（纹理、模型、脚本等）
- **info** — 查看 `.pkg` / `.tex` 文件的元数据信息
- **TEX 转图片** — 将 Wallpaper Engine 专用 `.tex` 纹理格式转换为 PNG/GIF/MP4
- **PKG/TEX 写回** — 支持将修改后的数据重新打包为 `.pkg` / `.tex` 格式

### 参考基准

- **C# 原版**：主要权威参考，枚举值、算法细节、行为语义均对齐 C# 原版
- **Go 移植版**：辅助参考，保留 Go 版增强功能（V4 TEX 写入、info TEX 信息展示）

---

## 架构设计

### 目录结构

```
rs/
├── Cargo.toml                     # 单 crate，bin + lib
├── AGENTS.md                      # 本文件（项目说明书）
├── README.md
├── src/
│   ├── main.rs                    # clap CLI 入口 + 信号处理 + 交互模式
│   ├── lib.rs                     # 公共 API 重导出
│   ├── cli/
│   │   ├── mod.rs                 # CLI 模块
│   │   ├── extract.rs            # extract 子命令
│   │   ├── info.rs               # info 子命令
│   │   └── interactive.rs        # 交互模式 REPL
│   ├── error.rs                   # 自定义错误类型
│   ├── binutil.rs                 # 二进制 I/O 辅助函数
│   ├── dxt.rs                     # DXT1/3/5 块解码算法
│   ├── format.rs                  # 枚举定义（MipmapFormat、TexFormat 等）
│   ├── pkgfile/
│   │   ├── mod.rs                 # PKG 数据模型
│   │   ├── reader.rs             # PKG 读取器
│   │   └── writer.rs             # PKG 写入器
│   └── tex/
│       ├── mod.rs                 # TEX 顶层模型
│       ├── header.rs             # Header + 魔数常量 + Flags
│       ├── image.rs              # Image/Mipmap 数据模型 + V1-V4 读写
│       ├── container.rs          # ImageContainer 读写
│       ├── frame.rs              # FrameInfo/FrameInfoContainer 读写
│       ├── reader.rs             # TEX 顶层读取器
│       ├── writer.rs             # TEX 顶层写入器
│       ├── compressor.rs         # LZ4 压缩/解压 + DXT 解码调度
│       ├── converter.rs          # TEX → PNG/GIF/MP4 转换器
│       └── json_info.rs          # .tex-json 元数据生成
├── tests/
│   └── integration_test.rs       # 集成测试
├── testdata/
│   └── test.pkg                   # 实景测试数据（约 5.8 MB）
└── benches/
    └── dxt_bench.rs               # DXT 性能基准（可选）
```

### 分层职责

| 层 | 路径 | 对应 C# | 职责 |
|----|------|---------|------|
| 入口层 | `src/main.rs` `src/cli/` | RePKG/ | 命令行参数解析、子命令处理、交互模式 |
| 业务逻辑层 | `src/tex/` `src/pkgfile/` | RePKG.Application | TEX/PKG 的读写、解压、转换等核心逻辑 |
| 纯算法层 | `src/dxt.rs` | DXT.cs | DXT 块解码（零外部依赖，可独立测试） |
| 工具层 | `src/binutil.rs` | Extensions.cs | 二进制流便捷读写方法 |
| 公共模型层 | `src/format.rs` | RePKG.Core 枚举 | 对外暴露的格式枚举定义 |

---

## 技术选型

| 需求 | 选型 | 理由 |
|------|------|------|
| LZ4 压缩/解压 | `lz4_flex` | 纯 Rust 实现，无需 C 编译，块模式 API 直接匹配 |
| CLI 命令行 | `clap` v4 (derive) | Rust 生态标准 CLI 框架，子命令+flags+自动 help |
| PNG/GIF 编码 | `image` crate | Rust 生态最成熟的图像库 |
| 二进制 I/O | `byteorder` + `std::io::Read` | 小端序读取，与 C# BinaryReader 对等 |
| 错误类型 | `thiserror` | derive(Error) 宏，减少样板代码 |
| 信号处理 | `ctrlc` | 跨平台 Ctrl+C 处理 |
| 测试 | `#[test]`（内置） | Rust 原生，表驱动测试 |

### 模块对照表

```
C# RePKG.Application.Exceptions          →  rs/src/error.rs
C# RePKG.Application.Extensions          →  rs/src/binutil.rs
C# RePKG.Application.Package             →  rs/src/pkgfile/
C# RePKG.Application.Texture             →  rs/src/tex/
C# RePKG.Application.Helpers.DXT         →  rs/src/dxt.rs
C# RePKG.Core.Package                    →  rs/src/pkgfile/mod.rs
C# RePKG.Core.Texture.Enums             →  rs/src/format.rs
C# RePKG.Core.Texture                    →  rs/src/tex/header.rs + image.rs
C# RePKG.Command                         →  rs/src/cli/
C# RePKG.Program                         →  rs/src/main.rs
```

---

## 测试策略

### 测试分层

所有测试通过 `cargo test` 一键运行。

#### 第一层：单元测试（内存自建数据）

不依赖任何外部文件，所有测试数据在内存中构造。使用 `#[cfg(test)] mod tests` 内嵌测试。

| 被测模块 | 测试内容 | 对应 C# 测试 |
|----------|----------|--------------|
| `binutil.rs` | ReadNString（空串、含 null、超长）、Write 往返 | - |
| `dxt.rs` | DXT1/3/5 单块解码、全图、非 4 对齐尺寸、alpha 极值 | TexDecompressingTests |
| `format.rs` | 枚举值验证、IsCompressed/IsImage/IsRawFormat 方法 | - |
| `pkgfile/` | 空包、单条多条目、大条目、写入→读取往返 | PkgWriterTests |
| `tex/` | V1-V4 各版本各格式解压、错误魔数拒绝、非法 Format 拒绝 | TexDecompressingTests |
| `tex/` | V1-V4 写入→读取往返、字节一致 | TexWriterTests |

#### 第二层：集成测试（test.pkg 实景数据）

| 被测功能 | 测试文件 | 测试内容 |
|----------|----------|----------|
| PKG 全流程 | `tests/integration_test.rs` | 读取 test.pkg → 提取所有条目 → TEX 转换为图片 |

### 测试编写原则

1. **表驱动测试** — 使用 `Vec<(name, input, want)>` 模式
2. **自描述** — 每个 test case 有清晰的名称和注释说明测试目的
3. **全覆盖** — 主流路径、边界条件、异常输入全部覆盖
4. **无外部依赖** — 单元测试不读文件，不连网络

---

## 开发工作流

所有开发者必须严格遵循此工作流。违反工作流的代码不得合入。

### 开发方法：测试驱动开发（TDD）

```
明确需求与设计
  ↓
[RED] 编写测试代码（预期失败）        # 阶段 1: 用测试定义功能行为
  ↓
[GREEN] 编写最小实现代码（通过测试）   # 阶段 2: 只写让测试通过的代码
  ↓
[REFACTOR] 优化代码结构、添加注释       # 阶段 3: 重构但不改变行为
  ↓
提交
```

### 工作流步骤

```
[RED] 编写测试代码
  ↓
cargo test（预期失败）                 # 步骤 1: 验证测试有效
  ↓
[GREEN] 编写功能实现代码
  ↓
cargo fmt --check ./...                # 步骤 2: 代码格式化
  ↓
cargo clippy -- -D warnings            # 步骤 3: 静态分析
  ↓ 有告警 → 修复代码 → 回到步骤 2
  ↓ 无告警
cargo test                             # 步骤 4: 测试应全部通过
  ↓ 有失败 → 修复代码 → 回到步骤 2
  ↓ 全部通过
[REFACTOR] 优化代码 + 补全注释
  ↓
同步更新注释和文档                     # 步骤 5: 检查所有注释与代码一致
  ↓
cargo test                             # 步骤 6: 重构后确认测试通过
  ↓
cargo build --release                  # 步骤 7: 构建可执行文件
  ↓
完成 ✓
```

### 快速参考

```bash
# 完整工作流
cargo fmt --check && cargo clippy -- -D warnings && cargo test && cargo build --release

# 仅测试
cargo test

# 仅 lint
cargo clippy -- -D warnings

# 仅格式化
cargo fmt

# 构建发布版
cargo build --release
```

---

## 注释与文档撰写规范

### 注释语言

- 所有注释使用**简体中文**
- 技术术语保留英文（如 DXT1、RGBA8888、mipmap）

### 注释层级

```
层级 1: 文件头注释（// 行注释块）
   位置: 每个 .rs 文件顶部
   内容: 描述本文件/模块的整体功能和职责

层级 2: 类型注释（/// 文档注释）
   位置: 每个公开 struct/enum 定义之前
   内容: 描述该类型的用途和设计意图

层级 3: 函数/方法注释（/// 文档注释）
   位置: 每个公开的函数或方法定义之前
   内容: 描述功能、参数含义、返回值、注意事项

层级 4: 常量/变量注释
   位置: 每个公开的常量或变量定义之前或同行
   内容: 描述用途和取值含义

层级 5: 内部逻辑注释
   位置: 复杂算法、非直观逻辑、边界条件处理处
   内容: 解释为什么这样写
```

---

## 代码审查：多视角分层审查（MPLR）

### 四层审查

```
L1 运行期安全 (Crash Layer)
  → 会 panic 吗？会 OOM 吗？会死循环吗？

L2 数据完整性 (Data Layer)
  → Reader 和 Writer 格式对称吗？字段顺序一致吗？

L3 边界与异常 (Boundary Layer)
  → 零值、空值、负数、超大值、截断数据每种情况的处理是否正确？

L4 语义正确性 (Semantic Layer)
  → 行为与 C# 原版一致吗？错误类型准确吗？
```

### 审查检查清单

```
□ L1 [运行期安全]
  - 所有数组/切片索引是否验证了边界？
  - 所有 .unwrap() / .expect() 是否合理？
  - 所有 alloc 分配的大小是否可能为负数或超大值？
  - 是否有潜在的整数溢出？

□ L2 [数据完整性]
  - Reader 读取的字段顺序 == Writer 写入的字段顺序？
  - Reader 读取的魔数 == Writer 写入的魔数？
  - 写入后重读能否字节一致？

□ L3 [边界]
  - len=0 时是否正常？
  - 字段值为负 / max 时是否被正确处理？
  - 输入数据截断时是否返回错误而非 panic？

□ L4 [语义]
  - 与 C# 同名函数的条件判断、算术表达式是否一致？
  - 默认值/降级值与 C# 行为一致？
```

---

## 编码规范

### 错误处理

- 使用 `Result<T, E>` 而非 `panic!`
- 自定义错误类型（UnknownMagicError、EnumNotValidError、UnsafeTexError）
- 读文件时立即检查 `?` 传播错误

### 命名

- 模块名使用 snake_case（`binutil`, `pkgfile`, `dxt`）
- 类型名使用 CamelCase（`ImageContainer`, `FrameInfo`）
- 常量使用 SCREAMING_SNAKE_CASE（`MAGIC_TEXV0005`）
- 函数使用 snake_case（`read_n_string`, `decompress_image`）

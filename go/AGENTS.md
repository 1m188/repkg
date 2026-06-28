# RePKG Go 移植项目

## 项目概述

本项目将 C# 实现的 [RePKG](https://github.com/1m188/repkg.git)（Wallpaper Engine PKG 解包 / TEX 纹理转换工具）完整移植到 Go 语言。

### 功能

- **extract** — 解压 `.pkg` 文件，提取其中的所有条目（纹理、模型、脚本等）
- **info** — 查看 `.pkg` / `.tex` 文件的元数据信息
- **TEX 转图片** — 将 Wallpaper Engine 专用 `.tex` 纹理格式转换为 PNG/GIF/MP4
- **PKG/TEX 写回** — 支持将修改后的数据重新打包为 `.pkg` / `.tex` 格式

### 背景

Wallpaper Engine 使用自定义二进制格式存储壁纸资源：
- `.pkg` 是打包容器，内部包含多个文件和纹理
- `.tex` 是纹理文件，支持 DXT1/3/5 压缩、LZ4 压缩、GIF 动画、MP4 视频

原 C# 项目由 notscuffed 开发，PKG 和 TEX 格式由其逆向工程得出。

---

## 架构设计

### 目录结构

```
go/
├── go.mod
├── go.sum
├── AGENTS.md                        # 本文件（项目说明书，给人看 + 给 AI 看）
├── CLAUDE.md                        # 指向 AGENTS.md
├── cmd/
│   └── repkg/
│       └── main.go                  # Cobra CLI 命令入口 + 交互模式
├── internal/
│   ├── binutil/
│   │   ├── read.go                  # 二进制读取扩展（ReadNString、ReadStringI32Size）
│   │   ├── write.go                 # 二进制写入扩展（WriteNString、WriteStringI32Size）
│   │   └── binutil_test.go          # 二进制工具测试
│   ├── dxt/
│   │   ├── dxt.go                   # DXT1/3/5 块解码算法（纯 Go，零外部依赖）
│   │   └── dxt_test.go              # DXT 解码测试
│   ├── pkgfile/
│   │   ├── entry.go                 # PackageEntry 数据模型
│   │   ├── reader.go                # PackageReader（.pkg 读取）
│   │   ├── writer.go                # PackageWriter（.pkg 写入）
│   │   ├── reader_test.go           # PKG 读取测试
│   │   └── writer_test.go           # PKG 读写往返测试
│   └── tex/
│       ├── header.go                # TexHeader 数据模型 + 读写器
│       ├── image.go                 # TexImage + mipmap 数据模型 + 读写器
│       ├── image_container.go      # TexImageContainer 数据模型 + 读写器
│       ├── frame_info.go            # TexFrameInfo / TexFrameInfoContainer 模型 + 读写器
│       ├── reader.go                # TexReader（顶层 TEX 读取编排器）
│       ├── writer.go                # TexWriter（顶层 TEX 写入编排器）
│       ├── decompressor.go          # Mipmap 解压器（LZ4 解压 + DXT 解码调度）
│       ├── compressor.go            # Mipmap 压缩器（LZ4 压缩）
│       ├── converter.go             # TEX → PNG/GIF/MP4 转换器
│       ├── json_info.go             # .tex-json 元数据生成器
│       ├── reader_test.go           # TEX 解压测试（内存构造数据）
│       └── writer_test.go           # TEX 读写往返测试
├── pkg/
│   └── mipmap/
│       └── format.go                # MipmapFormat 枚举 + IsCompressed/IsImage/GetFileExtension 等方法
├── testdata/
│   └── test.pkg                     # 实景测试数据（来自 Wallpaper Engine 的 .pkg 文件，约 5.8 MB）
└── test/
    └── integration_test.go          # 使用 test.pkg 的端到端集成测试
```

### 分层职责

| 层 | 路径 | 对应 C# | 职责 |
|----|------|---------|------|
| 入口层 | `cmd/repkg/` | RePKG/ | Cobra 命令注册、命令行参数解析、交互模式 |
| 业务逻辑层 | `internal/tex/` `internal/pkgfile/` | RePKG.Application | TEX/PKG 的读写、解压、转换等核心逻辑 |
| 纯算法层 | `internal/dxt/` | DXT.cs | DXT 块解码（零外部依赖，可独立测试） |
| 工具层 | `internal/binutil/` | Extensions.cs | 二进制流便捷读写方法 |
| 公共模型层 | `pkg/mipmap/` | RePKG.Core 枚举 | 对外暴露的 MipmapFormat 格式定义 |

### 数据流

```
.pkg 文件 → bufio.Reader → PackageReader → Package{Entries}
                                    ↓
                             Entry.Bytes（如果是 .tex 条目）
                                    ↓
.tex 文件 → bufio.Reader → TexReader → Tex{Header, Images, Frames}
                                    ↓
                    TexMipmapDecompressor（LZ4 解压 + DXT 解码）
                                    ↓
                    TexToImageConverter → PNG / GIF / MP4
```

### 原 C# 枚举 ↔ Go 常量映射

| C# enum | Go 实现 |
|----------|---------|
| `TexFormat`（RGBA8888=0, DXT5=4, DXT3=6, DXT1=7, RG88=8, R8=9） | `mipmap.TexFormat` iota 常量 |
| `MipmapFormat`（RGBA8888=1, R8=2, RG88=3, 压缩格式, 图片格式） | `mipmap.Format` iota 常量 |
| `DXTFlags`（DXT1=1, DXT3=2, DXT5=4） | `dxt.Flags` iota 常量 |
| `TexFlags`（NoInterpolation, ClampUVs, IsGif 等） | `tex.Flags` 位掩码常量 |
| `TexImageContainerVersion`（1-4） | `tex.ImageContainerVersion` iota 常量 |
| `FreeImageFormat`（FIF_UNKNOWN=-1 到 FIF_MP4=35） | `tex.FreeImageFormat` iota 常量 |
| `EntryType`（Binary, Tex） | `pkgfile.EntryType` iota 常量 |

---

## 技术选型

| 需求 | 选型 | 理由 |
|------|------|------|
| LZ4 压缩/解压 | `github.com/pierrec/lz4/v4` | Go 社区最成熟的 LZ4 库，API 稳定 |
| CLI 命令行 | `github.com/spf13/cobra` | Go 生态标准 CLI 框架，子命令+flags+自动 help |
| PNG 编码 | `image/png`（标准库） | Go 原生支持 |
| GIF 编码 | `image/gif`（标准库） | Go 原生支持 |
| 二进制 I/O | `encoding/binary` + `bufio`（标准库） | Go 原生支持，性能优秀 |
| 测试 | `testing`（标准库） | Go 原生，表驱动测试 |

### DXT 解码移植说明

原 C# `DXT.cs` 从 LibSquish 移植而来。Go 版本直接逐函数翻译：

- `DecompressImage(width, height int, data []byte, flags DXTFlags) []byte` — 顶层入口
- `decompressBlock(rgba []byte, block []byte, blockIndex int, flags DXTFlags)` — 单块解码
- `decompressAlphaDxt3(rgba, block []byte, blockIndex int)` — DXT3 alpha 解码
- `decompressAlphaDxt5(rgba, block []byte, blockIndex int)` — DXT5 alpha 解码
- `decompressColor(rgba, block []byte, blockIndex int, isDxt1 bool)` — 颜色块解码
- `unpack565(block []byte, blockIndex, packedOffset int, colour []byte, colourOffset int) int` — RGB565 解包

---

## PKG 二进制格式

```
[Magic: string + int32 长度前缀]    // 如 "PKGV0005"
[EntryCount: int32]
// 对于每条 Entry：
[FullPath: string + int32 长度前缀]  // 如 "materials/sky.mat"
[Offset: int32]                      // 数据体中的偏移（相对于数据体起始）
[Length: int32]                      // 数据长度（字节数）
// 数据体（按 Entry 顺序排列的原始字节）
```

## TEX 二进制格式

```
[Magic1: null-terminated string]     // "TEXV0005"
[Magic2: null-terminated string]     // "TEXI0001"
[TexHeader: 28 字节]
  Format:      int32                 // TexFormat 枚举值
  Flags:       int32                 // TexFlags 位掩码
  TextureWidth:  int32
  TextureHeight: int32
  ImageWidth:   int32
  ImageHeight:  int32
  UnkInt0:      uint32

[TexImageContainer]
  Magic:       null-terminated string // "TEXB0001" ~ "TEXB0004"
  ImageCount:  int32
  // 如果 Version >= 3： ImageFormat: int32
  // 如果 Version == 4： 额外字段（视频标志位）
  // 对每张 Image：
    [MipmapCount: int32]
    // 对每个 Mipmap（格式因版本而异）：
      // V1: Width, Height, [ByteCount, Bytes]
      // V2/V3: Width, Height, IsLZ4, DecompressedByteCount, [ByteCount, Bytes]
      // V4: param1=1, param2=2, conditionJson, param3=1, 然后同 V2/V3

[TexFrameInfoContainer]（仅当动画时存在）
  Magic: null-terminated string // "TEXS0001" ~ "TEXS0003"
  FrameCount: int32
  // V3: GifWidth, GifHeight
  // 对每帧（坐标类型因版本而异）：
    // V1: ImageId(int32), Frametime(float32), X/Y/Width/WidthY/HeightX/Height(int32)
    // V2/V3: 同上但坐标为 float32
```

---

## 测试策略

### 测试分层

所有测试通过 `go test ./...` 一键运行。

#### 第一层：单元测试（内存自建数据）

不依赖任何外部文件，所有测试数据在内存中构造。

| 被测包 | 测试文件 | 测试内容 | 用例数 |
|--------|----------|----------|--------|
| `internal/binutil` | `binutil_test.go` | ReadNString（空串、含 null、超长）、ReadStringI32Size（零长度、大长度、非法长度）、WriteNString/WriteStringI32Size 往返 | ~6 |
| `internal/dxt` | `dxt_test.go` | DXT1/3/5 单块解码验证、全图解码、非 4 对齐尺寸（1x1, 3x3, 5x5）、alpha 极端值（全透明/全不透明/渐变） | ~10 |
| `internal/pkgfile` | `reader_test.go` `writer_test.go` | 空包、单条目、多条目、大条目（1MB+）、嵌套路径、EntryType 识别、写入→读取往返字节一致 | ~8 |
| `internal/tex` | `reader_test.go` | TEX V1/V2/V3/V4 各版本各格式解压（DXT1/3/5、RGBA8888、R8、RG88、JPEG、PNG、GIF、MP4）、错误 Magic 拒绝、非法 Format 拒绝、超大 mipmap 拒绝、截断数据拒绝、零尺寸处理、Flags 验证 | ~20 |
| `internal/tex` | `writer_test.go` | 所有版本 TEX 写入→读取往返、写入后字节一致验证 | ~8 |

#### 第二层：集成测试（test.pkg 实景数据）

| 被测功能 | 测试文件 | 测试内容 |
|----------|----------|----------|
| PKG extract | `test/integration_test.go` | 读取 test.pkg → 提取所有条目 → 验证条目数 > 0 → 验证每个条目字节非空 → TEX 条目转换为图片 → 验证图片非空 |

### 测试编写原则

1. **表驱动测试** — 所有测试使用 Go 标准表驱动模式（`[]struct{name, input, want}`）
2. **自描述** — 每个 test case 有清晰的名称和注释说明测试目的
3. **全覆盖** — 主流路径、边界条件、异常输入全部覆盖
4. **无外部依赖** — 单元测试不读文件，不连网络

---

## 开发工作流

所有开发者（人类和 AI）必须严格遵循此工作流。违反工作流的代码不得合入。

### 开发方法：测试驱动开发（TDD）

所有功能开发必须遵循 TDD 模式：**先写测试，后写实现**。

```
明确需求与设计
  ↓
[RED] 编写测试代码（预期失败）           # 阶段 1: 用测试定义功能行为
  ↓ 测试未通过 ← 确认测试有效
[GREEN] 编写最小实现代码（通过测试）      # 阶段 2: 只写让测试通过的代码
  ↓ 测试全部通过
[REFACTOR] 优化代码结构、添加注释          # 阶段 3: 重构但不改变行为
  ↓
  ├→ 任何重构后重新运行测试，确保仍通过
  ↓
提交
```

### TDD 原则

| 原则 | 说明 |
|------|------|
| 先测试后代码 | 不写测试就写功能代码 = 不合规 |
| 测试即文档 | 测试描述功能行为，替代额外的规格说明 |
| 最小实现 | 只写让当前测试通过的最少代码 |
| 红→绿→重构 | 严格遵守 RED → GREEN → REFACTOR 循环 |
| 测试覆盖表驱动 | 使用 Go 标准表驱动模式，覆盖正常路径、边界条件、异常输入 |
| 每次提交前 | 必须经过完整的 `golangci-lint fmt` + `golangci-lint run` + `go test -race ./...` |

### TDD 测试编写要求

1. **表驱动测试** — 所有测试使用 `[]struct{name, input, want}` 模式
2. **测试名使用中文** — 描述测试意图（如 `测试DXT5解码/全不透明alpha`）
3. **覆盖三个维度** — 正常路径（happy path）、边界条件（1x1、空数据、超大值）、异常输入（错误 magic、截断数据）
4. **每个公开函数** — 至少有一个测试用例
5. **新增功能** — 测试必须先于实现代码提交

---

## 开发工作流

所有开发者（人类和 AI）必须严格遵循此工作流。违反工作流的代码不得合入。

### 工作流步骤

```
[RED] 编写测试代码
  ↓
go mod tidy                           # 步骤 1: 清理多余依赖
  ↓
golangci-lint fmt ./...               # 步骤 2: 代码格式化（替代 go fmt）
  ↓
golangci-lint run ./...               # 步骤 3: 全量静态分析
  ↓ 有告警 → 修复代码 → 回到步骤 1
  ↓ 无告警
go test -race ./...                   # 步骤 4: 验证测试框架正确
  ↓ ┌─ 测试失败（预期）→ 测试有效，进入步骤 5
  │ └─ 测试通过 → 测试无意义，重写测试
  ↓
[GREEN] 编写功能实现代码
  ↓
golangci-lint fmt ./...               # 步骤 5: 格式化新代码
  ↓
golangci-lint run ./...               # 步骤 6: 静态分析新代码
  ↓ 有告警 → 修复代码 → 回到步骤 5
  ↓ 无告警
go test -race ./...                   # 步骤 7: 测试应全部通过
  ↓ 有失败 → 修复代码 → 回到步骤 5
  ↓ 全部通过
[REFACTOR] 优化代码 + 补全注释
  ↓
同步更新注释和文档                    # 步骤 8: 检查所有注释与代码一致
  ↓
go test -race ./...                   # 步骤 9: 重构后确认测试通过
  ↓
go build -o repkg ./cmd/repkg/        # 步骤 10: 构建可执行文件
  ↓
完成 ✓
```

### 各步骤详解

| 步骤 | 命令 | 说明 |
|------|------|------|
| 1. 整理依赖 | `go mod tidy` | 清理 `go.mod` 和 `go.sum` 中多余的间接依赖 |
| 2. 格式化 | `golangci-lint fmt ./...` | 同时执行 gofmt + goimports，统一代码风格 |
| 3. 静态分析 | `golangci-lint run ./...` | 基于 `.golangci.yml` 配置运行 45+ 个 linter |
| 4. 验证测试 | `go test -race ./...` | RED 阶段：确认新测试失败，旧测试仍通过 |
| 5. 格式化 | `golangci-lint fmt ./...` | 格式化新写的功能代码 |
| 6. 静态分析 | `golangci-lint run ./...` | 检查新代码的静态质量 |
| 7. 功能测试 | `go test -race ./...` | GREEN 阶段：全部测试应通过 |
| 8. 同步文档 | 人工审查 + `grep` 检查 | 确认所有注释与代码一致（见下方规范） |
| 9. 重构验证 | `go test -race ./...` | REFACTOR 阶段：重构后测试仍通过 |
| 10. 构建 | `go build -o repkg ./cmd/repkg/` | 确保最终二进制可成功生成 |

### 静态检查覆盖范围

`.golangci.yml` 配置文件位于 `go/` 根目录，启用以下 linter 类别：

| 类别 | Linter | 覆盖内容 |
|------|--------|----------|
| 正确性 | `errcheck`, `govet`, `staticcheck`, `unused`, `ineffassign` | 错误忽略、可疑构造、死代码、无效赋值 |
| 安全 | `gosec`, `noctx`, `bodyclose`, `canonicalheader` | 安全漏洞、HTTP 规范 |
| 质量 | `gocritic`, `revive`, `goconst`, `dupword`, `misspell`, `unconvert`, `prealloc` | 代码风格、重复代码、拼写、预分配 |
| 复杂度 | `gocyclo`, `funlen`, `gocognit`, `cyclop`, `nestif`, `nakedret`, `maintidx` | 圈复杂度、函数长度、嵌套深度 |
| 测试 | `testableexamples`, `testifylint`, `thelper`, `tparallel` | 测试最佳实践 |
| 现代化 | `modernize`, `exptostd`, `intrange`, `usestdlibvars`, `perfsprint` | 推荐现代 Go 特性 |
| 健壮性 | `nilerr`, `nilnesserr`, `durationcheck`, `makezero`, `errorlint`, `wrapcheck` | nil 错误、时间计算、错误包装 |

### 常见告警修复方法

| 告警 | 修复 |
|------|------|
| `errcheck` 未检查 error | 添加 `if err != nil { return err }` |
| `funlen` 函数过长 | 拆分为多个小函数 |
| `gocyclo` 复杂度过高 | 提取分支逻辑为独立函数 |
| `goconst` 重复字面量 | 提取为具名常量 |
| `nestif` 嵌套过深 | 使用 early return 扁平化 |
| `wrapcheck` 未包装 error | 使用 `fmt.Errorf("...: %w", err)` |
| `revive` / 缺少注释 | 为公开符号添加中文注释 |

### 安装 golangci-lint

```bash
# 推荐直接下载二进制（免编译）
# 或通过 go install（要求 Go >= 1.22）
go install github.com/golangci/golangci-lint/v2/cmd/golangci-lint@latest

# 验证安装
golangci-lint --version
```

### 快速参考

```bash
# 完整工作流（一键，含 TDD 全流程）
cd go
go mod tidy && golangci-lint fmt ./... && golangci-lint run ./... && go test -race ./... && go build -o repkg ./cmd/repkg/

# 仅静态分析
golangci-lint run ./...

# 仅格式化
golangci-lint fmt ./...

# 仅测试（含竞态检测）
go test -race ./...

# 带自动修复的静态分析（部分问题可自动修复）
golangci-lint run --fix ./...
```

---

## 注释与文档撰写规范

**核心原则**：注释必须时刻与代码保持同步一致，不得有落后、错位、过时的情况。

### 注释同步要求

| 触发场景 | 必须更新的内容 |
|----------|---------------|
| 新增文件 | 文件顶部段注释描述模块功能 |
| 新增公开类型/函数/常量 | 添加中文注释，格式 `// 名称 描述。` |
| 修改函数行为 | 同步更新该函数的注释 |
| 修改函数签名（参数/返回值） | 同步更新参数和返回值的说明 |
| 删除函数 | 删除对应的注释 |
| 修改枚举/常量 | 同步更新每个值的注释 |
| 算法逻辑变更 | 更新算法原理注释 |

### 注释层级规范

```
层级 1: 文件头注释（段注释 /* */ 或 // 块）
   位置: 每个 .go 文件的 package 声明之前
   内容: 描述本文件/模块的整体功能和职责
   示例: // Package tex 提供 TEX 纹理文件的读写、解压和转换功能。

层级 2: 类型注释
   位置: 每个公开 struct/interface/enum 定义之前
   内容: 描述该类型的用途和设计意图
   示例: // Header 表示 TEX 文件的头部信息，包含格式、尺寸和标志位。

层级 3: 函数/方法注释
   位置: 每个公开的函数或方法定义之前
   内容: 描述功能、参数含义、返回值、注意事项
   示例: // ReadTex 从 reader 中读取并解析 TEX 数据，返回解析后的 TEX 结构。

层级 4: 常量/变量注释
   位置: 每个公开的常量或变量定义之前或同行
   内容: 描述用途和取值含义
   示例: // FlagIsGif 表示纹理为 GIF 动画格式。

层级 5: 内部逻辑注释
   位置: 复杂算法、非直观逻辑、边界条件处理处
   内容: 解释为什么这样写，而不仅仅是描述做了什么
   示例: // V4 非 MP4 格式降级为 V3 处理，与 C# 原版行为一致
```

### 注释与代码一致性检查清单

开发者在步骤 8（同步文档）时必须逐项确认：

```
□ 新增/修改的公开符号是否都有注释？
□ 修改行为的函数，其注释是否已更新？
□ 删除的代码，其注释是否已移除？
□ 注释中的格式名、字段名是否与代码一致？
□ 文件头注释是否准确描述当前文件的功能范围？
□ 算法注释是否与实际实现逻辑一致？
```

### 注释语言

- 所有注释使用**简体中文**
- 技术术语保留英文（如 DXT1、RGBA8888、mipmap）
- 代码引用使用反引号（如 `binutil.ReadNString`）

### 文档

- `AGENTS.md` — 本文档，项目说明书，给人看 + 给 AI 看
- `CLAUDE.md` — 指向 AGENTS.md
- `.golangci.yml` — 静态检查配置
- 测试代码 — 测试名称和注释即功能规格说明

---

## 代码审查方法：多视角分层审查（MPLR）

单次审查不可能发现所有问题。MPLR 将审查拆为 **4 层独立视角**，每层只关注一类问题，彻底查完再进入下一层。与 TDD 互补——TDD 保证**功能正确**，MPLR 保证**代码健壮**。

### 四层审查

```
L1 运行期安全 (Crash Layer)
  → 会 panic 吗？会 OOM 吗？会死循环吗？
  → 检查点: 数组索引、make 分配大小、指针解引用、类型断言、io.Reader 短读

L2 数据完整性 (Data Layer)
  → Reader 和 Writer 格式对称吗？字段顺序一致吗？
  → 检查点: 对照读取路径和写入路径，逐字段比对魔数/长度/偏移。

L3 边界与异常 (Boundary Layer)
  → 零值、空值、负数、超大值、截断数据每种情况的处理是否正确？
  → 检查点: 对每个输入字段穷举边界表 (0, -1, MaxInt, nil, "", [])

L4 语义正确性 (Semantic Layer)
  → 行为与 C# 原版一致吗？错误类型准确吗？
  → 检查点: 对照同名函数，逐条件/逐算术表达式比对
```

### 审查检查清单

代码提交前必须逐项自检：

```
□ L1 [运行期安全]
  - 所有 make([]T, n) 的 n 是否可能为负数或超大值？
  - 所有 slice[n]/slice[m:n] 是否验证了边界？
  - 所有 .Field 是否验证了接收者非 nil？
  - 所有 .(type) 类型断言是否处理了失败情况？

□ L2 [数据完整性]
  - Reader 读取的字段顺序 == Writer 写入的字段顺序？
  - Reader 读取的魔数 == Writer 写入的魔数？
  - 写入后重读能否字节一致（现有 V1/V2/V3/V4 往返测试已覆盖）？
  -版本降级/升级是否同步更新了相关的 Magic 字符串？

□ L3 [边界]
  - len=0 时是否正常（空字符串、空切片、空文件）？
  - 字段值为 -1 / MaxInt / MaxUint 时是否被正确处理？
  - 输入数据截断（少于最小合法长度）时是否返回错误而非 panic？
  - EOF 提前到达时是否返回错误？

□ L4 [语义]
  - 错误类型用的是 fmt.Errorf、errors.New 还是自定义 Error 类型？（优先使用自定义类型）
  - 与 C# 同名函数的条件判断、算术表达式是否一致？
  - 默认值/降级值与 C# 行为一致？
```

### 审查流程

MPLR 作为工作流步骤 **8.5**，位于同步文档之后、重构验证之前：

```
同步更新注释和文档      # 步骤 8
  ↓
MPLR 分层审查          # 步骤 8.5: 按 L1→L4 逐层自检
  ↓ 有问题 → 修复 → 回到步骤 2
  ↓ 无问题
go test -race           # 步骤 9
  ↓
go build                # 步骤 10
```

---

## 编码规范

### 错误处理

- 使用 Go 标准 `error` 接口，不 panic
- 自定义错误类型（UnknownMagicError、EnumNotValidError、UnsafeTexError）
- 读文件时立即检查 error 并返回

### 命名

- 包名使用小写单词（`binutil`, `pkgfile`, `dxt`）
- 接口名使用 I 前缀或 -er 后缀按场景决定
- 常量使用驼峰命名（`FlagNoInterpolation` 而非全大写）

---

## 设计决策记录

| 决策 | 选择 | 理由 |
|------|------|------|
| 模块路径 | `github.com/1m188/repkg-go` | 与原 C# 项目保持关联 |
| 内部包 | 使用 `internal/` 目录 | 防止外部导入内部实现细节 |
| CLI 框架 | Cobra | Go 社区标准，子命令和 flags 支持完备 |
| LZ4 库 | pierrec/lz4 | 最成熟、最广泛使用 |
| GIF 编码 | 标准库 `image/gif` | 零依赖，功能充足 |
| 测试数据 | test.pkg 入库（5.8 MB） | 体量合理，方便 CI 自动运行 |
| 注释语言 | 简体中文 | 开发者是中文用户 |
| 静态检查 | golangci-lint（最严格配置） | 单一工具覆盖 45+ linter，替代 go vet + staticcheck + go fmt + goimports |
| 格式化 | golangci-lint fmt（gofmt + goimports） | 内置 formatter，风格统一零争议 |

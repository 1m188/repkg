# RePKG Web

Wallpaper Engine **PKG 解包 / TEX 转换**的纯前端（static-only）版本。
由仓库内 C# 原版逐行移植到 TypeScript，**无后端、无上传** —— 所有解析、解压、转码都在浏览器本地完成。
`vite build` 产出的 `dist/` 即为纯静态站点，可直接部署到 GitHub Pages 等任意静态托管。

## 功能（MVP）

| 功能 | 说明 |
| ---- | ---- |
| 打开 PKG | 拖入 / 选择 `.pkg`，逐条目**按需惰性读取**（`File.slice`，不整包载入内存） |
| 打开 TEX | `.tex` 独立文件直接解析预览 |
| TEX → PNG | RGBA8888 / R8 / RG88 展开 + 顶部裁剪（与 C# `Mutate(Crop)` 语义一致），像素与桌面版逐位一致 |
| TEX → GIF | 帧几何（裁剪 / 90° 倍数旋转 / 帧延迟）按 C# 公式移植，gifenc 量化编码 |
| TEX 透传 | 容器内嵌 JPEG / PNG / MP4 等格式时原样输出（与桌面版行为一致） |
| info 面板 | header / 容器版本 / mipmap 链 / TXFlags / GIF 帧数，附 `.tex-json` 内容 |
| 下载与保存格式 | 下载名自动带正确扩展名与 MIME；PNG 产物可转存 WebP / JPEG（质量 0.92，浏览器内重编码）；透传图片可选保留原始字节或转存；GIF/MP4 按原格式 |
| 打包 | 全部条目 ZIP（防 zip-slip 路径清洗 + 重名去重），ZIP 内产物命名与桌面版一致（`{路径去扩展名}.{输出扩展名}` + `.tex-json` + 可选原始 `.tex`） |

## 使用

```bash
cd web
pnpm install        # 或 npm install
npm run dev         # 本地开发（http://localhost:5173）
npm run build       # 产出纯静态 dist/
npm run preview     # 本地预览 dist
npm test            # vitest：单元 + 与桌面实现对齐的 parity 测试
npm run e2e         # playwright chromium 真实浏览器冒烟（需先 npm run preview -- --port 4173）
npm run gen:baseline  # 重新生成基准（需要 go 仓库内 testdata/test.pkg 与 go/dotnet 工具链）
```

部署 GitHub Pages：`.github/workflows/build-web.yml` 已配置自动部署 `web/dist`。

> ⚠️ 不要双击 `dist/index.html` 用 `file://` 打开 —— 浏览器会拦截 ES 模块与 Worker。
> 页面自身会检测该情况并显示提示横幅；请通过 `npm run preview` 或部署后的 HTTPS 地址访问。

## 与桌面三实现（C# / Go / Rust）的一致性

`tests/parity/baseline.json` 由 `tools/gen-baseline.mjs` 从**桌面实现**生成：

1. 用 Go 版把 `go/testdata/test.pkg` 提取两次（`--no-tex-convert` 与默认），
   两树差集即「转换产物 + .tex-json」；
2. PNG 产物用 pngjs **解码成像素取 sha256** —— 与编码器无关的像素级校验；
3. 透传产物（容器内嵌 PNG/JPEG/MP4）记录其字节 CRC（= 首层 mipmap 字节）；
4. `tests/parity.test.ts` 用这些基准校验 TS 版的条目表 / 原始字节 CRC / 像素哈希 / JSON。

另外 `tests/fixtures/webfixtures.json` 固化了 **Go internal/dxt 与 pierrec/lz4 的 20 个位精确
参考向量**（DXT1/3/5 × 尺寸 1×1、3×3、8×8、13×11、32×32 + 各类 LZ4 块），保证
`src/lib/dxt.ts` / `src/lib/lz4.ts` 与桌面实现逐位一致。

金标准说明：仓库文档约定 **C# 为权威参考**。test.pkg 的基准目前由 Go 版生成
（像素值与 C# 一致的条件是双方格式理解正确 —— 正是这些测试要验证的）；
有 C# 工具链的机器可用 `GEN_BASELINE=csharp node tools/gen-baseline.mjs` 交叉重新生成。

## 目录结构

```
web/
├── index.html                  # 入口页（拖拽 / 说明 UI）
├── src/
│   ├── main.ts                 # UI 状态与渲染（docbar / 条目树 / 预览 / ZIP）
│   ├── style.css
│   ├── worker.ts               # Worker 薄壳：self.onmessage → Pipeline
│   └── lib/
│       ├── binutil.ts          # ByteReader（对齐 C# BinaryReader）、错误类型
│       ├── enums.ts            # 全部格式枚举 + 扩展名/映射（对应 RePKG.Core）
│       ├── pkg.ts              # PKG 条目表解析（惰性数据读取）
│       ├── tex.ts              # TEX V1~V4 解析 + LZ4/DXT 解压调度
│       ├── lz4.ts              # LZ4 原始块解码（纯 TS）
│       ├── dxt.ts              # DXT1/3/5 块解码（LibSquish 语义，纯 TS）
│       ├── convert.ts          # 像素级转换：展开 / 裁剪 / 旋转 / GIF 帧几何
│       ├── texjson.ts          # .tex-json 生成（对齐 Newtonsoft 输出语义）
│       ├── imaging.ts          # Worker 侧编码：OffscreenCanvas PNG、gifenc GIF
│       ├── zip.ts              # store 模式 ZIP 打包器（零依赖）+ 路径清洗
│       ├── pipeline.ts         # 解码管线（open/convert/zip 消息执行体，可注入编码器）
│       └── protocol.ts         # 主线程 ↔ Worker 消息类型
├── tests/                      # vitest（Node 环境跑通浏览器逻辑）
│   ├── unit.test.ts            # binutil / LZ4 / DXT / PKG / TEX / convert / zip
│   ├── parity.test.ts          # 与桌面实现基准的数据级校验
│   ├── pipeline.test.ts        # Worker 流水线集成（伪编码器注入）
│   ├── fixtures/webfixtures.json
│   └── parity/base…           # baseline.json（入库，CI 无需桌面工具链）
└── tools/gen-baseline.mjs      # 生成桌面版基准（go 或 dotnet）
```

## 已知边界（MVP 刻意不做）

- 不支持**写回**（tex/pkg writer、repack 合成）——桌面版承担该职责；
- GIF 量化调色板与 ImageSharp/Octree 不同（视觉级差异，帧结构/延迟一致）；
- TIFF/PSD 等 36 种 FreeImage 格式仅透传下载，不预览；
- File System Access API 未接入（当前为 ZIP / 单文件下载，Safari/Firefox 兼容一致）；
- `warns`：超大 mipmap（250 MB 上限由桌面版常量护住）在低端设备可能引发 OOM —— 报错可恢复。

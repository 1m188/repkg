/**
 * RePKG Web 主界面逻辑。
 *
 * 职责：文件选择（拖拽 / 文件选择器）、文档管理、条目树渲染、
 * 预览与下载触发、ZIP 打包进度。全部重活都在 worker.ts 里。
 */
import "./style.css";
import type { EntryMeta, TexSummary, WorkerResponse, ZipParams } from "./lib/protocol";

/** 一个打开的文档（pkg 或 tex） */
type DocState = {
  docId: number;
  filename: string;
  kind: "pkg" | "tex";
  magic: string;
  headerSize: number;
  totalBytes: number;
  entries: EntryMeta[];
  texSummary?: TexSummary;
};

/** 已生成一次的工件（供重复点击下载；URL 与对应字节由主线程持有） */
type ArtifactState = {
  url: string;
  mime: string;
  outPath: string;
  bytes: ArrayBuffer;
  source: "converted" | "raw";
  info?: TexSummary;
  texJson?: string;
};

const app = {
  worker: new Worker(new URL("./worker.ts", import.meta.url), { type: "module" }),
  docs: new Map<number, DocState>(),
  artifacts: new Map<string, ArtifactState>(), // key = `${docId}:${idx ?? -1}`
  selected: null as { docId: number; idx: number | null } | null,
  activeDocId: -1,
  nextDocId: 1,
  zipBusy: false,
};

/* ------------------------------------------------------------------ */
/* DOM 引用                                                            */
/* ------------------------------------------------------------------ */

const $ = <T extends HTMLElement>(selector: string): T => {
  const el = document.querySelector(selector);
  if (!el) throw new Error(`missing ${selector}`);
  return el as T;
};

const heroEl = $("#hero");
const dropzoneEl = $("#dropzone");
const fileInputEl = $<HTMLInputElement>("#file-input");
const workEl = $("#work");
const docbarEl = $("#docbar");
const filterEl = $("#filter") as HTMLInputElement;
const sortEl = $("#sort") as HTMLSelectElement;
const treeEl = $("#tree");
const detailEmptyEl = $("#detail-empty");
const detailViewEl = $("#detail-view");
const statusEl = $("#statusbar");
const statusTextEl = $("#status-text");
const statusProgEl = $("#status-prog");
const statusProgBar = statusProgEl.querySelector("i") as HTMLElement;
const toastEl = $("#toast");
const scriptWarningEl = $("#script-warning");

// 模块脚本已运行：隐藏“脚本未运行”提示；file:// 场景下保留并改写文案
if (location.protocol === "file:") {
  scriptWarningEl.innerHTML =
    "⚠️ 当前通过 <code>file://</code> 打开 —— 浏览器会拦截模块脚本与 Worker，功能不可用。请用 <code>npm run dev</code> / <code>npm run preview</code> 或部署后的 HTTPS 地址访问。";
} else {
  scriptWarningEl.classList.add("hidden");
}

/* ------------------------------------------------------------------ */
/* Worker 协议                                                        */
/* ------------------------------------------------------------------ */

app.worker.onmessage = (ev: MessageEvent<WorkerResponse>) => {
  const msg = ev.data;
  switch (msg.type) {
    case "opened":
      handleOpened(msg);
      break;
    case "artifact":
      handleArtifact(msg);
      break;
    case "zipProgress": {
      const note = msg.failed > 0 ? `（失败 ${msg.failed}，已回退原始）` : "";
      setStatus(`打包中… ${msg.done}/${msg.total}${note}`, msg.total > 0 ? msg.done / msg.total : 0);
      break;
    }
    case "zipDone": {
      const name = stripExtAny(app.docs.get(msg.docId)?.filename ?? "output") + ".zip";
      saveBytes(msg.bytes, name, "application/zip");
      setStatus(`ZIP 完成：${msg.fileCount} 个文件，${fmtSize(msg.size)}`);
      app.zipBusy = false;
      setBusyButtons();
      break;
    }
    case "error":
      toast(`❌ ${msg.message}`, true);
      break;
  }
};

function handleOpened(msg: Extract<WorkerResponse, { type: "opened" }>): void {
  if (app.docs.has(msg.docId)) return;

  app.docs.set(msg.docId, {
    docId: msg.docId,
    filename: msg.filename,
    kind: msg.kind,
    magic: msg.magic,
    headerSize: msg.headerSize,
    totalBytes: msg.totalBytes,
    entries: msg.entries ?? [],
    texSummary: msg.texSummary,
  });

  app.activeDocId = msg.docId;
  app.selected = null;
  showWorkspace(true);
  renderDocs();
  renderTree();
  renderDetail();
  setStatus(
    msg.kind === "pkg"
      ? `PKG ${msg.magic} · ${msg.entryCount} 条目 · 数据区起于 ${fmtSize(msg.headerSize)} · 共 ${fmtSize(msg.totalBytes)}`
      : `TEX 已打开 · ${fmtSize(msg.totalBytes)}`,
  );

  // tex 单文件文档自动预览
  if (msg.kind === "tex") {
    select(msg.docId, null);
  }
}

function handleArtifact(msg: Extract<WorkerResponse, { type: "artifact" }>): void {
  const key = artifactKey(msg.docId, msg.entryIndex);
  const prev = app.artifacts.get(key);
  if (prev) URL.revokeObjectURL(prev.url);

  app.artifacts.set(key, {
    url: URL.createObjectURL(new Blob([msg.bytes], { type: msg.mime })),
    mime: msg.mime,
    outPath: msg.outPath,
    bytes: msg.bytes,
    source: msg.source,
    info: msg.info,
    texJson: msg.texJson,
  });

  if (msg.source === "raw") {
    setStatus(`原始文件 ${fmtSize(msg.bytes.byteLength)}`);
  }
  if (msg.docId === app.activeDocId) renderDetail();
}

/* ------------------------------------------------------------------ */
/* 选区                                                               */
/* ------------------------------------------------------------------ */

function select(docId: number, idx: number | null): void {
  app.selected = { docId, idx };
  setDetailLoading();
  app.worker.postMessage({ type: "convert", docId, entryIndex: idx, raw: false });
}

/** 请求原始字节工件（不解析转换） */
function requestRaw(docId: number, idx: number | null): void {
  app.selected = { docId, idx };
  app.worker.postMessage({ type: "convert", docId, entryIndex: idx, raw: true });
}

/* ------------------------------------------------------------------ */
/* 打开文件（拖拽 / 选择器）                                            */
/* ------------------------------------------------------------------ */

// 注意：#btn-pick 在 #dropzone 内部，点击按钮会冒泡到 dropzone ——
// 只绑定 dropzone 一处，避免同一用户激活里连续两次 fileInput.click()
// （Chromium 会取消第一次的文件选择器，表现为点击毫无反应）。
dropzoneEl.addEventListener("click", () => fileInputEl.click());
fileInputEl.addEventListener("change", () => {
  addFiles(fileInputEl.files);
  fileInputEl.value = "";
});

["dragenter", "dragover"].forEach((ev) =>
  document.addEventListener(ev, (e) => e.preventDefault()),
);
document.addEventListener("drop", (e) => {
  e.preventDefault();
  if (e.dataTransfer && e.dataTransfer.files.length > 0) {
    addFiles(e.dataTransfer.files);
  }
});
dropzoneEl.addEventListener("dragenter", () => dropzoneEl.classList.add("drag"));
dropzoneEl.addEventListener("dragleave", () => dropzoneEl.classList.remove("drag"));
dropzoneEl.addEventListener("dragend", () => dropzoneEl.classList.remove("drag"));

function addFiles(files: FileList | null): void {
  if (!files) return;
  for (const file of files) {
    const docId = app.nextDocId++;
    setStatus(`正在读取 ${file.name}…`);
    app.worker.postMessage({ type: "open", docId, file, filename: file.name });
  }
}

function showWorkspace(visible: boolean): void {
  heroEl.classList.toggle("hidden", visible);
  workEl.classList.toggle("hidden", !visible);
  statusEl.classList.toggle("hidden", !visible);
}

/* ------------------------------------------------------------------ */
/* docbar                                                             */
/* ------------------------------------------------------------------ */

function renderDocs(): void {
  docbarEl.innerHTML = "";
  for (const doc of app.docs.values()) {
    const chip = document.createElement("div");
    chip.className = `docchip kind-${doc.kind}${doc.docId === app.activeDocId ? " active" : ""}`;
    chip.innerHTML = `<i class="dot"></i><span class="name"></span><span class="meta"></span><button class="close" title="关闭">✕</button>`;
    (chip.querySelector(".name") as HTMLElement).textContent = doc.filename;
    (chip.querySelector(".meta") as HTMLElement).textContent =
      doc.kind === "pkg" ? `${doc.entries.length} 条 · ${fmtSize(doc.totalBytes)}` : `tex · ${fmtSize(doc.totalBytes)}`;
    chip.addEventListener("click", () => {
      app.activeDocId = doc.docId;
      renderDocs();
      renderTree();
      renderDetail();
    });
    (chip.querySelector(".close") as HTMLButtonElement).addEventListener("click", (e) => {
      e.stopPropagation();
      closeDoc(doc.docId);
    });
    docbarEl.appendChild(chip);
  }

  const add = document.createElement("button");
  add.className = "btn-ghost";
  add.textContent = "＋ 打开文件";
  add.addEventListener("click", () => fileInputEl.click());
  docbarEl.appendChild(add);

  showWorkspace(app.docs.size > 0);
}

function closeDoc(docId: number): void {
  for (const [k, art] of app.artifacts) {
    if (k.startsWith(`${docId}:`)) {
      URL.revokeObjectURL(art.url);
      app.artifacts.delete(k);
    }
  }
  app.docs.delete(docId);
  if (app.selected?.docId === docId) app.selected = null;
  if (app.activeDocId === docId) {
    app.activeDocId = app.docs.size > 0 ? [...app.docs.keys()][0] : -1;
  }
  renderDocs();
  renderTree();
  renderDetail();
}

/* ------------------------------------------------------------------ */
/* tree                                                               */
/* ------------------------------------------------------------------ */

interface VisibleRow {
  entry: EntryMeta | null; // null = tex 文档自身
  dir: string;
  name: string;
}

function visibleRows(): VisibleRow[] {
  const doc = app.docs.get(app.activeDocId);
  if (!doc) return [];

  if (doc.kind === "tex") {
    return [{ entry: null, dir: "", name: doc.filename }];
  }

  const filter = filterEl.value.trim().toLowerCase();
  const sortMode = sortEl.value;

  let rows: VisibleRow[] = doc.entries.map((entry) => ({
    entry,
    dir: dirOf(entry.fullPath),
    name: baseName(entry.fullPath),
  }));

  if (filter) {
    rows = rows.filter((r) => {
      const hay = `${r.dir}/${r.name}`.toLowerCase();
      return hay.includes(filter) || hay.endsWith(filter) || baseName(r.name).toLowerCase() === filter;
    });
  }

  rows.sort((a, b) => {
    if (sortMode === "size") return (b.entry?.length ?? 0) - (a.entry?.length ?? 0);
    if (sortMode === "name") return a.name.localeCompare(b.name) || a.dir.localeCompare(b.dir);
    return `${a.dir}/${a.name}`.localeCompare(`${b.dir}/${b.name}`);
  });

  return rows;
}

function renderTree(): void {
  const doc = app.docs.get(app.activeDocId);
  treeEl.innerHTML = "";
  if (!doc) return;

  for (const row of visibleRows()) {
    const li = document.createElement("li");
    li.className = "row";
    const isTex = row.entry === null || row.entry.type === 1;
    const size = row.entry ? fmtSize(row.entry.length) : fmtSize(doc.totalBytes);

    const dirSpan = row.dir
      ? `<span class="dir" title="${escapeHtml(row.dir)}">${escapeHtml(row.dir)}/</span>`
      : "";
    li.innerHTML = `
      <span class="badge ${isTex ? "tex" : "bin"}">${isTex ? "TEX" : "BIN"}</span>
      ${dirSpan}<span class="fname">${escapeHtml(row.name)}</span>
      <span class="size">${size}</span>`;

    const idx = row.entry ? row.entry.index : null;
    li.dataset.idx = String(idx ?? -1);
    if (app.selected && app.selected.docId === doc.docId && app.selected.idx === idx) {
      li.classList.add("active");
    }
    li.addEventListener("click", () => select(doc.docId, idx));
    treeEl.appendChild(li);
  }
}

filterEl.addEventListener("input", renderTree);
sortEl.addEventListener("change", renderTree);

/* ------------------------------------------------------------------ */
/* detail                                                           */
/* ------------------------------------------------------------------ */

function setDetailLoading(): void {
  detailEmptyEl.classList.add("hidden");
  detailViewEl.classList.remove("hidden");
  detailViewEl.innerHTML = `<div class="canvas-wrap"><div class="placeholder">读取与转换中…</div></div>`;
}

function renderDetail(): void {
  const doc = app.docs.get(app.activeDocId);
  if (!doc) {
    detailViewEl.classList.add("hidden");
    detailEmptyEl.classList.remove("hidden");
    return;
  }
  if (!app.selected) {
    detailViewEl.classList.add("hidden");
    detailEmptyEl.classList.remove("hidden");
    return;
  }

  const key = artifactKey(app.selected.docId, app.selected.idx);
  const art = app.artifacts.get(key);
  if (!art) {
    setDetailLoading();
    return;
  }

  const entry = doc.kind === "pkg" ? (doc.entries[app.selected.idx ?? -1] ?? null) : null;
  const texRow = (entry !== null && entry.type === 1) || doc.kind === "tex";

  detailViewEl.innerHTML = "";

  // 操作行
  const actions = document.createElement("div");
  actions.className = "actions";

  const fpath = document.createElement("div");
  fpath.className = "fpath";
  fpath.textContent = art.outPath;
  actions.appendChild(fpath);

  // 保存格式选择：converted PNG 可转存 WebP/JPEG；透传图片可转存；
  // GIF 维持原格式（浏览器没有动图编码器）；mp4 / 非图像仅原始
  const formats = formatOptionsFor(art);
  let fmtSelect: HTMLSelectElement | null = null;
  if (formats.length > 1) {
    const label = document.createElement("span");
    label.className = "fmt-label";
    label.textContent = "格式";
    actions.appendChild(label);
    fmtSelect = document.createElement("select");
    fmtSelect.id = "fmt-select";
    for (const f of formats) {
      const option = document.createElement("option");
      option.value = f.value;
      option.textContent = f.label;
      fmtSelect.appendChild(option);
    }
    actions.appendChild(fmtSelect);
  }

  const dlBtn = document.createElement("button");
  dlBtn.className = "btn-ghost";
  dlBtn.id = "btn-dl";
  dlBtn.textContent = "⬇ 下载";
  dlBtn.title = "按所选格式保存（outPath 自带正确扩展名与 MIME）";
  dlBtn.addEventListener("click", async () => {
    const chosen = fmtSelect?.value ?? "current";
    try {
      if (chosen === "current" || chosen === "orig") {
        saveBytes(art.bytes, fileNameOf(art.outPath), art.mime);
        return;
      }
      setStatus(`正在转存为 ${chosen.toUpperCase()}…`);
      const buffer = await reencodeImage(art.bytes, art.mime, chosen);
      saveBytes(buffer, swapExt(fileNameOf(art.outPath), chosen), reencodeMime(chosen));
      setStatus(`已保存 ${chosen.toUpperCase()}（质量 0.92）`);
    } catch (err) {
      toast(`⚠️ 转存失败（${err instanceof Error ? err.message : String(err)}），已按原始格式保存`, true);
      saveBytes(art.bytes, fileNameOf(art.outPath), art.mime);
    }
  });
  actions.appendChild(dlBtn);

  if (texRow) {
    const rawBtn = document.createElement("button");
    rawBtn.className = "btn-ghost";
    rawBtn.textContent = "⬇ 原始 .tex";
    rawBtn.title = "下载未转换的原始 TEX 文件";
    rawBtn.addEventListener("click", () => {
      requestRaw(doc.docId, entry ? entry.index : null);
      setStatus("读取原始文件…");
    });
    actions.appendChild(rawBtn);
  }

  detailViewEl.appendChild(actions);

  // 预览区
  const wrap = document.createElement("div");
  wrap.className = "canvas-wrap";
  if (art.mime.startsWith("image/")) {
    const img = document.createElement("img");
    img.src = art.url;
    img.alt = art.outPath;
    wrap.appendChild(img);
  } else if (art.mime === "video/mp4") {
    const video = document.createElement("video");
    video.src = art.url;
    video.controls = true;
    video.loop = true;
    video.autoplay = true;
    wrap.appendChild(video);
  } else {
    wrap.innerHTML = `<div class="placeholder"><div>浏览器无法预览 ${escapeHtml(
      art.mime,
    )}</div><div class="hint">数据完整保留，可直接下载</div></div>`;
  }
  detailViewEl.appendChild(wrap);

  // 信息区
  const info = document.createElement("div");
  info.className = "info";
  info.appendChild(buildInfoTable(doc, entry, art));
  if (art.texJson) {
    const det = document.createElement("details");
    det.innerHTML = `<summary>.tex-json（与桌面版一致）</summary><pre class="detailjson"></pre>`;
    (det.querySelector("pre") as HTMLElement).textContent = art.texJson;
    info.appendChild(det);
  }
  detailViewEl.appendChild(info);
}

function buildInfoTable(
  doc: DocState,
  entry: EntryMeta | null,
  art: ArtifactState,
): HTMLElement {
  const root = document.createElement("table");
  const rows: [string, string][] = [];

  rows.push(["文件", doc.filename]);
  rows.push(["类型", doc.kind === "pkg" ? `PKG · ${doc.magic}` : "TEX"]);
  if (doc.kind === "pkg" && entry) {
    rows.push([
      "数据偏移",
      `${fmtSize(doc.headerSize + entry.offset)}（header ${fmtSize(doc.headerSize)} + ${fmtSize(entry.offset)}）`,
    ]);
    rows.push(["原始大小", fmtSize(entry.length)]);
    rows.push(["条目类型", entry.type === 1 ? "TEX 纹理" : "二进制"]);
  }
  if (doc.kind === "tex") {
    rows.push(["原始大小", fmtSize(doc.totalBytes)]);
  }
  if (art.info) {
    const s = art.info;
    const proc = art.source === "converted" ? "已转换" : "未转换";
    rows.push(["格式", `${s.formatName}${s.flags.length ? " · " + s.flags.join(", ") : ""} · ${proc}`]);
    rows.push(["尺寸", `${s.imageWidth} × ${s.imageHeight}（纹理 ${s.textureWidth} × ${s.textureHeight}）`]);
    rows.push(["容器", `${s.containerMagic} v${s.containerVersion}${s.isVideo ? " · MP4 视频" : ""}`]);
    rows.push([
      "Mipmap",
      s.images
        .map(
          (img, i) =>
            `#${i}: ${img.mipmaps.map((m) => `${m.width}×${m.height} ${m.formatName}`).join(" → ")}`,
        )
        .join("；"),
    ]);
    if (s.isGif) {
      rows.push(["GIF", `${s.frameCount} 帧 · ${s.gifWidth} × ${s.gifHeight}`]);
    }
  }

  for (const [k, v] of rows) {
    const tr = document.createElement("tr");
    tr.innerHTML = `<td class="k">${escapeHtml(k)}</td><td><code>${escapeHtml(v)}</code></td>`;
    root.appendChild(tr);
  }
  return root;
}

/* ------------------------------------------------------------------ */
/* ZIP 触发                                                            */
/* ------------------------------------------------------------------ */

$("#btn-zip").addEventListener("click", () => {
  if (app.zipBusy || app.activeDocId < 0) return;
  app.zipBusy = true;
  setBusyButtons();
  const params: ZipParams = {
    convertTex: ($("#opt-convert") as HTMLInputElement).checked,
    includeRawTex: ($("#opt-raw") as HTMLInputElement).checked,
    includeTexJson: ($("#opt-json") as HTMLInputElement).checked,
  };
  setStatus("开始打包…", 0);
  app.worker.postMessage({ type: "zip", docId: app.activeDocId, params });
});

function setBusyButtons(): void {
  const btn = $("#btn-zip") as HTMLButtonElement;
  btn.disabled = app.zipBusy;
  btn.textContent = app.zipBusy ? "打包中…" : "⬇ 打包下载 ZIP";
}

/* ------------------------------------------------------------------ */
/* 工具函数                                                            */
/* ------------------------------------------------------------------ */

function artifactKey(docId: number, entryIndex: number | null): string {
  return `${docId}:${entryIndex ?? -1}`;
}

function dirOf(fullPath: string): string {
  const idx = fullPath.lastIndexOf("/");
  return idx === -1 ? "" : fullPath.slice(0, idx);
}

function baseName(fullPath: string): string {
  const idx = fullPath.lastIndexOf("/");
  const name = idx === -1 ? fullPath : fullPath.slice(idx + 1);
  // 条目内嵌完整路径已含扩展名；文件名保留原样（桌面版在文件名上去除 .tex 后缀属于输出生成阶段）
  return name;
}

function stripExtAny(p: string): string {
  const slash = p.lastIndexOf("/");
  const dot = p.lastIndexOf(".");
  return dot > slash + 1 ? p.slice(0, dot) : p;
}

function fileNameOf(outPath: string): string {
  const idx = outPath.lastIndexOf("/");
  return idx === -1 ? outPath : outPath.slice(idx + 1);
}

/* ---------------- 保存格式（下载时重编码） ---------------- */

type FormatOption = { value: string; label: string };

/** 依当前产物 MIME 列出可选保存格式（至少一项） */
function formatOptionsFor(art: ArtifactState): FormatOption[] {
  if (art.mime === "image/png") {
    return [
      { value: "current", label: "PNG（无损）" },
      { value: "webp", label: "WebP" },
      { value: "jpeg", label: "JPEG" },
    ];
  }
  if (art.mime === "image/gif") {
    // 没有浏览器内置动图编码器，GIF 保持原格式
    return [{ value: "current", label: "GIF（动画）" }];
  }
  if (art.mime === "video/mp4") {
    return [{ value: "current", label: "MP4（原始）" }];
  }
  if (/^image\/(jpeg|bmp|webp)$/.test(art.mime)) {
    // 透传的可解码图片：保留原始字节或转存
    return [
      { value: "orig", label: "原始格式" },
      { value: "png", label: "PNG（无损）" },
      { value: "webp", label: "WebP" },
      { value: "jpeg", label: "JPEG" },
    ];
  }
  return [{ value: "orig", label: "原始格式" }];
}

function reencodeMime(format: string): string {
  switch (format) {
    case "png":
      return "image/png";
    case "webp":
      return "image/webp";
    case "jpeg":
      return "image/jpeg";
    default:
      return "application/octet-stream";
  }
}

/** 用浏览器解码器把当前产物重编码为目标格式（createImageBitmap → OffscreenCanvas） */
async function reencodeImage(bytes: ArrayBuffer, mime: string, format: string): Promise<ArrayBuffer> {
  const bitmap = await createImageBitmap(new Blob([bytes], { type: mime }));
  const canvas = new OffscreenCanvas(bitmap.width, bitmap.height);
  const ctx = canvas.getContext("2d");
  if (ctx === null) throw new Error("OffscreenCanvas 2D 不可用");
  ctx.drawImage(bitmap, 0, 0);
  bitmap.close();
  const blob = await canvas.convertToBlob({ type: reencodeMime(format), quality: 0.92 });
  return blob.arrayBuffer();
}

/** 替换文件名末尾的扩展名 */
function swapExt(name: string, newExt: string): string {
  const dot = name.lastIndexOf(".");
  return dot > 0 ? `${name.slice(0, dot)}.${newExt}` : `${name}.${newExt}`;
}

function saveBytes(bytes: ArrayBuffer, filename: string, mime?: string): void {
  const url = URL.createObjectURL(new Blob([bytes], mime ? { type: mime } : undefined));
  const a = document.createElement("a");
  a.href = url;
  a.download = filename;
  document.body.appendChild(a);
  a.click();
  a.remove();
  setTimeout(() => URL.revokeObjectURL(url), 30_000);
}

function escapeHtml(s: string): string {
  return s
    .replace(/&/g, "&amp;")
    .replace(/</g, "&lt;")
    .replace(/>/g, "&gt;")
    .replace(/"/g, "&quot;");
}

function fmtSize(n: number | undefined): string {
  if (n === undefined) return "";
  if (n < 1024) return `${n} B`;
  if (n < 1024 ** 2) return `${(n / 1024).toFixed(1)} KB`;
  if (n < 1024 ** 3) return `${(n / 1024 / 1024).toFixed(2)} MB`;
  return `${(n / 1024 / 1024 / 1024).toFixed(2)} GB`;
}

let toastTimer = 0;
function toast(message: string, isError: boolean): void {
  toastEl.textContent = message;
  toastEl.classList.toggle("err", isError);
  toastEl.classList.remove("hidden");
  clearTimeout(toastTimer);
  toastTimer = window.setTimeout(() => toastEl.classList.add("hidden"), 4200);
}

function setStatus(text: string, progress?: number): void {
  statusEl.classList.remove("hidden");
  statusTextEl.textContent = text;
  if (progress === undefined) {
    statusProgEl.classList.add("hidden");
  } else {
    statusProgEl.classList.remove("hidden");
    statusProgBar.style.width = `${Math.round(Math.min(1, Math.max(0, progress)) * 100)}%`;
  }
}

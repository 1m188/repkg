#!/usr/bin/env node
/**
 * e2e 冒烟（playwright chromium，真实浏览器）：
 *   1) 页面加载后「脚本未运行」横幅必须隐藏（模块脚本已执行）
 *   2) 点击「选择文件」按钮必须恰好产生 1 个文件选择器
 *      —— 回归：按钮/冒泡双 click() 会让 Chromium 取消文件对话框（点击毫无反应）
 *   3) 打开 test.pkg → 文档 chip 与条目树渲染
 *   4) 点击真实 TEX 条目 → Worker 内 OffscreenCanvas 转码 → 预览区出现真实 PNG <img>
 *
 * 运行：先另起 `npm run preview -- --port 4173 --strictPort`，再 `node tools/e2e-smoke.mjs`
 */
import { chromium } from "playwright";
import { readFileSync } from "node:fs";
import { resolve, dirname } from "node:path";
import { fileURLToPath } from "node:url";

const here = dirname(fileURLToPath(import.meta.url));
const webRoot = resolve(here, "..");
const pkgPath = resolve(webRoot, "..", "go", "testdata", "test.pkg");
const BASE = process.env.E2E_BASE ?? "http://localhost:4173/";

const fail = (msg) => {
  console.error(`❌ ${msg}`);
  process.exit(1);
};

const browser = await chromium.launch();
const page = await browser.newPage({
  viewport: { width: 1400, height: 900 },
  acceptDownloads: true,
});

const pageErrors = [];
page.on("pageerror", (err) => pageErrors.push(String(err)));
page.on("console", (msg) => {
  if (msg.type() === "error") console.log(`[console.error] ${msg.text()}`);
});

try {
  // ---------- 1) 页面加载与脚本横幅 ----------
  await page.goto(BASE, { waitUntil: "load" });
  await page.waitForTimeout(400);
  const warningHidden = await page.$eval("#script-warning", (el) => el.classList.contains("hidden"));
  if (!warningHidden) fail("模块脚本未运行：#script-warning 横幅仍然可见（脚本加载失败）");
  console.log("✓ 模块脚本已运行，横幅已隐藏");

  // ---------- 2) 按钮点击 → 恰好一个文件选择器 ----------
  let chooserCount = 0;
  page.on("filechooser", () => chooserCount++);

  const chooserPromise = page.waitForEvent("filechooser", { timeout: 5000 });
  await page.click("#btn-pick");
  const chooser = await chooserPromise;
  await page.waitForTimeout(400);
  if (chooserCount !== 1) {
    fail(`点击按钮后文件选择器事件次数 = ${chooserCount}（期望 1；>1 或 0 说明 double-click 回归）`);
  }
  console.log("✓ 点击「选择文件」按钮恰好打开 1 个文件选择器");

  // dropzone 空白处点击同样生效
  await chooser.setFiles([]); // 取消该对话框
  await page.waitForTimeout(200);
  const chooserPromise2 = page.waitForEvent("filechooser", { timeout: 5000 });
  await page.click("#dropzone", { position: { x: 40, y: 20 } });
  await chooserPromise2;
  console.log("✓ 点击拖拽区空白处同样打开文件选择器");

  // ---------- 3) 提供真实 PKG，等待条目树 ----------
  const chooserPromise3 = page.waitForEvent("filechooser", { timeout: 5000 });
  await page.click("#btn-pick");
  const chooser3 = await chooserPromise3;
  await chooser3.setFiles([pkgPath]);

  await page.waitForSelector("#docbar .docchip", { timeout: 15000 });
  const chipText = await page.textContent("#docbar .docchip .name");
  if (!chipText?.includes("test.pkg")) fail(`docchip 名称不符: ${chipText}`);
  console.log("✓ test.pkg 已打开，文档 chip 渲染");

  const rowCount = await page.locator("#tree li.row").count();
  if (rowCount < 10) fail(`条目树行数 ${rowCount} < 10`);
  console.log(`✓ 条目树渲染 ${rowCount} 行`);

  // ---------- 4) 点击 TEX 条目 → Worker 转码 → 真实 PNG 预览 ----------
  await page.locator("#tree li.row", { hasText: "waterripplenormal.tex" }).first().click();
  await page.waitForSelector("#detail-view img", { timeout: 30000 });

  const imgOk = await page.$eval("#detail-view img", (img) =>
    img.complete && img.naturalWidth > 0 && img.naturalHeight > 0);
  if (!imgOk) fail("TEX 转换后的 PNG 未成功解码显示");
  const natural = await page.$eval("#detail-view img", (img) => `${img.naturalWidth}x${img.naturalHeight}`);
  console.log(`✓ TEX → PNG 在真实浏览器中转码并显示（${natural}，OffscreenCanvas + Worker 管线贯通）`);

  // ---------- 5) 下载文件名与保存格式（回归：曾下载为无扩展名/被当 txt） ----------
  if (await page.$("#fmt-select") === null) fail("保存格式下拉框缺失");
  const fmtText = await page.textContent("#fmt-select");
  if (!fmtText?.includes("PNG")) fail(`格式下拉不含 PNG：${fmtText}`);

  const dl1Promise = page.waitForEvent("download", { timeout: 15000 });
  await page.click("#btn-dl");
  const dl1 = await dl1Promise;
  const suggested1 = dl1.suggestedFilename();
  if (suggested1 !== "waterripplenormal.png") {
    fail(`下载文件名不符: ${suggested1}（期望 waterripplenormal.png）`);
  }
  console.log("✓ 默认下载文件名带正确扩展名:", suggested1);

  await page.selectOption("#fmt-select", "webp");
  const dl2Promise = page.waitForEvent("download", { timeout: 20000 });
  await page.click("#btn-dl");
  const dl2 = await dl2Promise;
  const suggested2 = dl2.suggestedFilename();
  if (!suggested2.endsWith(".webp")) {
    fail(`切换 WebP 后下载文件名不符: ${suggested2}`);
  }
  const webpOk = await dl2.path().then((p) => {
    const head = readFileSync(p).subarray(0, 16);
    // RIFF....WEBP
    return head[0] === 0x52 && head[1] === 0x49 && head[8] === 0x57 && head[9] === 0x45 && head[10] === 0x42 && head[11] === 0x50;
  });
  if (!webpOk) fail("WebP 重编码产物魔数不对（RIFF/WEBP）");
  console.log("✓ 保存格式切换为 WebP 生效（重编码真实生效，魔数校验通过）");

  // 二进制条目 raw 预览判断 + 下载原始
  await page.locator("#tree li.row", { hasText: "effect.json" }).first().click();
  await page.waitForSelector("#detail-view .placeholder", { timeout: 10000 });
  console.log("✓ 二进制条目走「无法预览、可下载」分支");

  if (pageErrors.length > 0) {
    fail(`页面存在未捕获异常: ${pageErrors.join(" | ")}`);
  }
  console.log("✓ 无未捕获异常");

  console.log("\n全部 e2e 冒烟通过 ✅");
} finally {
  await browser.close();
}

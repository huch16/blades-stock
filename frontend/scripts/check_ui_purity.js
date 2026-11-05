#!/usr/bin/env node
/**
 * check_ui_purity.js —— 前端 UI 库纯净度门禁 (Single-UI-Library Guard)
 *
 * ── 背景 ──────────────────────────────────────────────────────────────────────
 * bladеs-stock 历史上同时存在两套 UI 库：
 *   · Naive UI  —— 全站主 UI 库（首屏已加载）
 *   · TDesign   —— 仅由 Agent 对话页 3 个聊天组件引入
 *
 * 代价实测：
 *   vendor-tdesign  raw 1,167,512 B / brotli 364,031 B (355 KB)
 *   实际换取：t-chat-sender / t-chat-content / t-avatar / t-button ≈ 4 个组件
 *   → 等于为「同一件事付了两次钱」，且第二次贵 355 KB。
 *
 * 因此本门禁把「双 UI 库回潮」升级为**构建期硬阻断**：
 *   规则 1：源码中禁止出现任何 tdesign 系 import；
 *   规则 2：模板中禁止出现任何 <t-xxx> TDesign 自定义元素；
 *   规则 3：package.json 依赖树中禁止出现 tdesign 系包；
 *   规则 4（可选）：若 dist/ 已存在，禁止产物中出现 vendor-tdesign chunk。
 *
 * 修复方式：统一改用 naive-ui（n-button / n-avatar / n-icon）或原生元素。
 */

import fs from 'fs';
import path from 'path';
import { fileURLToPath } from 'url';

const __filename = fileURLToPath(import.meta.url);
const __dirname = path.dirname(__filename);
const rootDir = path.resolve(__dirname, '..');
const srcDir = path.join(rootDir, 'src');

const FORBIDDEN_PACKAGES = [
  'tdesign-vue-next',
  '@tdesign-vue-next/chat',
  '@tdesign-vue-next/auto-import-resolver',
  'tdesign-icons-vue-next',
];

const IMPORT_PATTERN = new RegExp(
  `from\\s+['"](${FORBIDDEN_PACKAGES.map((p) => p.replace(/[/@]/g, (c) => `\\${c}`)).join('|')})['"]`
);
/** TDesign 自定义元素：<t-chat-sender> / <t-avatar> / <t-button> ... */
const TDESIGN_TAG_PATTERN = /<\s*t-[a-z][a-z0-9-]*[\s/>]/;
/** Vue 内置 <transition>/<teleport>/<template> 等不以 "t-" 开头，天然不冲突 */

function getAllFiles(dir, exts = ['.vue', '.js', '.ts', '.tsx', '.jsx']) {
  if (!fs.existsSync(dir)) return [];
  let results = [];
  for (const entry of fs.readdirSync(dir)) {
    const full = path.join(dir, entry);
    const stat = fs.statSync(full);
    if (stat.isDirectory()) results = results.concat(getAllFiles(full, exts));
    else if (exts.includes(path.extname(full))) results.push(full);
  }
  return results;
}

const failures = [];

// ── 规则 1 & 2：源码扫描 ─────────────────────────────────────────────────────
const sourceFiles = getAllFiles(srcDir);
for (const file of sourceFiles) {
  const content = fs.readFileSync(file, 'utf-8');
  const rel = path.relative(rootDir, file);

  const importMatch = content.match(IMPORT_PATTERN);
  if (importMatch) {
    failures.push(`❌ [UI-PURITY] ${rel}: 禁止再引入 TDesign 包 → "${importMatch[0]}"`);
  }

  // 仅对 .vue 的 <template> 段做标签检查，避免误伤注释与字符串
  if (file.endsWith('.vue')) {
    const tplMatch = content.match(/<template\b[^>]*>([\s\S]*)<\/template>/i);
    const template = tplMatch ? tplMatch[1] : content;
    const tagMatch = template.match(TDESIGN_TAG_PATTERN);
    if (tagMatch) {
      failures.push(
        `❌ [UI-PURITY] ${rel}: 禁止使用 TDesign 自定义元素 "${tagMatch[0].trim()}"，请改用 naive-ui 或原生元素`
      );
    }
  }
}

// ── 规则 3：package.json 依赖树 ──────────────────────────────────────────────
const pkgPath = path.join(rootDir, 'package.json');
if (fs.existsSync(pkgPath)) {
  const pkg = JSON.parse(fs.readFileSync(pkgPath, 'utf-8'));
  const allDeps = {
    ...(pkg.dependencies || {}),
    ...(pkg.devDependencies || {}),
    ...(pkg.peerDependencies || {}),
    ...(pkg.optionalDependencies || {}),
  };
  for (const name of Object.keys(allDeps)) {
    if (FORBIDDEN_PACKAGES.includes(name) || name.startsWith('@tdesign-vue-next/') || name.startsWith('tdesign-')) {
      failures.push(`❌ [UI-PURITY] package.json: 依赖 "${name}" 必须移除（双 UI 库税 355 KB）`);
    }
  }
}

// ── 规则 4：产物扫描（仅在显式传入 --dist 时执行，由 postbuild 调用）────────
const checkDist = process.argv.includes('--dist');
const assetsDir = path.join(rootDir, 'dist', 'assets');
if (checkDist && fs.existsSync(assetsDir)) {
  for (const entry of fs.readdirSync(assetsDir)) {
    if (/tdesign/i.test(entry)) {
      failures.push(`❌ [UI-PURITY] dist/assets/${entry}: 产物中仍存在 TDesign chunk，构建未真正解耦`);
    }
  }
}

if (failures.length > 0) {
  console.error('🔍 [UI-Purity] 前端 UI 库纯净度校验未通过：\n');
  failures.forEach((f) => console.error('   ' + f));
  console.error(
    '\n🚨 Build blocked: 本仓库只允许 Naive UI 单一 UI 库。\n' +
      '   替换建议：t-button → n-button / t-avatar → n-avatar / t-chat-content → 本地 Markdown 渲染器 / t-chat-sender → 本地 ChatSender。\n'
  );
  process.exit(1);
}

console.log(
  `✅ [UI-Purity] 已校验 ${sourceFiles.length} 个源文件与依赖树：未发现任何 TDesign 残留（单一 UI 库：Naive UI）。` +
    (checkDist ? ' 产物 chunk 亦已确认无 TDesign。' : '') +
    '\n'
);

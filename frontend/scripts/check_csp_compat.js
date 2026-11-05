#!/usr/bin/env node
/**
 * check_csp_compat.js —— CSP 与源码资源引用一致性门禁
 *
 * ── 为什么需要它 ──────────────────────────────────────────────────────────────
 * 生产环境 CSP 为：
 *   default-src 'self' 'unsafe-inline' data: blob: https:;
 *   img-src 'self' data: blob: https:; ...
 *
 * 即**只允许 https: 的外部资源**。但如果源码里残留 `http://` 资源引用，
 * 浏览器会静默阻断它 —— 界面表现是「图片空白」，而构建、Lint、单测、接口探测
 * 全部正常通过。这是一类典型的「安全加固把功能打坏」协同缺口：
 * 加了 CSP 的人与写 `http://` 的人都不是故意的，但没人回头看两边是否一致。
 *
 * 实测案例：stock.vue 中 6 处 `http://image.sinajs.cn/newchart/...`（分时图/日K图）
 * 被自己新增的 CSP 全部阻断，而 `image.sinajs.cn` 本身完全支持 HTTPS。
 *
 * ── 规则 ─────────────────────────────────────────────────────────────────────
 * 扫描 src/ 下所有源码中的绝对 URL 字面量，若其 scheme 不被当前 CSP 对应指令放行
 * 即构建失败。仅放行：
 *   · localhost / 127.0.0.1 / 0.0.0.0 / [::1]（本地开发调试）
 *   · www.w3.org 的 XML/SVG 命名空间（纯标识符，不产生网络请求）
 */

import fs from 'node:fs';
import path from 'node:path';
import { fileURLToPath } from 'node:url';

const __dirname = path.dirname(fileURLToPath(import.meta.url));
const rootDir = path.resolve(__dirname, '..');
const repoRoot = path.resolve(rootDir, '..');
const srcDir = path.join(rootDir, 'src');

const LOCAL_HOSTS = /^(localhost|127\.0\.0\.1|0\.0\.0\.0|\[::1\])(:\d+)?$/i;
const NAMESPACE_HOSTS = /^www\.w3\.org$/i;

/**
 * 从 Go 服务端源码读取唯一权威的 CSP（避免两处定义漂移）
 *
 * ⚠️ 路径解析必须同时兼容两种运行环境，否则门禁会在 CI/Docker 里直接炸掉：
 *   1. 本地 monorepo：cwd = <repo>/frontend，CSP 在 <repo>/cmd/server/main.go
 *   2. Docker 多阶段构建：cwd = /build/frontend，Dockerfile 会把 cmd/ 拷到 /build/cmd
 *
 * 因此这里不能写死单一路径，而是从脚本目录逐级向上查找。
 * 另支持 BLADES_CSP_SOURCE 显式指定（用于非常规目录布局）。
 */
function findServerCSPFiles() {
  const found = [];
  const explicit = process.env.BLADES_CSP_SOURCE;
  if (explicit) found.push(path.resolve(explicit));

  // 从脚本所在目录逐级向上查找，最多 5 级（frontend -> repo -> ... -> 根）
  let dir = rootDir;
  for (let i = 0; i < 5; i++) {
    found.push(path.join(dir, 'cmd', 'server', 'main.go'));
    found.push(path.join(dir, 'cmd', 'server', 'static.go'));
    const parent = path.dirname(dir);
    if (parent === dir) break;
    dir = parent;
  }
  return found;
}

function readServerCSP() {
  const tried = [];
  for (const file of findServerCSPFiles()) {
    tried.push(file);
    if (!fs.existsSync(file)) continue;
    const content = fs.readFileSync(file, 'utf-8');
    const m = content.match(/Content-Security-Policy",\s*"([^"]+)"/);
    if (m) return { csp: m[1], file };
  }
  return { csp: null, tried };
}

/** 解析 CSP，返回 { directive: [sources] } */
function parseCSP(csp) {
  const out = {};
  for (const part of csp.split(';')) {
    const tokens = part.trim().split(/\s+/).filter(Boolean);
    if (!tokens.length) continue;
    out[tokens[0].toLowerCase()] = tokens.slice(1);
  }
  return out;
}

/** 给定 scheme，判断 CSP 指令是否放行 */
function directiveAllows(directives, directive, scheme) {
  const sources = directives[directive] || directives['default-src'] || [];
  if (sources.includes(`${scheme}:`)) return true;
  if (sources.includes('*')) return true;
  return false;
}

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

const { csp, file: cspFile, tried } = readServerCSP();
if (!csp) {
  console.error('❌ [CSP-Compat] 无法从 cmd/server 读取 Content-Security-Policy，门禁无法工作。');
  console.error('   已尝试以下位置（均不存在或未匹配到 CSP 定义）：');
  for (const p of tried) console.error('     · ' + p);
  console.error('\n   → 本地运行：请在仓库根目录执行，或设置 BLADES_CSP_SOURCE=<路径>/main.go');
  console.error('   → Docker 构建：请确认 frontend-builder 阶段已 COPY cmd/（见 Dockerfile）\n');
  process.exit(1);
}
const directives = parseCSP(csp);

// scheme → 需要检查的 CSP 指令（取最严格的那条作为判据）
const SCHEME_GUARDS = [
  { scheme: 'http', directive: 'img-src', label: '图片' },
  { scheme: 'ws', directive: 'connect-src', label: 'WebSocket' },
];

const failures = [];
const files = getAllFiles(srcDir);

for (const file of files) {
  const content = fs.readFileSync(file, 'utf-8');
  const rel = path.relative(repoRoot, file);
  const lines = content.split(/\r?\n/);

  lines.forEach((line, idx) => {
    const trimmed = line.trim();
    // 跳过注释行，避免把说明文字当成真实引用
    if (trimmed.startsWith('//') || trimmed.startsWith('*') || trimmed.startsWith('/*')) return;

    for (const { scheme, directive, label } of SCHEME_GUARDS) {
      const re = new RegExp(`['"\`]${scheme}://([^/'"\`\\s]+)`, 'gi');
      let m;
      while ((m = re.exec(line)) !== null) {
        const host = m[1];
        if (LOCAL_HOSTS.test(host) || NAMESPACE_HOSTS.test(host)) continue;
        if (directiveAllows(directives, directive, scheme)) continue;
        failures.push(
          `❌ [CSP-Compat] ${rel}:${idx + 1} ${label}引用了 ${scheme}://${host}，` +
            `但 CSP 的 ${directive} 未放行 "${scheme}:"（当前: ${(directives[directive] || directives['default-src'] || []).join(' ')}）` +
            `\n      → 浏览器会静默阻断该资源（界面表现为空白），请改用 https://`
        );
      }
    }
  });
}

if (failures.length > 0) {
  console.error('🔍 [CSP-Compat] 源码资源引用与 CSP 不一致：\n');
  failures.forEach((f) => console.error('   ' + f));
  console.error('\n🚨 Build blocked: 请修正为 CSP 允许的 scheme，或同步放宽 cmd/server 中的 CSP。\n');
  process.exit(1);
}

console.log(
  `✅ [CSP-Compat] 已校验 ${files.length} 个源文件：无被 CSP 阻断的资源引用（img-src/connect-src 均已对齐）。`
);
console.log(`   ↳ CSP 权威来源: ${path.relative(repoRoot, cspFile) || cspFile}\n`);

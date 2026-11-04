import fs from 'fs';
import path from 'path';
import { fileURLToPath } from 'url';

const __filename = fileURLToPath(import.meta.url);
const __dirname = path.dirname(__filename);
const srcDir = path.resolve(__dirname, '../src');

function getAllFiles(dir, exts = ['.vue', '.js', '.ts']) {
  let results = [];
  const list = fs.readdirSync(dir);
  for (const file of list) {
    const fullPath = path.join(dir, file);
    const stat = fs.statSync(fullPath);
    if (stat.isDirectory()) {
      results = results.concat(getAllFiles(fullPath, exts));
    } else if (exts.includes(path.extname(fullPath))) {
      results.push(fullPath);
    }
  }
  return results;
}

function extractScript(content, isVue) {
  if (!isVue) return content;
  const scriptRegex = /<script\b[^>]*>([\s\S]*?)<\/script>/gi;
  let match;
  let combined = '';
  while ((match = scriptRegex.exec(content)) !== null) {
    combined += match[1] + '\n';
  }
  return combined;
}

function analyzeFile(filePath) {
  const content = fs.readFileSync(filePath, 'utf-8');
  const isVue = filePath.endsWith('.vue');
  const code = extractScript(content, isVue);
  if (!code) return [];

  // 1. 收集所有 import 语句中绑定的标识符
  const imports = new Set();
  const importRegex = /import\s+([\s\S]*?)\s+from\s+['"][^'"]+['"]/g;
  let m;
  while ((m = importRegex.exec(code)) !== null) {
    const clause = m[1].trim();
    // 处理 import { a, b as c }
    const namedMatch = clause.match(/\{([\s\S]*?)\}/);
    if (namedMatch) {
      namedMatch[1].split(',').forEach(part => {
        const item = part.trim();
        if (!item) return;
        const asMatch = item.match(/\bas\s+([A-Za-z0-9_$]+)/);
        if (asMatch) {
          imports.add(asMatch[1]);
        } else {
          const rawName = item.split(/\s+/)[0];
          if (rawName) imports.add(rawName);
        }
      });
    }
    // 处理 default import 或 namespace import: import Foo, * as Bar
    const withoutNamed = clause.replace(/\{[\s\S]*?\}/, '').trim();
    if (withoutNamed) {
      withoutNamed.split(',').forEach(part => {
        const item = part.trim();
        if (!item) return;
        const nsMatch = item.match(/\*\s+as\s+([A-Za-z0-9_$]+)/);
        if (nsMatch) {
          imports.add(nsMatch[1]);
        } else {
          imports.add(item);
        }
      });
    }
  }

  // 2. 收集本地声明 (const, let, var, function, class)
  const declRegex = /\b(?:const|let|var|function|class)\s+([A-Za-z0-9_$]+)/g;
  while ((m = declRegex.exec(code)) !== null) {
    imports.add(m[1]);
  }

  // 3. 收集常见全局与内置标识符
  const globals = new Set([
    'h', 'createVNode', 'resolveComponent', 'window', 'document', 'console',
    'Object', 'Array', 'String', 'Number', 'Boolean', 'Math', 'Date', 'RegExp',
    'JSON', 'Promise', 'Set', 'Map', 'WeakSet', 'WeakMap', 'Error', 'TypeError',
    'RangeError', 'ReferenceError', 'SyntaxError', 'localStorage', 'sessionStorage',
    'navigator', 'location', 'fetch', 'setTimeout', 'clearTimeout', 'setInterval',
    'clearInterval', 'requestAnimationFrame', 'cancelAnimationFrame', 'undefined',
    'null', 'NaN', 'Infinity', 'this', 'arguments'
  ]);

  // 4. 扫描 h(Identifier, ...) 或 createVNode(Identifier, ...)
  // 注意排查字符串如 h('div', ...)
  const hCallRegex = /\b(?:h|createVNode)\s*\(\s*([A-Za-z_$][A-Za-z0-9_$]*)/g;
  const errors = [];
  while ((m = hCallRegex.exec(code)) !== null) {
    const ident = m[1];
    if (globals.has(ident)) continue;
    // 凡是以大写字母开头的标识符，通常为组件、图标或枚举
    if (/^[A-Z]/.test(ident)) {
      if (!imports.has(ident)) {
        errors.push(ident);
      }
    }
  }

  return [...new Set(errors)];
}

console.log('🔍 [Prebuild Check] Scanning all Vue and JS/TS components for unimported h() render identifiers...');
const files = getAllFiles(srcDir);
let hasFailure = false;

for (const file of files) {
  const missing = analyzeFile(file);
  if (missing.length > 0) {
    const relPath = path.relative(path.resolve(__dirname, '..'), file);
    console.error(`❌ [BUILD-BLOCKER] In ${relPath}: Unimported component identifier(s) in h() render function: ${missing.join(', ')}`);
    hasFailure = true;
  }
}

if (hasFailure) {
  console.error('\n🚨 Build blocked: One or more component identifiers in h() calls are not explicitly imported or declared.');
  console.error('   Please import them from naive-ui / @vicons / their source file or use native HTML tags.\n');
  process.exit(1);
} else {
  console.log(`✅ [Prebuild Check] Verified ${files.length} source files. 0 unimported component identifiers detected in render functions.\n`);
}

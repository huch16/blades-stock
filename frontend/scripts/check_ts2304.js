// Build gate: block the build on runtime-breaking "Cannot find name" (TS2304)
// errors in <script> code, while tolerating vue-tsc template false positives.
//
// Background: the 2026-11-21 refactor split (R-22a/R-22b) dropped 7 imports
// (LineStyle, CLR_FALL/CLR_RISE, sortKey, toChartTime, parseNumStr,
// Environment, UpdateGroupSort, GetGroupList). `vite build` does not type-check,
// so they only surfaced as runtime ReferenceErrors in the browser (blank K-line
// chart). `vue-tsc --noEmit` catches them, but it also reports ~20 template
// TS2304s in settings.vue that are FALSE POSITIVES: `cfg`/`idx` are v-for
// aliases on <n-card v-for="(cfg, idx) in aiConfigs"> and the Vue compiler
// correctly scopes them (verified: compiled render puts cfg.baseUrl inside the
// _renderList callback). vue-tsc loses v-for scope after <template v-if> blocks.
// Template TS2304s cannot throw ReferenceError at runtime anyway (unbound
// template identifiers resolve via _ctx to undefined), so this gate only blocks
// on TS2304s in <script> / .ts code, and lists template ones as warnings.
import { spawnSync } from 'node:child_process';
import fs from 'node:fs';
import path from 'node:path';
import { fileURLToPath } from 'node:url';

const __dirname = path.dirname(fileURLToPath(import.meta.url));
const frontendDir = path.resolve(__dirname, '..');
const vueTscBin = path.join(
  frontendDir,
  'node_modules',
  '.bin',
  process.platform === 'win32' ? 'vue-tsc.cmd' : 'vue-tsc',
);

function templateRegionLines(filePath) {
  // Returns [startLine, endLine] (1-based, inclusive) of the SFC <template> block,
  // or null when the file has no template block.
  const lines = fs.readFileSync(filePath, 'utf-8').split('\n');
  let start = -1;
  let end = -1;
  lines.forEach((line, i) => {
    if (start === -1 && /<template(\s|>)/.test(line)) start = i + 1;
    if (/^\s*<\/template>/.test(line)) end = i + 1;
  });
  if (start === -1 || end === -1 || end < start) return null;
  return [start, end];
}

function main() {
  if (!fs.existsSync(vueTscBin)) {
    console.error('❌ [TS2304-Gate] vue-tsc not found. Run `npm install` first.');
    process.exit(1);
  }
  console.log('🔍 [TS2304-Gate] Running vue-tsc --noEmit ...');
  const res = spawnSync(vueTscBin, ['--noEmit'], {
    cwd: frontendDir,
    encoding: 'utf-8',
    maxBuffer: 64 * 1024 * 1024,
  });
  const output = (res.stdout || '') + (res.stderr || '');
  const re = /^(.+?)\((\d+),\d+\): error TS2304: Cannot find name '([^']+)'\./gm;
  const blocking = [];
  const templateIgnored = [];
  let m;
  while ((m = re.exec(output)) !== null) {
    const [, relPath, lineStr, name] = m;
    const line = Number(lineStr);
    const absPath = path.resolve(frontendDir, relPath);
    let isTemplate = false;
    if (relPath.endsWith('.vue') && fs.existsSync(absPath)) {
      const region = templateRegionLines(absPath);
      if (region && line >= region[0] && line <= region[1]) isTemplate = true;
    }
    const entry = `${relPath}(${line}): '${name}'`;
    if (isTemplate) templateIgnored.push(entry);
    else blocking.push(entry);
  }

  if (templateIgnored.length > 0) {
    console.log(
      `⚠️  [TS2304-Gate] ${templateIgnored.length} template TS2304(s) ignored ` +
        `(known vue-tsc v-for false positives, e.g. settings.vue cfg/idx):`,
    );
    templateIgnored.slice(0, 5).forEach((e) => console.log(`    - ${e}`));
    if (templateIgnored.length > 5) {
      console.log(`    ... and ${templateIgnored.length - 5} more (see vue-tsc output)`);
    }
  }
  if (blocking.length > 0) {
    console.error(
      `❌ [TS2304-Gate] ${blocking.length} blocking TS2304(s) in <script>/.ts code ` +
        `(unbound identifier = runtime ReferenceError):`,
    );
    blocking.forEach((e) => console.error(`    - ${e}`));
    console.error('🚨 Build blocked: restore the missing import(s) first.');
    process.exit(1);
  }
  console.log('✅ [TS2304-Gate] No blocking TS2304 errors.');
}

main();

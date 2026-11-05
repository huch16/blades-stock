import fs from 'node:fs';
import path from 'node:path';
import { fileURLToPath } from 'node:url';
import js from '@eslint/js';
import tseslint from 'typescript-eslint';
import vuePlugin from 'eslint-plugin-vue';
import unusedImports from 'eslint-plugin-unused-imports';
import globals from 'globals';

const __dirname = path.dirname(fileURLToPath(import.meta.url));

// ==============================================================================
// Blades Stock — 前端静态质量门禁
//
// 设计原则（为什么不是 vue/flat/recommended）：
//   1. **只保留「正确性」规则，剔除「排版」规则。**
//      `flat/recommended` 会打开 400+ 条 vue/max-attributes-per-line、
//      vue/singleline-html-element-content-newline 之类的纯格式规则，
//      让 lint 输出被噪声淹没、彻底失去门禁效力（实测单文件 600+ warning）。
//      本仓库没有 Prettier，格式规则只会制造噪声，因此统一使用 flat/essential。
//   2. **错误必须可修、可阻断；警告不得阻断 CI。** `npm run lint` 只对 error 返回非零。
//   3. **把历史踩过的坑固化为规则。** 详见下方 vue/no-undef-components。
// ==============================================================================

// 读取 unplugin-vue-components 自动生成的组件清单（src/components/** 自动注册）。
// 这些组件在模板中无需 import 即可使用，必须全部纳入白名单，否则误报。
function collectAutoRegisteredComponents() {
  const dtsPath = path.join(__dirname, 'components.d.ts');
  const names = new Set();
  try {
    const dts = fs.readFileSync(dtsPath, 'utf-8');
    const re = /^\s{4}([A-Z][A-Za-z0-9_$]*)\s*:\s*typeof import\(/gm;
    let m;
    while ((m = re.exec(dts)) !== null) names.add(m[1]);
  } catch {
    // components.d.ts 不存在（首次 checkout）时安全降级为空白名单
  }
  // PascalCase → kebab-case，覆盖 <stock-card /> 与 <StockCard /> 两种写法
  const kebab = (s) => s.replace(/([a-z0-9])([A-Z])/g, '$1-$2').toLowerCase();
  const patterns = [];
  for (const n of names) {
    patterns.push(`^${n}$`);
    patterns.push(`^${kebab(n)}$`);
  }
  return patterns;
}

export default [
  {
    ignores: [
      'dist/**',
      'node_modules/**',
      '*.config.*',
      '*.d.ts',
      'scripts/**',
      'wailsjs/**',
    ],
  },

  js.configs.recommended,
  ...tseslint.configs.recommended,
  ...vuePlugin.configs['flat/essential'],

  {
    files: ['**/*.{js,ts,vue}'],
    languageOptions: {
      ecmaVersion: 2022,
      sourceType: 'module',
      globals: {
        ...globals.browser,
        ...globals.node,
        ...globals.es2021,
      },
      parserOptions: {
        ecmaVersion: 2022,
        sourceType: 'module',
        // ── 关键修复 ──────────────────────────────────────────────
        // .vue 的 <template> 由 vue-eslint-parser 处理，但 <script setup lang="ts">
        // 内部的 TypeScript 语法（类型注解、泛型、defineProps<T>()、as 断言等）
        // 必须再交给 @typescript-eslint/parser，否则全线报
        // `Parsing error: Unexpected token :/{/?`，`npm run lint` 形同虚设。
        parser: tseslint.parser,
      },
    },
    plugins: {
      vue: vuePlugin,
      'unused-imports': unusedImports,
    },
    rules: {
      // ── 日志与调试 ───────────────────────────────────────────────
      'no-console': 'off',
      'no-debugger': process.env.NODE_ENV === 'production' ? 'error' : 'warn',

      // ── 未使用代码：import 交给 unused-imports 以便自动 --fix ──────
      '@typescript-eslint/no-unused-vars': 'off',
      'no-unused-vars': 'off',
      'unused-imports/no-unused-imports': 'error',
      'unused-imports/no-unused-vars': [
        'error',
        {
          vars: 'all',
          varsIgnorePattern: '^_',
          args: 'after-used',
          argsIgnorePattern: '^_',
          caughtErrors: 'all',
          caughtErrorsIgnorePattern: '^_',
          ignoreRestSiblings: true,
        },
      ],

      // ── 🚨 历史事故固化：模板中的未定义组件 ─────────────────────────
      // 事故背景：researchIndex.vue 曾在 DataTable 的 render 中调用未导入的
      // NSpace → `ReferenceError: NSpace is not defined` → 整视图被 ErrorBoundary
      // 熔断。模板侧同类问题（如误留 <t-chat-sender>）此前完全无门禁。
      // 白名单 = Naive UI 全局注册组件(app.use(naive)) + 路由/内置元素
      //          + unplugin-vue-components 自动注册清单(components.d.ts)
      // 因此任何「既没 import、也不在白名单」的组件都会被构建前拦截。
      'vue/no-undef-components': [
        'error',
        {
          ignorePatterns: [
            '^n-[a-z0-9-]+$',
            '^N[A-Z][A-Za-z0-9]*$',
            '^(router-view|router-link|RouterView|RouterLink)$',
            '^(transition|Transition|transition-group|TransitionGroup)$',
            '^(keep-alive|KeepAlive|teleport|Teleport|suspense|Suspense|component|slot)$',
            ...collectAutoRegisteredComponents(),
          ],
        },
      ],

      // ── 🚨 历史事故固化：模板中未声明的变量 ─────────────────────────
      // 姊妹规则。Vue 会把模板里未声明的标识符编译成 `_ctx.xxx`，渲染期静默得到
      // undefined，构建与首屏均不报错；但一旦在事件处理函数里对其 `.value` 赋值
      // 就会抛 ReferenceError。实测在 stockChangesMonitor.vue 命中 7 处
      // （klineCode / klineName / showKlineModal / diagnosisStockCode /
      //   diagnosisStockName / showDiagnosisDrawer / darkTheme），
      // 用户点击「📈 K线」或「📊 诊断」即被 ErrorBoundary 熔断。
      'vue/no-undef-properties': 'error',

      // ── 危险写法收口（正确性，非风格）─────────────────────────────
      // 未使用变量已由 unused-imports 接管；此处仅收紧真正易出错处
      eqeqeq: ['error', 'always', { null: 'ignore' }],
      'no-var': 'error',
      'prefer-const': 'error',

      // ── 类型收口：显式 any 记录为警告（历史存量较多，不阻断）──────
      '@typescript-eslint/no-explicit-any': 'warn',
      '@typescript-eslint/explicit-module-boundary-types': 'off',
      '@typescript-eslint/no-unsafe-function-type': 'error',
      '@typescript-eslint/no-empty-object-type': 'off',
      '@typescript-eslint/no-namespace': 'off',

      // ── 空块：允许带注释的空 catch（如 intentionally ignored）──────
      'no-empty': ['error', { allowEmptyCatch: true }],

      // ── 兼容既有工程习惯 ─────────────────────────────────────────
      'vue/multi-word-component-names': 'off',
      'vue/require-default-prop': 'off',
      'vue/attribute-hyphenation': 'off',
      'vue/v-on-event-hyphenation': 'off',
      'vue/html-self-closing': 'off',
    },
  },
];

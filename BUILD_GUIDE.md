# 构建与开发流程（v3.0.0）

> 早期版本的本文件曾引用 `.github/workflows/ci.yml` 与 `golangci-lint`。
> 该 CI 工作流已在精简过程中移除，`golangci-lint` 也未随项目安装，
> 因此本文件已按**当前真实可用的工具链**重写。

## 一键构建

```bash
make build          # 桌面端（Windows）：调用 build.ps1 wails
make build-web      # 纯 Go Web 服务端：产出 build/bin/blades-stock-server
make help           # 查看全部可用目标
```

Windows 上也可直接双击根目录的便捷脚本（它们都是 `build.ps1` 的薄封装）：

| 脚本 | 等价命令 |
|---|---|
| `build_frontend.bat` | `build.ps1 frontend` |
| `go_build.bat` | `build.ps1 go` |
| `wails_build.bat` | `build.ps1 wails` |
| `wails_help.bat` | `build.ps1 help` |

## 质量检查

```bash
make lint            # go vet（+ staticcheck，若已安装）
make frontend-lint   # ESLint：基线 0 errors / 150 warnings（仅 error 阻断）
```

前端还有三道**构建期硬门禁**，由 `frontend/package.json` 的
`prebuild` / `postbuild` 自动执行，无需手工调用：

| 脚本 | 拦截什么 | 需要构建上下文里的什么 |
|---|---|---|
| `frontend/scripts/check_vue_imports.js` | `h()` 调用中未导入的组件标识符（`NSpace` 事故原型） | 仅 `frontend/src` |
| `frontend/scripts/check_ui_purity.js` | TDesign 双 UI 库回潮（源码 / 模板 / 依赖树 / 产物 四重） | 仅 `frontend/` |
| `frontend/scripts/check_csp_compat.js` | 源码中的资源引用与 `cmd/server` 的 CSP 不一致（混合内容） | **`cmd/server/*.go`（仓库根，不在 frontend/ 内）** |

### ⚠️ 门禁必须在「两种目录布局」下都能工作

这是本仓库的一个**已踩过的坑**：`check_csp_compat.js` 以服务端为 CSP 的**唯一权威来源**
（避免两处定义漂移），因此它必须能够读到仓库根的 `cmd/server/main.go`。但 Docker 多阶段构建的
`frontend-builder` 阶段只看得到 `frontend/`，导致：

```text
本地 `npm run build`      → ✅ 通过（能找到 cmd/server/main.go）
Docker `docker compose build` → ❌ 失败，报「无法从 cmd/server 读取 Content-Security-Policy」
```

这是典型的「**门禁在开发环境有效、在交付环境失效**」协同缺口。当前约定：

1. **Dockerfile 必须**在 `frontend-builder` 阶段拷贝 `cmd/`：
   ```dockerfile
   COPY cmd/ /build/cmd/
   ```
   目标路径**故意**写成绝对的 `/build/cmd`，以匹配脚本从 `/build/frontend` 逐级向上查找的逻辑。
   （`npm run build` 只作用于 `frontend/`，`/build/cmd` 在 vite root 之外，不会被打包进产物。）
2. `check_csp_compat.js` 从自身目录**逐级向上查找** `cmd/server/{main.go,static.go}`（最多 5 级），
   因此本地 monorepo 与 Docker 两种布局均可工作。
3. 可用 `BLADES_CSP_SOURCE=<路径>/main.go` 显式指定权威 CSP 来源（应对非常规布局）。
4. 若确实找不到，门禁 **fail-closed**（退出码 1）并列出所有已尝试路径与修复指引 ——
   绝不静默跳过（能静默跳过的门禁等于没有门禁）。

改完 Dockerfile 或门禁脚本后，可直接验证 Docker 布局是否正常：

```bash
# 模拟 Docker 的目录布局，确认三条门禁在交付环境下也能跑通
TMP=$(mktemp -d)
mkdir -p "$TMP/frontend" "$TMP/cmd"
cp -r frontend/scripts frontend/src "$TMP/frontend/"
cp -r cmd/server "$TMP/cmd/"
(cd "$TMP/frontend" && node scripts/check_csp_compat.js)
```

## 测试

```bash
make test              # 单元测试（排除 integration 标签）
make test-integration  # 集成测试（需要本机安装 Chrome）
```

## 部署后自检

```bash
bash scripts/smoke-after-deploy.sh http://127.0.0.1:8000
```

27 项关键不变量（健康探针 / 发布韧性 / 首屏体积基线 / 缓存策略 / 敏感路径 404 /
SSE 链路 / 安全响应头 / 产物 CSP 一致性）。`scripts/deploy-vps.sh` 已自动调用它，
**自检失败会以 exit 1 结束并提示「请勿宣布发布成功」**。

## 版本发布

1. 更新版本号：`main.go`、`cmd/server/main.go`、`wails.json`、`frontend/package.json`、
   `frontend/index.html`、`scripts/deploy-vps.sh`、`docker-compose.yml`、`README.md`
2. 在 `RELEASE_NOTES.md` 顶部追加新版本章节
3. `cd frontend && npm run build`（触发三道门禁）
4. `go build ./...` && `make test`
5. `git tag -a vX.Y.Z -m "..."` 并推送
6. VPS：`bash scripts/deploy-vps.sh`

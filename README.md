# Blades Stock — 智能量化投研工作站

个人 A 股量化投研工作站：实时行情、K 线与技术指标、量化回测、ETF 轮动、AI Agent 辅助研判。同一套代码支持桌面端与 Web 服务端。

个人项目，仅供个人使用。

## 运行

### 桌面端

```bash
# Windows
.\build.ps1 wails

# Linux / macOS
./scripts/build.sh
```

产物在 `build/bin/`。

### Web 服务端（Docker，推荐）

```bash
cp .env.example .env   # 填好 BLADES_AUTH_TOKEN
docker compose up -d --build
```

默认监听 `8000` 端口，浏览器打开 `http://服务器地址:8000`，输入口令登录。

### Web 服务端（直接运行）

```bash
npm --prefix frontend run build
CGO_ENABLED=0 go build -o build/bin/blades-stock-server ./cmd/server
./build/bin/blades-stock-server --port 8000 --data-dir ./data --dist-dir ./frontend/dist
```

## 配置

`.env`（见 `.env.example`，不要提交）：

| 变量 | 说明 |
| --- | --- |
| `BLADES_AUTH_TOKEN` | Web 端访问口令，必填。未设定时首次启动会随机生成并固化到 `data/auth.token`，同时在启动日志里打印一次 |
| `PORT` | 服务监听端口，默认 `8000` |
| `BLADES_STOCK_DATA_DIR` | 数据目录，默认 `./data` |
| `SEARXNG_URL` | 聚合搜索端点，默认 `https://search.woha.men` |
| `TICKFLOW_API_KEY` | 外部行情服务 Key，留空表示不启用 |

## 开发

```bash
bash scripts/verify-all.sh       # 一键全量验证（推荐，含 gofmt/vet/分层/前后端测试与构建）
go build ./...                  # 编译 Go 后端
go vet ./...                    # 静态检查
go test ./...                   # 单元测试
bash scripts/check-deps.sh      # 架构分层检查，改完代码后必跑
cd frontend && npm run dev      # 前端开发模式
cd frontend && npm run build    # 前端构建
```

## 目录

| 目录 | 说明 |
| --- | --- |
| `backend/` | Go 后端：Agent、量化引擎、数据接入、服务层 |
| `frontend/src/` | Vue 3 前端 |
| `cmd/server/` | Web 服务端入口 |
| `scripts/` | 构建、部署、检查脚本 |
| `build/` | 打包文件与股票基础数据（程序启动时加载） |
| `data/` | 运行数据库（SQLite，唯一权威库） |

## 免责声明

1. **非投资建议**：本系统输出的全部内容仅供个人学习、量化研究与技术探讨使用，不构成任何形式的投资建议、买卖要约或收益承诺。
2. **市场风险提示**：金融市场有风险，投资需谨慎。基于本系统数据、AI 建议或量化模型做出的任何交易决策，须自行承担全部盈亏与合规责任。

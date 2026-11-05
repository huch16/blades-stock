# docs/ 目录索引

本目录只保留**仍然有效**的设计文档；一次性开发过程报告统一归档到 [`archive/`](./archive/)。

## 现行文档

| 文件 | 内容 | 何时读 |
|---|---|---|
| [`blades-stock使用手册.md`](./blades-stock使用手册.md) | 面向使用者的完整手册。**同时被 `//go:embed` 打包进二进制**，由 RPC `GetUserManual()` 返回 | 日常使用、答疑 |
| [`agent_architecture_sequence.md`](./agent_architecture_sequence.md) | Agent 核心时序与运行时约束架构规范 | 改动 Agent 执行链路前 |
| [`intelligent_capabilities_architecture.md`](./intelligent_capabilities_architecture.md) | 智能功能系统化架构全景图 | 理解整体能力地图 |
| [`settings_page_refactor_spec.md`](./settings_page_refactor_spec.md) | 设置页重构规格蓝图 | 改动设置页前 |
| [`intelligence/`](./intelligence/) | 投研知识库（catalog / extensions / prompts / strategies） | 扩展技能与策略 |

## 归档文档

`archive/` 下为带日期的一次性开发报告（AKShare / Baostock 集成、若干回测改进与测试报告、
VectorBT 引擎说明等）。它们**记录的是当时的过程，不代表当前状态**，
如需了解现状请以 `README.md` 与 `RELEASE_NOTES.md` 为准。

> ⚠️ 维护约定：新增设计文档请使用**不带日期**的语义化文件名（如 `etf_rotation_design.md`）；
> 一次性过程报告请直接放入 `archive/` 并在文件名中保留日期。

## 编码约定

`docs/blades-stock使用手册.md` 会被嵌入二进制，**必须保存为 UTF-8**。
历史版本曾误用 GBK，导致 `GetUserManual()` 返回乱码——请勿重复该错误。

// Package datasource 是纯 vendor 客户端层：发请求、解析、重试、超时。
//
// 规则（REFACTOR_DESIGN.md §2.2/§2.3）：
//   - 只依赖基础包（models、util、logger、paths 等），不碰 DB、不读设置表；
//   - 不 import agent、services、quant、data；
//   - 只有 backend/data 能 import 本包（根包、cmd、quant、tools 等一律经 data 间接使用）。
package datasource

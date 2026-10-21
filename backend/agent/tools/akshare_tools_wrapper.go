package tools

import (
	bladesTools "github.com/go-kratos/blades/tools"
)

// GetAkshareTools 获取 AKShare 数据工具集
// 注意：因远程 AKShare 服务（如 115.190.72.22:8089）失联，现已暂时禁用，
// 避免因 HTTP 请求挂起导致 Agent 陷入网络超时等待。
func GetAkshareTools() []bladesTools.Tool {
	return []bladesTools.Tool{}
}

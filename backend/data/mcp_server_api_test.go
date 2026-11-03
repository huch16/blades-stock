package data

import (
	"blades-stock/backend/models"
	"testing"
)

func TestMCPServerApi_NilQuerySafety(t *testing.T) {
	api := NewMCPServerApi()

	// 1. 测试 nil query 不会触发 nil pointer dereference panic
	resp := api.List(nil)
	if resp == nil {
		t.Fatalf("api.List(nil) 返回了 nil 响应对象")
	}
	if resp.Data == nil {
		t.Fatalf("api.List(nil).Data 为 nil，前端可能报错")
	}

	// 2. 测试空 query 不会触发 panic
	emptyQuery := &models.MCPServerQuery{}
	resp2 := api.List(emptyQuery)
	if resp2 == nil {
		t.Fatalf("api.List(&MCPServerQuery{}) 返回了 nil 响应对象")
	}

	// 3. 测试工具列表查询在各参数下的安全性
	tools := api.GetToolsByServerID(0)
	if tools == nil {
		t.Fatalf("api.GetToolsByServerID(0) 返回了 nil 切片")
	}

	allTools := api.GetAllTools()
	if allTools == nil {
		t.Fatalf("api.GetAllTools() 返回了 nil 切片")
	}
}

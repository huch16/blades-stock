package datasource

import "testing"

// TestNewWebSearchApi_TimeoutDefaults 构造器超时默认值：<=0 取 30。
// 原测试在 backend/data/web_search_api_test.go，以包外身份读未导出的 timeout
// 字段，R-25c 把 WebSearchApi 移入 datasource 后无法编译，故移植到本包内测。
func TestNewWebSearchApi_TimeoutDefaults(t *testing.T) {
	if got := NewWebSearchApi(0, nil, nil, nil).timeout; got != 30 {
		t.Errorf("NewWebSearchApi(0).timeout = %d, want 30", got)
	}
	if got := NewWebSearchApi(60, nil, nil, nil).timeout; got != 60 {
		t.Errorf("NewWebSearchApi(60).timeout = %d, want 60", got)
	}
	if got := NewWebSearchApi(-10, nil, nil, nil).timeout; got != 30 {
		t.Errorf("NewWebSearchApi(-10).timeout = %d, want 30", got)
	}
}

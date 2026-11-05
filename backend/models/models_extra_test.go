package models

import (
	"encoding/json"
	"reflect"
	"testing"
	"time"
)

func TestMCPServerJSONRoundTrip(t *testing.T) {
	orig := MCPServer{
		ID:         7,
		CreatedAt:  time.Date(2026, 1, 2, 3, 4, 5, 0, time.UTC),
		UpdatedAt:  time.Date(2026, 1, 2, 3, 4, 5, 0, time.UTC),
		Name:       "tushare",
		Type:       "sse",
		URL:        "http://localhost:9000",
		Enable:     true,
		Status:     "running",
		TestResult: "ok",
	}
	data, err := json.Marshal(orig)
	if err != nil {
		t.Fatalf("marshal MCPServer: %v", err)
	}
	var decoded MCPServer
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("unmarshal MCPServer: %v", err)
	}
	if !reflect.DeepEqual(decoded, orig) {
		t.Errorf("MCPServer round trip mismatch:\ngot  %+v\nwant %+v", decoded, orig)
	}
}

func TestMCPServerToolJSONRoundTrip(t *testing.T) {
	orig := MCPServerTool{
		ID:           3,
		MCPServerID:  7,
		ToolName:     "daily",
		Description:  "A股日线",
		ParamsSchema: `{"type":"object"}`,
		Enabled:      true,
	}
	data, err := json.Marshal(orig)
	if err != nil {
		t.Fatalf("marshal MCPServerTool: %v", err)
	}
	var decoded MCPServerTool
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("unmarshal MCPServerTool: %v", err)
	}
	if !reflect.DeepEqual(decoded, orig) {
		t.Errorf("MCPServerTool round trip mismatch:\ngot  %+v\nwant %+v", decoded, orig)
	}
}

func TestSkillJSONRoundTrip(t *testing.T) {
	orig := Skill{
		ID:              9,
		Name:            "ETF轮动",
		Description:     "多Agent分析",
		Category:        "analysis",
		SystemPrompt:    "prompt...",
		Directive:       "short directive",
		Examples:        "examples...",
		TriggerKeywords: "ETF,轮动",
		MCPServerIDs:    "1,2",
		Enable:          true,
		SortOrder:       1,
	}
	data, err := json.Marshal(orig)
	if err != nil {
		t.Fatalf("marshal Skill: %v", err)
	}
	var decoded Skill
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("unmarshal Skill: %v", err)
	}
	if decoded.Name != orig.Name || decoded.MCPServerIDs != orig.MCPServerIDs || decoded.SortOrder != orig.SortOrder {
		t.Errorf("Skill round trip mismatch:\ngot  %+v\nwant %+v", decoded, orig)
	}
}

func TestVersionInfoJSONRoundTrip(t *testing.T) {
	orig := VersionInfo{
		Version:        "1.2.3",
		Content:        "release notes",
		Icon:           "icon",
		Alipay:         "alipay",
		Wxpay:          "wxpay",
		BuildTimeStamp: 1767225600,
	}
	data, err := json.Marshal(orig)
	if err != nil {
		t.Fatalf("marshal VersionInfo: %v", err)
	}
	var decoded VersionInfo
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("unmarshal VersionInfo: %v", err)
	}
	if decoded.Version != orig.Version || decoded.BuildTimeStamp != orig.BuildTimeStamp {
		t.Errorf("VersionInfo round trip mismatch:\ngot  %+v\nwant %+v", decoded, orig)
	}
}

func TestTableNames(t *testing.T) {
	cases := []struct {
		name string
		got  string
		want string
	}{
		{"Telegraph", Telegraph{}.TableName(), "telegraph_list"},
		{"TelegraphTags", TelegraphTags{}.TableName(), "telegraph_tags"},
		{"Tags", Tags{}.TableName(), "tags"},
		{"LongTigerRankData", LongTigerRankData{}.TableName(), "long_tiger_rank"},
		{"OldSettings", OldSettings{}.TableName(), "settings"},
		{"CronTask", CronTask{}.TableName(), "cron_tasks"},
		{"AiAssistantSession", AiAssistantSession{}.TableName(), "ai_assistant_sessions"},
		{"Skill", Skill{}.TableName(), "skills"},
		{"CustomStrategy", CustomStrategy{}.TableName(), "custom_strategies"},
		{"CodeChunk", CodeChunk{}.TableName(), "code_chunks"},
		{"BacktestResult", BacktestResult{}.TableName(), "backtest_results"},
	}
	for _, c := range cases {
		if c.got != c.want {
			t.Errorf("%s TableName() = %q, want %q", c.name, c.got, c.want)
		}
	}
}

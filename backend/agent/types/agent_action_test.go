package types

import (
	"testing"
)

func TestParseAndExtractActions(t *testing.T) {
	raw := `分析完毕。当前天目药业量价形态出现顶背离，建议立刻查看 3D 量化诊断。
` + "```agent-action\n" + `{"type": "OPEN_DIAGNOSIS", "payload": {"code": "600671.SH", "name": "天目药业"}}` + "\n```\n" + `请务必保持纪律。`

	cleaned, actions := ParseAndExtractActions(raw)
	if len(actions) != 1 {
		t.Fatalf("expected 1 action, got %d", len(actions))
	}
	if actions[0].Type != ActionOpenDiagnosis {
		t.Errorf("expected OPEN_DIAGNOSIS, got %s", actions[0].Type)
	}
	if actions[0].Payload["code"] != "600671.SH" {
		t.Errorf("expected code 600671.SH, got %v", actions[0].Payload["code"])
	}
	if cleaned == "" || len(cleaned) >= len(raw) {
		t.Errorf("expected cleaned text to be shorter than raw text")
	}
}

package agent

import (
	"context"
	"os"
	"strings"
	"testing"
)

// fakeRetriever 是一个“随时能返回技能”的假 RAG 检索器：若 MatchSkills
// 仍走外部检索器回退，它会让下面的测试失败。
type fakeRetriever struct {
	searchCalled bool
}

func (f *fakeRetriever) EmbedText(ctx context.Context, text string) ([]float32, error) {
	return []float32{1, 0}, nil
}

func (f *fakeRetriever) Search(ctx context.Context, embedding []float32, topK int) ([]Document, error) {
	f.searchCalled = true
	// 指向 TestMain 预置的技能 id=1（'测试技能'）。
	return []Document{{ID: "1", Source: "skill", Meta: map[string]string{"skill_id": "1"}}}, nil
}

func (f *fakeRetriever) IndexDocuments(ctx context.Context, docs []Document) error { return nil }
func (f *fakeRetriever) DeleteDocuments(ctx context.Context, ids []string) error   { return nil }
func (f *fakeRetriever) Name() string                                              { return "fake" }
func (f *fakeRetriever) IsReady() bool                                             { return true }

// TestMatchSkills_RetrieverParamIgnored 回归测试：第四轮已删除
// matchSkillsViaRetriever 及其三处回退分支。即使调用方传入一个非 nil
// 且 IsReady 的 Retriever，MatchSkills 也必须走内部路径（此处服务
// 未启用 → 返回 nil），不得再走外部检索器。旧代码会在无关键词命中、
// 无 embedding 时回退到该检索器并返回技能 1。
func TestMatchSkills_RetrieverParamIgnored(t *testing.T) {
	fr := &fakeRetriever{}
	svc := GetSkillEmbeddingService(nil, fr)

	matched := svc.MatchSkills("完全不相关的提问内容 qwerty123", 5)
	if len(matched) != 0 {
		t.Fatalf("即使传入非 nil Retriever 也不应走外部检索，期望无匹配，实际 %v", matched)
	}
	if fr.searchCalled {
		t.Error("假检索器的 Search 被调用了：外部 Retriever 回退路径仍存在")
	}
}

// TestSkillEmbedding_NoDeadRetrieverPath 源码级守卫：
// matchSkillsViaRetriever 及其回退分支已删除，不得复活；Retriever
// 接口本身作为预留扩展缝保留在 retriever.go。
func TestSkillEmbedding_NoDeadRetrieverPath(t *testing.T) {
	src, err := os.ReadFile("skill_embedding.go")
	if err != nil {
		t.Fatalf("read skill_embedding.go: %v", err)
	}
	text := string(src)
	for _, dead := range []string{
		"func (s *SkillEmbeddingService) matchSkillsViaRetriever",
		"s.retr != nil",
		"retr:          retr",
	} {
		if strings.Contains(text, dead) {
			t.Errorf("skill_embedding.go 不应再含死 Retriever 路径 %q", dead)
		}
	}
	if !strings.Contains(text, "GetSkillEmbeddingService(es *EmbeddingService, retr Retriever)") {
		t.Error("GetSkillEmbeddingService 签名应保留 retr 参数（agent.go 只读，预留扩展缝）")
	}

	retSrc, err := os.ReadFile("retriever.go")
	if err != nil {
		t.Fatalf("read retriever.go: %v", err)
	}
	if !strings.Contains(string(retSrc), "type Retriever interface") {
		t.Error("retriever.go 应保留 Retriever 接口作为预留扩展缝")
	}
	if !strings.Contains(string(retSrc), "没有任何实现") {
		t.Error("retriever.go 应注明当前无实现，避免后来者误以为已接通")
	}
}

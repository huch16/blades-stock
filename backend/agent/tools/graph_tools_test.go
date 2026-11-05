package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// 写入一个 fixture graph.json 用于测试
func setupTestGraph(t *testing.T, nodes []graphNode, edges []graphEdge) {
	t.Helper()
	dir := t.TempDir()
	graphDir := filepath.Join(dir, ".graph")
	if err := os.MkdirAll(graphDir, 0755); err != nil {
		t.Fatal(err)
	}
	g := graphData{Nodes: nodes, Edges: edges}
	data, _ := json.Marshal(g)
	if err := os.WriteFile(filepath.Join(graphDir, "graph.json"), data, 0644); err != nil {
		t.Fatal(err)
	}
	// 替换路径：tools 包内部用 paths.BaseDir() 决定路径
	// 简单方案：通过 chdir
	cwd, _ := os.Getwd()
	if err := os.Chdir(dir); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chdir(cwd) })
	resetGraphCache() // 让懒加载在新路径生效
}

func TestLoadGraph_Missing(t *testing.T) {
	dir := t.TempDir()
	cwd, _ := os.Getwd()
	os.Chdir(dir)
	t.Cleanup(func() { _ = os.Chdir(cwd) })
	resetGraphCache()

	_, err := loadGraph()
	if err == nil {
		t.Fatal("expected error when graph.json missing")
	}
	if !strings.Contains(err.Error(), "graph.json 不存在") {
		t.Errorf("error should mention missing file: %v", err)
	}
}

func TestLoadGraph_OK(t *testing.T) {
	setupTestGraph(t,
		[]graphNode{{ID: "n1", Label: "n1", Kind: "function", SourceFile: "a.go", SourceLocation: "a.go:1"}},
		[]graphEdge{{Source: "a.go", Target: "n1", Relation: "defines"}},
	)
	g, err := loadGraph()
	if err != nil {
		t.Fatal(err)
	}
	if len(g.Nodes) != 1 || len(g.Edges) != 1 {
		t.Errorf("expected 1 node/edge, got %d/%d", len(g.Nodes), len(g.Edges))
	}
}

func TestLoadGraph_InvalidJSON(t *testing.T) {
	dir := t.TempDir()
	graphDir := filepath.Join(dir, ".graph")
	os.MkdirAll(graphDir, 0755)
	os.WriteFile(filepath.Join(graphDir, "graph.json"), []byte("{not json"), 0644)
	cwd, _ := os.Getwd()
	os.Chdir(dir)
	t.Cleanup(func() { _ = os.Chdir(cwd) })
	resetGraphCache()

	_, err := loadGraph()
	if err == nil {
		t.Fatal("expected parse error")
	}
	if !strings.Contains(err.Error(), "解析失败") {
		t.Errorf("error should mention parse: %v", err)
	}
}

// =============================================================================
// GraphQuery tool 行为测试
// =============================================================================

func TestGraphQuery_Callers(t *testing.T) {
	setupTestGraph(t,
		[]graphNode{
			{ID: "a.go#Foo", Label: "Foo", Kind: "function", SourceFile: "a.go", SourceLocation: "a.go:10"},
			{ID: "b.go", Label: "b.go", Kind: "file", SourceFile: "b.go", SourceLocation: "b.go"},
			{ID: "c.go", Label: "c.go", Kind: "file", SourceFile: "c.go", SourceLocation: "c.go"},
		},
		[]graphEdge{
			{Source: "a.go", Target: "a.go#Foo", Relation: "defines"},
			{Source: "b.go", Target: "a.go#Foo", Relation: "imports"},
			{Source: "c.go", Target: "a.go#Foo", Relation: "imports"},
		},
	)

	tool := GetGraphQueryTool()
	input := `{"action":"callers","target":"Foo"}`
	result, err := tool.Handle(context.Background(), input)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(result, "调用者") {
		t.Errorf("result should mention callers: %q", result)
	}
	if !strings.Contains(result, "b.go") || !strings.Contains(result, "c.go") {
		t.Errorf("result should list both b.go and c.go as callers: %q", result)
	}
}

func TestGraphQuery_Callees(t *testing.T) {
	setupTestGraph(t,
		[]graphNode{
			{ID: "a.go", Label: "a.go", Kind: "file", SourceFile: "a.go", SourceLocation: "a.go"},
			{ID: "b.go", Label: "b.go", Kind: "file", SourceFile: "b.go", SourceLocation: "b.go"},
			{ID: "c.go", Label: "c.go", Kind: "file", SourceFile: "c.go", SourceLocation: "c.go"},
		},
		[]graphEdge{
			{Source: "a.go", Target: "b.go", Relation: "imports"},
			{Source: "a.go", Target: "c.go", Relation: "imports"},
		},
	)

	tool := GetGraphQueryTool()
	input := `{"action":"callees","target":"a.go"}`
	result, err := tool.Handle(context.Background(), input)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(result, "b.go") || !strings.Contains(result, "c.go") {
		t.Errorf("result should list b.go and c.go: %q", result)
	}
}

func TestGraphQuery_Search(t *testing.T) {
	setupTestGraph(t,
		[]graphNode{
			{ID: "a.go#NewApp", Label: "NewApp", Kind: "function", SourceFile: "a.go", SourceLocation: "a.go:57"},
			{ID: "a.go#GetMachineId", Label: "GetMachineId", Kind: "function", SourceFile: "a.go", SourceLocation: "a.go:90"},
		},
		nil,
	)

	tool := GetGraphQueryTool()
	input := `{"action":"search","target":"App"}`
	result, err := tool.Handle(context.Background(), input)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(result, "NewApp") {
		t.Errorf("search for 'App' should find NewApp: %q", result)
	}
	if strings.Contains(result, "GetMachineId") {
		t.Errorf("search for 'App' should NOT find GetMachineId: %q", result)
	}
}

func TestGraphQuery_Community(t *testing.T) {
	setupTestGraph(t,
		[]graphNode{
			{ID: "a.go#F1", Label: "F1", Kind: "function", SourceFile: "a.go", SourceLocation: "a.go:1"},
			{ID: "a.go#F2", Label: "F2", Kind: "function", SourceFile: "a.go", SourceLocation: "a.go:10"},
			{ID: "b.go#F3", Label: "F3", Kind: "function", SourceFile: "b.go", SourceLocation: "b.go:1"},
		},
		nil,
	)

	tool := GetGraphQueryTool()
	input := `{"action":"community","target":"F1"}`
	result, err := tool.Handle(context.Background(), input)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(result, "F1") || !strings.Contains(result, "F2") {
		t.Errorf("community of F1 should include F1 and F2 (same file): %q", result)
	}
	if strings.Contains(result, "F3") {
		t.Errorf("community should not include F3 (different file): %q", result)
	}
}

func TestGraphQuery_UnknownAction(t *testing.T) {
	setupTestGraph(t, nil, nil)
	tool := GetGraphQueryTool()
	input := `{"action":"banana","target":"x"}`
	_, err := tool.Handle(context.Background(), input)
	if err == nil {
		t.Fatal("unknown action should error")
	}
}

func TestGraphQuery_MissingFile(t *testing.T) {
	dir := t.TempDir()
	cwd, _ := os.Getwd()
	os.Chdir(dir)
	t.Cleanup(func() { _ = os.Chdir(cwd) })
	resetGraphCache()

	tool := GetGraphQueryTool()
	input := `{"action":"search","target":"anything"}`
	result, err := tool.Handle(context.Background(), input)
	if err != nil {
		t.Fatal("missing file should return nil error with informative message")
	}
	if !strings.Contains(result, "代码图谱不可用") {
		t.Errorf("should return user-readable message: %q", result)
	}
}

func TestGraphQuery_NotFound(t *testing.T) {
	setupTestGraph(t,
		[]graphNode{{ID: "a.go#Foo", Label: "Foo", Kind: "function", SourceFile: "a.go", SourceLocation: "a.go:10"}},
		nil,
	)
	tool := GetGraphQueryTool()
	input := `{"action":"callers","target":"NonExistent"}`
	result, err := tool.Handle(context.Background(), input)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(result, "未找到节点") {
		t.Errorf("should report not found: %q", result)
	}
}

func TestGraphQuery_InvalidJSON(t *testing.T) {
	setupTestGraph(t, nil, nil)
	tool := GetGraphQueryTool()
	_, err := tool.Handle(context.Background(), "{not json")
	if err == nil {
		t.Fatal("invalid JSON should error")
	}
}

func TestCheckManifestStale_Fresh(t *testing.T) {
	dir := t.TempDir()
	manifest := map[string]interface{}{
		"commit_sha":     "abc1234",
		"stale_after":    time.Now().Add(24 * time.Hour).Format(time.RFC3339),
		"max_stale_days": 7,
	}
	data, _ := json.Marshal(manifest)
	os.WriteFile(filepath.Join(dir, "manifest.json"), data, 0644)
	if got := checkManifestStale(filepath.Join(dir, "manifest.json")); got != "" {
		t.Errorf("fresh manifest should not warn, got: %q", got)
	}
}

func TestCheckManifestStale_Expired(t *testing.T) {
	dir := t.TempDir()
	manifest := map[string]interface{}{
		"commit_sha":     "old1234",
		"stale_after":    time.Now().Add(-24 * time.Hour).Format(time.RFC3339),
		"max_stale_days": 7,
	}
	data, _ := json.Marshal(manifest)
	os.WriteFile(filepath.Join(dir, "manifest.json"), data, 0644)
	got := checkManifestStale(filepath.Join(dir, "manifest.json"))
	if got == "" {
		t.Fatal("expired manifest should warn")
	}
	if !strings.Contains(got, "old1234") {
		t.Errorf("warning should mention commit SHA: %q", got)
	}
	if !strings.Contains(got, "build.ps1") {
		t.Errorf("warning should suggest rebuild: %q", got)
	}
}

func TestCheckManifestStale_MissingFile(t *testing.T) {
	// 无 manifest 应静默（不影响功能）
	got := checkManifestStale(filepath.Join(t.TempDir(), "nonexistent.json"))
	if got != "" {
		t.Errorf("missing manifest should not warn: %q", got)
	}
}

func TestCheckManifestStale_InvalidJSON(t *testing.T) {
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "manifest.json"), []byte("{not valid"), 0644)
	got := checkManifestStale(filepath.Join(dir, "manifest.json"))
	if got != "" {
		t.Errorf("invalid JSON manifest should not warn: %q", got)
	}
}

// =============================================================================
// P4.1: checkCommitMismatch
// =============================================================================

// withMockGitSHA 在测试期间替换 gitHeadSHA，结束后恢复。
func withMockGitSHA(t *testing.T, mock func() (string, error)) {
	t.Helper()
	orig := gitHeadSHA
	gitHeadSHA = mock
	t.Cleanup(func() { gitHeadSHA = orig })
}

func writeManifest(t *testing.T, dir string, m graphManifest) {
	t.Helper()
	data, _ := json.Marshal(m)
	if err := os.WriteFile(filepath.Join(dir, "manifest.json"), data, 0644); err != nil {
		t.Fatal(err)
	}
}

func TestCheckCommitMismatch_Match(t *testing.T) {
	dir := t.TempDir()
	withMockGitSHA(t, func() (string, error) { return "abc1234", nil })
	writeManifest(t, dir, graphManifest{CommitSHA: "abc1234"})

	if got := checkCommitMismatch(filepath.Join(dir, "manifest.json")); got != "" {
		t.Errorf("matching commits should not warn, got: %q", got)
	}
}

func TestCheckCommitMismatch_Mismatch(t *testing.T) {
	dir := t.TempDir()
	withMockGitSHA(t, func() (string, error) { return "newhash", nil })
	writeManifest(t, dir, graphManifest{CommitSHA: "oldhash"})

	got := checkCommitMismatch(filepath.Join(dir, "manifest.json"))
	if got == "" {
		t.Fatal("mismatch should warn")
	}
	if !strings.Contains(got, "oldhash") || !strings.Contains(got, "newhash") {
		t.Errorf("warning should mention both commits: %q", got)
	}
	if !strings.Contains(got, "build.ps1") {
		t.Errorf("warning should suggest rebuild: %q", got)
	}
}

func TestCheckCommitMismatch_UnknownCommit(t *testing.T) {
	dir := t.TempDir()
	// "unknown" 是 build_graph.py 的 fallback，应静默不报
	writeManifest(t, dir, graphManifest{CommitSHA: "unknown"})
	withMockGitSHA(t, func() (string, error) { return "whatever", nil })
	if got := checkCommitMismatch(filepath.Join(dir, "manifest.json")); got != "" {
		t.Errorf("unknown commit should not warn: %q", got)
	}
}

func TestCheckCommitMismatch_EmptyCommit(t *testing.T) {
	dir := t.TempDir()
	writeManifest(t, dir, graphManifest{CommitSHA: ""})
	withMockGitSHA(t, func() (string, error) { return "head", nil })
	if got := checkCommitMismatch(filepath.Join(dir, "manifest.json")); got != "" {
		t.Errorf("empty commit should not warn: %q", got)
	}
}

func TestCheckCommitMismatch_GitError(t *testing.T) {
	dir := t.TempDir()
	writeManifest(t, dir, graphManifest{CommitSHA: "oldhash"})
	withMockGitSHA(t, func() (string, error) { return "", fmt.Errorf("git not found") })
	if got := checkCommitMismatch(filepath.Join(dir, "manifest.json")); got != "" {
		t.Errorf("git error should silently skip: %q", got)
	}
}

func TestCheckCommitMismatch_GitEmptyOutput(t *testing.T) {
	dir := t.TempDir()
	writeManifest(t, dir, graphManifest{CommitSHA: "oldhash"})
	withMockGitSHA(t, func() (string, error) { return "", nil })
	if got := checkCommitMismatch(filepath.Join(dir, "manifest.json")); got != "" {
		t.Errorf("empty git output should silently skip: %q", got)
	}
}

func TestCheckCommitMismatch_MissingManifest(t *testing.T) {
	withMockGitSHA(t, func() (string, error) { return "head", nil })
	if got := checkCommitMismatch(filepath.Join(t.TempDir(), "nonexistent.json")); got != "" {
		t.Errorf("missing manifest should not warn: %q", got)
	}
}

func TestCheckCommitMismatch_InvalidJSON(t *testing.T) {
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "manifest.json"), []byte("{not valid"), 0644)
	withMockGitSHA(t, func() (string, error) { return "head", nil })
	if got := checkCommitMismatch(filepath.Join(dir, "manifest.json")); got != "" {
		t.Errorf("invalid JSON should not warn: %q", got)
	}
}

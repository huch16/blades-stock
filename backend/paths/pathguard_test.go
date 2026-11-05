package paths

import (
	"crypto/sha256"
	"encoding/hex"
	"os"
	"path/filepath"
	"testing"
)

// writeFakeDB 在指定路径写一个内容可控的文件，返回内容与对应 SHA-256。
// 返回的 fingerprint 用于跨测试用例做"已知好值"的断言。
func writeFakeDB(t *testing.T, path, content string) (fingerprint string) {
	t.Helper()
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatalf("write %s: %v", path, err)
	}
	h := sha256.Sum256([]byte(content))
	return hex.EncodeToString(h[:])
}

func TestResolveAndFingerprint_BothMissing(t *testing.T) {
	dir := t.TempDir()
	a := filepath.Join(dir, "missing_a.db")
	b := filepath.Join(dir, "missing_b.db")

	res, err := ResolveAndFingerprint(a, b)
	if err != nil {
		t.Fatalf("expected nil error when both missing, got %v", err)
	}
	if res.RealExists || res.BuildBinExists {
		t.Error("both should be reported as not existing")
	}
	if res.EqualToBuildBin {
		t.Error("EqualToBuildBin must be false when files missing")
	}
	if res.RealFingerprint != "" || res.BuildBinFingerprint != "" {
		t.Error("fingerprints should be empty for missing files")
	}
}

func TestResolveAndFingerprint_RealOnly(t *testing.T) {
	dir := t.TempDir()
	real := filepath.Join(dir, "real.db")
	buildBin := filepath.Join(dir, "buildbin.db")
	want := writeFakeDB(t, real, "real-stock-content-v1")

	res, err := ResolveAndFingerprint(real, buildBin)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !res.RealExists {
		t.Error("RealExists should be true")
	}
	if res.BuildBinExists {
		t.Error("BuildBinExists should be false")
	}
	if res.RealFingerprint != want {
		t.Errorf("RealFingerprint = %q, want %q", res.RealFingerprint, want)
	}
	if res.EqualToBuildBin {
		t.Error("EqualToBuildBin must be false when buildBin missing")
	}
}

func TestResolveAndFingerprint_BuildBinOnly(t *testing.T) {
	dir := t.TempDir()
	real := filepath.Join(dir, "real.db")
	buildBin := filepath.Join(dir, "buildbin.db")
	wantBuildBin := writeFakeDB(t, buildBin, "deployment-snapshot-v1")

	res, err := ResolveAndFingerprint(real, buildBin)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if res.RealExists {
		t.Error("RealExists should be false")
	}
	if !res.BuildBinExists {
		t.Error("BuildBinExists should be true")
	}
	if res.BuildBinFingerprint != wantBuildBin {
		t.Errorf("BuildBinFingerprint = %q, want %q", res.BuildBinFingerprint, wantBuildBin)
	}
	if res.EqualToBuildBin {
		t.Error("EqualToBuildBin must be false when real missing")
	}
}

func TestResolveAndFingerprint_SameFingerprint(t *testing.T) {
	dir := t.TempDir()
	real := filepath.Join(dir, "real.db")
	buildBin := filepath.Join(dir, "buildbin.db")
	// 同样的内容 → 同样的 SHA-256 → 触发 "同源" 警告
	want := writeFakeDB(t, real, "identical-content")
	writeFakeDB(t, buildBin, "identical-content")

	res, err := ResolveAndFingerprint(real, buildBin)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !res.EqualToBuildBin {
		t.Error("EqualToBuildBin should be true for identical content")
	}
	if res.RealFingerprint != want {
		t.Errorf("RealFingerprint = %q, want %q", res.RealFingerprint, want)
	}
	if res.BuildBinFingerprint != want {
		t.Errorf("BuildBinFingerprint = %q, want %q", res.BuildBinFingerprint, want)
	}
}

func TestResolveAndFingerprint_DifferentContent(t *testing.T) {
	dir := t.TempDir()
	real := filepath.Join(dir, "real.db")
	buildBin := filepath.Join(dir, "buildbin.db")
	writeFakeDB(t, real, "real-v1")
	writeFakeDB(t, buildBin, "buildbin-v2-different")

	res, err := ResolveAndFingerprint(real, buildBin)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if res.EqualToBuildBin {
		t.Error("EqualToBuildBin should be false when content differs")
	}
	if res.RealFingerprint == res.BuildBinFingerprint {
		t.Error("fingerprints should differ")
	}
}

func TestResolveAndFingerprint_DirectoryInsteadOfFile(t *testing.T) {
	dir := t.TempDir()
	// 把 realDBPath 指向一个目录（PathIsDir 应报错）
	real := filepath.Join(dir, "real-dir")
	if err := os.Mkdir(real, 0755); err != nil {
		t.Fatal(err)
	}
	buildBin := filepath.Join(dir, "buildbin.db")
	writeFakeDB(t, buildBin, "buildbin-content")

	_, err := ResolveAndFingerprint(real, buildBin)
	if err == nil {
		t.Fatal("expected error when realDBPath is a directory, got nil")
	}
}

func TestResolveAndFingerprint_LargeFileStableHash(t *testing.T) {
	// 验证 fileSHA256 在大文件上分块读也能算出稳定 hash
	dir := t.TempDir()
	real := filepath.Join(dir, "big.db")
	// 构造一个大于 64KB buffer 的内容（多块读触发）
	content := make([]byte, 200*1024)
	for i := range content {
		content[i] = byte(i % 251)
	}
	if err := os.WriteFile(real, content, 0644); err != nil {
		t.Fatal(err)
	}
	h := sha256.Sum256(content)
	want := hex.EncodeToString(h[:])

	res, err := ResolveAndFingerprint(real, filepath.Join(dir, "missing.db"))
	if err != nil {
		t.Fatal(err)
	}
	if res.RealFingerprint != want {
		t.Errorf("large file fingerprint mismatch:\n  got  %s\n  want %s", res.RealFingerprint, want)
	}
}

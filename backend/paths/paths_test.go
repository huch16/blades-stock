package paths

import (
	"os"
	"path/filepath"
	"testing"
)

func TestBaseDirEnvOverride(t *testing.T) {
	t.Setenv("BLADES_STOCK_DATA_DIR", `G:\test\custom`)
	got := BaseDir()
	if got != `G:\test\custom` {
		t.Fatalf("env override: got %q, want %q", got, `G:\test\custom`)
	}
}

func TestBaseDirEnvEmptyFallsBack(t *testing.T) {
	t.Setenv("BLADES_STOCK_DATA_DIR", "")
	got := BaseDir()
	if got == "" {
		t.Fatal("BaseDir returned empty")
	}
	cwd, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	// 测试进程（go test）在无 data/ 的临时 exe 目录运行，应回退到 CWD。
	if filepath.Clean(got) != filepath.Clean(cwd) {
		t.Fatalf("fallback: got %q, want %q", got, cwd)
	}
}

func TestDataDirAndLogDir(t *testing.T) {
	base := BaseDir()
	if DataDir() != filepath.Join(base, "data") {
		t.Fatalf("DataDir() = %q, want %q", DataDir(), filepath.Join(base, "data"))
	}
	if LogDir() != filepath.Join(base, "logs") {
		t.Fatalf("LogDir() = %q, want %q", LogDir(), filepath.Join(base, "logs"))
	}
}

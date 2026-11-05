// db_smoke 模拟启动器集成检查：在真实仓库根 stock.db 上跑 db.Init + 路径指纹。
// 用法：go run ./cmd/db_smoke/
package main

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime/debug"

	"blades-stock/backend/db"
	log "blades-stock/backend/logger"
	"blades-stock/backend/paths"
)

func main() {
	defer func() {
		if r := recover(); r != nil {
			fmt.Fprintf(os.Stderr, "panic: %v\n%s\n", r, debug.Stack())
			os.Exit(2)
		}
	}()

	// 不要 chdir，让 paths.BaseDir() 走默认探测逻辑
	baseDir := paths.BaseDir()
	fmt.Printf("[smoke] baseDir = %q\n", baseDir)

	// 1. db.Init（含健康检查）
	db.Init("")
	fmt.Println("[smoke] db.Init OK（含 health check）")

	// 2. 路径指纹
	realDB := db.ResolveDBPath(baseDir)
	exe, _ := os.Executable()
	buildBinDB := filepath.Join(filepath.Dir(exe), "data", "stock.db")
	fp, err := paths.ResolveAndFingerprint(realDB, buildBinDB)
	if err != nil {
		fmt.Printf("[smoke] fingerprint error: %v\n", err)
	} else {
		fmt.Printf("[smoke] real=%s fp=%s build_bin=%s fp=%s equal=%v\n",
			realDB, fp.RealFingerprint, buildBinDB, fp.BuildBinFingerprint, fp.EqualToBuildBin)
	}

	log.SugaredLogger.Info("[smoke] completed without panic")
	fmt.Println("[smoke] PASS")
}

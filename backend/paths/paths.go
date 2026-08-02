// Package paths 统一解析应用运行时基准目录，消除对"当前工作目录"的依赖。
package paths

import (
	"os"
	"path/filepath"
)

// BaseDir 返回应用基准目录。所有相对路径（data/、logs/、memory/、stock_basic.json 等）
// 都以它为锚点派生，使开发模式与本地发布 exe 共用同一份数据，部署包则自包含。
//
// 解析优先级：
//  1. 环境变量 BLADES_STOCK_DATA_DIR（显式指定，最高优先）
//  2. 本地开发检出：exe 位于 <仓库>/build/bin/，且 <仓库> 下存在 go.mod 与 data/stock.db，
//     则返回 <仓库>（开发与本地发布共用同一份数据）
//  3. 自包含部署：exe 同目录下存在 data/（发布包形态），返回 exe 所在目录
//  4. 回退：进程当前工作目录（go run / wails dev 时通常即项目根）
func BaseDir() string {
	if env := os.Getenv("BLADES_STOCK_DATA_DIR"); env != "" {
		return env
	}
	if exe, err := os.Executable(); err == nil {
		exeDir := filepath.Dir(exe)
		// 本地开发检出：<仓库>/build/bin/<exe>
		if filepath.Base(exeDir) == "bin" {
			root := filepath.Dir(filepath.Dir(exeDir))
			if isFile(filepath.Join(root, "go.mod")) && isFile(filepath.Join(root, "data", "stock.db")) {
				return root
			}
		}
		// 自包含部署：data/ 紧邻 exe
		if isDir(filepath.Join(exeDir, "data")) {
			return exeDir
		}
	}
	if cwd, err := os.Getwd(); err == nil {
		return cwd
	}
	return "."
}

// DataDir 返回数据目录（含 stock.db、dict 等）。
func DataDir() string {
	return filepath.Join(BaseDir(), "data")
}

// LogDir 返回日志目录。
func LogDir() string {
	return filepath.Join(BaseDir(), "logs")
}

func isFile(p string) bool {
	fi, err := os.Stat(p)
	return err == nil && !fi.IsDir()
}

func isDir(p string) bool {
	fi, err := os.Stat(p)
	return err == nil && fi.IsDir()
}

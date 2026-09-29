// Package paths 的运行期路径防护模块。
//
// 设计目标：
//   - 把"仓库根 stock.db" 与 "build/bin/data/stock.db" 的指纹 / 同源关系做出来
//   - 当两者指纹相同 → 警告（说明仓库根库被部署包污染，或反之）
//   - 是 P1.1 路径硬规则的一部分：让"两个 DB 文件共存"这个现象可观测、可警告
//
// 不替代 BaseDir() 的解析逻辑，仅在 BaseDir 解析之后做指纹比较。
package paths

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"
)

// FingerprintResult 是 ResolveAndFingerprint 的返回值。
// 字段为值语义（不是指针）：调用方直接打印或比较，无需 nil 检查。
type FingerprintResult struct {
	// RealPath 是 realDBPath 经过 EvalSymlinks 后的真实绝对路径。
	// 若 realDBPath 文件不存在，仍返回 EvalSymlinks 的最佳努力结果。
	RealPath string

	// RealFingerprint 是 realDBPath 的 SHA-256 十六进制；文件不存在时为空。
	RealFingerprint string

	// BuildBinFingerprint 是 buildBinDBPath 的 SHA-256；不存在时为空。
	BuildBinFingerprint string

	// EqualToBuildBin = 两个文件都存在且 SHA-256 完全相同。
	// 这是"自包含部署包与仓库根库同源"的判定条件。
	EqualToBuildBin bool

	// RealExists / BuildBinExists 各自标记对应文件是否存在。
	RealExists      bool
	BuildBinExists bool
}

// ResolveAndFingerprint 计算两个 DB 文件的指纹并判定是否同源。
//
// 参数：
//   - realDBPath：仓库根 data/stock.db（来自 BaseDir()/data/stock.db）
//   - buildBinDBPath：构建产物路径（通常 <仓库>/build/bin/data/stock.db，可不存在）
//
// 行为：
//   - 任一路径不存在 → 仍计算另一侧的指纹，但 EqualToBuildBin 必为 false
//   - SHA-256 用一次性 io.Copy 全文件读入（DB 不会太大，< 200MB）
//
// 错误处理：
//   - 文件读取 / EvalSymlinks 失败 → 返回 error，调用方降级为只打日志
//   - 不修改任何文件，仅只读
func ResolveAndFingerprint(realDBPath, buildBinDBPath string) (FingerprintResult, error) {
	var res FingerprintResult

	// 1. 解析真实路径（处理 Windows 8.3 短名 / 符号链接 / 大小写）
	if resolved, err := filepath.EvalSymlinks(realDBPath); err == nil {
		res.RealPath = resolved
	} else {
		// 文件不存在时 EvalSymlinks 会失败，保留原始路径供日志使用
		res.RealPath = realDBPath
	}

	// 2. 仓库根库指纹
	if fp, ok, err := fileSHA256(realDBPath); err != nil {
		return res, fmt.Errorf("real DB 指纹计算失败: %w", err)
	} else if ok {
		res.RealFingerprint = fp
		res.RealExists = true
	}

	// 3. build/bin 库指纹
	if fp, ok, err := fileSHA256(buildBinDBPath); err != nil {
		return res, fmt.Errorf("build/bin DB 指纹计算失败: %w", err)
	} else if ok {
		res.BuildBinFingerprint = fp
		res.BuildBinExists = true
	}

	// 4. 同源判定
	res.EqualToBuildBin = res.RealExists && res.BuildBinExists &&
		res.RealFingerprint != "" && res.RealFingerprint == res.BuildBinFingerprint

	return res, nil
}

// fileSHA256 计算文件 SHA-256 十六进制；不存在返回 (空, false, nil)。
func fileSHA256(path string) (string, bool, error) {
	info, err := os.Stat(path)
	if err != nil {
		if os.IsNotExist(err) {
			return "", false, nil
		}
		return "", false, err
	}
	if info.IsDir() {
		return "", false, fmt.Errorf("path is a directory: %s", path)
	}
	f, err := os.Open(path)
	if err != nil {
		return "", false, err
	}
	defer f.Close()

	h := sha256.New()
	buf := make([]byte, 64*1024)
	for {
		n, rerr := f.Read(buf)
		if n > 0 {
			h.Write(buf[:n])
		}
		if rerr != nil {
			break
		}
	}
	return hex.EncodeToString(h.Sum(nil)), true, nil
}
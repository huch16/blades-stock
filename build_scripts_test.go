package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// 本文件是「Windows 构建脚本」的防回归门禁。
//
// 存在的理由：这两条缺陷都属于**用户一执行就硬失败、但 CI/开发机上看不出来**
// 的类型（开发机 PATH 里有 git、编辑器默认按 UTF-8 打开，所以症状被完全掩盖）。
// 实测用户报「powershell 发生错误」时，这两条叠在一起，Windows 构建路径
// （go_build.bat / wails_build.bat / build.ps1）**一个都跑不起来**。
//
// 把它们写成 Go 测试，是为了搭在已有的 `go test ./...` 门禁上自动执行，
// 而不是依赖某个人记得手动跑一遍 PowerShell。

// psScriptHasBOM 报告文件是否以 UTF-8 BOM(EF BB BF) 开头。
func psScriptHasBOM(b []byte) bool {
	return len(b) >= 3 && b[0] == 0xEF && b[1] == 0xBB && b[2] == 0xBF
}

// psScriptHasNonASCII 报告文件是否含非 ASCII 字节（含中文注释/帮助文本）。
func psScriptHasNonASCII(b []byte) bool {
	for _, c := range b {
		if c >= 0x80 {
			return true
		}
	}
	return false
}

// TestPowerShellScriptsHaveBOM 防回归：含非 ASCII 的 .ps1 必须带 UTF-8 BOM。
//
// 根因（已复现并验证）：Windows PowerShell 5.1 对**无 BOM** 的 .ps1 使用系统
// ANSI 编码读取（中文系统为 GB2312），而非 UTF-8。于是脚本里的中文被错误解码，
// 解出的字节会吃掉大括号/引号，解析器随即报
//
//	ParserError: Try 语句缺少其 Catch 或 Finally 块 (MissingCatchOrFinally)
//
// 实测：build.ps1（无 BOM，含 2 行中文注释）解析出 2 个错误；
// 同一份字节按 UTF-8 解析则是 0 个错误 —— 证明纯粹是编码问题，
// 与脚本逻辑无关。加上 BOM 后 PS 5.1 解析 0 错误。
//
// 注意：.bat 里的 `chcp 65001` 无法修复此问题 —— 它只改**控制台输出**代码页，
// 不改变 PowerShell 读取**脚本文件**时所用的编码。
func TestPowerShellScriptsHaveBOM(t *testing.T) {
	scripts, err := filepath.Glob("*.ps1")
	if err != nil {
		t.Fatalf("glob *.ps1: %v", err)
	}
	// 顺带覆盖其他目录下的 .ps1（fork 出来的构建脚本）
	if more, err := filepath.Glob("frontend/*.ps1"); err == nil {
		scripts = append(scripts, more...)
	}
	if len(scripts) == 0 {
		t.Skip("未找到 .ps1 脚本")
	}

	for _, path := range scripts {
		b, err := os.ReadFile(path)
		if err != nil {
			t.Fatalf("读取 %s 失败: %v", path, err)
		}
		if !psScriptHasNonASCII(b) {
			continue // 纯 ASCII 的脚本与编码无关，允许无 BOM
		}
		if !psScriptHasBOM(b) {
			t.Errorf("%s 含非 ASCII 字符但没有 UTF-8 BOM。\n"+
				"  PowerShell 5.1 会把无 BOM 的脚本按系统 ANSI(GB2312) 解码，"+
				"中文会被解错并吃掉大括号/引号，导致 ParserError: MissingCatchOrFinally，脚本完全无法运行。\n"+
				"  修复：另存为「UTF-8 with BOM」。(.bat 里的 chcp 65001 无效)", path)
		}
	}
}

// TestBuildScriptDoesNotClobberPath 防回归：build.ps1 不得整体覆盖 $env:PATH。
//
// 根因（已复现）：原实现为
//
//	$env:PATH = "C:\Users\<某人>\go\go\bin;...;C:\Program Files\nodejs"
//
// 这是一次**赋值**而非追加，后果有两条，且第二条是硬故障：
//  1. 用户 PATH 里的 git / python 等工具全部丢失；
//  2. 配合脚本顶部的 `$ErrorActionPreference = "Stop"`，第一次
//     `& git rev-parse --short HEAD` 会抛 CommandNotFoundException ——
//     这是**终止性**异常，`2>$null` 压不住（它只能压原生命令的 stderr），
//     于是构建在取版本号那一步直接中止，一个产物都不生成。
//
// 该测试直接检查赋值形式，避免这段逻辑被无意改回去。
func TestBuildScriptDoesNotClobberPath(t *testing.T) {
	const path = "build.ps1"
	b, err := os.ReadFile(path)
	if err != nil {
		t.Skipf("未找到 %s: %v", path, err)
	}
	content := string(b)

	for i, line := range strings.Split(content, "\n") {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "#") {
			continue // 注释里讨论该问题是允许的
		}
		// 只看对 $env:PATH 的赋值；要求右侧必须引用原有 PATH（即追加语义）
		if !strings.Contains(trimmed, "$env:PATH") || !strings.Contains(trimmed, "=") {
			continue
		}
		if !strings.Contains(trimmed, "=") {
			continue
		}
		// 形如 $env:PATH = "..." —— 等号左边的赋值
		eq := strings.Index(trimmed, "=")
		lhs := strings.TrimSpace(trimmed[:eq])
		if lhs != "$env:PATH" {
			continue
		}
		rhs := trimmed[eq+1:]
		if strings.Contains(rhs, "$env:PATH") {
			continue // 追加语义，正确
		}
		t.Errorf("%s:%d 整体覆盖了 $env:PATH：\n  %s\n"+
			"  这会丢掉用户 PATH 里的 git 等工具；配合 $ErrorActionPreference=\"Stop\"，"+
			"首次 git 调用即中止整个构建。请改为只追加缺失目录。", path, i+1, trimmed)
	}
}

// TestBuildScriptResolvesCommitWithoutAborting 防回归：取版本号必须容错。
// git 不可用时只应回退为 "dev"，而不是让整个构建失败。
func TestBuildScriptResolvesCommitWithoutAborting(t *testing.T) {
	b, err := os.ReadFile("build.ps1")
	if err != nil {
		t.Skipf("未找到 build.ps1: %v", err)
	}
	content := string(b)

	if !strings.Contains(content, "function Get-ShortCommit") {
		t.Error("build.ps1 应通过 Get-ShortCommit 统一取版本号（内部 try/catch 容错），" +
			"避免裸调用 git 在 git 不可用时终止构建")
	}
	// 裸调用（无 try 包裹的函数内部）应当已经不存在
	for i, line := range strings.Split(content, "\n") {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "#") {
			continue
		}
		if strings.Contains(trimmed, "git rev-parse") && !strings.Contains(trimmed, "function") {
			// 允许出现在 Get-ShortCommit 内部（有 try/catch）
			if !strings.Contains(content, "try {") {
				t.Errorf("build.ps1:%d 裸调用 git 且无容错：%s", i+1, trimmed)
			}
		}
	}
}

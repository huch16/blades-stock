package tools

import (
	"blades-stock/backend/agent/types"
	"blades-stock/backend/logger"
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	bladesTools "github.com/go-kratos/blades/tools"
)

var allowedCommands = map[string]bool{
	"go":     true,
	"npm":    true,
	"npx":    true,
	"make":   true,
	"node":   true,
	"pwsh":   true,
	"python": true,
}

const maxWriteFileSize = 1 << 20

var AllowedProjectRoot = "."

const maxFileSize = 1 << 20

func GetReadProjectFileTool() bladesTools.Tool {
	return types.NewTool("ReadProjectFile", "读取项目中的文件内容。适用于查看源代码、配置文件、文档等。支持 .go, .ts, .vue, .json, .yaml, .md 等常见代码文件。",
		map[string]*types.ParamDef{
			"path": {Type: "string", Desc: "文件路径，相对于项目根目录", Required: true},
		},
		func(args string) (string, error) {
			path := types.ExtractStringParam(args, "path", "filePath", "filepath")
			if path == "" {
				return "缺少 path 参数", nil
			}

			fullPath, err := resolvePath(path)
			if err != nil {
				return "路径不合法: " + err.Error(), nil
			}
			info, err := os.Stat(fullPath)
			if err != nil {
				return "文件不存在: " + path, nil
			}
			if info.Size() > maxFileSize {
				return "文件过大（超过1MB），无法读取", nil
			}
			if info.IsDir() {
				return "指定路径是一个目录，请使用 ListProjectDir", nil
			}

			data, err := os.ReadFile(fullPath)
			if err != nil {
				return "读取文件失败: " + err.Error(), nil
			}
			return "```\n" + string(data) + "\n```", nil
		},
	)
}

func GetListProjectDirTool() bladesTools.Tool {
	return types.NewTool("ListProjectDir", "列出项目目录结构。用于了解项目的文件组织和模块划分。默认深度为3层。",
		map[string]*types.ParamDef{
			"path":  {Type: "string", Desc: "目录路径，相对于项目根目录。留空或 '.' 表示根目录", Required: false},
			"depth": {Type: "integer", Desc: "递归深度，默认3，最大10", Required: false},
		},
		func(args string) (string, error) {
			path := types.ExtractStringParam(args, "path", "filePath", "filepath")
			if path == "" {
				path = "."
			}
			depth := 3
			if d, ok := types.ExtractIntParam(args, "depth"); ok {
				depth = d
				if depth > 10 {
					depth = 10
				}
				if depth < 1 {
					depth = 1
				}
			}

			fullPath, err := resolvePath(path)
			if err != nil {
				return "路径不合法: " + err.Error(), nil
			}
			info, err := os.Stat(fullPath)
			if err != nil {
				return "目录不存在: " + path, nil
			}
			if !info.IsDir() {
				return "指定路径是一个文件，请使用 ReadProjectFile", nil
			}

			var result strings.Builder
			result.WriteString("项目目录结构: " + path + "\n")
			walkDir(fullPath, "", depth, &result)
			return result.String(), nil
		},
	)
}

func GetSearchProjectFilesTool() bladesTools.Tool {
	return types.NewTool("SearchProjectFiles", "在项目中按 glob 模式匹配查找文件。例如 '**/*.go', '**/*_test.go', '**/main.go', '**/vue.config.*'",
		map[string]*types.ParamDef{
			"pattern": {Type: "string", Desc: "glob 搜索模式，如 '**/*.go' 查找所有Go文件, '**/*_test.go' 查找测试文件", Required: true},
			"root":    {Type: "string", Desc: "搜索根目录，相对于项目根目录。留空表示整个项目", Required: false},
		},
		func(args string) (string, error) {
			pattern := types.ExtractStringParam(args, "pattern", "glob", "query")
			if pattern == "" {
				return "缺少 pattern 参数", nil
			}
			root := types.ExtractStringParam(args, "root", "path", "dir")
			searchRoot := "."
			if root != "" {
				searchRoot = root
			}

			fullRoot, err := resolvePath(searchRoot)
			if err != nil {
				return "搜索根目录不合法: " + err.Error(), nil
			}
			matches, err := filepath.Glob(filepath.Join(fullRoot, pattern))
			if err != nil {
				return "搜索模式无效: " + err.Error(), nil
			}
			// Glob 模式里的 ".." 仍可能把匹配结果带出根目录，逐个过滤。
			kept := matches[:0]
			for _, m := range matches {
				if rel, rerr := filepath.Rel(fullRoot, m); rerr == nil && rel != ".." && !strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
					kept = append(kept, m)
				}
			}
			matches = kept

			var result strings.Builder
			result.WriteString(fmt.Sprintf("找到 %d 个匹配文件:\n", len(matches)))
			for _, m := range matches {
				rel, _ := filepath.Rel(projectRoot(), m)
				result.WriteString("  - " + rel + "\n")
				if result.Len() > 4000 {
					result.WriteString("  ... 结果过多，已截断\n")
					break
				}
			}
			return result.String(), nil
		},
	)
}

func GetGitOperationTool() bladesTools.Tool {
	return types.NewTool("GitOperation", "在项目上执行 git 操作。支持 clone/log/diff/show/branch/status 等常用子命令。注意：clone 时 url 必须提供，其他操作在已有仓库上执行。",
		map[string]*types.ParamDef{
			"op":   {Type: "string", Desc: "git 子命令", Required: true, Enum: []string{"clone", "log", "diff", "show", "branch", "status", "ls-tree", "grep"}},
			"args": {Type: "string", Desc: "传递给 git 的参数。例如 clone 时填写 'https://github.com/user/repo.git'；log 时填写 '--oneline -10'；diff 时留空或 'HEAD~1'", Required: false},
			"path": {Type: "string", Desc: "目标路径。clone 时指定存放目录，其他操作指定已有仓库路径。留空表示当前项目根目录", Required: false},
		},
		func(args string) (string, error) {
			op := types.ExtractStringParam(args, "op", "command")
			gitArgs := types.ExtractStringParam(args, "args", "arguments")
			path := types.ExtractStringParam(args, "path", "filePath", "filepath")

			workDir := projectRoot()
			if path != "" {
				var err error
				workDir, err = resolvePath(path)
				if err != nil {
					return "路径不合法: " + err.Error(), nil
				}
			}

			if op == "" {
				return "缺少 op 参数", nil
			}

			cmdArgs := []string{op}
			if gitArgs != "" {
				cmdArgs = append(cmdArgs, strings.Fields(gitArgs)...)
			}

			logger.SugaredLogger.Infof("GitOperation: %s %v in %s", op, cmdArgs, workDir)

			cmd := exec.Command("git", cmdArgs...)
			cmd.Dir = workDir
			output, err := cmd.CombinedOutput()
			if err != nil {
				return "git 操作失败: " + err.Error() + "\n输出: " + string(output), nil
			}
			result := string(output)
			if len(result) > 8000 {
				result = result[:8000] + "\n... (结果已截断)"
			}
			return result, nil
		},
	)
}

var learnProjectHandler func(source, name string) string

func RegisterLearnProjectHandler(h func(source, name string) string) {
	learnProjectHandler = h
}

func GetLearnProjectTool() bladesTools.Tool {
	return types.NewTool("LearnProject", "学习一个远程或本地项目，将其转化为你的专业技能。工作原理：1) 如果是远程 git URL会自动 clone 到本地 2) 分析项目结构、代码、文档 3) 生成 Skill 记录。以后同类问题你可以直接调用已有知识回答。",
		map[string]*types.ParamDef{
			"source": {Type: "string", Desc: "项目来源。可以是 git clone URL（如 'https://github.com/user/repo.git'）或本地路径", Required: true},
			"name":   {Type: "string", Desc: "项目名称。留空则从 URL/路径自动生成", Required: false},
		},
		func(args string) (string, error) {
			source := types.ExtractStringParam(args, "source", "url", "path")
			name := types.ExtractStringParam(args, "name")
			if source == "" {
				return "缺少 source 参数（git URL 或本地路径）", nil
			}
			if learnProjectHandler == nil {
				return "学习功能尚未初始化，请重启应用", nil
			}
			return learnProjectHandler(source, name), nil
		},
	)
}

func GetWriteProjectFileTool() bladesTools.Tool {
	return types.NewTool("WriteProjectFile", "写入内容到项目文件。适用于创建新文件或修改已有文件。会自动创建父目录。注意：不要用于大于1MB的大文件写入。",
		map[string]*types.ParamDef{
			"path":    {Type: "string", Desc: "文件路径，相对于项目根目录", Required: true},
			"content": {Type: "string", Desc: "文件内容", Required: true},
		},
		func(args string) (string, error) {
			path := types.ExtractStringParam(args, "path", "filePath", "filepath")
			content := types.ExtractStringParam(args, "content", "text")
			if path == "" {
				return "缺少 path 参数", nil
			}
			if len(content) > maxWriteFileSize {
				return "内容过大（超过1MB），无法写入", nil
			}

			fullPath, err := resolvePath(path)
			if err != nil {
				return "路径不合法: " + err.Error(), nil
			}
			dir := filepath.Dir(fullPath)
			if err := os.MkdirAll(dir, 0755); err != nil {
				return "创建目录失败: " + err.Error(), nil
			}
			if err := os.WriteFile(fullPath, []byte(content), 0644); err != nil {
				return "写入文件失败: " + err.Error(), nil
			}
			return "✅ 已写入文件: " + path, nil
		},
	)
}

func GetRunCommandTool() bladesTools.Tool {
	return types.NewTool("RunCommand", "在项目根目录执行shell命令（白名单：go/npm/npx/node/make/pwsh/python）。适用于编译、测试、格式化等操作。注意：此工具严禁用于安装软件包或执行不受信任的命令。",
		map[string]*types.ParamDef{
			"command": {Type: "string", Desc: "要执行的命令（白名单：go/npm/npx/node/make/pwsh/python），如 'go' 或 'npm'", Required: true, Enum: []string{"go", "npm", "npx", "node", "make", "pwsh", "python"}},
			"args":    {Type: "string", Desc: "命令参数，如 'build ./...' 或 'run build' 或 'test ./...'", Required: true},
			"timeout": {Type: "integer", Desc: "超时秒数，默认60。编译等耗时任务可设为120-300", Required: false},
		},
		func(args string) (string, error) {
			command := types.ExtractStringParam(args, "command", "cmd")
			cmdArgs := types.ExtractStringParam(args, "args", "arguments")
			if command == "" {
				return "缺少 command 参数", nil
			}
			if !allowedCommands[command] {
				return fmt.Sprintf("命令 %s 不在白名单中，允许的命令：go/npm/npx/node/make/pwsh/python", command), nil
			}

			timeoutSec := 60
			if t, ok := types.ExtractIntParam(args, "timeout", "timeoutSec"); ok && t > 0 {
				timeoutSec = t
			}

			workDir := projectRoot()
			ctx, cancel := context.WithTimeout(context.Background(), time.Duration(timeoutSec)*time.Second)
			defer cancel()

			cmd := exec.CommandContext(ctx, command, strings.Fields(cmdArgs)...)
			cmd.Dir = workDir
			output, err := cmd.CombinedOutput()

			if ctx.Err() == context.DeadlineExceeded {
				return fmt.Sprintf("⏱️ 命令执行超时（%d秒），请考虑增加 timeout 参数\n最后输出: %s", timeoutSec, truncateOutput(string(output))), nil
			}
			result := truncateOutput(string(output))
			if err != nil {
				return fmt.Sprintf("命令执行失败: %v\n输出:\n%s", err, result), nil
			}
			return fmt.Sprintf("✅ 命令执行成功\n输出:\n%s", result), nil
		},
	)
}

func truncateOutput(s string) string {
	if len(s) > 8000 {
		return s[:8000] + "\n... (输出已截断)"
	}
	return s
}

func GetProjectTools() []bladesTools.Tool {
	return []bladesTools.Tool{
		GetReadProjectFileTool(),
		GetListProjectDirTool(),
		GetSearchProjectFilesTool(),
		GetGitOperationTool(),
		GetLearnProjectTool(),
		GetWriteProjectFileTool(),
		GetRunCommandTool(),
	}
}

// projectRoot 返回项目根目录的绝对路径。
func projectRoot() string {
	absRoot, err := filepath.Abs(AllowedProjectRoot)
	if err != nil {
		return AllowedProjectRoot
	}
	return absRoot
}

// resolvePath 把调用方给出的路径解析为项目根目录内的绝对路径。
// 绝对路径与含 ".." 的相对路径一律做越界校验，逃出项目根目录时返回错误，
// 防止经 ReadProjectFile/WriteProjectFile 等工具读写根目录外的任意文件。
func resolvePath(path string) (string, error) {
	absRoot := projectRoot()
	var abs string
	if filepath.IsAbs(path) {
		abs = filepath.Clean(path)
	} else {
		abs = filepath.Join(absRoot, path)
	}
	if abs != absRoot && !strings.HasPrefix(abs, absRoot+string(filepath.Separator)) {
		return "", fmt.Errorf("路径 %q 超出项目根目录", path)
	}
	return abs, nil
}

func walkDir(path, prefix string, depth int, result *strings.Builder) {
	if depth <= 0 {
		result.WriteString(prefix + "  ...\n")
		return
	}
	entries, err := os.ReadDir(path)
	if err != nil {
		return
	}
	for i, entry := range entries {
		if result.Len() > 6000 {
			result.WriteString(prefix + "  ... (目录过大，已截断)\n")
			break
		}

		name := entry.Name()
		if strings.HasPrefix(name, ".") {
			continue
		}
		if name == "node_modules" || name == "vendor" || name == ".git" {
			continue
		}

		isLast := i == len(entries)-1
		var mark string
		if isLast {
			mark = "└── "
		} else {
			mark = "├── "
		}

		if entry.IsDir() {
			result.WriteString(prefix + mark + name + "/\n")
			subPrefix := prefix
			if isLast {
				subPrefix += "    "
			} else {
				subPrefix += "│   "
			}
			walkDir(filepath.Join(path, name), subPrefix, depth-1, result)
		} else {
			info, _ := entry.Info()
			size := ""
			if info != nil {
				size = " (" + formatSize(info.Size()) + ")"
			}
			result.WriteString(prefix + mark + name + size + "\n")
		}
	}
}

func formatSize(bytes int64) string {
	switch {
	case bytes < 1024:
		return fmt.Sprintf("%dB", bytes)
	case bytes < 1024*1024:
		return fmt.Sprintf("%dKB", bytes/1024)
	default:
		return fmt.Sprintf("%dMB", bytes/(1024*1024))
	}
}

func init() {
	abs, err := filepath.Abs(".")
	if err == nil {
		AllowedProjectRoot = abs
	}
}

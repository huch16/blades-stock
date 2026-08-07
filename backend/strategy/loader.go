package strategy

import (
	"embed"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
)

//go:embed strategies/*.yaml
var embeddedStrategies embed.FS

// LoadEmbedded 加载编译进二进制的默认策略（backend/strategy/strategies/*.yaml）。
func LoadEmbedded() ([]Strategy, error) {
	files, err := fs.Glob(embeddedStrategies, "strategies/*.yaml")
	if err != nil {
		return nil, fmt.Errorf("枚举内嵌策略失败: %w", err)
	}
	var result []Strategy
	for _, f := range files {
		data, err := embeddedStrategies.ReadFile(f)
		if err != nil {
			return nil, fmt.Errorf("读取内嵌策略 %s 失败: %w", f, err)
		}
		s, err := Parse(data)
		if err != nil {
			return nil, fmt.Errorf("%s: %w", f, err)
		}
		result = append(result, *s)
	}
	return result, nil
}

// LoadFromDir 从目录加载 *.yaml 策略文件，目录不存在时返回空列表（不视为错误）。
func LoadFromDir(dir string) ([]Strategy, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}
	var result []Strategy
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(strings.ToLower(e.Name()), ".yaml") {
			continue
		}
		data, err := os.ReadFile(filepath.Join(dir, e.Name()))
		if err != nil {
			return nil, fmt.Errorf("读取策略文件 %s 失败: %w", e.Name(), err)
		}
		s, err := Parse(data)
		if err != nil {
			return nil, fmt.Errorf("%s: %w", e.Name(), err)
		}
		result = append(result, *s)
	}
	return result, nil
}

// LoadWithOverride 先加载内嵌默认策略，再用 overrideDir 下同名策略覆盖。
// overrideDir 不存在时静默使用内嵌策略。
func LoadWithOverride(overrideDir string) ([]Strategy, error) {
	list, err := LoadEmbedded()
	if err != nil {
		return nil, err
	}
	overrides, err := LoadFromDir(overrideDir)
	if err != nil {
		return nil, err
	}
	for _, o := range overrides {
		replaced := false
		for i := range list {
			if list[i].Name == o.Name {
				list[i] = o
				replaced = true
				break
			}
		}
		if !replaced {
			list = append(list, o)
		}
	}
	return list, nil
}

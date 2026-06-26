package agent

import (
	"encoding/json"
	"fmt"
	"go-stock/backend/db"
	"go-stock/backend/logger"
	"go-stock/backend/models"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

type ProjectAnalyzer struct {
	embeddingService *EmbeddingService
}

func NewProjectAnalyzer(es *EmbeddingService) *ProjectAnalyzer {
	return &ProjectAnalyzer{embeddingService: es}
}

type AnalyzeResult struct {
	Name         string
	Language     string
	Summary      string
	Structure    string
	Readme       string
	EntryPoints  []string
	APISummary   string
	ModelSummary string
	TechStack    string
	Chunks       []CodeChunkItem
}

type CodeChunkItem struct {
	FilePath  string
	Content   string
	StartLine int
	EndLine   int
	ChunkType string
}

func (pa *ProjectAnalyzer) AnalyzeProject(projectID uint, localPath string) error {
	logger.SugaredLogger.Infof("开始分析项目: %s", localPath)

	info, err := os.Stat(localPath)
	if err != nil {
		return fmt.Errorf("项目路径不存在: %w", err)
	}
	if !info.IsDir() {
		return fmt.Errorf("项目路径不是目录: %s", localPath)
	}

	result := pa.analyze(localPath)

	entryJSON, _ := json.Marshal(result.EntryPoints)
	analysis := &models.ProjectAnalysis{}
	if err := db.Dao.First(analysis, projectID).Error; err != nil {
		return fmt.Errorf("项目记录不存在: %w", err)
	}

	analysis.Name = result.Name
	analysis.Language = result.Language
	analysis.Summary = result.Summary
	analysis.Structure = result.Structure
	analysis.Readme = result.Readme
	analysis.EntryPoints = string(entryJSON)
	analysis.APISummary = result.APISummary
	analysis.ModelSummary = result.ModelSummary
	analysis.TechStack = result.TechStack
	analysis.ChunkCount = len(result.Chunks)
	analysis.Status = "indexed"

	if err := db.Dao.Save(analysis).Error; err != nil {
		return fmt.Errorf("保存项目分析失败: %w", err)
	}

	for _, chunk := range result.Chunks {
		codeChunk := &models.CodeChunk{
			ProjectAnalysisID: projectID,
			FilePath:          chunk.FilePath,
			Content:           chunk.Content,
			StartLine:         chunk.StartLine,
			EndLine:           chunk.EndLine,
			ChunkType:         chunk.ChunkType,
		}
		if pa.embeddingService != nil {
			vec, err := pa.embeddingService.Embed(chunk.Content)
			if err == nil && len(vec) > 0 {
				codeChunk.Embedding, _ = json.Marshal(vec)
			}
		}
		if err := db.Dao.Create(codeChunk).Error; err != nil {
			logger.SugaredLogger.Warnf("保存代码块失败 [%s:%d-%d]: %v", chunk.FilePath, chunk.StartLine, chunk.EndLine, err)
		}
	}

	logger.SugaredLogger.Infof("项目分析完成: %s, 语言=%s, 代码块=%d", result.Name, result.Language, len(result.Chunks))
	return nil
}

func (pa *ProjectAnalyzer) analyze(localPath string) *AnalyzeResult {
	result := &AnalyzeResult{}

	baseName := filepath.Base(localPath)
	result.Name = baseName

	readmePath := filepath.Join(localPath, "README.md")
	if data, err := os.ReadFile(readmePath); err == nil {
		result.Readme = string(data)
	} else {
		readmePath = filepath.Join(localPath, "README")
		if data, err := os.ReadFile(readmePath); err == nil {
			result.Readme = string(data)
		}
	}

	fileCounts := make(map[string]int)
	var goModFound bool
	var pkgJSONFound bool

	filepath.Walk(localPath, func(path string, info os.FileInfo, err error) error {
		if err != nil || info.IsDir() {
			return nil
		}
		rel, _ := filepath.Rel(localPath, path)
		if strings.HasPrefix(rel, ".") || strings.Contains(rel, "node_modules") || strings.Contains(rel, "vendor") || strings.Contains(rel, ".git") {
			if info.IsDir() {
				return filepath.SkipDir
			}
			return nil
		}
		ext := filepath.Ext(rel)
		fileCounts[ext]++

		base := filepath.Base(rel)
		if base == "go.mod" {
			goModFound = true
		}
		if base == "package.json" {
			pkgJSONFound = true
		}
		return nil
	})

	if goModFound {
		result.Language = "Go"
		goModData, _ := os.ReadFile(filepath.Join(localPath, "go.mod"))
		if goModData != nil {
			firstLine := strings.SplitN(string(goModData), "\n", 2)[0]
			result.TechStack = "Go module: " + strings.TrimSpace(firstLine)
		}
	} else if pkgJSONFound {
		pkgData, _ := os.ReadFile(filepath.Join(localPath, "package.json"))
		if pkgData != nil {
			var pkg struct {
				Name         string            `json:"name"`
				Dependencies map[string]string `json:"dependencies"`
			}
			if json.Unmarshal(pkgData, &pkg) == nil {
				result.TechStack = "Node.js"
				if pkg.Dependencies != nil {
					techs := []string{}
					for dep := range pkg.Dependencies {
						techs = append(techs, dep)
						if len(techs) >= 8 {
							break
						}
					}
					if len(techs) > 0 {
						result.TechStack += ": " + strings.Join(techs, ", ")
					}
				}
			}
		}
	}

	totalFiles := 0
	for _, count := range fileCounts {
		totalFiles += count
	}
	var extList []string
	for ext, count := range fileCounts {
		extList = append(extList, fmt.Sprintf("%s: %d", ext, count))
	}
	sort.Slice(extList, func(i, j int) bool {
		return fileCounts[strings.Split(extList[i], ":")[0]] > fileCounts[strings.Split(extList[j], ":")[0]]
	})
	summary := fmt.Sprintf("项目 %s | 主语言: %s | 总文件数: %d\n文件类型分布: %s",
		result.Name, result.Language, totalFiles, strings.Join(extList, ", "))
	result.Summary = summary

	var structure strings.Builder
	structure.WriteString(baseName + "/\n")
	pa.walkDirForStructure(localPath, localPath, "", 3, &structure)
	result.Structure = structure.String()

	result.EntryPoints = pa.findEntryPoints(localPath)

	apiPaths := pa.findAPIDefinitions(localPath)
	if len(apiPaths) > 0 {
		result.APISummary = "API 路由:\n" + strings.Join(apiPaths, "\n")
	}

	modelPaths := pa.findModelDefinitions(localPath)
	if len(modelPaths) > 0 {
		modelSummaries := []string{}
		for _, mp := range modelPaths {
			modelSummaries = append(modelSummaries, "  - "+mp)
		}
		result.ModelSummary = "数据模型/结构体:\n" + strings.Join(modelSummaries, "\n")
	}

	result.Chunks = pa.chunkCodeFiles(localPath)

	return result
}

func (pa *ProjectAnalyzer) walkDirForStructure(root, path, prefix string, depth int, result *strings.Builder) {
	if depth <= 0 {
		return
	}
	entries, err := os.ReadDir(path)
	if err != nil {
		return
	}
	for i, entry := range entries {
		name := entry.Name()
		if strings.HasPrefix(name, ".") || name == "node_modules" || name == "vendor" || name == ".git" {
			continue
		}
		isLast := i == len(entries)-1
		mark := "├── "
		if isLast {
			mark = "└── "
		}
		if entry.IsDir() {
			result.WriteString(prefix + mark + name + "/\n")
			subPrefix := prefix
			if isLast {
				subPrefix += "    "
			} else {
				subPrefix += "│   "
			}
			pa.walkDirForStructure(root, filepath.Join(path, name), subPrefix, depth-1, result)
		} else {
			info, _ := entry.Info()
			size := ""
			if info != nil {
				size = fmt.Sprintf(" (%dKB)", info.Size()/1024)
			}
			result.WriteString(fmt.Sprintf("%s%s%s%s\n", prefix, mark, name, size))
		}
	}
}

func (pa *ProjectAnalyzer) findEntryPoints(localPath string) []string {
	var entries []string

	commonEntries := []string{"main.go", "cmd/", "app.go", "index.js", "index.ts", "src/main.ts", "src/index.ts", "src/app.ts", "server.go", "api.go"}
	for _, entry := range commonEntries {
		fullPath := filepath.Join(localPath, entry)
		if info, err := os.Stat(fullPath); err == nil {
			rel, _ := filepath.Rel(localPath, fullPath)
			if info.IsDir() {
				entries = append(entries, rel+"/")
			} else {
				entries = append(entries, rel)
			}
		}
	}

	return entries
}

func (pa *ProjectAnalyzer) findAPIDefinitions(localPath string) []string {
	var apiPaths []string

	filepath.Walk(localPath, func(path string, info os.FileInfo, err error) error {
		if err != nil || info.IsDir() {
			return nil
		}
		rel, _ := filepath.Rel(localPath, path)
		if strings.HasPrefix(rel, ".") || strings.Contains(rel, "node_modules") || strings.Contains(rel, "vendor") || strings.Contains(rel, ".git") {
			if info.IsDir() {
				return filepath.SkipDir
			}
			return nil
		}
		ext := filepath.Ext(rel)
		if ext != ".go" && ext != ".ts" && ext != ".js" {
			return nil
		}

		data, err := os.ReadFile(path)
		if err != nil || len(data) > maxAnalyzeFileSize {
			return nil
		}
		content := string(data)

		lines := strings.Split(content, "\n")
		for _, line := range lines {
			trimmed := strings.TrimSpace(line)
			if strings.Contains(trimmed, "func(") && (strings.Contains(trimmed, "http.") || strings.Contains(trimmed, "gin") || strings.Contains(trimmed, "HandleFunc") || strings.Contains(trimmed, "POST(") || strings.Contains(trimmed, "GET(") || strings.Contains(trimmed, "PUT(") || strings.Contains(trimmed, "DELETE(")) {
				short := trimmed
				if len(short) > 100 {
					short = short[:100] + "..."
				}
				apiPaths = append(apiPaths, fmt.Sprintf("  [%s] %s", rel, short))
			}
		}
		return nil
	})

	return apiPaths
}

func (pa *ProjectAnalyzer) findModelDefinitions(localPath string) []string {
	var models []string

	filepath.Walk(localPath, func(path string, info os.FileInfo, err error) error {
		if err != nil || info.IsDir() {
			return nil
		}
		rel, _ := filepath.Rel(localPath, path)
		if strings.HasPrefix(rel, ".") || strings.Contains(rel, "node_modules") || strings.Contains(rel, "vendor") || strings.Contains(rel, ".git") {
			if info.IsDir() {
				return filepath.SkipDir
			}
			return nil
		}
		ext := filepath.Ext(rel)
		if ext == ".go" && (strings.Contains(rel, "model") || strings.Contains(rel, "type") || strings.Contains(rel, "schema") || strings.HasSuffix(filepath.Base(rel), "model.go") || strings.HasSuffix(filepath.Base(rel), "type.go")) {
			models = append(models, rel)
		}
		return nil
	})

	return models
}

func (pa *ProjectAnalyzer) chunkCodeFiles(localPath string) []CodeChunkItem {
	var chunks []CodeChunkItem

	filepath.Walk(localPath, func(path string, info os.FileInfo, err error) error {
		if err != nil || info.IsDir() {
			return nil
		}
		rel, _ := filepath.Rel(localPath, path)
		if strings.HasPrefix(rel, ".") || strings.Contains(rel, "node_modules") || strings.Contains(rel, "vendor") || strings.Contains(rel, ".git") {
			if info.IsDir() {
				return filepath.SkipDir
			}
			return nil
		}
		ext := filepath.Ext(rel)
		codeExts := map[string]bool{
			".go": true, ".ts": true, ".js": true, ".tsx": true, ".jsx": true,
			".vue": true, ".py": true, ".rs": true, ".java": true, ".rb": true,
			".php": true, ".c": true, ".h": true, ".cpp": true, ".yaml": true,
			".yml": true, ".json": true, ".xml": true, ".toml": true, ".mod": true,
			".sql": true, ".css": true, ".scss": true, ".html": true,
		}
		if !codeExts[ext] {
			return nil
		}

		data, err := os.ReadFile(path)
		if err != nil || len(data) > maxAnalyzeFileSize/2 {
			return nil
		}
		content := string(data)
		lines := strings.Split(content, "\n")

		if len(lines) <= 50 {
			chunkType := "code"
			if ext == ".json" || ext == ".yaml" || ext == ".yml" || ext == ".toml" || ext == ".xml" {
				chunkType = "config"
			}
			chunks = append(chunks, CodeChunkItem{
				FilePath:  rel,
				Content:   content,
				StartLine: 1,
				EndLine:   len(lines),
				ChunkType: chunkType,
			})
			return nil
		}

		const chunkSize = 40
		for i := 0; i < len(lines); i += chunkSize {
			end := i + chunkSize
			if end > len(lines) {
				end = len(lines)
			}
			chunkContent := strings.Join(lines[i:end], "\n")
			if strings.TrimSpace(chunkContent) == "" {
				continue
			}
			chunks = append(chunks, CodeChunkItem{
				FilePath:  rel,
				Content:   chunkContent,
				StartLine: i + 1,
				EndLine:   end,
				ChunkType: "code",
			})
		}
		return nil
	})

	return chunks
}

func (pa *ProjectAnalyzer) SearchCodeChunks(projectID uint, query string, limit int) ([]models.CodeChunk, error) {
	if limit <= 0 {
		limit = 10
	}

	if pa.embeddingService != nil && query != "" {
		queryVec, err := pa.embeddingService.Embed(query)
		if err == nil && len(queryVec) > 0 {
			var allChunks []models.CodeChunk
			if err := db.Dao.Where("project_analysis_id = ?", projectID).Find(&allChunks).Error; err != nil {
				return nil, err
			}

			type scored struct {
				chunk models.CodeChunk
				score float32
			}
			var scoredChunks []scored
			for _, chunk := range allChunks {
				if len(chunk.Embedding) == 0 {
					continue
				}
				var vec []float32
				if err := json.Unmarshal(chunk.Embedding, &vec); err != nil || len(vec) == 0 {
					continue
				}
				sim := CosineSimilarity(queryVec, vec)
				if sim > 0.3 {
					scoredChunks = append(scoredChunks, scored{chunk, sim})
				}
			}

			sort.Slice(scoredChunks, func(i, j int) bool {
				return scoredChunks[i].score > scoredChunks[j].score
			})

			if limit > len(scoredChunks) {
				limit = len(scoredChunks)
			}
			result := make([]models.CodeChunk, limit)
			for i := 0; i < limit; i++ {
				result[i] = scoredChunks[i].chunk
			}
			return result, nil
		}
	}

	var chunks []models.CodeChunk
	if err := db.Dao.Where("project_analysis_id = ?", projectID).
		Order("created_at ASC").
		Limit(limit).
		Find(&chunks).Error; err != nil {
		return nil, err
	}
	return chunks, nil
}

func (pa *ProjectAnalyzer) GetProjectSummary(projectID uint) (*models.ProjectAnalysis, error) {
	var analysis models.ProjectAnalysis
	if err := db.Dao.First(&analysis, projectID).Error; err != nil {
		return nil, err
	}
	return &analysis, nil
}

func (pa *ProjectAnalyzer) DeleteProjectAnalysis(projectID uint) error {
	if err := db.Dao.Where("project_analysis_id = ?", projectID).Delete(&models.CodeChunk{}).Error; err != nil {
		return err
	}
	return db.Dao.Delete(&models.ProjectAnalysis{}, projectID).Error
}

func (pa *ProjectAnalyzer) ListProjects() ([]models.ProjectAnalysis, error) {
	var projects []models.ProjectAnalysis
	if err := db.Dao.Order("updated_at DESC").Find(&projects).Error; err != nil {
		return nil, err
	}
	return projects, nil
}

var maxAnalyzeFileSize = 1 << 20

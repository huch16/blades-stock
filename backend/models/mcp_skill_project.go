package models

import "time"

type MCPServer struct {
	ID          uint      `json:"id" gorm:"primarykey"`
	CreatedAt   time.Time `json:"createdAt" wails:"-"`
	UpdatedAt   time.Time `json:"updatedAt" wails:"-"`
	Name        string    `json:"name" gorm:"size:255;not null"`
	Description string    `json:"description" gorm:"size:500"`
	URL         string    `json:"url" gorm:"size:500"`
	Type        string    `json:"type" gorm:"size:20;default:streamable-http"`
	Headers     string    `json:"headers" gorm:"type:text"`
	Command     string    `json:"command" gorm:"size:500"`
	Args        string    `json:"args" gorm:"type:text"`
	Env         string    `json:"env" gorm:"type:text"`
	Enable      bool      `json:"enable" gorm:"default:true"`
	Status      string    `json:"status" gorm:"size:20;default:stopped"`
	TestResult  string    `json:"testResult" gorm:"size:500"`
}

func (MCPServer) TableName() string {
	return "mcp_servers"
}

type MCPServerQuery struct {
	Page     int    `json:"page"`
	PageSize int    `json:"pageSize"`
	Name     string `json:"name"`
	Status   string `json:"status"`
	Enable   *bool  `json:"enable"`
}

type MCPServerPageResp struct {
	Total int         `json:"total"`
	Data  []MCPServer `json:"data"`
}

type MCPServerTool struct {
	ID           uint      `json:"id" gorm:"primarykey"`
	CreatedAt    time.Time `json:"createdAt" wails:"-"`
	UpdatedAt    time.Time `json:"updatedAt" wails:"-"`
	MCPServerID  uint      `json:"mcpServerId" gorm:"index;not null"`
	ToolName     string    `json:"toolName" gorm:"size:255;not null"`
	Description  string    `json:"description" gorm:"type:text"`
	ParamsSchema string    `json:"paramsSchema" gorm:"type:text"`
	Enabled      bool      `json:"enabled" gorm:"default:true"`
}

func (MCPServerTool) TableName() string {
	return "mcp_server_tools"
}

type Skill struct {
	ID           uint      `json:"id" gorm:"primarykey"`
	CreatedAt    time.Time `json:"createdAt" wails:"-"`
	UpdatedAt    time.Time `json:"updatedAt" wails:"-"`
	Name         string    `json:"name" gorm:"size:255;not null"`
	Description  string    `json:"description" gorm:"size:500"`
	Category     string    `json:"category" gorm:"size:50"`
	SystemPrompt string    `json:"systemPrompt" gorm:"type:text"`
	// Directive is a short (~800 char) summary of SystemPrompt, used for bge-m3 embedding.
	// Stored separately to avoid storing large duplicated content.
	Directive       string `json:"directive" gorm:"type:text"`
	Examples        string `json:"examples" gorm:"type:text"`
	TriggerKeywords string `json:"triggerKeywords" gorm:"size:500"`
	// Synonyms (P4.2) is a comma-separated list of alternative phrasings that
	// should also trigger this skill. Distinct from TriggerKeywords (which is
	// the canonical set): Synonyms covers cross-expressions like 深市↔深圳证券交易所,
	// ETF轮动↔ETF调仓. Empty value = backward-compatible (only TriggerKeywords match).
	Synonyms string `json:"synonyms" gorm:"size:500"`
	// ToolGroups is a comma-separated list of tool group names (see
	// tools.ToolGroup) that must be force-enabled whenever this skill is
	// matched, so a matched skill's directive can actually execute its tools
	// even when keyword classification would not include them.
	ToolGroups   string `json:"toolGroups" gorm:"size:500"`
	MCPServerIDs string `json:"mcpServerIds" gorm:"size:500"`
	Enable       bool   `json:"enable" gorm:"default:true"`
	SortOrder    int    `json:"sortOrder" gorm:"default:0"`
	// Sandbox mechanism: if true, skill is only active for a percentage of requests.
	Sandbox           bool `json:"sandbox" gorm:"default:false"`
	SandboxPercentage int  `json:"sandbox_percentage" gorm:"default:0"` // 0-100, 0 means disabled
	// Version is the semantic version of the skill (e.g., "1.0.0").
	Version string `json:"version" gorm:"size:64;default:'1.0.0'"`
	// Status indicates the lifecycle state: draft, active, archived.
	Status string `json:"status" gorm:"size:32;default:'draft'"`
	// InputSchema is the JSON Schema for skill input validation.
	InputSchema string `json:"inputSchema" gorm:"type:text"`
	// OutputSchema is the JSON Schema for skill output validation.
	OutputSchema string `json:"outputSchema" gorm:"type:text"`
}

func (Skill) TableName() string {
	return "skills"
}

type SkillQuery struct {
	Page     int    `json:"page"`
	PageSize int    `json:"pageSize"`
	Name     string `json:"name"`
	Category string `json:"category"`
	Enable   *bool  `json:"enable"`
}

type SkillPageResp struct {
	Total int64   `json:"total"`
	Data  []Skill `json:"data"`
}

type ProjectAnalysis struct {
	ID           uint      `json:"id" gorm:"primarykey"`
	CreatedAt    time.Time `json:"createdAt" wails:"-"`
	UpdatedAt    time.Time `json:"updatedAt" wails:"-"`
	SourceURL    string    `json:"sourceUrl" gorm:"size:500"`              // git clone URL or local path
	LocalPath    string    `json:"localPath" gorm:"size:500"`              // 本地存储路径
	Name         string    `json:"name" gorm:"size:255;not null"`          // 项目名
	Language     string    `json:"language" gorm:"size:100"`               // 主语言
	Summary      string    `json:"summary" gorm:"type:text"`               // 项目摘要
	Structure    string    `json:"structure" gorm:"type:text"`             // 目录结构
	Readme       string    `json:"readme" gorm:"type:text"`                // README 内容
	EntryPoints  string    `json:"entryPoints" gorm:"type:text"`           // 入口文件列表
	APISummary   string    `json:"apiSummary" gorm:"type:text"`            // API 路由摘要
	ModelSummary string    `json:"modelSummary" gorm:"type:text"`          // 数据模型摘要
	TechStack    string    `json:"techStack" gorm:"size:500"`              // 技术栈
	SkillID      uint      `json:"skillId" gorm:"default:0"`               // 生成的 Skill ID
	ChunkCount   int       `json:"chunkCount" gorm:"default:0"`            // 代码块数量
	Status       string    `json:"status" gorm:"size:20;default:indexing"` // indexing/indexed/failed
}

func (ProjectAnalysis) TableName() string {
	return "project_analysis"
}

type CodeChunk struct {
	ID                uint      `json:"id" gorm:"primarykey"`
	CreatedAt         time.Time `json:"createdAt" wails:"-"`
	ProjectAnalysisID uint      `json:"projectAnalysisId" gorm:"index"`
	FilePath          string    `json:"filePath" gorm:"size:500"`
	Content           string    `json:"content" gorm:"type:text"`
	Embedding         []byte    `json:"-" gorm:"type:blob"`
	StartLine         int       `json:"startLine"`
	EndLine           int       `json:"endLine"`
	ChunkType         string    `json:"chunkType" gorm:"size:50"` // code, comment, doc, config
	Summary           string    `json:"summary" gorm:"size:500"`
}

func (CodeChunk) TableName() string {
	return "code_chunks"
}

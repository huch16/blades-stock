// Package agent — Retriever 预留扩展缝。
//
// Retriever 接口与 Document 类型是为未来 RAG 检索器预留的扩展点，
// 当前仓库内没有任何实现：EmbeddingService 只有 EmbedText，没有
// Search/IndexDocuments/DeleteDocuments/Name/IsReady；唯一调用方
// agent.go:377 恒传 retr=nil，skill_embedding.go 的回退分支已于
// 第四轮删除。不要把这里当作已接通的向量检索——真要接 RAG 时，
// 再实现该接口并把 GetSkillEmbeddingService 的 retr 参数接回
// SkillEmbeddingService 结构体。
package agent

import (
	"context"
	"time"
)

// Document represents a document in the vector store.
type Document struct {
	Vector     []float32         `json:"-"`
	ID         string            `json:"id"`
	Source     string            `json:"source"` // skill, doc, news, memory
	Text       string            `json:"text"`
	Meta       map[string]string `json:"meta"`
	Score      float32           `json:"score,omitempty"`
	EmbeddedAt time.Time         `json:"embedded_at,omitempty"`
}

// Retriever defines the interface for vector-based retrieval.
type Retriever interface {
	// EmbedText converts text to embedding vector.
	EmbedText(ctx context.Context, text string) ([]float32, error)

	// Search finds top-K similar documents by vector similarity.
	Search(ctx context.Context, embedding []float32, topK int) ([]Document, error)

	// IndexDocuments adds/updates documents in the vector store.
	IndexDocuments(ctx context.Context, docs []Document) error

	// DeleteDocuments removes documents by IDs.
	DeleteDocuments(ctx context.Context, ids []string) error

	// Name returns the retriever identifier.
	Name() string

	// IsReady returns true if the retriever is initialized and ready.
	IsReady() bool
}

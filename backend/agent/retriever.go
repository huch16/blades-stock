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

// RetrieverConfig holds configuration for retriever implementations.
type RetrieverConfig struct {
	// EmbeddingModel is the name of the embedding model to use.
	EmbeddingModel string

	// VectorDimension is the dimension of embedding vectors.
	VectorDimension int

	// TopK is the default number of results to return.
	TopK int

	// ScoreThreshold is the minimum similarity score for results.
	ScoreThreshold float32

	// EnableCache enables in-memory caching of embeddings.
	EnableCache bool

	// CacheTTL is the time-to-live for cached embeddings.
	CacheTTL time.Duration
}

// RetrieverError represents a retriever-specific error.
type RetrieverError struct {
	Code    string
	Message string
	Err     error
}

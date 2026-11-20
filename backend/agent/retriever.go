package agent

import (
	"context"
	"strconv"
	"sync"
	"time"

	"blades-stock/backend/logger"
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

// DefaultRetrieverConfig returns a sensible default configuration.
func DefaultRetrieverConfig() RetrieverConfig {
	return RetrieverConfig{
		EmbeddingModel:  "bge-m3",
		VectorDimension: 1024,
		TopK:            5,
		ScoreThreshold:  0.3,
		EnableCache:     true,
		CacheTTL:        24 * time.Hour,
	}
}

// InMemoryRetriever is a simple in-memory vector store for testing/development.
// Production should use Redis Vector, Milvus, Weaviate, etc.
type InMemoryRetriever struct {
	config   RetrieverConfig
	embedSvc *EmbeddingService

	mu      sync.RWMutex
	docs    map[string]Document
	vectors map[string][]float32
	cache   map[string][]float32 // text hash -> embedding

	ready bool
}

var _ Retriever = (*InMemoryRetriever)(nil)

// NewInMemoryRetriever creates a new in-memory retriever.
func NewInMemoryRetriever(config RetrieverConfig, embedSvc *EmbeddingService) *InMemoryRetriever {
	if config.TopK <= 0 {
		config.TopK = 5
	}
	if config.ScoreThreshold <= 0 {
		config.ScoreThreshold = 0.3
	}
	if config.VectorDimension <= 0 {
		config.VectorDimension = 1024
	}

	r := &InMemoryRetriever{
		config:   config,
		embedSvc: embedSvc,
		docs:     make(map[string]Document),
		vectors:  make(map[string][]float32),
		cache:    make(map[string][]float32),
		ready:    embedSvc != nil && embedSvc.IsConfigured(),
	}

	if r.ready {
		logger.SugaredLogger.Infof("InMemoryRetriever initialized with model: %s, dim: %d", config.EmbeddingModel, config.VectorDimension)
	} else {
		logger.SugaredLogger.Warn("InMemoryRetriever created but EmbeddingService not configured - will operate in degraded mode")
	}
	return r
}

// Name returns the retriever identifier.
func (r *InMemoryRetriever) Name() string {
	return "in_memory"
}

// IsReady returns true if the retriever is initialized and ready.
func (r *InMemoryRetriever) IsReady() bool {
	return r.ready
}

// EmbedText converts text to embedding vector using the embedding service.
func (r *InMemoryRetriever) EmbedText(ctx context.Context, text string) ([]float32, error) {
	if !r.ready || r.embedSvc == nil {
		return nil, ErrEmbeddingServiceNotReady
	}

	// Check cache first
	hash := HashText(text)
	r.mu.RLock()
	if cached, ok := r.cache[hash]; ok {
		r.mu.RUnlock()
		return cached, nil
	}
	r.mu.RUnlock()

	// Generate embedding
	embedding, err := r.embedSvc.Embed(text)
	if err != nil {
		return nil, err
	}

	// Cache the result
	if r.config.EnableCache {
		r.mu.Lock()
		r.cache[hash] = embedding
		r.mu.Unlock()
	}

	return embedding, nil
}

// Search finds top-K similar documents by cosine similarity.
func (r *InMemoryRetriever) Search(ctx context.Context, embedding []float32, topK int) ([]Document, error) {
	if topK <= 0 {
		topK = r.config.TopK
	}

	r.mu.RLock()
	defer r.mu.RUnlock()

	if len(r.vectors) == 0 {
		return []Document{}, nil
	}

	type scoredDoc struct {
		doc   Document
		score float32
	}

	var results []scoredDoc
	for id, vec := range r.vectors {
		if doc, ok := r.docs[id]; ok {
			score := CosineSimilarity(embedding, vec)
			if score >= r.config.ScoreThreshold {
				results = append(results, scoredDoc{doc: doc, score: score})
			}
		}
	}

	// Sort by score descending
	for i := 0; i < len(results); i++ {
		for j := i + 1; j < len(results); j++ {
			if results[j].score > results[i].score {
				results[i], results[j] = results[j], results[i]
			}
		}
	}

	// Limit to topK
	if len(results) > topK {
		results = results[:topK]
	}

	docs := make([]Document, len(results))
	for i, r := range results {
		docs[i] = r.doc
		docs[i].Score = r.score
	}

	return docs, nil
}

// IndexDocuments adds/updates documents in the vector store.
func (r *InMemoryRetriever) IndexDocuments(ctx context.Context, docs []Document) error {
	if !r.ready || r.embedSvc == nil {
		return ErrEmbeddingServiceNotReady
	}

	r.mu.Lock()
	defer r.mu.Unlock()

	for _, doc := range docs {
		// Generate embedding if not provided
		if len(doc.Text) > 0 {
			embedding, err := r.embedSvc.Embed(doc.Text)
			if err != nil {
				logger.SugaredLogger.Warnf("Failed to embed document %s: %v", doc.ID, err)
				continue
			}
			r.vectors[doc.ID] = embedding
			doc.EmbeddedAt = time.Now()
		}
		r.docs[doc.ID] = doc
	}

	logger.SugaredLogger.Infof("Indexed %d documents in InMemoryRetriever", len(docs))
	return nil
}

// DeleteDocuments removes documents by IDs.
func (r *InMemoryRetriever) DeleteDocuments(ctx context.Context, ids []string) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	for _, id := range ids {
		delete(r.docs, id)
		delete(r.vectors, id)
	}
	return nil
}

// Stats returns retriever statistics.
func (r *InMemoryRetriever) Stats() map[string]any {
	r.mu.RLock()
	defer r.mu.RUnlock()

	return map[string]any{
		"document_count": len(r.docs),
		"vector_count":   len(r.vectors),
		"cache_size":     len(r.cache),
		"ready":          r.ready,
		"model":          r.config.EmbeddingModel,
	}
}

// ErrEmbeddingServiceNotReady is returned when embedding service is not available.
var ErrEmbeddingServiceNotReady = &RetrieverError{Code: "EMBEDDING_NOT_READY", Message: "embedding service not configured or not ready"}

// RetrieverError represents a retriever-specific error.
type RetrieverError struct {
	Code    string
	Message string
	Err     error
}

func (e *RetrieverError) Error() string {
	if e.Err != nil {
		return e.Message + ": " + e.Err.Error()
	}
	return e.Message
}

// HashText creates a simple hash for text caching.
func HashText(text string) string {
	// Simple hash - in production use crypto/sha256
	h := uint64(0)
	for i := 0; i < len(text); i++ {
		h = h*31 + uint64(text[i])
	}
	return strconv.FormatUint(h, 36)
}

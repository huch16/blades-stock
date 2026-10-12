package agent

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/redis/go-redis/v9"
	"blades-stock/backend/logger"
)

// RedisRetriever implements Retriever using Redis (simplified version).
type RedisRetriever struct {
	client      *redis.Client
	indexName   string
	vectorDim   int
	prefix      string
}

// NewRedisRetriever creates a new Redis-based retriever.
// addr: Redis address (e.g., "localhost:6379")
// password: Redis password (empty for no password)
// indexName: Name of the RediSearch index
// vectorDim: Dimension of the embedding vectors
func NewRedisRetriever(addr, password, indexName string, vectorDim int) (*RedisRetriever, error) {
	r := &RedisRetriever{
		client:   redis.NewClient(&redis.Options{
			Addr:     addr,
			Password: password,
			DB:       0,
		}),
		indexName:   indexName,
		vectorDim:   vectorDim,
		prefix:      indexName + ":",
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// Test connection
	if err := r.client.Ping(ctx).Err(); err != nil {
		return nil, fmt.Errorf("failed to connect to Redis: %w", err)
	}

	logger.SugaredLogger.Infof("RedisRetriever initialized: index=%s, dim=%d, addr=%s", 
		indexName, vectorDim, addr)
	return r, nil
}

// Name returns the retriever identifier.
func (r *RedisRetriever) Name() string {
	return "redis"
}

// IsReady returns true if the retriever is initialized and ready.
func (r *RedisRetriever) IsReady() bool {
	if r.client == nil {
		return false
	}
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	return r.client.Ping(ctx).Err() == nil
}

// EmbedText converts text to embedding vector (placeholder).
func (r *RedisRetriever) EmbedText(ctx context.Context, text string) ([]float32, error) {
	// This would normally call an embedding service
	// For now, return a fixed-size vector with dummy values
	vector := make([]float32, r.vectorDim)
	for i := range vector {
		vector[i] = 0.1
	}
	return vector, nil
}

// IndexDocuments adds/updates documents in the vector store (simplified).
func (r *RedisRetriever) IndexDocuments(ctx context.Context, docs []Document) error {
	if len(docs) == 0 {
		return nil
	}
	if r.client == nil {
		return fmt.Errorf("redis client not initialized")
	}

	pipe := r.client.Pipeline()

	for _, doc := range docs {
		docID := fmt.Sprintf("%s%s", r.prefix, doc.ID)
		// Store as JSON for simplicity
		jsonData, err := json.Marshal(doc)
		if err != nil {
			return err
		}
		pipe.Set(ctx, docID, jsonData, 0)
	}

	if _, err := pipe.Exec(ctx); err != nil {
		return fmt.Errorf("failed to execute pipeline: %w", err)
	}

	return nil
}

// Search finds documents (simplified - just returns all documents for now).
func (r *RedisRetriever) Search(ctx context.Context, embedding []float32, topK int) ([]Document, error) {
	if r.client == nil {
		return []Document{}, fmt.Errorf("redis client not initialized")
	}
	// This is a simplified implementation that doesn't do actual vector search
	// In a real implementation, we would use RediSearch for KNN vector search
	keys, err := r.client.Keys(ctx, r.prefix+"*").Result()
	if err != nil {
		return nil, err
	}

	var docs []Document
	for _, key := range keys {
		if len(docs) >= topK {
			break
		}
		data, err := r.client.Get(ctx, key).Result()
		if err != nil {
			continue
		}
		var doc Document
		if err := json.Unmarshal([]byte(data), &doc); err != nil {
			continue
		}
		docs = append(docs, doc)
	}

	return docs, nil
}

// DeleteDocuments removes documents by IDs.
func (r *RedisRetriever) DeleteDocuments(ctx context.Context, ids []string) error {
	if len(ids) == 0 {
		return nil
	}
	if r.client == nil {
		return fmt.Errorf("redis client not initialized")
	}

	pipe := r.client.Pipeline()
	for _, id := range ids {
		docID := fmt.Sprintf("%s%s", r.prefix, id)
		pipe.Del(ctx, docID)
	}
	
	if _, err := pipe.Exec(ctx); err != nil {
		return fmt.Errorf("failed to delete documents: %w", err)
	}

	return nil
}

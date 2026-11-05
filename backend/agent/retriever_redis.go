package agent

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/binary"
	"encoding/json"
	"fmt"
	"math"
	"time"

	"blades-stock/backend/logger"
	"github.com/redis/go-redis/v9"
)

// RedisRetriever stores documents in Redis HASHes with a vector blob field.
// It expects a RediSearch index created with ON HASH PREFIX "agent:doc:" and
// a VECTOR field configured suitable for the model dimension.
type RedisRetriever struct {
	client    *redis.Client
	indexName string
	prefix    string
	dim       int
	embedSvc  *EmbeddingService
}

func NewRedisRetriever(addr, password, indexName, prefix string, dim int, embedSvc *EmbeddingService) (*RedisRetriever, error) {
	client := redis.NewClient(&redis.Options{
		Addr:     addr,
		Password: password,
		DB:       0,
		PoolSize: 20,
	})
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := client.Ping(ctx).Err(); err != nil {
		return nil, fmt.Errorf("redis ping failed: %w", err)
	}
	return &RedisRetriever{
		client:    client,
		indexName: indexName,
		prefix:    prefix,
		dim:       dim,
		embedSvc:  embedSvc,
	}, nil
}

func (r *RedisRetriever) Name() string { return "redis" }

func (r *RedisRetriever) IsReady() bool {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	return r.client.Ping(ctx).Err() == nil
}

func (r *RedisRetriever) EmbedText(ctx context.Context, text string) ([]float32, error) {
	if r.embedSvc == nil {
		return nil, fmt.Errorf("embedding service not configured")
	}
	return r.embedSvc.EmbedText(ctx, text)
}

func (r *RedisRetriever) IndexDocuments(ctx context.Context, docs []Document) error {
	if len(docs) == 0 {
		return nil
	}
	pipe := r.client.Pipeline()
	for _, d := range docs {
		key := r.prefix + d.ID
		meta := map[string]string{"source": d.Source}
		for k, v := range d.Meta {
			meta[k] = v
		}
		metaJSON, err := json.Marshal(meta)
		if err != nil {
			logger.SugaredLogger.Warnf("marshal meta for %s failed: %v", d.ID, err)
			metaJSON = []byte("{}")
		}
		vecBytes := float32sToBytes(d.Vector)
		vecBase64 := base64.StdEncoding.EncodeToString(vecBytes)
		pipe.HSet(ctx, key, map[string]interface{}{
			"text":   d.Text,
			"meta":   string(metaJSON),
			"vector": vecBase64,
		})
	}
	if _, err := pipe.Exec(ctx); err != nil {
		return fmt.Errorf("redis pipeline exec failed: %w", err)
	}
	return nil
}

func (r *RedisRetriever) Search(ctx context.Context, embedding []float32, topK int) ([]Document, error) {
	if len(embedding) == 0 {
		return nil, fmt.Errorf("embedding empty")
	}
	enc := base64.StdEncoding.EncodeToString(float32sToBytes(embedding))
	args := []interface{}{
		r.indexName,
		fmt.Sprintf("*=>[KNN %d @vector $vec]", topK),
		"PARAMS", 2, "vec", enc,
		"RETURN", 3, "text", "meta", "score",
	}
	res, err := r.client.Do(ctx, append([]interface{}{"FT.SEARCH"}, args...)...).Result()
	if err != nil {
		return nil, fmt.Errorf("FT.SEARCH failed: %w", err)
	}

	var docs []Document
	arr, ok := res.([]interface{})
	if !ok || len(arr) < 1 {
		return docs, nil
	}
	for i := 1; i < len(arr); i += 2 {
		if i+1 >= len(arr) {
			break
		}
		id, _ := arr[i].(string)
		fieldPairs, _ := arr[i+1].([]interface{})
		doc := Document{ID: id}
		for j := 0; j < len(fieldPairs)-1; j += 2 {
			k, _ := fieldPairs[j].(string)
			v, _ := fieldPairs[j+1].(string)
			switch k {
			case "text":
				doc.Text = v
			case "meta":
				var meta map[string]string
				if err := json.Unmarshal([]byte(v), &meta); err == nil {
					doc.Meta = meta
				}
			}
		}
		if doc.Meta != nil {
			doc.Source = doc.Meta["source"]
		}
		docs = append(docs, doc)
	}
	return docs, nil
}

func (r *RedisRetriever) DeleteDocuments(ctx context.Context, ids []string) error {
	if len(ids) == 0 {
		return nil
	}
	keys := make([]string, 0, len(ids))
	for _, id := range ids {
		keys = append(keys, r.prefix+id)
	}
	return r.client.Del(ctx, keys...).Err()
}

func float32sToBytes(v []float32) []byte {
	buf := &bytes.Buffer{}
	for _, f := range v {
		bits := math.Float32bits(f)
		b := make([]byte, 4)
		binary.LittleEndian.PutUint32(b, bits)
		_, _ = buf.Write(b)
	}
	return buf.Bytes()
}

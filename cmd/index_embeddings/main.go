//go:build ignore

package main

import (
	"context"
	"encoding/json"
	"flag"
	"net"
	"net/http"
	"strings"
	"time"

	"blades-stock/backend/agent"
	"blades-stock/backend/data"
	"blades-stock/backend/db"
	"blades-stock/backend/logger"
)

func main() {
	var (
		retrieverType = flag.String("type", "memory", "retriever type: memory, redis, sqlite")
		redisAddr     = flag.String("redis", "localhost:6379", "redis address")
		types         = flag.String("types", "skills,docs,memories", "comma-separated list of types to index: skills,docs,memories")
		batchSize     = flag.Int("batch", 100, "batch size for indexing")
		dryRun        = flag.Bool("dry-run", false, "print what would be indexed without writing")
		force         = flag.Bool("force", false, "force reindexing of all documents")
		since         = flag.String("since", "0", "duration for incremental memory indexing (e.g., 24h, 7d; 0 disables incremental)")
	)
	flag.Parse()

	// Initialize DB
	db.Init("")
	if err := db.RunVersionedMigrations(db.Dao); err != nil {
		logger.SugaredLogger.Fatalf("migration failed: %v", err)
	}

	// Start a mock embedding HTTP server if needed
	var mockSrv *http.Server
	var mockListener net.Listener
	var baseURL string
	settingConfig := data.GetSettingConfig()
	if settingConfig == nil {
		logger.SugaredLogger.Fatal("settingConfig is nil")
	}
	useMock := false
	if settingConfig.AiConfigs == nil || len(settingConfig.AiConfigs) == 0 {
		useMock = true
	} else {
		cfg := settingConfig.AiConfigs[0]
		if !cfg.EmbeddingEnabled || cfg.EmbeddingBaseUrl == "" || cfg.EmbeddingModelName == "" {
			useMock = true
		}
	}
	if useMock {
		if *dryRun {
			logger.SugaredLogger.Warn("EmbeddingService not configured, starting mock server for dry-run")
		} else {
			logger.SugaredLogger.Warn("EmbeddingService not configured, starting mock server")
		}
		mux := http.NewServeMux()
		mux.HandleFunc("/embeddings", func(w http.ResponseWriter, r *http.Request) {
			logger.SugaredLogger.Infof("mock server received request: %s %s", r.Method, r.URL.Path)
			if r.Method != http.MethodPost {
				http.Error(w, "only POST allowed", http.StatusMethodNotAllowed)
				return
			}
			var req struct {
				Model string `json:"model"`
				Input string `json:"input"`
			}
			if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
				http.Error(w, "invalid JSON", http.StatusBadRequest)
				return
			}
			// We ignore the model and input for simplicity; return a fixed vector
			vectorDim := 128
			resp := struct {
				Data []struct {
					Embedding []float32 `json:"embedding"`
					Index     int       `json:"index"`
				} `json:"data"`
				Usage struct {
					PromptTokens     int `json:"prompt_tokens"`
					TotalTokens      int `json:"total_tokens"`
				} `json:"usage"`
			}{
				Data: []struct {
					Embedding []float32 `json:"embedding"`
					Index     int       `json:"index"`
				}{
					{Embedding: make([]float32, vectorDim), Index: 0},
				},
				Usage: struct {
					PromptTokens     int `json:"prompt_tokens"`
					TotalTokens      int `json:"total_tokens"`
				}{PromptTokens: 0, TotalTokens: 0},
			}
			// Fill the embedding with 0.1
			for i := range resp.Data[0].Embedding {
				resp.Data[0].Embedding[i] = 0.1
			}
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(resp)
		})
		// Also handle any other path to log it
		mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
			logger.SugaredLogger.Infof("mock server received request (other): %s %s", r.Method, r.URL.Path)
			http.NotFound(w, r)
		})
		mockSrv = &http.Server{Handler: mux}
		var err error
		mockListener, err = net.Listen("tcp", "127.0.0.1:0")
		if err != nil {
			logger.SugaredLogger.Fatalf("failed to listen on random port: %v", err)
		}
		// We need to serve in a goroutine
		go func() {
			logger.SugaredLogger.Infof("mock server starting on %s", mockListener.Addr())
			if err := mockSrv.Serve(mockListener); err != nil && err != http.ErrServerClosed {
				logger.SugaredLogger.Errorf("mock server error: %v", err)
			}
		}()
		baseURL = "http://" + mockListener.Addr().String()
		logger.SugaredLogger.Infof("mock server baseURL: %s", baseURL)
		// Give the server a moment to start
		time.Sleep(100 * time.Millisecond)
	} else {
		// Use the real configuration
		cfg := settingConfig.AiConfigs[0]
		baseURL = cfg.EmbeddingBaseUrl
		logger.SugaredLogger.Infof("using real baseURL: %s", baseURL)
	}

	// Get embedding service
	var embedSvc *agent.EmbeddingService
	if !useMock {
		if settingConfig.AiConfigs != nil && len(settingConfig.AiConfigs) > 0 {
			cfg := settingConfig.AiConfigs[0]
			if cfg.EmbeddingEnabled && cfg.EmbeddingBaseUrl != "" && cfg.EmbeddingModelName != "" {
				embedSvc = agent.NewEmbeddingService(cfg.EmbeddingBaseUrl, cfg.ApiKey, cfg.EmbeddingModelName)
			}
		}
	} else {
		// Use mock service
		embedSvc = agent.NewEmbeddingService(baseURL, "mock-key", "mock-model")
	}
	if embedSvc == nil || !embedSvc.IsConfigured() {
		logger.SugaredLogger.Fatalf("EmbeddingService not configured - baseURL=%s", baseURL)
	}

	ctx := context.Background()

	// Create retriever based on type
	var retr agent.Retriever
	switch *retrieverType {
	case "memory":
		retr = agent.NewInMemoryRetriever(agent.DefaultRetrieverConfig(), embedSvc)
		logger.SugaredLogger.Info("Using InMemoryRetriever")
	case "sqlite":
		sqlPath := "data/vectors.db"
		sqlRetr, err := agent.NewSQLiteRetriever(sqlPath)
		if err != nil {
			logger.SugaredLogger.Fatalf("Failed to create SQLite retriever: %v", err)
		}
		retr = sqlRetr
		logger.SugaredLogger.Infof("Using SQLiteRetriever with DB %s", sqlPath)
	case "redis":
		dummyVec, err := embedSvc.EmbedText(ctx, "dummy")
		if err != nil {
			logger.SugaredLogger.Fatalf("Failed to get embedding dimension: %v", err)
		}
		vecDim := len(dummyVec)
		retr, err = agent.NewRedisRetriever(*redisAddr, "", "agent_vectors", "agent:doc:", vecDim, embedSvc)
		if err != nil {
			logger.SugaredLogger.Fatalf("Failed to create Redis retriever: %v", err)
		}
		logger.SugaredLogger.Infof("Using RedisRetriever with vector dimension %d", vecDim)
	default:
		logger.SugaredLogger.Fatalf("Unknown retriever type: %s", *retrieverType)
	}

	if !retr.IsReady() {
		logger.SugaredLogger.Fatal("Retriever not ready")
	}

	// Parse types
	typeList := strings.Split(*types, ",")
	typeSet := make(map[string]bool)
	for _, t := range typeList {
		typeSet[strings.TrimSpace(t)] = true
	}

	// Parse since duration
	sinceDuration, err := time.ParseDuration(*since)
	if err != nil {
		logger.SugaredLogger.Fatalf("Invalid since duration %s: %v", *since, err)
	}

	// Build indexer config
	cfg := agent.IndexerConfig{
		Backend:       *retrieverType,
		BatchSize:     *batchSize,
		DryRun:        *dryRun,
		ForceReindex:  *force,
		DocsPath:      "docs/intelligence", // relative to current directory
		SinceDuration: sinceDuration,
	}

	// Run the indexer
	if err := agent.IndexDocumentsFromAllSources(ctx, cfg, retr, embedSvc); err != nil {
		logger.SugaredLogger.Fatalf("Indexing failed: %v", err)
	}

	if *dryRun {
		logger.SugaredLogger.Info("Dry run completed")
	} else {
		logger.SugaredLogger.Info("Indexing completed")
	}

	// Shut down mock server if we started one
	if mockSrv != nil {
		mockSrv.Close()
		mockListener.Close()
	}
}

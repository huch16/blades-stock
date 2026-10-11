// Package chaos provides chaos engineering experiments for Blades-Stock.
// Can run as standalone or be triggered via HTTP endpoint.
package main

import (
	"context"
	"flag"
	"fmt"
	"math/rand"
	"net/http"
	_ "net/http/pprof"
	"os"
	"os/signal"
	"syscall"
	"time"

	"blades-stock/backend/agent"
	"blades-stock/backend/data"
	"blades-stock/backend/db"
	"blades-stock/backend/logger"
	"blades-stock/backend/paths"
)

func main() {
	var (
		mode         = flag.String("mode", "http", "Run mode: http, latency, error, cpu, mem, disk")
		port         = flag.Int("port", 9465, "HTTP server port for chaos API")
		latencyMs    = flag.Int("latency", 200, "Latency to inject (ms)")
		errorRate    = flag.Float64("error-rate", 0.1, "Error injection rate (0-1)")
		duration     = flag.Duration("duration", 30*time.Second, "Experiment duration")
		targetURL    = flag.String("target", "", "Target URL for HTTP chaos proxy")
	)
	flag.Parse()

	logger.InitLogger()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Initialize paths and DB
	paths.InitBaseDir()
	db.InitDB()

	// Start HTTP server for chaos API
	go func() {
		http.HandleFunc("/healthz", func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusOK)
			w.Write([]byte("OK"))
		})
		http.HandleFunc("/chaos/latency", func(w http.ResponseWriter, r *http.Request) {
			latency := *latencyMs
			if l := r.URL.Query().Get("ms"); l != "" {
				fmt.Sscanf(l, "%d", &latency)
			}
			data.SetMockLatency(latency)
			fmt.Fprintf(w, "Latency set to %dms", latency)
		})
		http.HandleFunc("/chaos/error", func(w http.ResponseWriter, r *http.Request) {
			rate := *errorRate
			if e := r.URL.Query().Get("rate"); e != "" {
				fmt.Sscanf(e, "%f", &rate)
			}
			data.SetMockErrorRate(rate)
			fmt.Fprintf(w, "Error rate set to %.2f", rate)
		})
		http.HandleFunc("/chaos/reset", func(w http.ResponseWriter, r *http.Request) {
			data.SetMockLatency(0)
			data.SetMockErrorRate(0)
			fmt.Fprint(w, "Chaos reset")
		})
		http.HandleFunc("/chaos/status", func(w http.ResponseWriter, r *http.Request) {
			fmt.Fprintf(w, "Latency: %dms, ErrorRate: %.2f", data.GetMockLatency(), data.GetMockErrorRate())
		})
		logger.SugaredLogger.Infof("Chaos HTTP server listening on :%d", *port)
		http.ListenAndServe(fmt.Sprintf(":%d", *port), nil)
	}()

	// Run experiment based on mode
	switch *mode {
	case "http":
		// Just run HTTP server
		waitForSignal()
	case "latency":
		runLatencyExperiment(ctx, *latencyMs, *duration)
	case "error":
		runErrorExperiment(ctx, *errorRate, *duration)
	case "cpu":
		runCPUStress(ctx, *duration)
	case "mem":
		runMemoryPressure(ctx, *duration)
	case "disk":
		runDiskFill(ctx, *duration)
	case "proxy":
		if *targetURL == "" {
			logger.SugaredLogger.Fatal("Target URL required for proxy mode")
		}
		runChaosProxy(ctx, *targetURL, *latencyMs, *errorRate, *duration)
	default:
		logger.SugaredLogger.Fatalf("Unknown mode: %s", *mode)
	}
}

func waitForSignal() {
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
	<-sigCh
	logger.SugaredLogger.Info("Shutdown signal received")
}

func runLatencyExperiment(ctx context.Context, latencyMs int, duration time.Duration) {
	logger.SugaredLogger.Infof("Starting latency experiment: %dms for %v", latencyMs, duration)
	data.SetMockLatency(latencyMs)
	defer data.SetMockLatency(0)

	select {
	case <-ctx.Done():
	case <-time.After(duration):
	}
	logger.SugaredLogger.Info("Latency experiment completed")
}

func runErrorExperiment(ctx context.Context, errorRate float64, duration time.Duration) {
	logger.SugaredLogger.Infof("Starting error injection: %.2f for %v", errorRate, duration)
	data.SetMockErrorRate(errorRate)
	defer data.SetMockErrorRate(0)

	select {
	case <-ctx.Done():
	case <-time.After(duration):
	}
	logger.SugaredLogger.Info("Error injection experiment completed")
}

func runCPUStress(ctx context.Context, duration time.Duration) {
	logger.SugaredLogger.Infof("Starting CPU stress for %v", duration)
	// Spin up workers to consume CPU
	workers := 4
	done := make(chan struct{})
	for i := 0; i < workers; i++ {
		go func() {
			for {
				select {
				case <-done:
					return
				default:
					// Busy loop
					_ = rand.Int63()
				}
			}
		}()
	}

	select {
	case <-ctx.Done():
		close(done)
	case <-time.After(duration):
		close(done)
	}
	logger.SugaredLogger.Info("CPU stress completed")
}

func runMemoryPressure(ctx context.Context, duration time.Duration) {
	logger.SugaredLogger.Infof("Starting memory pressure for %v", duration)
	// Allocate memory in chunks
	var allocations [][]byte
	chunkSize := 100 * 1024 * 1024 // 100MB chunks
	ticker := time.NewTicker(500 * time.Millisecond)
	defer ticker.Stop()

	go func() {
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				buf := make([]byte, chunkSize)
				// Touch memory
				for i := 0; i < len(buf); i += 4096 {
					buf[i] = byte(rand.Intn(256))
				}
				allocations = append(allocations, buf)
				logger.SugaredLogger.Infof("Allocated %d MB", len(allocations)*100)
			}
		}
	}()

	select {
	case <-ctx.Done():
	case <-time.After(duration):
	}
	allocations = nil // Release
	logger.SugaredLogger.Info("Memory pressure completed")
}

func runDiskFill(ctx context.Context, duration time.Duration) {
	logger.SugaredLogger.Infof("Starting disk fill for %v", duration)
	logDir := paths.LogDir()
	file, err := os.CreateTemp(logDir, "chaos-disk-*")
	if err != nil {
		logger.SugaredLogger.Errorf("Failed to create temp file: %v", err)
		return
	}
	defer os.Remove(file.Name())

	ticker := time.NewTicker(1 * time.Second)
	defer ticker.Stop()

	go func() {
		buf := make([]byte, 10*1024*1024) // 10MB writes
		rand.Read(buf)
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				if _, err := file.Write(buf); err != nil {
					logger.SugaredLogger.Errorf("Disk write error: %v", err)
					return
				}
				file.Sync()
			}
		}
	}()

	select {
	case <-ctx.Done():
	case <-time.After(duration):
	}
	logger.SugaredLogger.Info("Disk fill completed")
}

func runChaosProxy(ctx context.Context, targetURL string, latencyMs int, errorRate float64, duration time.Duration) {
	logger.SugaredLogger.Infof("Starting chaos proxy to %s: latency=%dms, errorRate=%.2f", targetURL, latencyMs, errorRate)
	// This would implement an HTTP proxy that injects chaos
	// For simplicity, we just log the intent
	select {
	case <-ctx.Done():
	case <-time.After(duration):
	}
	logger.SugaredLogger.Info("Chaos proxy experiment completed")
}

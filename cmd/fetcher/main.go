package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"runtime"
	"syscall"

	"github.com/lancelop89/youtube-trend-tracker/internal/config"
	"github.com/lancelop89/youtube-trend-tracker/internal/fetcher"
	"github.com/lancelop89/youtube-trend-tracker/internal/logger"
	"github.com/lancelop89/youtube-trend-tracker/internal/storage"
	"github.com/lancelop89/youtube-trend-tracker/internal/youtube"
)

// Global configuration
var (
	cfg *config.Config
	log = logger.New()
)

func main() {
	// Parse command line flags
	configPath := flag.String("config", "configs/config.yaml", "Path to configuration file")
	flag.Parse()

	// Load configuration
	var err error
	cfg, err = config.Load(*configPath)
	if err != nil {
		log.Fatal("Failed to load configuration", err, nil)
	}

	// Update logger based on configuration
	log = logger.New()

	// Setup HTTP handlers
	http.HandleFunc("/", handler)
	http.HandleFunc("/healthz", healthzHandler)
	http.HandleFunc("/info", infoHandler)

	// Create HTTP server
	srv := &http.Server{
		Addr:           ":" + cfg.Server.Port,
		ReadTimeout:    cfg.Server.ReadTimeout,
		WriteTimeout:   cfg.Server.WriteTimeout,
		MaxHeaderBytes: cfg.Server.MaxHeaderBytes,
	}

	// Setup graceful shutdown
	idleConnsClosed := make(chan struct{})
	go func() {
		sigint := make(chan os.Signal, 1)
		signal.Notify(sigint, os.Interrupt, syscall.SIGTERM)
		<-sigint

		log.Info("Shutting down server...", nil)
		ctx, cancel := context.WithTimeout(context.Background(), cfg.Server.ShutdownTimeout)
		defer cancel()

		if err := srv.Shutdown(ctx); err != nil {
			log.Error("Server shutdown error", err, nil)
		}
		close(idleConnsClosed)
	}()

	// Start server
	log.Info(fmt.Sprintf("Starting server on port %s", cfg.Server.Port), map[string]string{
		"environment": cfg.App.Environment,
		"project_id":  cfg.GCP.ProjectID,
	})

	if err := srv.ListenAndServe(); err != http.ErrServerClosed {
		log.Fatal("Server failed to start", err, nil)
	}

	<-idleConnsClosed
	log.Info("Server stopped", nil)
}

func healthzHandler(w http.ResponseWriter, r *http.Request) {
	w.WriteHeader(http.StatusOK)
}

var ( // These variables are set at build time
	version   = "dev"
	commit    = "none"
	buildTime = "unknown"
)

func infoHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	info := map[string]string{
		"version":   version,
		"commit":    commit,
		"buildTime": buildTime,
		"goVersion": runtime.Version(),
		"os":        runtime.GOOS,
		"arch":      runtime.GOARCH,
	}
	json.NewEncoder(w).Encode(info)
}

func handler(w http.ResponseWriter, r *http.Request) {
	ctx := context.Background()

	// Get enabled channel IDs from configuration
	channelIDs := cfg.GetEnabledChannelIDs()
	if len(channelIDs) == 0 {
		log.Error("No enabled channels in configuration", nil, nil)
		http.Error(w, "No channels configured", http.StatusInternalServerError)
		return
	}

	// --- Initialization ---
	ytClient, err := youtube.NewClient(ctx, cfg.YouTube.APIKey)
	if err != nil {
		log.Error("Error creating YouTube client", err, nil)
		http.Error(w, "Failed to create YouTube client", http.StatusInternalServerError)
		return
	}

	bqWriter, err := storage.NewBigQueryWriterWithConfig(ctx, cfg.GCP.ProjectID, cfg.BigQuery.DatasetID, cfg.BigQuery.TableID)
	if err != nil {
		log.Error("Error creating BigQuery writer", err, nil)
		http.Error(w, "Failed to create BigQuery writer", http.StatusInternalServerError)
		return
	}

	// Ensure the table exists before proceeding.
	if err := bqWriter.EnsureTableExists(ctx); err != nil {
		log.Error("Error ensuring BigQuery table exists", err, nil)
		http.Error(w, "Failed to setup BigQuery table", http.StatusInternalServerError)
		return
	}

	// --- Execution ---
	f := fetcher.NewFetcher(ytClient, bqWriter)
	if err := f.FetchAndStore(ctx, channelIDs, cfg.App.MaxVideosPerChannel); err != nil {
		log.Error("An error occurred during the fetch and store process", err, nil)
		http.Error(w, "An error occurred during the fetch and store process", http.StatusInternalServerError)
		return
	}

	// --- Response ---
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{"status": "success"})
}

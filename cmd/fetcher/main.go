package main

import (
	"context"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"os"
	"runtime"
	"time"

	"github.com/joho/godotenv"
	"github.com/lancelop89/youtube-trend-tracker/internal/fetcher"
	"github.com/lancelop89/youtube-trend-tracker/internal/storage"
	"github.com/lancelop89/youtube-trend-tracker/internal/youtube"
	"gopkg.in/yaml.v2"
)

// Config defines the structure for channel configuration.
type Config struct {
	Channels []struct {
		ID string `yaml:"id"`
	} `yaml:"channels"`
}

// logEntry represents a structured log entry.
type logEntry struct {
	Timestamp string            `json:"timestamp"`	
	Level     string            `json:"level"`
	Message   string            `json:"message"`
	Error     string            `json:"error,omitempty"`
	Labels    map[string]string `json:"labels,omitempty"` // Add this field
}

func logJSON(level, msg string, err error, labels map[string]string) {
	entry := logEntry{
		Timestamp: time.Now().Format(time.RFC3339),
		Level:     level,
		Message:   msg,
		Labels:    labels,
	}
	if err != nil {
		entry.Error = err.Error()
	}

	jsonBytes, _ := json.Marshal(entry)
	fmt.Println(string(jsonBytes)) // Use fmt.Println to ensure newline
}

func isLocal() bool {
	return os.Getenv("GO_ENV") == "local"
}

func main() {
	if isLocal() {
		err := godotenv.Load()
		if err != nil {
			logJSON("warning", "Error loading .env file", err, nil)
		}
	}

	http.HandleFunc("/", handler)
	http.HandleFunc("/healthz", healthzHandler)
	http.HandleFunc("/info", infoHandler)

	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}

	logJSON("info", fmt.Sprintf("Listening on port %s", port), nil, nil)
	if err := http.ListenAndServe(":"+port, nil); err != nil {
		logJSON("fatal", "Server failed to start", err, nil)
		os.Exit(1) // Exit on fatal error
	}
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
		"version":    version,
		"commit":     commit,
		"buildTime":  buildTime,
		"goVersion":  runtime.Version(),
		"os":         runtime.GOOS,
		"arch":       runtime.GOARCH,
	}
	json.NewEncoder(w).Encode(info)
}

func handler(w http.ResponseWriter, r *http.Request) {
	ctx := context.Background()

	// --- Configuration ---
	projectID := os.Getenv("GOOGLE_CLOUD_PROJECT")
	apiKey := os.Getenv("YOUTUBE_API_KEY")
	channelConfigPath := os.Getenv("CHANNEL_CONFIG_PATH")

	if projectID == "" || apiKey == "" || channelConfigPath == "" {
		logJSON("error", "Missing required environment variables (PROJECT_ID, YOUTUBE_API_KEY, CHANNEL_CONFIG_PATH)", nil, nil)
		http.Error(w, "Server configuration error", http.StatusInternalServerError)
		return
	}

	// Read channel config from file
	channelConfigBytes, err := ioutil.ReadFile(channelConfigPath)
	if err != nil {
		logJSON("error", "Error reading channel config file", err, nil)
		http.Error(w, "Invalid channel configuration file", http.StatusInternalServerError)
		return
	}

	var config Config
	if err := yaml.Unmarshal(channelConfigBytes, &config); err != nil {
		logJSON("error", "Error parsing channel config", err, nil)
		http.Error(w, "Invalid channel configuration", http.StatusBadRequest)
		return
	}

	var channelIDs []string
	for _, ch := range config.Channels {
		channelIDs = append(channelIDs, ch.ID)
	}

	// --- Initialization ---
	ytClient, err := youtube.NewClient(ctx, apiKey)
	if err != nil {
		logJSON("error", "Error creating YouTube client", err, nil)
		http.Error(w, "Failed to create YouTube client", http.StatusInternalServerError)
		return
	}

	bqWriter, err := storage.NewBigQueryWriter(ctx, projectID)
	if err != nil {
		logJSON("error", "Error creating BigQuery writer", err, nil)
		http.Error(w, "Failed to create BigQuery writer", http.StatusInternalServerError)
		return
	}

	// Ensure the table exists before proceeding.
	if err := bqWriter.EnsureTableExists(ctx); err != nil {
		logJSON("error", "Error ensuring BigQuery table exists", err, nil)
		http.Error(w, "Failed to setup BigQuery table", http.StatusInternalServerError)
		return
	}

	// --- Execution ---
	f := fetcher.NewFetcher(ytClient, bqWriter)
	if err := f.FetchAndStore(ctx, channelIDs); err != nil {
		logJSON("error", "An error occurred during the fetch and store process", err, nil)
		http.Error(w, "An error occurred during the fetch and store process", http.StatusInternalServerError)
		return
	}

	// --- Response ---
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{"status": "success"})
}
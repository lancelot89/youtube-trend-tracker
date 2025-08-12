package main

import (
	"context"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"os"
	"runtime"
	"strconv"
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
	Labels    map[string]string `json:"labels,omitempty"`
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
	fmt.Println(string(jsonBytes))
}

func isLocal() bool {
	return os.Getenv("GO_ENV") == "local"
}

func main() {
	// Attempt to load .env file for local development.
	// This will be ignored in production environments where .env doesn't exist.
	err := godotenv.Load()
	if err != nil {
		// This is not a fatal error, just a warning for local dev.
		logJSON("warning", "Could not load .env file", err, nil)
	}

	// Check if running as a server or a one-off script
	if os.Getenv("RUN_AS_SERVER") == "true" {
		startServer()
	} else {
		runScript()
	}
}

// runScript executes the fetcher logic as a command-line script.
func runScript() {
	logJSON("info", "Running fetcher as a one-off script.", nil, nil)
	ctx := context.Background()
	if err := runFetcher(ctx); err != nil {
		logJSON("fatal", "Fetcher script failed", err, nil)
		os.Exit(1)
	}
	logJSON("info", "Fetcher script completed successfully.", nil, nil)
}

// startServer starts the HTTP server.
func startServer() {
	http.HandleFunc("/", handler)
	http.HandleFunc("/healthz", healthzHandler)
	http.HandleFunc("/info", infoHandler)

	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}

	logJSON("info", fmt.Sprintf("Starting HTTP server, listening on port %s", port), nil, nil)
	if err := http.ListenAndServe(":"+port, nil); err != nil {
		logJSON("fatal", "Server failed to start", err, nil)
		os.Exit(1)
	}
}

func handler(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	if err := runFetcher(ctx); err != nil {
		logJSON("error", "An error occurred during the fetch and store process", err, nil)
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{"status": "success"})
}

func runFetcher(ctx context.Context) error {
	// --- Configuration ---
	projectID := os.Getenv("GOOGLE_CLOUD_PROJECT")
	apiKey := os.Getenv("YOUTUBE_API_KEY")
	channelConfigPath := os.Getenv("CHANNEL_CONFIG_PATH")
	maxVideosPerChannelStr := os.Getenv("MAX_VIDEOS_PER_CHANNEL")

	if projectID == "" || apiKey == "" || channelConfigPath == "" {
		return fmt.Errorf("missing required environment variables (GOOGLE_CLOUD_PROJECT, YOUTUBE_API_KEY, CHANNEL_CONFIG_PATH)")
	}
	if apiKey == "YOUR_YOUTUBE_API_KEY" {
		return fmt.Errorf("placeholder API key found. Please replace YOUR_YOUTUBE_API_KEY in your .env file")
	}

	maxVideosPerChannel, err := strconv.ParseInt(maxVideosPerChannelStr, 10, 64)
	if err != nil || maxVideosPerChannel <= 0 {
		logJSON("warning", fmt.Sprintf("Invalid or missing MAX_VIDEOS_PER_CHANNEL, using default 10."), nil, nil)
		maxVideosPerChannel = 10 // Default value
	}

	// Read channel config from file
	logJSON("info", "Reading channel configuration", nil, map[string]string{"path": channelConfigPath})
	channelConfigBytes, err := ioutil.ReadFile(channelConfigPath)
	if err != nil {
		return fmt.Errorf("error reading channel config file at %s: %w", channelConfigPath, err)
	}

	var config Config
	if err := yaml.Unmarshal(channelConfigBytes, &config); err != nil {
		return fmt.Errorf("error parsing channel config yaml: %w", err)
	}

	var channelIDs []string
	for _, ch := range config.Channels {
		channelIDs = append(channelIDs, ch.ID)
	}
	logJSON("info", fmt.Sprintf("Found %d channels to process", len(channelIDs)), nil, nil)

	// --- Initialization ---
	logJSON("info", "Initializing clients...", nil, nil)
	ytClient, err := youtube.NewClient(ctx, apiKey)
	if err != nil {
		return fmt.Errorf("error creating YouTube client: %w", err)
	}

	bqWriter, err := storage.NewBigQueryWriter(ctx, projectID)
	if err != nil {
		return fmt.Errorf("error creating BigQuery writer: %w", err)
	}

	// Ensure the table exists before proceeding.
	if err := bqWriter.EnsureTableExists(ctx); err != nil {
		return fmt.Errorf("error ensuring BigQuery table exists: %w", err)
	}
	logJSON("info", "Clients initialized and table checked successfully.", nil, nil)

	// --- Execution ---
	f := fetcher.NewFetcher(ytClient, bqWriter)
	if err := f.FetchAndStore(ctx, channelIDs, maxVideosPerChannel); err != nil {
		return fmt.Errorf("an error occurred during the fetch and store process: %w", err)
	}

	return nil
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

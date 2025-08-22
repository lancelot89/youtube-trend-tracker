package main

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"regexp"
	"runtime"
	"strconv"
	"strings"

	"github.com/joho/godotenv"
	"github.com/lancelop89/youtube-trend-tracker/internal/fetcher"
	"github.com/lancelop89/youtube-trend-tracker/internal/logger"
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

// Backward compatibility - will be removed in future
var log = logger.New()

// logJSON is deprecated, use logger.Logger methods instead
func logJSON(level, msg string, err error, labels map[string]string) {
	logger.LogJSON(level, msg, err, labels)
}

func isLocal() bool {
	return os.Getenv("GO_ENV") == "local"
}

// isValidAPIKey performs basic validation on the YouTube API key format
func isValidAPIKey(apiKey string) bool {
	// YouTube API keys are typically 39 characters long and contain alphanumeric chars with dashes/underscores
	// This is a basic check - the actual validation happens when calling the API
	apiKey = strings.TrimSpace(apiKey)
	if len(apiKey) < 30 || len(apiKey) > 50 {
		return false
	}
	// Check for obviously invalid patterns
	if strings.Contains(apiKey, " ") || strings.Contains(apiKey, "\n") || strings.Contains(apiKey, "\t") {
		return false
	}
	// Basic pattern check (alphanumeric with dashes and underscores)
	matched, _ := regexp.MatchString(`^[A-Za-z0-9_-]+$`, apiKey)
	return matched
}

func getProjectID() (string, error) {
	if v := os.Getenv("PROJECT_ID"); v != "" {
		return v, nil
	}
	if v := os.Getenv("GOOGLE_CLOUD_PROJECT"); v != "" {
		return v, nil
	}
	// if metadata.OnGCE() {
	// 	return metadata.ProjectID()
	// }
	return "", fmt.Errorf("project ID not found")
}

func main() {
	if isLocal() {
		err := godotenv.Load()
		if err != nil {
			log.Warning("Error loading .env file", err, nil)
		}
	}

	http.HandleFunc("/", handler)
	http.HandleFunc("/healthz", healthzHandler)
	http.HandleFunc("/info", infoHandler)

	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}

	log.Info(fmt.Sprintf("Listening on port %s", port), nil)
	if err := http.ListenAndServe(":"+port, nil); err != nil {
		log.Fatal("Server failed to start", err, nil)
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

	// --- Configuration ---
	projectID := os.Getenv("GOOGLE_CLOUD_PROJECT")
	apiKey := os.Getenv("YOUTUBE_API_KEY")
	channelConfigPath := os.Getenv("CHANNEL_CONFIG_PATH")
	maxVideosPerChannelStr := os.Getenv("MAX_VIDEOS_PER_CHANNEL")

	if projectID == "" || apiKey == "" || channelConfigPath == "" {
		log.Error("Missing required environment variables (PROJECT_ID, YOUTUBE_API_KEY, CHANNEL_CONFIG_PATH)", nil, nil)
		http.Error(w, "Server configuration error", http.StatusInternalServerError)
		return
	}

	// Validate API key format (basic check)
	if !isValidAPIKey(apiKey) {
		log.Error("Invalid YouTube API key format", nil, nil)
		http.Error(w, "Invalid API key configuration", http.StatusInternalServerError)
		return
	}

	maxVideosPerChannel, err := strconv.ParseInt(maxVideosPerChannelStr, 10, 64)
	if err != nil || maxVideosPerChannel <= 0 {
		log.Warning(fmt.Sprintf("Invalid MAX_VIDEOS_PER_CHANNEL: %s. Using default 10.", maxVideosPerChannelStr), err, nil)
		maxVideosPerChannel = 10 // Default value
	}

	// Read channel config from file
	channelConfigBytes, err := os.ReadFile(channelConfigPath)
	if err != nil {
		log.Error("Error reading channel config file", err, nil)
		http.Error(w, "Invalid channel configuration file", http.StatusInternalServerError)
		return
	}

	var config Config
	if err := yaml.Unmarshal(channelConfigBytes, &config); err != nil {
		log.Error("Error parsing channel config", err, nil)
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
		log.Error("Error creating YouTube client", err, nil)
		http.Error(w, "Failed to create YouTube client", http.StatusInternalServerError)
		return
	}

	bqWriter, err := storage.NewBigQueryWriter(ctx, projectID)
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
	if err := f.FetchAndStore(ctx, channelIDs, maxVideosPerChannel); err != nil {
		log.Error("An error occurred during the fetch and store process", err, nil)
		http.Error(w, "An error occurred during the fetch and store process", http.StatusInternalServerError)
		return
	}

	// --- Response ---
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{"status": "success"})
}

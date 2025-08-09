package main

import (
	"context"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"os"
	"time"

	"github.com/joho/godotenv"
	"github.com/user/youtube-trend-tracker/internal/fetcher"
	"github.com/user/youtube-trend-tracker/internal/storage"
	"github.com/user/youtube-trend-tracker/internal/youtube"
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
	Timestamp string `json:"timestamp"`	
	Level     string `json:"level"`
	Message   string `json:"message"`
	Error     string `json:"error,omitempty"`
}

func logJSON(level, msg string, err error) {
	entry := logEntry{
		Timestamp: time.Now().Format(time.RFC3339),
		Level:     level,
		Message:   msg,
	}
	if err != nil {
		entry.Error = err.Error()
	}

	jsonBytes, _ := json.Marshal(entry)
	fmt.Println(string(jsonBytes)) // Use fmt.Println to ensure newline
}

func main() {
	// Load .env file
	err := godotenv.Load()
	if err != nil {
		logJSON("warning", "Error loading .env file", err)
		// Continue without .env if it's not found, assuming env vars are set elsewhere
	}

	http.HandleFunc("/", handler)

	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}

	logJSON("info", fmt.Sprintf("Listening on port %s", port), nil)
	if err := http.ListenAndServe(":"+port, nil); err != nil {
		logJSON("fatal", "Server failed to start", err)
		os.Exit(1) // Exit on fatal error
	}
}

func handler(w http.ResponseWriter, r *http.Request) {
	ctx := context.Background()

	// --- Configuration ---
	projectID := os.Getenv("PROJECT_ID")
	apiKey := os.Getenv("YOUTUBE_API_KEY")
	channelConfigPath := os.Getenv("CHANNEL_CONFIG_PATH")

	if projectID == "" || apiKey == "" || channelConfigPath == "" {
		logJSON("error", "Missing required environment variables (PROJECT_ID, YOUTUBE_API_KEY, CHANNEL_CONFIG_PATH)", nil)
		http.Error(w, "Server configuration error", http.StatusInternalServerError)
		return
	}

	// Read channel config from file
	channelConfigBytes, err := ioutil.ReadFile(channelConfigPath)
	if err != nil {
		logJSON("error", "Error reading channel config file", err)
		http.Error(w, "Invalid channel configuration file", http.StatusInternalServerError)
		return
	}

	var config Config
	if err := yaml.Unmarshal(channelConfigBytes, &config); err != nil {
		logJSON("error", "Error parsing channel config", err)
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
		logJSON("error", "Error creating YouTube client", err)
		http.Error(w, "Failed to create YouTube client", http.StatusInternalServerError)
		return
	}

	bqWriter, err := storage.NewBigQueryWriter(ctx, projectID)
	if err != nil {
		logJSON("error", "Error creating BigQuery writer", err)
		http.Error(w, "Failed to create BigQuery writer", http.StatusInternalServerError)
		return
	}

	// Ensure the table exists before proceeding.
	if err := bqWriter.EnsureTableExists(ctx); err != nil {
		logJSON("error", "Error ensuring BigQuery table exists", err)
		http.Error(w, "Failed to setup BigQuery table", http.StatusInternalServerError)
		return
	}

	// --- Execution ---
	f := fetcher.NewFetcher(ytClient, bqWriter)
	if err := f.FetchAndStore(ctx, channelIDs); err != nil {
		logJSON("error", "An error occurred during the fetch and store process", err)
		http.Error(w, "An error occurred during the fetch and store process", http.StatusInternalServerError)
		return
	}

	// --- Response ---
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{"status": "success"})
}
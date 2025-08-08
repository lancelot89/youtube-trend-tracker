package main

import (
	"context"
	"encoding/json"
	"io/ioutil"
	"log"
	"net/http"
	"os"

	"github.com/joho/godotenv" // Add this import
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

func main() {
	// Load .env file
	err := godotenv.Load()
	if err != nil {
		log.Printf("Error loading .env file: %v", err)
		// Continue without .env if it's not found, assuming env vars are set elsewhere
	}

	http.HandleFunc("/", handler)

	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}

	log.Printf("Listening on port %s", port)
	if err := http.ListenAndServe(":"+port, nil); err != nil {
		log.Fatal(err)
	}
}

func handler(w http.ResponseWriter, r *http.Request) {
	ctx := context.Background()

	// --- Configuration ---
	projectID := os.Getenv("PROJECT_ID")
	apiKey := os.Getenv("YOUTUBE_API_KEY")
	channelConfigPath := os.Getenv("CHANNEL_CONFIG_PATH")

	if projectID == "" || apiKey == "" || channelConfigPath == "" {
		log.Println("Error: Missing required environment variables (PROJECT_ID, YOUTUBE_API_KEY, CHANNEL_CONFIG_PATH)")
		http.Error(w, "Server configuration error", http.StatusInternalServerError)
		return
	}

	// Read channel config from file
	channelConfigBytes, err := ioutil.ReadFile(channelConfigPath)
	if err != nil {
		log.Printf("Error reading channel config file: %v", err)
		http.Error(w, "Invalid channel configuration file", http.StatusInternalServerError)
		return
	}

	var config Config
	if err := yaml.Unmarshal(channelConfigBytes, &config); err != nil {
		log.Printf("Error parsing channel config: %v", err)
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
		log.Printf("Error creating YouTube client: %v", err)
		http.Error(w, "Failed to create YouTube client", http.StatusInternalServerError)
		return
	}

	bqWriter, err := storage.NewBigQueryWriter(ctx, projectID)
	if err != nil {
		log.Printf("Error creating BigQuery writer: %v", err)
		http.Error(w, "Failed to create BigQuery writer", http.StatusInternalServerError)
		return
	}

	// Ensure the table exists before proceeding.
	if err := bqWriter.EnsureTableExists(ctx); err != nil {
		log.Printf("Error ensuring BigQuery table exists: %v", err)
		http.Error(w, "Failed to setup BigQuery table", http.StatusInternalServerError)
		return
	}

	// --- Execution ---
	f := fetcher.NewFetcher(ytClient, bqWriter)
	if err := f.FetchAndStore(ctx, channelIDs); err != nil {
		// Errors are logged within FetchAndStore, just return a generic server error.
		http.Error(w, "An error occurred during the fetch and store process", http.StatusInternalServerError)
		return
	}

	// --- Response ---
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{"status": "success"})
}
package fetcher

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/lancelop89/youtube-trend-tracker/internal/storage"
	"github.com/lancelop89/youtube-trend-tracker/internal/youtube"
)

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

// Fetcher orchestrates the data fetching and storing process.
type Fetcher struct {
	ytClient *youtube.Client
	bqWriter *storage.BigQueryWriter
}

// NewFetcher creates a new Fetcher.
func NewFetcher(ytClient *youtube.Client, bqWriter *storage.BigQueryWriter) *Fetcher {
	return &Fetcher{
		ytClient: ytClient,
		bqWriter: bqWriter,
	}
}

// FetchAndStore fetches video statistics from YouTube and stores them in BigQuery.
func (f *Fetcher) FetchAndStore(ctx context.Context, channelIDs []string, maxVideosPerChannel int64) error {
	logJSON("info", "Starting fetch and store process...", nil, nil)

	for _, channelID := range channelIDs {
		logJSON("info", fmt.Sprintf("Processing channel: %s", channelID), nil, map[string]string{"channel_id": channelID})

		// Use the unified FetchChannelVideos method
		videos, err := f.ytClient.FetchChannelVideos(ctx, channelID, maxVideosPerChannel) // Fetch latest N videos
		if err != nil {
			logJSON("error", fmt.Sprintf("Error fetching videos for channel %s", channelID), err, map[string]string{"channel_id": channelID})
			continue
		}

		var records []*storage.VideoStatsRecord
		for _, video := range videos {
			records = append(records, &storage.VideoStatsRecord{
				TS:          time.Now(),
				SnapshotDate: time.Now().Truncate(24 * time.Hour), // Set SnapshotDate
				ChannelID:   channelID, // Use the channelID from the loop
				VideoID:     video.ID,
				Title:       video.Title,
				Views:       int64(video.Views),
				Likes:       int64(video.Likes),
				Comments:    int64(video.Comments),
				PublishedAt: video.PublishedAt,
				InsertID:    fmt.Sprintf("%s-%s", video.ID, time.Now().Format("2006-01-02")),
			})
		}

		if err := f.bqWriter.InsertVideoStats(ctx, records); err != nil {
			logJSON("error", "Error inserting video stats to BigQuery", err, nil)
			continue
		}
		logJSON("info", fmt.Sprintf("Successfully stored %d records for channel %s", len(records), channelID), nil, map[string]string{"channel_id": channelID})
	}

	logJSON("info", "Fetch and store process completed.", nil, nil)
	return nil
}
package fetcher

import (
	"context"
	"log"
	"time"

	"github.com/user/youtube-trend-tracker/internal/storage"
	"github.com/user/youtube-trend-tracker/internal/youtube"
)

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
func (f *Fetcher) FetchAndStore(ctx context.Context, channelIDs []string) error {
	log.Println("Starting fetch and store process...")

	for _, channelID := range channelIDs {
		log.Printf("Processing channel: %s", channelID)

		// Use the unified FetchChannelVideos method
		videos, err := f.ytClient.FetchChannelVideos(ctx, channelID, 10) // Fetch latest 10 videos
		if err != nil {
			log.Printf("Error fetching videos for channel %s: %v", channelID, err)
			continue
		}

		var records []*storage.VideoStatsRecord
		for _, video := range videos {
			records = append(records, &storage.VideoStatsRecord{
				TS:          time.Now(),
				ChannelID:   channelID, // Use the channelID from the loop
				VideoID:     video.ID,
				Title:       video.Title,
				Views:       video.Views,
				Likes:       video.Likes,
				Comments:    video.Comments,
				PublishedAt: video.PublishedAt,
			})
		}

		if err := f.bqWriter.InsertVideoStats(ctx, records); err != nil {
			log.Printf("Error inserting video stats to BigQuery: %v", err)
			continue
		}
		log.Printf("Successfully stored %d records for channel %s", len(records), channelID)
	}

	log.Println("Fetch and store process completed.")
	return nil
}
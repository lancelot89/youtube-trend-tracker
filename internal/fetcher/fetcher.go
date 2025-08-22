package fetcher

import (
	"context"
	"fmt"
	"time"

	"cloud.google.com/go/civil"
	"github.com/lancelop89/youtube-trend-tracker/internal/logger"
	"github.com/lancelop89/youtube-trend-tracker/internal/storage"
	"github.com/lancelop89/youtube-trend-tracker/internal/youtube"
)

// Initialize logger
var log = logger.New()

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
	log.Info("Starting fetch and store process...", nil)

	for _, channelID := range channelIDs {
		log.Info(fmt.Sprintf("Processing channel: %s", channelID), map[string]string{"channel_id": channelID})

		// Use the unified FetchChannelVideos method
		videos, err := f.ytClient.FetchChannelVideos(ctx, channelID, maxVideosPerChannel) // Fetch latest N videos
		if err != nil {
			log.Error(fmt.Sprintf("Error fetching videos for channel %s", channelID), err, map[string]string{"channel_id": channelID})
			continue
		}

		var records []*storage.VideoStatsRecord
		for _, video := range videos {
			records = append(records, &storage.VideoStatsRecord{
				CreatedAt:      time.Now(),
				Dt:             todayJST(),
				ChannelID:      channelID,
				VideoID:        video.ID,
				Title:          video.Title,
				ChannelName:    video.ChannelName,
				Tags:           video.Tags,
				IsShort:        video.IsShort,
				Views:          int64(video.Views),
				Likes:          int64(video.Likes),
				Comments:       int64(video.Comments),
				PublishedAt:    video.PublishedAt,
				DurationSec:    video.DurationSec,
				ContentDetails: video.ContentDetails,
				TopicDetails:   video.TopicDetails,
			})
		}

		if err := f.bqWriter.InsertVideoStats(ctx, records); err != nil {
			log.Error("Error inserting video stats to BigQuery", err, nil)
			continue
		}
		log.Info(fmt.Sprintf("Successfully stored %d records for channel %s", len(records), channelID), map[string]string{"channel_id": channelID})
	}

	log.Info("Fetch and store process completed.", nil)
	return nil
}

func todayJST() civil.Date {
	t := time.Now()
	return civil.DateOf(t)
}

package fetcher

import (
	"context"
	"fmt"
	"time"

	"cloud.google.com/go/civil"
	"github.com/lancelop89/youtube-trend-tracker/internal/errors"
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

// FetchResult contains the result of a fetch operation
type FetchResult struct {
	SuccessfulChannels []string
	FailedChannels     map[string]error
	TotalVideos        int
}

// FetchAndStore fetches video statistics from YouTube and stores them in BigQuery.
func (f *Fetcher) FetchAndStore(ctx context.Context, channelIDs []string, maxVideosPerChannel int64) error {
	log.Info("Starting fetch and store process...", nil)

	result := &FetchResult{
		SuccessfulChannels: make([]string, 0),
		FailedChannels:     make(map[string]error),
	}

	for _, channelID := range channelIDs {
		log.Info(fmt.Sprintf("Processing channel: %s", channelID), map[string]string{"channel_id": channelID})

		// Use the unified FetchChannelVideos method
		videos, err := f.ytClient.FetchChannelVideos(ctx, channelID, maxVideosPerChannel) // Fetch latest N videos
		if err != nil {
			appErr := errors.API(fmt.Sprintf("Error fetching videos for channel %s", channelID), err)
			log.Error(appErr.Message, appErr, map[string]string{"channel_id": channelID})
			result.FailedChannels[channelID] = appErr
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
			appErr := errors.Storage("Error inserting video stats to BigQuery", err)
			log.Error(appErr.Message, appErr, map[string]string{"channel_id": channelID})
			result.FailedChannels[channelID] = appErr
			continue
		}

		result.SuccessfulChannels = append(result.SuccessfulChannels, channelID)
		result.TotalVideos += len(records)
		log.Info(fmt.Sprintf("Successfully stored %d records for channel %s", len(records), channelID), map[string]string{"channel_id": channelID})
	}

	// Log summary
	log.Info(fmt.Sprintf("Fetch and store process completed. Success: %d/%d channels, Total videos: %d",
		len(result.SuccessfulChannels), len(channelIDs), result.TotalVideos),
		map[string]string{
			"successful_channels": fmt.Sprintf("%d", len(result.SuccessfulChannels)),
			"failed_channels":     fmt.Sprintf("%d", len(result.FailedChannels)),
			"total_videos":        fmt.Sprintf("%d", result.TotalVideos),
		})

	// Return error if all channels failed
	if len(result.FailedChannels) == len(channelIDs) {
		return errors.New(errors.ErrTypeAPI, "All channels failed to process", nil)
	}

	return nil
}

func todayJST() civil.Date {
	t := time.Now()
	return civil.DateOf(t)
}

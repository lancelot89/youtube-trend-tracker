package youtube

import (
	"context"
	"os"
	"testing"
)

func TestNewClient(t *testing.T) {
	// This is a basic test.
	// TODO: Add more comprehensive tests.
	_, err := NewClient(context.Background(), "fake-api-key")
	if err != nil {
		t.Errorf("NewClient() error = %v, wantErr %v", err, false)
	}
}

// TestFetchChannelVideos requires a valid YouTube API key set in the YOUTUBE_API_KEY environment variable.
// This is an integration test and will be skipped if the API key is not provided.
func TestFetchChannelVideos_Integration(t *testing.T) {
	apiKey := os.Getenv("YOUTUBE_API_KEY")
	if apiKey == "" {
		t.Skip("Skipping integration test: YOUTUBE_API_KEY is not set")
	}

	// Google Developers channel ID
	channelID := "UC_x5XG1OV2P6uZZ5FSM9Ttw"

	ctx := context.Background()
	client, err := NewClient(ctx, apiKey)
	if err != nil {
		t.Fatalf("NewClient() error = %v", err)
	}

	videos, err := client.FetchChannelVideos(ctx, channelID, 5)
	if err != nil {
		t.Fatalf("FetchChannelVideos() error = %v", err)
	}

	if len(videos) == 0 {
		t.Errorf("FetchChannelVideos() returned 0 videos, expected at least one")
	}

	for _, video := range videos {
		if video.ID == "" {
			t.Error("Video ID is empty")
		}
		if video.Title == "" {
			t.Error("Video Title is empty")
		}
		t.Logf("Found video: %s (%s)", video.Title, video.ID)
	}
}

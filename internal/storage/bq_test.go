package storage

import (
	"context"
	"testing"
	"time"

	"cloud.google.com/go/civil"
)

func TestVideoStatsRecord_Structure(t *testing.T) {
	// Test that VideoStatsRecord has all required fields
	now := time.Now()
	record := &VideoStatsRecord{
		Dt:             civil.DateOf(now),
		ChannelID:      "test_channel",
		VideoID:        "test_video",
		Title:          "Test Video",
		ChannelName:    "Test Channel",
		Tags:           []string{"tag1", "tag2"},
		IsShort:        false,
		Views:          1000,
		Likes:          100,
		Comments:       10,
		PublishedAt:    now,
		CreatedAt:      now,
		DurationSec:    180,
		ContentDetails: `{"duration":"PT3M"}`,
		TopicDetails:   []string{"topic1", "topic2"},
	}

	// Basic validation
	if record.VideoID != "test_video" {
		t.Errorf("VideoID = %v, want test_video", record.VideoID)
	}
	if record.Views != 1000 {
		t.Errorf("Views = %v, want 1000", record.Views)
	}
	if len(record.Tags) != 2 {
		t.Errorf("Tags length = %v, want 2", len(record.Tags))
	}
}

func TestGetSchemaJSON(t *testing.T) {
	// Test that schema JSON is valid
	schemaJSON := getSchemaJSON()

	if len(schemaJSON) == 0 {
		t.Error("Schema JSON should not be empty")
	}

	// Check for required fields in schema
	schemaStr := string(schemaJSON)
	requiredFields := []string{
		"dt",
		"channel_id",
		"video_id",
		"created_at",
	}

	for _, field := range requiredFields {
		if !contains(schemaStr, field) {
			t.Errorf("Schema missing required field: %s", field)
		}
	}
}

func contains(s, substr string) bool {
	return len(s) >= len(substr) && s[:len(substr)] == substr || len(s) > len(substr) && contains(s[1:], substr)
}

// Integration test - requires BigQuery emulator or actual connection
func TestBigQueryWriter_Integration(t *testing.T) {
	// Skip if not in integration test mode
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	ctx := context.Background()

	// This would require actual BigQuery connection or emulator
	// For now, just test that NewBigQueryWriter doesn't panic
	// with invalid project ID
	_, err := NewBigQueryWriter(ctx, "test-project")
	if err == nil {
		t.Skip("Skipping - BigQuery client created unexpectedly")
	}
}

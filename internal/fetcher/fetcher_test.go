package fetcher

import (
	"testing"
	"time"

	"cloud.google.com/go/civil"
)

// Mock implementations are commented out until dependency injection is refactored
// These will be needed when Fetcher is updated to accept interfaces

// // Mock YouTube Client
// type mockYouTubeClient struct {
// 	videos []*youtube.Video
// 	err    error
// }

// func (m *mockYouTubeClient) FetchChannelVideos(ctx context.Context, channelID string, maxResults int64) ([]*youtube.Video, error) {
// 	if m.err != nil {
// 		return nil, m.err
// 	}
// 	return m.videos, nil
// }

// // Mock BigQuery Writer
// type mockBigQueryWriter struct {
// 	insertedRecords []*storage.VideoStatsRecord
// 	err             error
// }

// func (m *mockBigQueryWriter) InsertVideoStats(ctx context.Context, records []*storage.VideoStatsRecord) error {
// 	if m.err != nil {
// 		return m.err
// 	}
// 	m.insertedRecords = append(m.insertedRecords, records...)
// 	return nil
// }

// func (m *mockBigQueryWriter) EnsureTableExists(ctx context.Context) error {
// 	return nil
// }

func TestFetchAndStore_Success(t *testing.T) {
	// This test demonstrates the need for dependency injection
	// Currently, the Fetcher is tightly coupled with concrete implementations
	// Making it difficult to unit test without actual YouTube/BigQuery connections

	// TODO: Refactor Fetcher to accept interfaces instead of concrete types
	// This would allow proper mocking and unit testing
	t.Skip("Skipping - requires refactoring for dependency injection")
}

func TestFetchAndStore_PartialFailure(t *testing.T) {
	// Test when some channels succeed and others fail
	// This would require proper mocking support
}

func TestFetchAndStore_AllChannelsFail(t *testing.T) {
	// Test when all channels fail to fetch
	// This would require proper mocking support
}

func TestTodayJST(t *testing.T) {
	// Test the todayJST function
	result := todayJST()

	// Should return today's date in JST
	now := time.Now()
	expected := civil.DateOf(now)

	if result != expected {
		t.Errorf("todayJST() = %v, want %v", result, expected)
	}
}

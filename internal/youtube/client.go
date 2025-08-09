	"github.com/lancelop89/youtube-trend-tracker/internal/youtube"

import (
	"context"
	"fmt"
	"time"

	"google.golang.org/api/googleapi"
	"google.golang.org/api/option"
	yt "google.golang.org/api/youtube/v3"
)

type Client struct {
	service *yt.Service
}

type Video struct {
	ID          string
	Title       string
	Views       uint64
	Likes       uint64
	Comments    uint64
	PublishedAt time.Time
}

func NewClient(ctx context.Context, apiKey string) (*Client, error) {
	svc, err := yt.NewService(ctx, option.WithAPIKey(apiKey))
	if err != nil {
		return nil, fmt.Errorf("youtube.NewService: %w", err)
	}
	return &Client{service: svc}, nil
}

// FetchChannelVideos returns latest N videos with snippet/statistics.
func (c *Client) FetchChannelVideos(ctx context.Context, channelID string, maxResults int64) ([]*Video, error) {
	ch, err := c.service.Channels.List([]string{"contentDetails"}).Id(channelID).Do()
	if err != nil || len(ch.Items) == 0 {
		return nil, fmt.Errorf("channels.list: %w", err)
	}
	uploads := ch.Items[0].ContentDetails.RelatedPlaylists.Uploads

	var allVideoIDs []string
	nextPageToken := ""

	for {
		itCall := c.service.PlaylistItems.List([]string{"contentDetails"}).PlaylistId(uploads).MaxResults(maxResults)
		if nextPageToken != "" {
			itCall = itCall.PageToken(nextPageToken)
		}
		var itResp *yt.PlaylistItemListResponse
		var err error
		for i := 0; i < 5; i++ { // Max 5 retries
			itResp, err = itCall.Do()
			if err == nil {
				break
			}
			// Check if it's a retriable error (e.g., 429 Too Many Requests, 5xx Server Error)
			if e, ok := err.(*googleapi.Error); ok && (e.Code == 429 || (e.Code >= 500 && e.Code < 600)) {
				sleepTime := time.Duration(1<<uint(i)) * time.Second // Exponential backoff
				time.Sleep(sleepTime)
				continue
			}
			return nil, fmt.Errorf("playlistItems.list: %w", err) // Non-retriable error
		}
		if err != nil {
			return nil, fmt.Errorf("playlistItems.list: %w", err) // All retries failed
		}

		for _, it := range itResp.Items {
			allVideoIDs = append(allVideoIDs, it.ContentDetails.VideoId)
		}

		nextPageToken = itResp.NextPageToken
		if nextPageToken == "" || int64(len(allVideoIDs)) >= maxResults {
			break
		}
	}

	if len(allVideoIDs) == 0 {
		return nil, nil
	}

	// Fetch video details in batches of 50 (YouTube API limit)
	var allVideos []*Video
	for i := 0; i < len(allVideoIDs); i += 50 {
		end := i + 50
		if end > len(allVideoIDs) {
			end = len(allVideoIDs)
		}
		batchIDs := allVideoIDs[i:end]

		var vResp *yt.VideoListResponse
		var err error
		for i := 0; i < 5; i++ { // Max 5 retries
			vResp, err = c.service.Videos.List([]string{"snippet", "statistics"}).Id(batchIDs...).Do()
			if err == nil {
				break
			}
			// Check if it's a retriable error
			if e, ok := err.(*googleapi.Error); ok && (e.Code == 429 || (e.Code >= 500 && e.Code < 600)) {
				sleepTime := time.Duration(1<<uint(i)) * time.Second // Exponential backoff
				time.Sleep(sleepTime)
				continue
			}
			return nil, fmt.Errorf("videos.list: %w", err) // Non-retriable error
		}
		if err != nil {
			return nil, fmt.Errorf("videos.list: %w", err) // All retries failed
		}

		for _, item := range vResp.Items {
			var views, likes, comments uint64
			if item.Statistics != nil {
				views = item.Statistics.ViewCount
				likes = item.Statistics.LikeCount
				comments = item.Statistics.CommentCount
			}
			pub, _ := time.Parse(time.RFC3339, item.Snippet.PublishedAt)
			allVideos = append(allVideos, &Video{ID: item.Id, Title: item.Snippet.Title, Views: views, Likes: likes, Comments: comments, PublishedAt: pub})
		}
	}
	return allVideos, nil
}

package youtube

import (
	"context"
	"fmt"
	"time"

	"google.golang.org/api/option"
	"google.golang.org/api/youtube/v3"
)

// Client provides a wrapper around the YouTube Data API.
type Client struct {
	service *youtube.Service
}

// Video represents a simplified YouTube video resource.
type Video struct {
	ID          string
	Title       string
	Views       uint64
	Likes       uint64
	Comments    uint64
	PublishedAt time.Time
}

// NewClient creates a new YouTube API client.
func NewClient(ctx context.Context, apiKey string) (*Client, error) {
	service, err := youtube.NewService(ctx, option.WithAPIKey(apiKey))
	if err != nil {
		return nil, fmt.Errorf("youtube.NewService: %w", err)
	}
	return &Client{service: service}, nil
}

// FetchChannelVideos fetches the most recent videos for a given channel ID.
func (c *Client) FetchChannelVideos(ctx context.Context, channelID string, maxResults int64) ([]*Video, error) {
	// 1. Find the playlist ID for the channel's uploads.
	channelCall := c.service.Channels.List([]string{"contentDetails"}).Id(channelID)
	channelResponse, err := channelCall.Do()
	if err != nil {
		return nil, fmt.Errorf("failed to get channel details: %w", err)
	}
	if len(channelResponse.Items) == 0 {
		return nil, fmt.Errorf("channel not found: %s", channelID)
	}
	uploadsPlaylistID := channelResponse.Items[0].ContentDetails.RelatedPlaylists.Uploads

	// 2. Fetch the latest videos from the uploads playlist.
	playlistCall := c.service.PlaylistItems.List([]string{"snippet"}).
		PlaylistId(uploadsPlaylistID).
		MaxResults(maxResults)

	playlistResponse, err := playlistCall.Do()
	if err != nil {
		return nil, fmt.Errorf("failed to get playlist items: %w", err)
	}

	var videoIDs []string
	for _, item := range playlistResponse.Items {
		videoIDs = append(videoIDs, item.Snippet.ResourceId.VideoId)
	}

	// 3. Fetch detailed statistics for each video.
	videoCall := c.service.Videos.List([]string{"snippet", "statistics"}).Id(videoIDs...)
	videoResponse, err := videoCall.Do()
	if err != nil {
		return nil, fmt.Errorf("failed to get video details: %w", err)
	}

	var videos []*Video
	for _, item := range videoResponse.Items {
		publishedAt, _ := time.Parse(time.RFC3339, item.Snippet.PublishedAt)
		video := &Video{
			ID:          item.Id,
			Title:       item.Snippet.Title,
			Views:       item.Statistics.ViewCount,
			Likes:       item.Statistics.LikeCount,
			Comments:    item.Statistics.CommentCount,
			PublishedAt: publishedAt,
		}
		videos = append(videos, video)
	}

	return videos, nil
}

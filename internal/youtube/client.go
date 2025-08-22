package youtube

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/lancelop89/youtube-trend-tracker/internal/errors"
	"github.com/lancelop89/youtube-trend-tracker/internal/retry"
	"google.golang.org/api/googleapi"
	"google.golang.org/api/option"
	yt "google.golang.org/api/youtube/v3"
)

type Client struct {
	service *yt.Service
}

type Video struct {
	ID             string
	Title          string
	ChannelName    string
	Tags           []string
	IsShort        bool
	Views          uint64
	Likes          uint64
	Comments       uint64
	PublishedAt    time.Time
	DurationSec    int64
	ContentDetails string
	TopicDetails   []string
}

func NewClient(ctx context.Context, apiKey string) (*Client, error) {
	svc, err := yt.NewService(ctx, option.WithAPIKey(apiKey))
	if err != nil {
		return nil, fmt.Errorf("youtube.NewService: %w", err)
	}
	return &Client{service: svc}, nil
}

// parseISODuration converts a YouTube ISO 8601 duration (e.g., "PT1M30S") into a time.Duration.
func parseISODuration(isoDuration string) (time.Duration, error) {
	// Go's time.ParseDuration doesn't support the "P" or "T" prefixes of ISO 8601.
	// It also requires lowercase unit specifiers (h, m, s).
	s := strings.TrimPrefix(isoDuration, "P")
	replacer := strings.NewReplacer("T", "", "H", "h", "M", "m", "S", "s")
	s = replacer.Replace(s)
	return time.ParseDuration(s)
}

// FetchChannelVideos returns latest N videos with snippet/statistics.
func (c *Client) FetchChannelVideos(ctx context.Context, channelID string, maxResults int64) ([]*Video, error) {
	ch, err := c.service.Channels.List([]string{"contentDetails", "snippet"}).Id(channelID).Do()
	if err != nil || len(ch.Items) == 0 {
		return nil, fmt.Errorf("channels.list: %w", err)
	}
	channelName := ch.Items[0].Snippet.Title
	uploads := ch.Items[0].ContentDetails.RelatedPlaylists.Uploads

	var allVideoIDs []string
	nextPageToken := ""

	for {
		itCall := c.service.PlaylistItems.List([]string{"contentDetails"}).PlaylistId(uploads).MaxResults(maxResults)
		if nextPageToken != "" {
			itCall = itCall.PageToken(nextPageToken)
		}
		
		var itResp *yt.PlaylistItemListResponse
		err := retry.Do(func() error {
			var apiErr error
			itResp, apiErr = itCall.Do()
			if apiErr != nil {
				if e, ok := apiErr.(*googleapi.Error); ok {
					if e.Code == 429 || (e.Code >= 500 && e.Code < 600) {
						return errors.Temporary("YouTube API temporary error", apiErr)
					}
					return errors.API("YouTube API error", apiErr)
				}
				return apiErr
			}
			return nil
		}, retry.DefaultConfig())
		
		if err != nil {
			return nil, fmt.Errorf("playlistItems.list: %w", err)
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

	var allVideos []*Video
	for i := 0; i < len(allVideoIDs); i += 50 {
		end := i + 50
		if end > len(allVideoIDs) {
			end = len(allVideoIDs)
		}
		batchIDs := allVideoIDs[i:end]

		var vResp *yt.VideoListResponse
		err := retry.Do(func() error {
			var apiErr error
			vResp, apiErr = c.service.Videos.List([]string{"snippet", "statistics", "contentDetails", "topicDetails"}).Id(batchIDs...).Do()
			if apiErr != nil {
				if e, ok := apiErr.(*googleapi.Error); ok {
					if e.Code == 429 || (e.Code >= 500 && e.Code < 600) {
						return errors.Temporary("YouTube API temporary error", apiErr)
					}
					return errors.API("YouTube API error", apiErr)
				}
				return apiErr
			}
			return nil
		}, retry.DefaultConfig())
		
		if err != nil {
			return nil, fmt.Errorf("videos.list: %w", err)
		}

		for _, item := range vResp.Items {
			var views, likes, comments uint64
			if item.Statistics != nil {
				views = item.Statistics.ViewCount
				likes = item.Statistics.LikeCount
				comments = item.Statistics.CommentCount
			}
			pub, _ := time.Parse(time.RFC3339, item.Snippet.PublishedAt)

			var durationSec int64
			var isShort bool
			var contentDetailsJSON string
			if item.ContentDetails != nil {
				duration, err := parseISODuration(item.ContentDetails.Duration)
				if err == nil {
					durationSec = int64(duration.Seconds())
					if duration <= 60*time.Second {
						isShort = true
					}
				}

				cd, err := json.Marshal(item.ContentDetails)
				if err == nil {
					contentDetailsJSON = string(cd)
				}
			}

			var topicDetails []string
			if item.TopicDetails != nil {
				topicDetails = item.TopicDetails.TopicCategories
			}

			allVideos = append(allVideos, &Video{
				ID:             item.Id,
				Title:          item.Snippet.Title,
				ChannelName:    channelName,
				Tags:           item.Snippet.Tags,
				IsShort:        isShort,
				Views:          views,
				Likes:          likes,
				Comments:       comments,
				PublishedAt:    pub,
				DurationSec:    durationSec,
				ContentDetails: contentDetailsJSON,
				TopicDetails:   topicDetails,
			})
		}
	}
	return allVideos, nil
}
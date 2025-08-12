package storage

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"time"

	"cloud.google.com/go/bigquery"
	"cloud.google.com/go/civil"
	"google.golang.org/api/googleapi"
	"google.golang.org/api/option"
)

const (
	DatasetID = "youtube"
	TableID   = "video_trends"
)

// BigQueryWriter provides methods to write data to BigQuery.
type BigQueryWriter struct {
	client *bigquery.Client
}

// VideoStatsRecord represents a record to be inserted into BigQuery.
type VideoStatsRecord struct {
	Dt             civil.Date `bigquery:"dt"`
	ChannelID      string     `bigquery:"channel_id"`
	VideoID        string     `bigquery:"video_id"`
	Title          string     `bigquery:"title"`
	ChannelName    string     `bigquery:"channel_name"`
	Tags           []string   `bigquery:"tags"`
	IsShort        bool       `bigquery:"is_short"`
	Views          int64      `bigquery:"views"`
	Likes          int64      `bigquery:"likes"`
	Comments       int64      `bigquery:"comments"`
	PublishedAt    time.Time  `bigquery:"published_at"`
	CreatedAt      time.Time  `bigquery:"created_at"`
	DurationSec    int64      `bigquery:"duration_sec"`
	ContentDetails string     `bigquery:"content_details"`
	TopicDetails   []string   `bigquery:"topic_details"`
}

// EnsureTableExists checks if the dataset and table exist, and creates them if they don't.
func (w *BigQueryWriter) EnsureTableExists(ctx context.Context) error {
	_, err := w.client.Dataset(DatasetID).Metadata(ctx)
	if err != nil {
		if e, ok := err.(*googleapi.Error); ok && e.Code == http.StatusNotFound {
			// Dataset doesn't exist, create it.
			if err := w.client.Dataset(DatasetID).Create(ctx, &bigquery.DatasetMetadata{}); err != nil {
				return fmt.Errorf("failed to create dataset: %w", err)
			}
		} else {
			return fmt.Errorf("failed to get dataset metadata: %w", err)
		}
	}

	table := w.client.Dataset(DatasetID).Table(TableID)
	if _, err := table.Metadata(ctx); err != nil {
		if e, ok := err.(*googleapi.Error); ok && e.Code == http.StatusNotFound {
			// Table doesn't exist, create it.
			schema, err := bigquery.SchemaFromJSON(getSchemaJSON())
			if err != nil {
				return fmt.Errorf("failed to load schema: %w", err)
			}
			tableMetadata := &bigquery.TableMetadata{
				Schema: schema,
				TimePartitioning: &bigquery.TimePartitioning{
					Field:      "dt",
					Type:       "DAY",
					Expiration: 0, // No expiration
				},
				Clustering: &bigquery.Clustering{
					Fields: []string{"channel_id", "video_id"},
				},
			}
			if err := table.Create(ctx, tableMetadata); err != nil {
				return fmt.Errorf("failed to create table: %w", err)
			}
		} else {
			return fmt.Errorf("failed to get table metadata: %w", err)
		}
	}
	return nil
}

func getSchemaJSON() []byte {
	// In a real application, you would load this from a file.
	// For simplicity here, it's embedded.
	return []byte(`[
	  {"name": "dt",               "type": "DATE",      "mode": "REQUIRED"},
	  {"name": "channel_id",       "type": "STRING",    "mode": "REQUIRED"},
	  {"name": "video_id",         "type": "STRING",    "mode": "REQUIRED"},
	  {"name": "title",            "type": "STRING",    "mode": "NULLABLE"},
	  {"name": "channel_name",     "type": "STRING",    "mode": "NULLABLE"},
	  {"name": "tags",             "type": "STRING",    "mode": "REPEATED"},
	  {"name": "is_short",         "type": "BOOLEAN",   "mode": "NULLABLE"},
	  {"name": "views",            "type": "INTEGER",   "mode": "NULLABLE"},
	  {"name": "likes",            "type": "INTEGER",   "mode": "NULLABLE"},
	  {"name": "comments",         "type": "INTEGER",   "mode": "NULLABLE"},
	  {"name": "published_at",     "type": "TIMESTAMP", "mode": "NULLABLE"},
	  {"name": "created_at",       "type": "TIMESTAMP", "mode": "REQUIRED"},
	  {"name": "duration_sec",     "type": "INTEGER",   "mode": "NULLABLE"},
	  {"name": "content_details",  "type": "STRING",    "mode": "NULLABLE"},
	  {"name": "topic_details",    "type": "STRING",    "mode": "REPEATED"}
	]`)
}

// NewBigQueryWriter creates a new BigQuery writer.
func NewBigQueryWriter(ctx context.Context, projectID string) (*BigQueryWriter, error) {
	var opts []option.ClientOption
	if host := os.Getenv("BIGQUERY_EMULATOR_HOST"); host != "" {
		// For connecting to the emulator's HTTP endpoint
		endpoint := "http://" + host // Use HTTP for the REST API
		opts = append(opts, option.WithEndpoint(endpoint))
		opts = append(opts, option.WithoutAuthentication())
	}

	client, err := bigquery.NewClient(ctx, projectID, opts...)
	if err != nil {
		return nil, fmt.Errorf("bigquery.NewClient: %w", err)
	}
	return &BigQueryWriter{client: client}, nil
}

// InsertVideoStats inserts video statistics into the BigQuery table.
func (w *BigQueryWriter) InsertVideoStats(ctx context.Context, records []*VideoStatsRecord) error {
	if len(records) == 0 {
		return nil // No records to insert
	}

	inserter := w.client.Dataset(DatasetID).Table(TableID).Inserter()
	if err := inserter.Put(ctx, records); err != nil {
		return fmt.Errorf("failed to insert records into BigQuery: %w", err)
	}

	return nil
}

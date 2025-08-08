package storage

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"time"

	"cloud.google.com/go/bigquery"
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
	TS          time.Time `bigquery:"ts"`
	ChannelID   string    `bigquery:"channel_id"`
	VideoID     string    `bigquery:"video_id"`
	Title       string    `bigquery:"title"`
	Views       uint64    `bigquery:"views"` // Changed from int64 to uint64
	Likes       uint64    `bigquery:"likes"` // Changed from int64 to uint64
	Comments    uint64    `bigquery:"comments"` // Changed from int64 to uint64
	PublishedAt time.Time `bigquery:"published_at"`
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
			if err := table.Create(ctx, &bigquery.TableMetadata{Schema: schema}); err != nil {
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
	  {"name": "ts",           "type": "TIMESTAMP", "mode": "REQUIRED"},
	  {"name": "channel_id",   "type": "STRING",    "mode": "REQUIRED"},
	  {"name": "video_id",     "type": "STRING",    "mode": "REQUIRED"},
	  {"name": "title",        "type": "STRING",    "mode": "NULLABLE"},
	  {"name": "views",        "type": "INTEGER",   "mode": "NULLABLE"},
	  {"name": "likes",        "type": "INTEGER",   "mode": "NULLABLE"},
	  {"name": "comments",     "type": "INTEGER",   "mode": "NULLABLE"},
	  {"name": "published_at", "type": "TIMESTAMP", "mode": "NULLABLE"}
	]`)
}

// NewBigQueryWriter creates a new BigQuery writer.
func NewBigQueryWriter(ctx context.Context, projectID string) (*BigQueryWriter, error) {
	var opts []option.ClientOption
	if host := os.Getenv("BIGQUERY_EMULATOR_HOST"); host != "" {
		// The BigQuery emulator requires a specific endpoint format.
		// Note: The goccy/bigquery-emulator uses gRPC by default on port 9060.
		// The endpoint needs to be set without the http/https scheme.
		opts = append(opts, option.WithEndpoint(host))
		// For emulators, we often don't need actual authentication.
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
	inserter := w.client.Dataset(DatasetID).Table(TableID).Inserter()
	if err := inserter.Put(ctx, records); err != nil {
		return fmt.Errorf("failed to insert records into BigQuery: %w", err)
	}
	return nil
}
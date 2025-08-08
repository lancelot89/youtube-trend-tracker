#!/bin/bash
# This script is for local development and testing.
# It runs the fetcher service locally with a BigQuery emulator.

set -e

# --- Configuration ---
PROJECT_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
export GCP_PROJECT="test-project"
# Use host.docker.internal to connect from the host to the container on Windows/Mac
export BIGQUERY_EMULATOR_HOST="host.docker.internal:9060"
BIGQUERY_EMULATOR_API_HOST="http://host.docker.internal:9050"
export PORT=8080
export YOUTUBE_API_KEY="your-api-key"
DATASET_NAME="youtube"
TABLE_NAME="video_trends"
SCHEMA_FILE="$PROJECT_ROOT/deployments/bq/schema.json"

# --- Start Emulator ---
echo "Starting BigQuery emulator..."
cd "$PROJECT_ROOT"
docker-compose up -d bq-emulator

# --- Cleanup ---
trap 'echo "Stopping BigQuery emulator..."; docker-compose down' EXIT

# --- Wait for Emulator ---
echo "Waiting for BigQuery emulator to be ready..."
timeout 60s bash -c "until bq --project_id=$GCP_PROJECT --api=$BIGQUERY_EMULATOR_API_HOST ls > /dev/null 2>&1; do echo 'Waiting...'; sleep 2; done"
echo "Emulator is ready!"

# --- Setup BigQuery Resources ---
echo "Checking for BigQuery dataset: $DATASET_NAME"
if ! bq --project_id=$GCP_PROJECT --api=$BIGQUERY_EMULATOR_API_HOST ls --format=prettyjson | grep -q '"datasetId": "$DATASET_NAME"'; then
  echo "Creating BigQuery dataset: $DATASET_NAME"
  bq --project_id=$GCP_PROJECT --api=$BIGQUERY_EMULATOR_API_HOST mk --dataset $DATASET_NAME
else
  echo "Dataset '$DATASET_NAME' already exists."
fi

echo "Checking for BigQuery table: $TABLE_NAME"
if ! bq --project_id=$GCP_PROJECT --api=$BIGQUERY_EMULATOR_API_HOST ls --format=prettyjson "$DATASET_NAME" | grep -q '"tableId": "$TABLE_NAME"'; then
  echo "Creating BigQuery table: $TABLE_NAME"
  bq --project_id=$GCP_PROJECT --api=$BIGQUERY_EMULATOR_API_HOST mk --table "$DATASET_NAME.$TABLE_NAME" "$SCHEMA_FILE"
else
  echo "Table '$TABLE_NAME' already exists."
fi

# --- Run Application ---
echo "Starting Go application..."
cd "$PROJECT_ROOT/cmd/fetcher"
export CHANNEL_CONFIG=$(cat "$PROJECT_ROOT/configs/channels.yaml")

go run ./main.go

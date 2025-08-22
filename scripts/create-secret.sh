#!/bin/bash
set -eu

SECRET_NAME="youtube-api-key"
PROJECT_ID=$(gcloud config get-value project)

if [ -z "${YOUTUBE_API_KEY:-}" ]; then
    echo "Error: YOUTUBE_API_KEY environment variable is not set."
    echo "Please set it to your YouTube Data API v3 key."
    exit 1
fi

# Check if the secret already exists
if gcloud secrets describe "$SECRET_NAME" --project="$PROJECT_ID" >/dev/null 2>&1; then
    echo "Secret '$SECRET_NAME' already exists. Skipping creation."
else
    echo "Creating secret '$SECRET_NAME'..."
    echo -n "$YOUTUBE_API_KEY" | gcloud secrets create "$SECRET_NAME" \
        --project="$PROJECT_ID" \
        --replication-policy="automatic" \
        --data-file=-
    echo "Secret '$SECRET_NAME' created successfully."
fi

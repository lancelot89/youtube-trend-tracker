#!/bin/bash
set -eu

if [ -z "$4" ]; then
    echo "Usage: $0 <project_id> <region> <service_name> <repo_name>"
    exit 1
fi

PROJECT_ID=$1
REGION=$2
SERVICE=$3
AR_REPO=$4

IMAGE_URI="${REGION}-docker.pkg.dev/${PROJECT_ID}/${AR_REPO}/${SERVICE}:latest"
SERVICE_ACCOUNT="trend-tracker-sa@${PROJECT_ID}.iam.gserviceaccount.com"

echo "Deploying Cloud Run service: $SERVICE..."

# Create service account (if it doesn't exist)
gcloud iam service-accounts describe "$SERVICE_ACCOUNT" --project="$PROJECT_ID" >/dev/null 2>&1 || \
    gcloud iam service-accounts create trend-tracker-sa \
        --display-name="YouTube Trend Tracker Service Account" \
        --project="$PROJECT_ID"

# Grant necessary roles to the service account
gcloud projects add-iam-policy-binding "$PROJECT_ID" \
    --member="serviceAccount:$SERVICE_ACCOUNT" \
    --role="roles/run.invoker" >/dev/null
gcloud projects add-iam-policy-binding "$PROJECT_ID" \
    --member="serviceAccount:$SERVICE_ACCOUNT" \
    --role="roles/bigquery.dataEditor" >/dev/null

# Deploy to Cloud Run
gcloud run deploy "$SERVICE" \
    --image="$IMAGE_URI" \
    --service-account="$SERVICE_ACCOUNT" \
    --set-secrets="YOUTUBE_API_KEY=youtube-api-key:latest" \
    --region="$REGION" \
    --platform=managed \
    --allow-unauthenticated

echo "Cloud Run deployment complete."


#!/bin/bash
set -euo pipefail

if [ $# -lt 4 ]; then
  echo "Usage: $0 <project_id> <region> <repo_name> <service_name>"
  exit 1
fi

PROJECT_ID="$1"
REGION="$2"             # 例: asia-northeast1
AR_REPO="$3"            # 例: youtube-trend-repo
SERVICE="$4"            # 例: youtube-trend-tracker

TAG="$(date +%Y%m%d-%H%M)"
IMAGE_URI="${REGION}-docker.pkg.dev/${PROJECT_ID}/${AR_REPO}/${SERVICE}:${TAG}"
SERVICE_ACCOUNT="trend-tracker-sa@${PROJECT_ID}.iam.gserviceaccount.com"

gcloud config set project "$PROJECT_ID" >/dev/null

# APIs
gcloud services enable run.googleapis.com artifactregistry.googleapis.com secretmanager.googleapis.com bigquery.googleapis.com >/dev/null

# Authenticate Docker to Artifact Registry
gcloud auth configure-docker "${REGION}-docker.pkg.dev" --project="$PROJECT_ID"

# Artifact Registry repo（なければ作成）
gcloud artifacts repositories describe "$AR_REPO" --location="$REGION" >/dev/null 2>&1 || \
  gcloud artifacts repositories create "$AR_REPO" --repository-format=docker --location="$REGION"

# Build & Push
docker build -t "$IMAGE_URI" .
docker push "$IMAGE_URI"

gcloud run deploy "$SERVICE" \
  --image "$IMAGE_URI" \
  --region "$REGION" \
  --project "$PROJECT_ID" \
  --service-account "$SERVICE_ACCOUNT" \
  --set-secrets YOUTUBE_API_KEY=youtube-api-key:latest \
  --set-env-vars GOOGLE_CLOUD_PROJECT="${PROJECT_ID}",MAX_VIDEOS_PER_CHANNEL=200 \
  --no-allow-unauthenticated \
  --port 8080 \
  --memory 512Mi \
  --cpu 1 \
  --max-instances 10 \
  --timeout 300

echo -e "\n✅ Redeployment complete!"


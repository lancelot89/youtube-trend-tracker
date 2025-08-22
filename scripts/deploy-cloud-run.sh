#!/bin/bash
set -euo pipefail

if [ $# -lt 4 ]; then
  echo "Usage: $0 <project_id> <region> <service_name> <repo_name>"
  exit 1
fi

PROJECT_ID="$1"
REGION="$2"             # 例: asia-northeast1
SERVICE="$3"            # 例: fetcher
AR_REPO="$4"            # 例: trend-tracker-repo

TAG="$(date +%Y%m%d-%H%M)"
IMAGE_URI="${REGION}-docker.pkg.dev/${PROJECT_ID}/${AR_REPO}/${SERVICE}:${TAG}"
SERVICE_ACCOUNT="trend-tracker-sa@${PROJECT_ID}.iam.gserviceaccount.com"

gcloud config set project "$PROJECT_ID" >/dev/null

# APIs
gcloud services enable run.googleapis.com artifactregistry.googleapis.com secretmanager.googleapis.com bigquery.googleapis.com >/dev/null

# Artifact Registry repo（なければ作成）
gcloud artifacts repositories describe "$AR_REPO" --location="$REGION" >/dev/null 2>&1 || \
  gcloud artifacts repositories create "$AR_REPO" --repository-format=docker --location="$REGION"

# Build & Push
docker build -t "$IMAGE_URI" .
docker push "$IMAGE_URI"

# Service Account（なければ作成）
gcloud iam service-accounts describe "$SERVICE_ACCOUNT" >/dev/null 2>&1 || \
  gcloud iam service-accounts create "trend-tracker-sa" \
    --display-name="YouTube Trend Tracker SA"

# ランタイム SA に必要な権限
# - Artifact Registry Reader（プライベートイメージをPull）
gcloud projects add-iam-policy-binding "$PROJECT_ID" \
  --member="serviceAccount:${SERVICE_ACCOUNT}" \
  --role="roles/artifactregistry.reader" >/dev/null

# - BigQuery 書き込み
gcloud projects add-iam-policy-binding "$PROJECT_ID" \
  --member="serviceAccount:${SERVICE_ACCOUNT}" \
  --role="roles/bigquery.dataEditor" >/dev/null

# - Secret Manager 参照
gcloud secrets add-iam-policy-binding youtube-api-key \
  --member="serviceAccount:${SERVICE_ACCOUNT}" \
  --role="roles/secretmanager.secretAccessor" \
  --project="$PROJECT_ID" >/dev/null

# Deploy to Cloud Run
gcloud run deploy "$SERVICE" \
  --image "$IMAGE_URI" \
  --region "$REGION" \
  --service-account "$SERVICE_ACCOUNT" \
  --set-secrets YOUTUBE_API_KEY=youtube-api-key:latest \
  --set-env-vars GOOGLE_CLOUD_PROJECT="${PROJECT_ID}" \
  --no-allow-unauthenticated

# URL 確認
SERVICE_URL=$(gcloud run services describe "$SERVICE" --region "$REGION" --format='value(status.url)')
echo "==> Deployed: $SERVICE_URL"

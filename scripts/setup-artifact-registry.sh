#!/bin/bash
set -eu

if [ -z "$3" ]; then
  echo "Usage: $0 <project_id> <region> <repo_name>"
  exit 1
fi

PROJECT_ID=$1
REGION=$2
AR_REPO=$3

# Check if the repository already exists
if gcloud artifacts repositories describe "$AR_REPO" --location="$REGION" --project="$PROJECT_ID" &>/dev/null; then
  echo "Artifact Registry repository '$AR_REPO' already exists in '$REGION'."
else
  echo "Creating Artifact Registry repository: $AR_REPO in $REGION..."
  gcloud artifacts repositories create "$AR_REPO" \
    --repository-format=docker \
    --location="$REGION" \
    --description="YouTube Trend Tracker container repository" \
    --project="$PROJECT_ID"
  echo "Artifact Registry repository created."
fi

echo "Artifact Registry setup complete."


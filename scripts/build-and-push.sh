#!/bin/bash
set -euo pipefail

if [ $# -lt 4 ]; then
  echo "Usage: $0 <project_id> <region> <repo_name> <service_name>"
  exit 1
fi

PROJECT_ID=$1
REGION=$2
AR_REPO=$3
SERVICE=$4

IMAGE_URI="${REGION}-docker.pkg.dev/${PROJECT_ID}/${AR_REPO}/${SERVICE}:latest"

echo "Building and pushing container image: $IMAGE_URI..."

# Authenticate Docker to Artifact Registry
gcloud auth configure-docker "${REGION}-docker.pkg.dev" --project="$PROJECT_ID"

# Build and push the container
docker build --platform linux/amd64 -t "$IMAGE_URI" .
docker push "$IMAGE_URI"

echo "Build and push complete."

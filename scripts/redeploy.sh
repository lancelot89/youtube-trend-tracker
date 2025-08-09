#!/bin/bash
set -eu

# Usage: ./redeploy.sh <project_id> <region> <artifact_registry_repo> <service_name>

if [ -z "$4" ]; then
    echo "Usage: $0 <project_id> <region> <artifact_registry_repo> <service_name>"
    exit 1
fi

PROJECT_ID=$1
REGION=$2
AR_REPO=$3
SERVICE_NAME=$4

echo "🚀 Starting redeployment process..."

echo -e "\nStep 1/2: Building and pushing container image..."
./scripts/build-and-push.sh "${PROJECT_ID}" "${REGION}" "${AR_REPO}" "${SERVICE_NAME}"

echo -e "\nStep 2/2: Deploying Cloud Run service..."
./scripts/deploy-cloud-run.sh "${PROJECT_ID}" "${REGION}" "${SERVICE_NAME}" "${AR_REPO}"

echo -e "\n✅ Redeployment complete!"


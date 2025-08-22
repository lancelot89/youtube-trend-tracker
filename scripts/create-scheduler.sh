#!/bin/bash
set -eu

if [ -z "$3" ]; then
    echo "Usage: $0 <project_id> <region> <service_name>"
    exit 1
fi

PROJECT_ID=$1
REGION=$2
SERVICE=$3

SCHEDULER_SA="scheduler-sa@${PROJECT_ID}.iam.gserviceaccount.com"

# Get Cloud Run service URL
CRON_SVC_URL=$(gcloud run services describe "$SERVICE" --region="$REGION" --format="value(status.url)" --project="$PROJECT_ID")

if [ -z "$CRON_SVC_URL" ]; then
    echo "Error: Could not retrieve Cloud Run service URL for $SERVICE."
    exit 1
fi

# Service Account Setup
# Note: Service account creation and permission grants are now handled by
# ./scripts/setup-service-accounts.sh for better maintainability.
if ! gcloud iam service-accounts describe "$SCHEDULER_SA" --project="$PROJECT_ID" >/dev/null 2>&1; then
    echo "Error: Service account '$SCHEDULER_SA' does not exist."
    echo "Please run: ./scripts/setup-service-accounts.sh $PROJECT_ID $REGION $SERVICE"
    exit 1
fi

# Verify that scheduler-sa has invoker permission (should be set by setup-service-accounts.sh)
if ! gcloud run services get-iam-policy "$SERVICE" --region="$REGION" --project="$PROJECT_ID" \
    --flatten="bindings[].members" \
    --filter="bindings.members:serviceAccount:$SCHEDULER_SA" \
    --format="value(bindings.members)" 2>/dev/null | grep -q "$SCHEDULER_SA"; then
    echo "Warning: $SCHEDULER_SA may not have run.invoker permission."
    echo "Please run: ./scripts/setup-service-accounts.sh $PROJECT_ID $REGION $SERVICE"
fi

# Check if the job already exists
if gcloud scheduler jobs describe trend-tracker-hourly --location="$REGION" --project="$PROJECT_ID" >/dev/null 2>&1; then
    echo "Updating existing Cloud Scheduler job 'trend-tracker-hourly' to trigger $SERVICE..."
    gcloud scheduler jobs update http trend-tracker-hourly \
        --schedule="0 * * * *" \
        --uri="$CRON_SVC_URL" \
        --http-method=POST \
        --oidc-service-account-email="$SCHEDULER_SA" \
        --location="$REGION" \
        --project="$PROJECT_ID"
    echo "Cloud Scheduler job updated."
else
    echo "Creating new Cloud Scheduler job 'trend-tracker-hourly' to trigger $SERVICE..."
    gcloud scheduler jobs create http trend-tracker-hourly \
        --schedule="0 * * * *" \
        --uri="$CRON_SVC_URL" \
        --http-method=POST \
        --oidc-service-account-email="$SCHEDULER_SA" \
        --location="$REGION" \
        --project="$PROJECT_ID"
    echo "Cloud Scheduler job created."
fi
"
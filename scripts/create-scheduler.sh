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

# Create service account for scheduler (if it doesn't exist)
gcloud iam service-accounts describe "$SCHEDULER_SA" --project="$PROJECT_ID" >/dev/null 2>&1 || \
    gcloud iam service-accounts create scheduler-sa \
        --display-name="Cloud Scheduler Invoker SA" \
        --project="$PROJECT_ID"

# Grant invoker role to the scheduler service account for the specific Cloud Run service
gcloud run services add-iam-policy-binding "$SERVICE" \
    --region="$REGION" \
    --member="serviceAccount:$SCHEDULER_SA" \
    --role="roles/run.invoker" \
    --project="$PROJECT_ID" >/dev/null

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
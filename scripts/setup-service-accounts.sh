#!/bin/bash
set -euo pipefail

# ==============================================================================
# setup-service-accounts.sh
# 
# YouTube Trend Trackerで使用する全サービスアカウントの作成と権限設定を
# 一元管理するスクリプト
# 
# Usage: ./setup-service-accounts.sh <project_id> <region> <service_name>
# ==============================================================================

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m' # No Color

# Validate arguments
if [ $# -lt 3 ]; then
    echo -e "${RED}Error: Missing required arguments${NC}"
    echo "Usage: $0 <project_id> <region> <service_name>"
    echo ""
    echo "Arguments:"
    echo "  project_id   - GCP project ID"
    echo "  region       - GCP region (e.g., asia-northeast1)"
    echo "  service_name - Cloud Run service name"
    exit 1
fi

PROJECT_ID="$1"
REGION="$2"
SERVICE_NAME="$3"

# Service Account definitions
TREND_TRACKER_SA="trend-tracker-sa"
TREND_TRACKER_SA_EMAIL="${TREND_TRACKER_SA}@${PROJECT_ID}.iam.gserviceaccount.com"
SCHEDULER_SA="scheduler-sa"
SCHEDULER_SA_EMAIL="${SCHEDULER_SA}@${PROJECT_ID}.iam.gserviceaccount.com"

echo -e "${GREEN}=== Setting up Service Accounts for YouTube Trend Tracker ===${NC}"
echo "Project ID: $PROJECT_ID"
echo "Region: $REGION"
echo "Service Name: $SERVICE_NAME"
echo ""

# Set the project
gcloud config set project "$PROJECT_ID" >/dev/null

# ==============================================================================
# 1. Create Service Accounts
# ==============================================================================
echo -e "${YELLOW}[1/3] Creating Service Accounts...${NC}"

# Create trend-tracker-sa
if gcloud iam service-accounts describe "$TREND_TRACKER_SA_EMAIL" --project="$PROJECT_ID" >/dev/null 2>&1; then
    echo "  ✓ Service account '$TREND_TRACKER_SA' already exists"
else
    echo "  Creating service account '$TREND_TRACKER_SA'..."
    gcloud iam service-accounts create "$TREND_TRACKER_SA" \
        --display-name="YouTube Trend Tracker Service Account" \
        --description="Service account for Cloud Run service that fetches YouTube trends" \
        --project="$PROJECT_ID"
    echo "  ✓ Service account '$TREND_TRACKER_SA' created"
fi

# Create scheduler-sa
if gcloud iam service-accounts describe "$SCHEDULER_SA_EMAIL" --project="$PROJECT_ID" >/dev/null 2>&1; then
    echo "  ✓ Service account '$SCHEDULER_SA' already exists"
else
    echo "  Creating service account '$SCHEDULER_SA'..."
    gcloud iam service-accounts create "$SCHEDULER_SA" \
        --display-name="Cloud Scheduler Service Account" \
        --description="Service account for Cloud Scheduler to invoke Cloud Run" \
        --project="$PROJECT_ID"
    echo "  ✓ Service account '$SCHEDULER_SA' created"
fi

# ==============================================================================
# 2. Grant Permissions to trend-tracker-sa
# ==============================================================================
echo ""
echo -e "${YELLOW}[2/3] Configuring permissions for trend-tracker-sa...${NC}"

# Helper function to add IAM policy binding with idempotency
add_iam_policy_binding() {
    local member="$1"
    local role="$2"
    local resource_type="$3"  # "project" or "secret" or "service"
    local resource_name="$4"
    local description="$5"
    
    echo -n "  Granting $role ($description)..."
    
    if [ "$resource_type" = "project" ]; then
        if gcloud projects get-iam-policy "$PROJECT_ID" --flatten="bindings[].members" \
            --filter="bindings.role:$role AND bindings.members:$member" \
            --format="value(bindings.members)" 2>/dev/null | grep -q "$member"; then
            echo " ✓ (already granted)"
        else
            gcloud projects add-iam-policy-binding "$PROJECT_ID" \
                --member="$member" \
                --role="$role" \
                --condition=None >/dev/null 2>&1
            echo " ✓"
        fi
    elif [ "$resource_type" = "secret" ]; then
        if gcloud secrets get-iam-policy "$resource_name" --project="$PROJECT_ID" \
            --flatten="bindings[].members" \
            --filter="bindings.role:$role AND bindings.members:$member" \
            --format="value(bindings.members)" 2>/dev/null | grep -q "$member"; then
            echo " ✓ (already granted)"
        else
            gcloud secrets add-iam-policy-binding "$resource_name" \
                --member="$member" \
                --role="$role" \
                --project="$PROJECT_ID" >/dev/null 2>&1
            echo " ✓"
        fi
    elif [ "$resource_type" = "service" ]; then
        if gcloud run services get-iam-policy "$SERVICE_NAME" --region="$REGION" --project="$PROJECT_ID" \
            --flatten="bindings[].members" \
            --filter="bindings.role:$role AND bindings.members:$member" \
            --format="value(bindings.members)" 2>/dev/null | grep -q "$member"; then
            echo " ✓ (already granted)"
        else
            gcloud run services add-iam-policy-binding "$SERVICE_NAME" \
                --region="$REGION" \
                --member="$member" \
                --role="$role" \
                --project="$PROJECT_ID" >/dev/null 2>&1
            echo " ✓"
        fi
    fi
}

# Grant permissions to trend-tracker-sa
echo "  Permissions for ${TREND_TRACKER_SA}:"
add_iam_policy_binding "serviceAccount:$TREND_TRACKER_SA_EMAIL" \
    "roles/artifactregistry.reader" \
    "project" "$PROJECT_ID" \
    "Pull container images"

add_iam_policy_binding "serviceAccount:$TREND_TRACKER_SA_EMAIL" \
    "roles/bigquery.dataEditor" \
    "project" "$PROJECT_ID" \
    "Write to BigQuery tables"

add_iam_policy_binding "serviceAccount:$TREND_TRACKER_SA_EMAIL" \
    "roles/bigquery.jobUser" \
    "project" "$PROJECT_ID" \
    "Run BigQuery jobs"

# Check if youtube-api-key secret exists before granting access
if gcloud secrets describe "youtube-api-key" --project="$PROJECT_ID" >/dev/null 2>&1; then
    add_iam_policy_binding "serviceAccount:$TREND_TRACKER_SA_EMAIL" \
        "roles/secretmanager.secretAccessor" \
        "secret" "youtube-api-key" \
        "Access YouTube API key"
else
    echo -e "  ${YELLOW}⚠ Warning: Secret 'youtube-api-key' does not exist. Skipping permission grant.${NC}"
    echo -e "  ${YELLOW}  Run './scripts/create-secret.sh' to create the secret.${NC}"
fi

# ==============================================================================
# 3. Grant Permissions to scheduler-sa
# ==============================================================================
echo ""
echo -e "${YELLOW}[3/3] Configuring permissions for scheduler-sa...${NC}"

# Check if Cloud Run service exists
if gcloud run services describe "$SERVICE_NAME" --region="$REGION" --project="$PROJECT_ID" >/dev/null 2>&1; then
    echo "  Permissions for ${SCHEDULER_SA}:"
    add_iam_policy_binding "serviceAccount:$SCHEDULER_SA_EMAIL" \
        "roles/run.invoker" \
        "service" "$SERVICE_NAME" \
        "Invoke Cloud Run service"
else
    echo -e "  ${YELLOW}⚠ Warning: Cloud Run service '$SERVICE_NAME' does not exist in region '$REGION'.${NC}"
    echo -e "  ${YELLOW}  Skipping Cloud Run invoker permission. Deploy the service first.${NC}"
fi

# ==============================================================================
# Summary
# ==============================================================================
echo ""
echo -e "${GREEN}=== Service Account Setup Complete ===${NC}"
echo ""
echo "Service Accounts Created/Configured:"
echo "  • ${TREND_TRACKER_SA_EMAIL}"
echo "    - roles/artifactregistry.reader (Project)"
echo "    - roles/bigquery.dataEditor (Project)"
echo "    - roles/bigquery.jobUser (Project)"
echo "    - roles/secretmanager.secretAccessor (Secret: youtube-api-key)"
echo ""
echo "  • ${SCHEDULER_SA_EMAIL}"
echo "    - roles/run.invoker (Service: $SERVICE_NAME)"
echo ""
echo "Next Steps:"
echo "  1. If you haven't already, create the YouTube API key secret:"
echo "     ./scripts/create-secret.sh"
echo "  2. Deploy the Cloud Run service:"
echo "     ./scripts/deploy-cloud-run.sh $PROJECT_ID $REGION $SERVICE_NAME <ar_repo>"
echo "  3. Set up Cloud Scheduler:"
echo "     ./scripts/create-scheduler.sh $PROJECT_ID $REGION $SERVICE_NAME"
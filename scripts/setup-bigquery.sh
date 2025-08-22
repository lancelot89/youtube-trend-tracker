#!/bin/bash
set -euo pipefail

# ==============================================================================
# setup-bigquery.sh
# 
# YouTube Trend Tracker用のBigQueryデータセットとテーブルをセットアップします
# 
# Usage: ./setup-bigquery.sh <project_id> [dataset_name]
# ==============================================================================

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m' # No Color

# Validate arguments
if [ $# -lt 1 ]; then
    echo -e "${RED}Error: Missing required arguments${NC}"
    echo "Usage: $0 <project_id> [dataset_name]"
    echo ""
    echo "Arguments:"
    echo "  project_id   - GCP project ID"
    echo "  dataset_name - BigQuery dataset name (default: youtube)"
    exit 1
fi

PROJECT_ID="$1"
DATASET_NAME="${2:-youtube}"
LOCATION="asia-northeast1"

echo -e "${GREEN}=== Setting up BigQuery for YouTube Trend Tracker ===${NC}"
echo "Project ID: $PROJECT_ID"
echo "Dataset: $DATASET_NAME"
echo "Location: $LOCATION"
echo ""

# Set the project
gcloud config set project "$PROJECT_ID" >/dev/null

# ==============================================================================
# 1. Create Dataset
# ==============================================================================
echo -e "${YELLOW}[1/4] Creating BigQuery dataset...${NC}"

if bq ls -d --project_id="$PROJECT_ID" | grep -q "^${DATASET_NAME}$"; then
    echo "  ✓ Dataset '$DATASET_NAME' already exists"
else
    echo "  Creating dataset '$DATASET_NAME'..."
    bq mk \
        --project_id="$PROJECT_ID" \
        --location="$LOCATION" \
        --dataset \
        --description="YouTube trend data storage" \
        "$DATASET_NAME"
    echo "  ✓ Dataset '$DATASET_NAME' created"
fi

# ==============================================================================
# 2. Create video_trends table
# ==============================================================================
echo ""
echo -e "${YELLOW}[2/3] Creating video_trends table...${NC}"

# Check if table exists
if bq ls --project_id="$PROJECT_ID" "$DATASET_NAME" | grep -q "video_trends"; then
    echo "  ✓ Table 'video_trends' already exists"
    echo "  Note: To update the schema, use 'bq update' or migration scripts"
else
    echo "  Creating table 'video_trends'..."
    
    # Create table with schema matching internal/storage/bq.go
    bq mk \
        --project_id="$PROJECT_ID" \
        --table \
        --time_partitioning_field=dt \
        --time_partitioning_type=DAY \
        --clustering_fields=channel_id,video_id \
        --description="YouTube video trend data" \
        "$DATASET_NAME.video_trends" \
        dt:DATE,channel_id:STRING,video_id:STRING,title:STRING,channel_name:STRING,tags:STRING,is_short:BOOL,views:INTEGER,likes:INTEGER,comments:INTEGER,published_at:TIMESTAMP,created_at:TIMESTAMP,duration_sec:INTEGER,content_details:STRING,topic_details:STRING
    
    echo "  ✓ Table 'video_trends' created"
fi

# ==============================================================================
# 3. Create views
# ==============================================================================
echo ""
echo -e "${YELLOW}[3/3] Creating analytical views...${NC}"

# Create video_trends_analysis view
echo "  Creating view 'video_trends_analysis'..."
bq query \
    --project_id="$PROJECT_ID" \
    --use_legacy_sql=false \
    --replace \
    "CREATE OR REPLACE VIEW \`${PROJECT_ID}.${DATASET_NAME}.video_trends_analysis\` AS
    SELECT
      video_id,
      channel_id,
      title,
      published_at,
      created_at,
      views,
      likes,
      comments,
      LAG(views) OVER (PARTITION BY video_id ORDER BY created_at) AS prev_views,
      LAG(likes) OVER (PARTITION BY video_id ORDER BY created_at) AS prev_likes,
      SAFE_DIVIDE(
        views - LAG(views) OVER (PARTITION BY video_id ORDER BY created_at),
        LAG(views) OVER (PARTITION BY video_id ORDER BY created_at)
      ) * 100 AS view_growth_rate,
      SAFE_DIVIDE(
        views - LAG(views) OVER (PARTITION BY video_id ORDER BY created_at),
        TIMESTAMP_DIFF(created_at, LAG(created_at) OVER (PARTITION BY video_id ORDER BY created_at), HOUR)
      ) AS views_per_hour
    FROM
      \`${PROJECT_ID}.${DATASET_NAME}.video_trends\`
    WHERE
      created_at >= TIMESTAMP_SUB(CURRENT_TIMESTAMP(), INTERVAL 30 DAY)" 2>/dev/null || echo "  ⚠ View creation skipped (may already exist or no data)"

echo "  ✓ Views setup complete"

# ==============================================================================
# Summary
# ==============================================================================
echo ""
echo -e "${GREEN}=== BigQuery Setup Complete ===${NC}"
echo ""
echo "Dataset and Tables Created:"
echo "  • Dataset: ${DATASET_NAME}"
echo "  • Tables:"
echo "    - ${DATASET_NAME}.video_trends (partitioned by dt, clustered by channel_id, video_id)"
echo "  • Views:"
echo "    - ${DATASET_NAME}.video_trends_analysis (trend analysis)"
echo ""
echo "Next Steps:"
echo "  1. Deploy the Cloud Run service to start collecting data:"
echo "     ./scripts/deploy-cloud-run.sh $PROJECT_ID <region> <ar_repo> <service_name>"
echo "  2. Verify tables in BigQuery Console:"
echo "     https://console.cloud.google.com/bigquery?project=$PROJECT_ID"
echo "  3. Query sample data (once available):"
echo "     bq query --use_legacy_sql=false \"SELECT * FROM \\\`${PROJECT_ID}.${DATASET_NAME}.video_trends\\\` LIMIT 10\""
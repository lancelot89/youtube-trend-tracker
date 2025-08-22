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
# 2. Create videos table
# ==============================================================================
echo ""
echo -e "${YELLOW}[2/4] Creating videos table...${NC}"

# Check if table exists
if bq ls --project_id="$PROJECT_ID" "$DATASET_NAME" | grep -q "videos"; then
    echo "  ✓ Table 'videos' already exists"
    echo "  Note: To update the schema, use 'bq update' or migration scripts"
else
    echo "  Creating table 'videos'..."
    
    # Create table with schema
    bq mk \
        --project_id="$PROJECT_ID" \
        --table \
        --time_partitioning_field=captured_at \
        --time_partitioning_type=DAY \
        --clustering_fields=channel_id,video_id \
        --description="YouTube video trend data" \
        "$DATASET_NAME.videos" \
        video_id:STRING,channel_id:STRING,title:STRING,description:STRING,channel_title:STRING,published_at:TIMESTAMP,captured_at:TIMESTAMP,category_id:STRING,tags:STRING,view_count:INT64,like_count:INT64,comment_count:INT64,favorite_count:INT64,duration:STRING,definition:STRING,caption:STRING,licensed_content:BOOL,region_code:STRING,is_short:BOOL
    
    echo "  ✓ Table 'videos' created"
fi

# ==============================================================================
# 3. Create channels table
# ==============================================================================
echo ""
echo -e "${YELLOW}[3/4] Creating channels table...${NC}"

# Check if table exists
if bq ls --project_id="$PROJECT_ID" "$DATASET_NAME" | grep -q "channels"; then
    echo "  ✓ Table 'channels' already exists"
    echo "  Note: To update the schema, use 'bq update' or migration scripts"
else
    echo "  Creating table 'channels'..."
    
    # Create table with schema
    bq mk \
        --project_id="$PROJECT_ID" \
        --table \
        --time_partitioning_field=captured_at \
        --time_partitioning_type=DAY \
        --clustering_fields=channel_id \
        --description="YouTube channel information" \
        "$DATASET_NAME.channels" \
        channel_id:STRING,title:STRING,description:STRING,custom_url:STRING,country:STRING,published_at:TIMESTAMP,captured_at:TIMESTAMP,view_count:INT64,subscriber_count:INT64,video_count:INT64,uploads_playlist_id:STRING,thumbnail_url:STRING,topic_categories:STRING,keywords:STRING
    
    echo "  ✓ Table 'channels' created"
fi

# ==============================================================================
# 4. Create views
# ==============================================================================
echo ""
echo -e "${YELLOW}[4/4] Creating analytical views...${NC}"

# Create video_trends view
echo "  Creating view 'video_trends'..."
bq query \
    --project_id="$PROJECT_ID" \
    --use_legacy_sql=false \
    --replace \
    "CREATE OR REPLACE VIEW \`${PROJECT_ID}.${DATASET_NAME}.video_trends\` AS
    SELECT
      video_id,
      channel_id,
      title,
      published_at,
      captured_at,
      view_count,
      like_count,
      comment_count,
      LAG(view_count) OVER (PARTITION BY video_id ORDER BY captured_at) AS prev_view_count,
      LAG(like_count) OVER (PARTITION BY video_id ORDER BY captured_at) AS prev_like_count,
      SAFE_DIVIDE(
        view_count - LAG(view_count) OVER (PARTITION BY video_id ORDER BY captured_at),
        LAG(view_count) OVER (PARTITION BY video_id ORDER BY captured_at)
      ) * 100 AS view_growth_rate,
      SAFE_DIVIDE(
        view_count - LAG(view_count) OVER (PARTITION BY video_id ORDER BY captured_at),
        TIMESTAMP_DIFF(captured_at, LAG(captured_at) OVER (PARTITION BY video_id ORDER BY captured_at), HOUR)
      ) AS views_per_hour,
      region_code
    FROM
      \`${PROJECT_ID}.${DATASET_NAME}.videos\`
    WHERE
      captured_at >= TIMESTAMP_SUB(CURRENT_TIMESTAMP(), INTERVAL 30 DAY)" 2>/dev/null || echo "  ⚠ View creation skipped (may already exist or no data)"

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
echo "    - ${DATASET_NAME}.videos (partitioned by captured_at, clustered by channel_id, video_id)"
echo "    - ${DATASET_NAME}.channels (partitioned by captured_at, clustered by channel_id)"
echo "  • Views:"
echo "    - ${DATASET_NAME}.video_trends (trend analysis)"
echo ""
echo "Next Steps:"
echo "  1. Deploy the Cloud Run service to start collecting data:"
echo "     ./scripts/deploy-cloud-run.sh $PROJECT_ID <region> <service_name> <ar_repo>"
echo "  2. Verify tables in BigQuery Console:"
echo "     https://console.cloud.google.com/bigquery?project=$PROJECT_ID"
echo "  3. Query sample data (once available):"
echo "     bq query --use_legacy_sql=false \"SELECT * FROM \\\`${PROJECT_ID}.${DATASET_NAME}.videos\\\` LIMIT 10\""
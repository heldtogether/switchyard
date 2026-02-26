#!/bin/bash
# List job artefacts

set -e

API_URL="${API_URL:-http://localhost:8080}"
API_KEY="${API_KEY:-your-api-key}"
WORKSPACE_SLUG="${WORKSPACE_SLUG:-default}"
PROJECT_SLUG="${PROJECT_SLUG:-test-project}"
RUN_SLUG="${RUN_SLUG:-test-run}"

if [ -z "$1" ]; then
    echo "Usage: $0 <job-id>"
    exit 1
fi

JOB_ID="$1"

echo "Listing artefacts for job $JOB_ID..."
echo ""

RESPONSE=$(curl -s "$API_URL/v1/workspaces/$WORKSPACE_SLUG/projects/$PROJECT_SLUG/runs/$RUN_SLUG/jobs/$JOB_ID/artefacts" \
  -H "X-API-Key: $API_KEY")

echo "$RESPONSE" | jq .

# Show download commands
echo ""
echo "Download artefacts with:"
echo "$RESPONSE" | jq -r '.artefacts[] | "  curl -H \"X-API-Key: '$API_KEY'\" -O \"'$API_URL'/v1/workspaces/'$WORKSPACE_SLUG'/projects/'$PROJECT_SLUG'/runs/'$RUN_SLUG'/jobs/'$JOB_ID'/artefacts/\(.path)\""'

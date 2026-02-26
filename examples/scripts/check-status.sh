#!/bin/bash
# Check the status of a job

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

echo "Checking status of job $JOB_ID..."
echo ""

curl -s "$API_URL/v1/workspaces/$WORKSPACE_SLUG/projects/$PROJECT_SLUG/runs/$RUN_SLUG/jobs/$JOB_ID" \
  -H "X-API-Key: $API_KEY" \
  | jq .

echo ""
echo "To view logs:"
echo "  ./fetch-logs.sh $JOB_ID"
echo ""
echo "To list artefacts:"
echo "  ./list-artefacts.sh $JOB_ID"

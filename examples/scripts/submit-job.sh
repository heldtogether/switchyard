#!/bin/bash
# Submit an example job to the Switchyard API

set -e

API_URL="${API_URL:-http://localhost:8080}"
API_KEY="${API_KEY:-your-api-key}"
WORKSPACE_SLUG="${WORKSPACE_SLUG:-default}"
PROJECT_SLUG="${PROJECT_SLUG:-test-project}"
RUN_SLUG="${RUN_SLUG:-test-run}"

echo "Submitting example job to $API_URL..."

# Note: System environment variables (SWITCHYARD_*) are automatically injected
# and include: SWITCHYARD_JOB_ID, SWITCHYARD_JOB_CREATED_AT, SWITCHYARD_JOB_TIMEOUT,
# SWITCHYARD_EXECUTOR_TYPE, SWITCHYARD_IMAGE, SWITCHYARD_OUTPUTS_DIR, SWITCHYARD_BUCKET,
# SWITCHYARD_VERSION, SWITCHYARD_API_URL, and resource limits
RESPONSE=$(curl -s -X POST "$API_URL/v1/workspaces/$WORKSPACE_SLUG/projects/$PROJECT_SLUG/runs/$RUN_SLUG/jobs" \
  -H "X-API-Key: $API_KEY" \
  -H "Content-Type: application/json" \
  -d '{
    "image": "switchyard-example-job:latest",
    "command": ["/app/entrypoint.sh"],
    "env": {
      "CUSTOM_VAR": "example-value"
    },
    "outputs": ["/outputs"],
    "resources": {
      "cpu": "0.5",
      "memory": "512m",
      "gpu": 2
    },
    "timeout_seconds": 300,
    "metadata": {
      "description": "Example job from submit script"
    }
  }')

echo "Response:"
echo "$RESPONSE" | jq .

# Extract job ID
JOB_ID=$(echo "$RESPONSE" | jq -r '.id')
echo ""
echo "Job submitted! ID: $JOB_ID"
echo ""
echo "Check status with:"
echo "  ./check-status.sh $JOB_ID"

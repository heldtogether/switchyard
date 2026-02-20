#!/bin/bash
# Submit an example job to the Switchyard API

set -e

API_URL="${API_URL:-http://localhost:8080}"
API_KEY="${API_KEY:-your-api-key}"

echo "Submitting example job to $API_URL..."

RESPONSE=$(curl -s -X POST "$API_URL/v1/jobs" \
  -H "X-API-Key: $API_KEY" \
  -H "Content-Type: application/json" \
  -d '{
    "image": "switchyard-example-job:latest",
    "command": ["/app/entrypoint.sh"],
    "env": {
      "JOB_ID": "example-001"
    },
    "outputs": ["/outputs"],
    "resources": {
      "cpu": "0.5",
      "memory": "512m"
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

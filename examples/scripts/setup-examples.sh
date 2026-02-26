#!/bin/bash
# Ensure example workspace, project, and run exist

set -e

API_URL="${API_URL:-http://localhost:8080}"
API_KEY="${API_KEY:-your-api-key}"
WORKSPACE_SLUG="${WORKSPACE_SLUG:-default}"
PROJECT_SLUG="${PROJECT_SLUG:-test-project}"
RUN_SLUG="${RUN_SLUG:-test-run}"

request() {
  local method="$1"
  local url="$2"
  local data="$3"

  if [ -n "$data" ]; then
    curl -s -o /tmp/resp.json -w "%{http_code}" \
      -X "$method" "$url" \
      -H "X-API-Key: $API_KEY" \
      -H "Content-Type: application/json" \
      -d "$data"
  else
    curl -s -o /tmp/resp.json -w "%{http_code}" \
      -X "$method" "$url" \
      -H "X-API-Key: $API_KEY"
  fi
}

ensure_workspace() {
  local url="$API_URL/v1/workspaces/$WORKSPACE_SLUG"
  local status
  status=$(request GET "$url" "")
  if [ "$status" = "200" ]; then
    echo "Workspace '$WORKSPACE_SLUG' exists."
    return
  fi
  if [ "$status" != "404" ]; then
    echo "Unexpected response checking workspace: $status"
    cat /tmp/resp.json
    exit 1
  fi

  echo "Creating workspace '$WORKSPACE_SLUG'..."
  status=$(request POST "$API_URL/v1/workspaces" "{\"slug\":\"$WORKSPACE_SLUG\",\"name\":\"Default Workspace\"}")
  if [ "$status" != "201" ]; then
    echo "Failed to create workspace: $status"
    cat /tmp/resp.json
    exit 1
  fi
}

ensure_project() {
  local url="$API_URL/v1/workspaces/$WORKSPACE_SLUG/projects/$PROJECT_SLUG"
  local status
  status=$(request GET "$url" "")
  if [ "$status" = "200" ]; then
    echo "Project '$PROJECT_SLUG' exists."
    return
  fi
  if [ "$status" != "404" ]; then
    echo "Unexpected response checking project: $status"
    cat /tmp/resp.json
    exit 1
  fi

  echo "Creating project '$PROJECT_SLUG'..."
  status=$(request POST "$API_URL/v1/workspaces/$WORKSPACE_SLUG/projects" "{\"slug\":\"$PROJECT_SLUG\",\"name\":\"Test Project\"}")
  if [ "$status" != "201" ]; then
    echo "Failed to create project: $status"
    cat /tmp/resp.json
    exit 1
  fi
}

ensure_run() {
  local url="$API_URL/v1/workspaces/$WORKSPACE_SLUG/projects/$PROJECT_SLUG/runs/$RUN_SLUG"
  local status
  status=$(request GET "$url" "")
  if [ "$status" = "200" ]; then
    echo "Run '$RUN_SLUG' exists."
    return
  fi
  if [ "$status" != "404" ]; then
    echo "Unexpected response checking run: $status"
    cat /tmp/resp.json
    exit 1
  fi

  echo "Creating run '$RUN_SLUG'..."
  status=$(request POST "$API_URL/v1/workspaces/$WORKSPACE_SLUG/projects/$PROJECT_SLUG/runs" "{\"slug\":\"$RUN_SLUG\",\"name\":\"Test Run\"}")
  if [ "$status" != "201" ]; then
    echo "Failed to create run: $status"
    cat /tmp/resp.json
    exit 1
  fi
}

ensure_workspace
ensure_project
ensure_run

echo "Setup complete."

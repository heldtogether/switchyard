#!/bin/sh
set -e

API_BASE_URL=${UI_API_BASE_URL:-http://localhost:8080}
API_KEY=${UI_API_KEY:-}
WORKSPACE_SLUG=${UI_WORKSPACE_SLUG:-default}
USE_MOCKS=${UI_USE_MOCKS:-false}
AGGREGATE_LIMIT=${UI_AGGREGATE_LIMIT:-5}

cat > /usr/share/nginx/html/config.js <<CONFIG
window.__ENV = {
  API_BASE_URL: "${API_BASE_URL}",
  API_KEY: "${API_KEY}",
  WORKSPACE_SLUG: "${WORKSPACE_SLUG}",
  USE_MOCKS: "${USE_MOCKS}",
  AGGREGATE_LIMIT: "${AGGREGATE_LIMIT}"
};
CONFIG

exec nginx -g "daemon off;"

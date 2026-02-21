/// <reference types="vite/client" />

interface Window {
  __ENV?: {
    API_BASE_URL?: string;
    API_KEY?: string;
    WORKSPACE_SLUG?: string;
    USE_MOCKS?: string;
    AGGREGATE_LIMIT?: string;
  };
}

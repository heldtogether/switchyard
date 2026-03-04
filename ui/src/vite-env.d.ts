/// <reference types="vite/client" />

interface Window {
  __ENV?: {
    API_BASE_URL?: string;
    AUTH_LOGIN_URL?: string;
    AUTH_LOGOUT_URL?: string;
    WORKSPACE_SLUG?: string;
    USE_MOCKS?: string;
    AGGREGATE_LIMIT?: string;
  };
}

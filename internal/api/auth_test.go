package api

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestSanitizeRedirectTarget(t *testing.T) {
	fallback := "http://ui.local/login"

	if got := resolveRedirectTarget("", fallback); got != fallback {
		t.Fatalf("expected fallback for empty next, got %q", got)
	}
	if got := resolveRedirectTarget("/runs?view=all", fallback); got != "http://ui.local/runs?view=all" {
		t.Fatalf("expected ui absolute redirect, got %q", got)
	}
	if got := resolveRedirectTarget("https://evil.example/logout", fallback); got != fallback {
		t.Fatalf("expected fallback for absolute external next, got %q", got)
	}
	if got := resolveRedirectTarget("//evil.example/logout", fallback); got != fallback {
		t.Fatalf("expected fallback for protocol-relative next, got %q", got)
	}
	if got := resolveRedirectTarget("/runs", "/login"); got != "/runs" {
		t.Fatalf("expected local path for relative fallback, got %q", got)
	}
}

func TestLogoutRedirectClearsCookieAndRedirects(t *testing.T) {
	manager := &AuthManager{
		cookieName:         "switchyard_session",
		cookieDomain:       "api.local",
		cookieSecure:       true,
		cookieSameSite:     0,
		cookieTTL:          24 * time.Hour,
		postLogoutRedirect: "http://ui.local/login",
	}

	recorder := httptest.NewRecorder()
	request := httptest.NewRequest("GET", "/v1/auth/logout?next=/login", nil)

	manager.LogoutRedirect(recorder, request)

	if recorder.Code != 302 {
		t.Fatalf("expected 302 redirect, got %d", recorder.Code)
	}
	if location := recorder.Header().Get("Location"); location != "http://ui.local/login" {
		t.Fatalf("expected redirect to absolute UI URL, got %q", location)
	}

	cookieHeader := recorder.Header().Get("Set-Cookie")
	if cookieHeader == "" {
		t.Fatalf("expected logout to set cookie header")
	}
	if !strings.Contains(cookieHeader, "switchyard_session=") {
		t.Fatalf("expected session cookie to be cleared, got %q", cookieHeader)
	}
	if !strings.Contains(cookieHeader, "Max-Age=0") {
		t.Fatalf("expected Max-Age=0 on cleared cookie, got %q", cookieHeader)
	}
}

func TestLogoutRedirectUsesProviderLogoutURL(t *testing.T) {
	manager := &AuthManager{
		cookieName:        "switchyard_session",
		cookieDomain:      "api.local",
		cookieSecure:      true,
		cookieSameSite:    0,
		cookieTTL:         24 * time.Hour,
		providerLogoutURL: "https://issuer.example.com/logout?client_id=abc",
	}

	recorder := httptest.NewRecorder()
	request := httptest.NewRequest("GET", "/v1/auth/logout?next=/runs", nil)

	manager.LogoutRedirect(recorder, request)

	if recorder.Code != 302 {
		t.Fatalf("expected 302 redirect, got %d", recorder.Code)
	}
	if location := recorder.Header().Get("Location"); location != manager.providerLogoutURL {
		t.Fatalf("expected redirect to provider logout URL, got %q", location)
	}
}

func TestIssueAndVerifyBearerToken(t *testing.T) {
	manager := &AuthManager{
		signingKey:     []byte("test-signing-key"),
		tokenIssuer:    "http://api.local",
		bearerTokenTTL: 15 * time.Minute,
	}

	principal := Principal{
		Subject:    "user-123",
		Email:      "user@example.com",
		Name:       "User",
		PictureURL: "https://example.com/user.png",
		Provider:   "oidc",
		AuthMethod: "oidc",
	}

	token, _, err := manager.issueBearerToken(principal)
	if err != nil {
		t.Fatalf("issueBearerToken failed: %v", err)
	}

	got, err := manager.verifyBearerToken(token)
	if err != nil {
		t.Fatalf("verifyBearerToken failed: %v", err)
	}
	if got.Subject != principal.Subject || got.Email != principal.Email {
		t.Fatalf("unexpected principal from token: %+v", got)
	}
}

func TestVerifyBearerTokenRejectsExpired(t *testing.T) {
	manager := &AuthManager{
		signingKey:     []byte("test-signing-key"),
		tokenIssuer:    "http://api.local",
		bearerTokenTTL: -1 * time.Minute,
	}

	token, _, err := manager.issueBearerToken(Principal{Subject: "user-123", Provider: "oidc", AuthMethod: "oidc"})
	if err != nil {
		t.Fatalf("issueBearerToken failed: %v", err)
	}

	if _, err := manager.verifyBearerToken(token); err == nil {
		t.Fatalf("expected expired token verification to fail")
	}
}

func TestMiddlewareAcceptsBearerToken(t *testing.T) {
	manager := &AuthManager{
		mode:           "oidc",
		signingKey:     []byte("test-signing-key"),
		tokenIssuer:    "http://api.local",
		bearerTokenTTL: 15 * time.Minute,
	}

	token, _, err := manager.issueBearerToken(Principal{
		Subject:    "user-123",
		Provider:   "oidc",
		AuthMethod: "oidc",
	})
	if err != nil {
		t.Fatalf("issueBearerToken failed: %v", err)
	}

	handler := manager.Middleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		principal, ok := PrincipalFromContext(r.Context())
		if !ok || principal.Subject != "user-123" {
			t.Fatalf("expected principal in context, got %+v", principal)
		}
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest(http.MethodGet, "/v1/workspaces", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	recorder := httptest.NewRecorder()

	handler.ServeHTTP(recorder, req)
	if recorder.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d", recorder.Code)
	}
}

func TestMiddlewareAcceptsServiceAccountKey(t *testing.T) {
	manager := &AuthManager{mode: "oidc"}
	manager.SetServiceAccountKeyResolver(func(ctx context.Context, token string) (Principal, bool) {
		if token != "swy_sa_test_secret" {
			return Principal{}, false
		}
		return Principal{
			Subject:    "service_account:123",
			Name:       "CI",
			Provider:   "service_account",
			AuthMethod: "service_account",
		}, true
	})

	handler := manager.Middleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		principal, ok := PrincipalFromContext(r.Context())
		if !ok || principal.Subject != "service_account:123" || principal.AuthMethod != "service_account" {
			t.Fatalf("expected service account principal in context, got %+v", principal)
		}
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest(http.MethodPost, "/v1/workspaces/default/projects/proj/runs", nil)
	req.Header.Set("Authorization", "Bearer swy_sa_test_secret")
	recorder := httptest.NewRecorder()

	handler.ServeHTTP(recorder, req)
	if recorder.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d", recorder.Code)
	}
}

func TestMiddlewareRejectsInvalidServiceAccountKey(t *testing.T) {
	manager := &AuthManager{mode: "oidc"}
	manager.SetServiceAccountKeyResolver(func(ctx context.Context, token string) (Principal, bool) {
		return Principal{}, false
	})

	handler := manager.Middleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Fatalf("handler should not be called")
	}))

	req := httptest.NewRequest(http.MethodGet, "/v1/workspaces", nil)
	req.Header.Set("X-API-Key", "swy_sa_bad_secret")
	recorder := httptest.NewRecorder()

	handler.ServeHTTP(recorder, req)
	if recorder.Code != http.StatusUnauthorized {
		t.Fatalf("expected status 401, got %d", recorder.Code)
	}
}

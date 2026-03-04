package api

import (
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

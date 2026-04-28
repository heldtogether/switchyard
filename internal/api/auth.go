package api

import (
	"context"
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	urlpkg "net/url"
	"strings"
	"sync"
	"time"

	"github.com/coreos/go-oidc/v3/oidc"
	"github.com/heldtogether/switchyard/internal/config"
	"golang.org/x/oauth2"
)

type principalContextKey struct{}

type Principal struct {
	Subject    string `json:"subject"`
	Email      string `json:"email,omitempty"`
	Name       string `json:"name,omitempty"`
	PictureURL string `json:"picture_url,omitempty"`
	Provider   string `json:"provider"`
	AuthMethod string `json:"auth_method"`
}

type oidcState struct {
	Nonce     string
	Next      string
	ExpiresAt time.Time
}

type sessionClaims struct {
	Principal Principal `json:"principal"`
	Exp       int64     `json:"exp"`
}

type jwtClaims struct {
	Subject    string `json:"sub"`
	Email      string `json:"email,omitempty"`
	Name       string `json:"name,omitempty"`
	PictureURL string `json:"picture_url,omitempty"`
	Provider   string `json:"provider"`
	AuthMethod string `json:"auth_method"`
	IssuedAt   int64  `json:"iat"`
	ExpiresAt  int64  `json:"exp"`
	Issuer     string `json:"iss"`
}

type AuthManager struct {
	mode               string
	apiKeyEnabled      bool
	apiKey             string
	oidcEnabled        bool
	oauthConfig        oauth2.Config
	verifier           *oidc.IDTokenVerifier
	logger             *slog.Logger
	cookieName         string
	cookieDomain       string
	cookieSecure       bool
	cookieSameSite     http.SameSite
	cookieTTL          time.Duration
	signingKey         []byte
	postLoginRedirect  string
	postLogoutRedirect string
	providerLogoutURL  string
	tokenIssuer        string
	bearerTokenTTL     time.Duration
	stateTTL           time.Duration
	stateMu            sync.Mutex
	states             map[string]oidcState
}

func NewAuthManager(cfg *config.Config, logger *slog.Logger) (*AuthManager, error) {
	auth := cfg.API.Auth
	auth.Normalize()

	manager := &AuthManager{
		mode:               auth.Mode,
		apiKeyEnabled:      auth.APIKeyAuth.Enabled || auth.APIKeyAuth.Key != "",
		apiKey:             auth.APIKeyAuth.Key,
		oidcEnabled:        auth.OIDC.Enabled,
		logger:             logger,
		cookieName:         auth.OIDC.Cookie.Name,
		cookieDomain:       auth.OIDC.Cookie.Domain,
		cookieSecure:       auth.OIDC.Cookie.Secure,
		cookieSameSite:     parseSameSite(auth.OIDC.Cookie.SameSite),
		cookieTTL:          auth.OIDC.Cookie.TTL,
		postLoginRedirect:  auth.OIDC.PostLoginRedirect,
		postLogoutRedirect: auth.OIDC.PostLogoutRedirect,
		providerLogoutURL:  auth.OIDC.LogoutURL,
		tokenIssuer:        strings.TrimSpace(cfg.API.BaseURL),
		bearerTokenTTL:     auth.OIDC.BearerTokenTTL,
		stateTTL:           10 * time.Minute,
		states:             map[string]oidcState{},
	}
	if manager.tokenIssuer == "" {
		manager.tokenIssuer = fmt.Sprintf("http://%s:%d", cfg.API.Host, cfg.API.Port)
	}

	if manager.oidcEnabled {
		ctx := context.Background()
		provider, err := oidc.NewProvider(ctx, auth.OIDC.IssuerURL)
		if err != nil {
			return nil, fmt.Errorf("initialize oidc provider: %w", err)
		}
		manager.oauthConfig = oauth2.Config{
			ClientID:     auth.OIDC.ClientID,
			ClientSecret: auth.OIDC.ClientSecret,
			Endpoint:     provider.Endpoint(),
			RedirectURL:  auth.OIDC.RedirectURL,
			Scopes:       auth.OIDC.Scopes,
		}
		manager.verifier = provider.Verifier(&oidc.Config{ClientID: auth.OIDC.ClientID})
		manager.signingKey = []byte(auth.OIDC.SessionSigningKey)
	}

	return manager, nil
}

func (a *AuthManager) Enabled() bool {
	return a.mode != "disabled"
}

func (a *AuthManager) IsPublicPath(path string) bool {
	switch path {
	case "/healthz", "/readyz", "/v1/auth/login", "/v1/auth/callback", "/v1/auth/logout":
		return true
	default:
		return false
	}
}

func (a *AuthManager) Middleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !a.Enabled() || a.IsPublicPath(r.URL.Path) {
			next.ServeHTTP(w, r)
			return
		}

		if principal, ok := a.principalFromSession(r); ok {
			next.ServeHTTP(w, r.WithContext(context.WithValue(r.Context(), principalContextKey{}, principal)))
			return
		}

		if principal, ok := a.principalFromBearerToken(r); ok {
			next.ServeHTTP(w, r.WithContext(context.WithValue(r.Context(), principalContextKey{}, principal)))
			return
		}

		if principal, ok := a.principalFromAPIKey(r); ok {
			next.ServeHTTP(w, r.WithContext(context.WithValue(r.Context(), principalContextKey{}, principal)))
			return
		}

		a.logger.Warn("unauthorized request", "path", r.URL.Path, "remote", r.RemoteAddr)
		writeJSON(w, http.StatusUnauthorized, ErrorResponse{
			Error:   "unauthorized",
			Message: "Authentication required",
			Code:    http.StatusUnauthorized,
		})
	})
}

func (a *AuthManager) principalFromBearerToken(r *http.Request) (Principal, bool) {
	authz := strings.TrimSpace(r.Header.Get("Authorization"))
	if !strings.HasPrefix(strings.ToLower(authz), "bearer ") {
		return Principal{}, false
	}
	token := strings.TrimSpace(authz[7:])
	if token == "" {
		return Principal{}, false
	}
	principal, err := a.verifyBearerToken(token)
	if err != nil {
		return Principal{}, false
	}
	return principal, true
}

func (a *AuthManager) principalFromAPIKey(r *http.Request) (Principal, bool) {
	if !a.apiKeyEnabled || a.apiKey == "" {
		return Principal{}, false
	}

	provided := strings.TrimSpace(r.Header.Get("X-API-Key"))
	if provided == "" {
		authz := strings.TrimSpace(r.Header.Get("Authorization"))
		if strings.HasPrefix(strings.ToLower(authz), "bearer ") {
			provided = strings.TrimSpace(authz[7:])
		} else {
			provided = authz
		}
	}
	if provided == "" || provided != a.apiKey {
		return Principal{}, false
	}

	return Principal{
		Subject:    "api-key",
		Name:       "API Key",
		Provider:   "api_key",
		AuthMethod: "api_key",
	}, true
}

func (a *AuthManager) principalFromSession(r *http.Request) (Principal, bool) {
	if !a.oidcEnabled {
		return Principal{}, false
	}
	cookie, err := r.Cookie(a.cookieName)
	if err != nil || cookie.Value == "" {
		return Principal{}, false
	}
	principal, err := a.verifySession(cookie.Value)
	if err != nil {
		return Principal{}, false
	}
	return principal, true
}

func (a *AuthManager) StartLogin(w http.ResponseWriter, r *http.Request) {
	if !a.oidcEnabled {
		writeJSON(w, http.StatusNotFound, ErrorResponse{
			Error:   "not_found",
			Message: "OIDC is not enabled",
			Code:    http.StatusNotFound,
		})
		return
	}

	state := randomToken(32)
	nonce := randomToken(32)
	next := resolveRedirectTarget(r.URL.Query().Get("next"), a.postLoginRedirect)

	a.stateMu.Lock()
	a.states[state] = oidcState{
		Nonce:     nonce,
		Next:      next,
		ExpiresAt: time.Now().Add(a.stateTTL),
	}
	a.stateMu.Unlock()

	url := a.oauthConfig.AuthCodeURL(state, oidc.Nonce(nonce))
	http.Redirect(w, r, url, http.StatusFound)
}

func (a *AuthManager) CompleteLogin(w http.ResponseWriter, r *http.Request) {
	if !a.oidcEnabled {
		writeJSON(w, http.StatusNotFound, ErrorResponse{
			Error:   "not_found",
			Message: "OIDC is not enabled",
			Code:    http.StatusNotFound,
		})
		return
	}

	state := r.URL.Query().Get("state")
	code := r.URL.Query().Get("code")
	if state == "" || code == "" {
		writeJSON(w, http.StatusBadRequest, ErrorResponse{
			Error:   "invalid_request",
			Message: "Missing state or code",
			Code:    http.StatusBadRequest,
		})
		return
	}

	stored, ok := a.consumeState(state)
	if !ok {
		writeJSON(w, http.StatusBadRequest, ErrorResponse{
			Error:   "invalid_request",
			Message: "Invalid or expired OIDC state",
			Code:    http.StatusBadRequest,
		})
		return
	}

	token, err := a.oauthConfig.Exchange(r.Context(), code)
	if err != nil {
		writeJSON(w, http.StatusUnauthorized, ErrorResponse{
			Error:   "unauthorized",
			Message: "Failed to exchange OIDC code",
			Code:    http.StatusUnauthorized,
		})
		return
	}

	rawIDToken, _ := token.Extra("id_token").(string)
	if rawIDToken == "" {
		writeJSON(w, http.StatusUnauthorized, ErrorResponse{
			Error:   "unauthorized",
			Message: "Missing id_token in callback response",
			Code:    http.StatusUnauthorized,
		})
		return
	}

	idToken, err := a.verifier.Verify(r.Context(), rawIDToken)
	if err != nil {
		writeJSON(w, http.StatusUnauthorized, ErrorResponse{
			Error:   "unauthorized",
			Message: "Invalid OIDC id_token",
			Code:    http.StatusUnauthorized,
		})
		return
	}

	var claims struct {
		Subject string `json:"sub"`
		Email   string `json:"email"`
		Name    string `json:"name"`
		Picture string `json:"picture"`
		Nonce   string `json:"nonce"`
	}
	if err := idToken.Claims(&claims); err != nil {
		writeJSON(w, http.StatusUnauthorized, ErrorResponse{
			Error:   "unauthorized",
			Message: "Failed to read OIDC claims",
			Code:    http.StatusUnauthorized,
		})
		return
	}

	if claims.Nonce != "" && claims.Nonce != stored.Nonce {
		writeJSON(w, http.StatusUnauthorized, ErrorResponse{
			Error:   "unauthorized",
			Message: "OIDC nonce mismatch",
			Code:    http.StatusUnauthorized,
		})
		return
	}
	if claims.Subject == "" {
		writeJSON(w, http.StatusUnauthorized, ErrorResponse{
			Error:   "unauthorized",
			Message: "OIDC subject claim is missing",
			Code:    http.StatusUnauthorized,
		})
		return
	}

	principal := Principal{
		Subject:    claims.Subject,
		Email:      claims.Email,
		Name:       claims.Name,
		PictureURL: claims.Picture,
		Provider:   "oidc",
		AuthMethod: "oidc",
	}

	session, err := a.signSession(principal)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, ErrorResponse{
			Error:   "internal_error",
			Message: "Failed to create session",
			Code:    http.StatusInternalServerError,
		})
		return
	}

	a.setSessionCookie(w, session)

	if strings.EqualFold(strings.TrimSpace(r.URL.Query().Get("format")), "json") {
		token, expiresAt, err := a.issueBearerToken(principal)
		if err != nil {
			writeJSON(w, http.StatusInternalServerError, ErrorResponse{
				Error:   "internal_error",
				Message: "Failed to create token",
				Code:    http.StatusInternalServerError,
			})
			return
		}
		writeJSON(w, http.StatusOK, AuthCallbackTokenResponse{
			AccessToken: token,
			TokenType:   "Bearer",
			ExpiresIn:   int(time.Until(expiresAt).Seconds()),
			ExpiresAt:   expiresAt.UTC(),
			User:        principal,
		})
		return
	}

	http.Redirect(w, r, stored.Next, http.StatusFound)
}

func (a *AuthManager) Logout(w http.ResponseWriter) {
	a.clearSessionCookie(w)
}

func (a *AuthManager) LogoutRedirect(w http.ResponseWriter, r *http.Request) {
	a.clearSessionCookie(w)

	if target := strings.TrimSpace(a.providerLogoutURL); target != "" {
		if parsed, err := urlpkg.Parse(target); err == nil && parsed.IsAbs() {
			http.Redirect(w, r, target, http.StatusFound)
			return
		}
		a.logger.Warn("invalid configured provider logout url", "logout_url", target)
	}

	target := resolveRedirectTarget(r.URL.Query().Get("next"), a.postLogoutRedirect)
	http.Redirect(w, r, target, http.StatusFound)
}

func (a *AuthManager) setSessionCookie(w http.ResponseWriter, value string) {
	http.SetCookie(w, &http.Cookie{
		Name:     a.cookieName,
		Value:    value,
		Path:     "/",
		Domain:   a.cookieDomain,
		HttpOnly: true,
		Secure:   a.cookieSecure,
		SameSite: a.cookieSameSite,
		MaxAge:   int(a.cookieTTL.Seconds()),
	})
}

func (a *AuthManager) clearSessionCookie(w http.ResponseWriter) {
	http.SetCookie(w, &http.Cookie{
		Name:     a.cookieName,
		Value:    "",
		Path:     "/",
		Domain:   a.cookieDomain,
		HttpOnly: true,
		Secure:   a.cookieSecure,
		SameSite: a.cookieSameSite,
		MaxAge:   -1,
		Expires:  time.Unix(0, 0),
	})
}

func PrincipalFromContext(ctx context.Context) (Principal, bool) {
	principal, ok := ctx.Value(principalContextKey{}).(Principal)
	return principal, ok
}

func ActorFromRequest(r *http.Request) string {
	principal, ok := PrincipalFromContext(r.Context())
	if !ok {
		return "unknown"
	}
	if principal.Email != "" {
		return principal.Email
	}
	if principal.Name != "" {
		return principal.Name
	}
	if principal.Subject != "" {
		return principal.Subject
	}
	return "unknown"
}

func (a *AuthManager) consumeState(state string) (oidcState, bool) {
	a.stateMu.Lock()
	defer a.stateMu.Unlock()

	now := time.Now()
	for k, v := range a.states {
		if now.After(v.ExpiresAt) {
			delete(a.states, k)
		}
	}

	value, ok := a.states[state]
	if !ok {
		return oidcState{}, false
	}
	delete(a.states, state)
	return value, true
}

func (a *AuthManager) signSession(principal Principal) (string, error) {
	if len(a.signingKey) == 0 {
		return "", errors.New("missing signing key")
	}
	claims := sessionClaims{
		Principal: principal,
		Exp:       time.Now().Add(a.cookieTTL).Unix(),
	}
	payload, err := json.Marshal(claims)
	if err != nil {
		return "", err
	}
	payloadEncoded := base64.RawURLEncoding.EncodeToString(payload)

	mac := hmac.New(sha256.New, a.signingKey)
	mac.Write([]byte(payloadEncoded))
	signature := base64.RawURLEncoding.EncodeToString(mac.Sum(nil))
	return payloadEncoded + "." + signature, nil
}

func (a *AuthManager) verifySession(token string) (Principal, error) {
	if len(a.signingKey) == 0 {
		return Principal{}, errors.New("missing signing key")
	}
	parts := strings.Split(token, ".")
	if len(parts) != 2 {
		return Principal{}, errors.New("invalid token format")
	}

	payloadEncoded := parts[0]
	signatureEncoded := parts[1]

	mac := hmac.New(sha256.New, a.signingKey)
	mac.Write([]byte(payloadEncoded))
	expected := mac.Sum(nil)

	actual, err := base64.RawURLEncoding.DecodeString(signatureEncoded)
	if err != nil {
		return Principal{}, errors.New("invalid token signature encoding")
	}
	if !hmac.Equal(expected, actual) {
		return Principal{}, errors.New("invalid token signature")
	}

	payload, err := base64.RawURLEncoding.DecodeString(payloadEncoded)
	if err != nil {
		return Principal{}, errors.New("invalid token payload encoding")
	}

	var claims sessionClaims
	if err := json.Unmarshal(payload, &claims); err != nil {
		return Principal{}, errors.New("invalid token payload")
	}
	if time.Now().Unix() >= claims.Exp {
		return Principal{}, errors.New("session expired")
	}
	return claims.Principal, nil
}

func (a *AuthManager) issueBearerToken(principal Principal) (string, time.Time, error) {
	if len(a.signingKey) == 0 {
		return "", time.Time{}, errors.New("missing signing key")
	}
	now := time.Now().UTC()
	expiresAt := now.Add(a.bearerTokenTTL)
	claims := jwtClaims{
		Subject:    principal.Subject,
		Email:      principal.Email,
		Name:       principal.Name,
		PictureURL: principal.PictureURL,
		Provider:   principal.Provider,
		AuthMethod: principal.AuthMethod,
		IssuedAt:   now.Unix(),
		ExpiresAt:  expiresAt.Unix(),
		Issuer:     a.tokenIssuer,
	}

	header := base64.RawURLEncoding.EncodeToString([]byte(`{"alg":"HS256","typ":"JWT"}`))
	payloadBytes, err := json.Marshal(claims)
	if err != nil {
		return "", time.Time{}, err
	}
	payload := base64.RawURLEncoding.EncodeToString(payloadBytes)
	unsigned := header + "." + payload

	mac := hmac.New(sha256.New, a.signingKey)
	mac.Write([]byte(unsigned))
	signature := base64.RawURLEncoding.EncodeToString(mac.Sum(nil))

	return unsigned + "." + signature, expiresAt, nil
}

func (a *AuthManager) verifyBearerToken(token string) (Principal, error) {
	if len(a.signingKey) == 0 {
		return Principal{}, errors.New("missing signing key")
	}
	parts := strings.Split(token, ".")
	if len(parts) != 3 {
		return Principal{}, errors.New("invalid token format")
	}

	unsigned := parts[0] + "." + parts[1]
	mac := hmac.New(sha256.New, a.signingKey)
	mac.Write([]byte(unsigned))
	expected := mac.Sum(nil)

	actual, err := base64.RawURLEncoding.DecodeString(parts[2])
	if err != nil {
		return Principal{}, errors.New("invalid token signature encoding")
	}
	if !hmac.Equal(expected, actual) {
		return Principal{}, errors.New("invalid token signature")
	}

	payload, err := base64.RawURLEncoding.DecodeString(parts[1])
	if err != nil {
		return Principal{}, errors.New("invalid token payload encoding")
	}

	var claims jwtClaims
	if err := json.Unmarshal(payload, &claims); err != nil {
		return Principal{}, errors.New("invalid token payload")
	}
	if claims.Subject == "" {
		return Principal{}, errors.New("missing subject")
	}
	if claims.ExpiresAt == 0 || time.Now().Unix() >= claims.ExpiresAt {
		return Principal{}, errors.New("token expired")
	}
	if claims.Issuer != "" && a.tokenIssuer != "" && claims.Issuer != a.tokenIssuer {
		return Principal{}, errors.New("invalid token issuer")
	}

	return Principal{
		Subject:    claims.Subject,
		Email:      claims.Email,
		Name:       claims.Name,
		PictureURL: claims.PictureURL,
		Provider:   claims.Provider,
		AuthMethod: claims.AuthMethod,
	}, nil
}

func parseSameSite(value string) http.SameSite {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "strict":
		return http.SameSiteStrictMode
	case "none":
		return http.SameSiteNoneMode
	default:
		return http.SameSiteLaxMode
	}
}

func randomToken(size int) string {
	b := make([]byte, size)
	if _, err := rand.Read(b); err != nil {
		return fmt.Sprintf("%d", time.Now().UnixNano())
	}
	return base64.RawURLEncoding.EncodeToString(b)
}

func resolveRedirectTarget(next, fallback string) string {
	base := strings.TrimSpace(fallback)
	if base == "" {
		base = "/"
	}

	value := strings.TrimSpace(next)
	if value == "" {
		return base
	}

	nextURL, err := urlpkg.Parse(value)
	if err != nil || nextURL.IsAbs() || nextURL.Host != "" || !strings.HasPrefix(nextURL.Path, "/") || strings.HasPrefix(value, "//") {
		return base
	}

	baseURL, err := urlpkg.Parse(base)
	if err == nil && baseURL.IsAbs() {
		return baseURL.ResolveReference(nextURL).String()
	}
	return nextURL.String()
}

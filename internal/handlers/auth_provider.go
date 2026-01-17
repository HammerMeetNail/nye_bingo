package handlers

import (
	"crypto/rand"
	"crypto/subtle"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"net/http"
	"strings"
	"time"

	"github.com/HammerMeetNail/yearofbingo/internal/models"
	"github.com/HammerMeetNail/yearofbingo/internal/services"
)

const (
	oauthStateCookieName = "oauth_state"
	oauthNonceCookieName = "oauth_nonce"
	oauthNextCookieName  = "oauth_next"
	oauthCookieMaxAge    = 10 * 60 // 10 minutes
	oauthPendingTTL      = 10 * time.Minute
)

type ProviderAuthHandler struct {
	providerAuth services.ProviderAuthServiceInterface
	authService  services.AuthServiceInterface
	redis        services.RedisClient
	providers    map[string]services.OAuthProvider
	secure       bool
}

func NewProviderAuthHandler(providerAuth services.ProviderAuthServiceInterface, authService services.AuthServiceInterface, redis services.RedisClient, providers map[services.Provider]services.OAuthProvider, secure bool) *ProviderAuthHandler {
	normalized := make(map[string]services.OAuthProvider, len(providers))
	for key, provider := range providers {
		normalized[strings.ToLower(string(key))] = provider
	}

	return &ProviderAuthHandler{
		providerAuth: providerAuth,
		authService:  authService,
		redis:        redis,
		providers:    normalized,
		secure:       secure,
	}
}

func (h *ProviderAuthHandler) ProviderStart(w http.ResponseWriter, r *http.Request) {
	provider, _ := h.getProvider(r)
	if provider == nil {
		http.NotFound(w, r)
		return
	}

	state, err := generateSecureToken(32)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "Failed to start provider auth")
		return
	}
	nonce, err := generateSecureToken(32)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "Failed to start provider auth")
		return
	}

	h.setOAuthCookie(w, oauthStateCookieName, state)
	h.setOAuthCookie(w, oauthNonceCookieName, nonce)

	if next := sanitizeNext(r.URL.Query().Get("next")); next != "" {
		h.setOAuthCookie(w, oauthNextCookieName, next)
	} else {
		h.clearOAuthCookie(w, oauthNextCookieName)
	}

	redirectURL := provider.AuthCodeURL(state, nonce)
	http.Redirect(w, r, redirectURL, http.StatusFound)
}

func (h *ProviderAuthHandler) ProviderCallback(w http.ResponseWriter, r *http.Request) {
	provider, providerKey := h.getProvider(r)
	if provider == nil {
		http.NotFound(w, r)
		return
	}

	if providerErr := r.URL.Query().Get("error"); providerErr != "" {
		h.redirectToLoginError(w, r, providerErr)
		return
	}

	code := r.URL.Query().Get("code")
	state := r.URL.Query().Get("state")
	if code == "" || state == "" {
		h.redirectToLoginError(w, r, "oauth_missing")
		return
	}

	stateCookie, err := r.Cookie(oauthStateCookieName)
	if err != nil || !secureCompare(stateCookie.Value, state) {
		h.redirectToLoginError(w, r, "oauth_invalid")
		return
	}

	nonceCookie, err := r.Cookie(oauthNonceCookieName)
	if err != nil || nonceCookie.Value == "" {
		h.redirectToLoginError(w, r, "oauth_invalid")
		return
	}

	claims, err := provider.ExchangeAndVerify(r.Context(), code, nonceCookie.Value)
	if err != nil {
		log.Printf("Provider exchange failed: %v", err)
		h.redirectToLoginError(w, r, "oauth_exchange")
		return
	}

	linkResult, err := h.providerAuth.LinkOrFindUserFromProvider(r.Context(), claims)
	if err != nil {
		if errors.Is(err, services.ErrProviderEmailUnverified) {
			h.redirectToLoginError(w, r, "oauth_unverified")
			return
		}
		log.Printf("Provider link failed: %v", err)
		h.redirectToLoginError(w, r, "oauth_link")
		return
	}

	h.clearOAuthCookie(w, oauthStateCookieName)
	h.clearOAuthCookie(w, oauthNonceCookieName)

	if linkResult.User != nil {
		token, err := h.authService.CreateSession(r.Context(), linkResult.User.ID)
		if err != nil {
			log.Printf("Provider session failed: %v", err)
			writeError(w, http.StatusInternalServerError, "Internal server error")
			return
		}
		h.setSessionCookie(w, token)
		next := h.readOAuthNext(r)
		h.clearOAuthCookie(w, oauthNextCookieName)
		http.Redirect(w, r, h.redirectTarget(next, "#dashboard"), http.StatusFound)
		return
	}

	if linkResult.Pending == nil {
		writeError(w, http.StatusInternalServerError, "Internal server error")
		return
	}

	pendingToken, err := generateSecureToken(32)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "Failed to start provider auth")
		return
	}

	pendingRecord := providerPendingRecord{
		Provider: string(linkResult.Pending.Provider),
		Subject:  linkResult.Pending.Subject,
		Email:    linkResult.Pending.Email,
	}
	payload, err := json.Marshal(pendingRecord)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "Failed to start provider auth")
		return
	}

	pendingKey := providerPendingRedisKey(pendingToken)
	if err := h.redis.Set(r.Context(), pendingKey, string(payload), oauthPendingTTL); err != nil {
		log.Printf("Provider pending save failed: %v", err)
		writeError(w, http.StatusInternalServerError, "Internal server error")
		return
	}

	h.setOAuthCookie(w, providerPendingCookieName(providerKey), pendingToken)
	http.Redirect(w, r, h.redirectTarget("", fmt.Sprintf("#%s-complete", providerKey)), http.StatusFound)
}

func (h *ProviderAuthHandler) ProviderComplete(w http.ResponseWriter, r *http.Request) {
	provider, providerKey := h.getProvider(r)
	if provider == nil {
		http.NotFound(w, r)
		return
	}

	if GetUserFromContext(r.Context()) != nil {
		writeError(w, http.StatusBadRequest, "Already authenticated")
		return
	}

	pendingCookie, err := r.Cookie(providerPendingCookieName(providerKey))
	if err != nil || pendingCookie.Value == "" {
		writeError(w, http.StatusBadRequest, "Signup session expired. Please restart OAuth login.")
		return
	}

	pendingKey := providerPendingRedisKey(pendingCookie.Value)
	pendingJSON, err := h.redis.Get(r.Context(), pendingKey)
	if err != nil || pendingJSON == "" {
		writeError(w, http.StatusBadRequest, "Signup session expired. Please restart OAuth login.")
		return
	}

	var pending providerPendingRecord
	if err := json.Unmarshal([]byte(pendingJSON), &pending); err != nil {
		writeError(w, http.StatusBadRequest, "Signup session expired. Please restart OAuth login.")
		return
	}
	if pending.Provider != string(provider.Provider()) {
		writeError(w, http.StatusBadRequest, "Invalid signup session. Please restart OAuth login.")
		return
	}

	var req struct {
		Username   string `json:"username"`
		Searchable bool   `json:"searchable"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "Invalid request body")
		return
	}

	user, err := h.providerAuth.CreateUserFromProviderPending(r.Context(), services.PendingProviderUser{
		Provider: provider.Provider(),
		Subject:  pending.Subject,
		Email:    pending.Email,
	}, req.Username, req.Searchable)
	if err != nil {
		switch {
		case errors.Is(err, services.ErrUsernameAlreadyExists):
			writeError(w, http.StatusConflict, "Username already taken")
		case errors.Is(err, services.ErrEmailAlreadyExists):
			writeError(w, http.StatusConflict, "Email already registered")
		case errors.Is(err, services.ErrInvalidUsername):
			writeError(w, http.StatusBadRequest, "Username must be between 2 and 100 characters")
		case errors.Is(err, services.ErrInvalidProviderPending):
			writeError(w, http.StatusBadRequest, "Signup session expired. Please restart OAuth login.")
		default:
			log.Printf("Provider complete failed: %v", err)
			writeError(w, http.StatusInternalServerError, "Internal server error")
		}
		return
	}

	token, err := h.authService.CreateSession(r.Context(), user.ID)
	if err != nil {
		log.Printf("Provider session failed: %v", err)
		writeError(w, http.StatusInternalServerError, "Internal server error")
		return
	}

	h.setSessionCookie(w, token)
	h.clearOAuthCookie(w, providerPendingCookieName(providerKey))
	h.clearOAuthCookie(w, oauthNextCookieName)
	_ = h.redis.Del(r.Context(), pendingKey)

	next := h.readOAuthNext(r)
	writeJSON(w, http.StatusCreated, providerCompleteResponse{
		User: user,
		Next: next,
	})
}

type providerCompleteResponse struct {
	User *models.User `json:"user"`
	Next string       `json:"next,omitempty"`
}

type providerPendingRecord struct {
	Provider string `json:"provider"`
	Subject  string `json:"subject"`
	Email    string `json:"email"`
}

func providerPendingCookieName(provider string) string {
	return provider + "_pending"
}

func providerPendingRedisKey(token string) string {
	return "oauth_pending:" + token
}

func (h *ProviderAuthHandler) getProvider(r *http.Request) (services.OAuthProvider, string) {
	providerKey := strings.ToLower(r.PathValue("provider"))
	if providerKey == "" {
		return nil, ""
	}
	provider, ok := h.providers[providerKey]
	if !ok {
		return nil, providerKey
	}
	return provider, providerKey
}

func (h *ProviderAuthHandler) setOAuthCookie(w http.ResponseWriter, name, value string) {
	http.SetCookie(w, &http.Cookie{
		Name:     name,
		Value:    value,
		Path:     "/",
		MaxAge:   oauthCookieMaxAge,
		HttpOnly: true,
		Secure:   h.secure,
		SameSite: http.SameSiteLaxMode,
	})
}

func (h *ProviderAuthHandler) clearOAuthCookie(w http.ResponseWriter, name string) {
	http.SetCookie(w, &http.Cookie{
		Name:     name,
		Value:    "",
		Path:     "/",
		MaxAge:   -1,
		HttpOnly: true,
		Secure:   h.secure,
		SameSite: http.SameSiteLaxMode,
		Expires:  time.Unix(0, 0),
	})
}

func (h *ProviderAuthHandler) setSessionCookie(w http.ResponseWriter, token string) {
	http.SetCookie(w, &http.Cookie{
		Name:     sessionCookieName,
		Value:    token,
		Path:     "/",
		MaxAge:   cookieMaxAge,
		HttpOnly: true,
		Secure:   h.secure,
		SameSite: http.SameSiteStrictMode,
	})
}

func (h *ProviderAuthHandler) redirectToLoginError(w http.ResponseWriter, r *http.Request, code string) {
	message := sanitizeErrorParam(code)
	http.Redirect(w, r, "/#login?error="+message, http.StatusFound)
}

func (h *ProviderAuthHandler) readOAuthNext(r *http.Request) string {
	nextCookie, err := r.Cookie(oauthNextCookieName)
	if err != nil {
		return ""
	}
	return sanitizeNext(nextCookie.Value)
}

func (h *ProviderAuthHandler) redirectTarget(next, fallback string) string {
	if next != "" {
		return "/" + next
	}
	return "/" + fallback
}

func generateSecureToken(size int) (string, error) {
	data := make([]byte, size)
	if _, err := rand.Read(data); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(data), nil
}

func secureCompare(a, b string) bool {
	if a == "" || b == "" {
		return false
	}
	return subtle.ConstantTimeCompare([]byte(a), []byte(b)) == 1
}

func sanitizeNext(value string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return ""
	}
	if !strings.HasPrefix(value, "#") {
		return ""
	}
	if strings.Contains(value, "\n") || strings.Contains(value, "\r") {
		return ""
	}
	return value
}

func sanitizeErrorParam(value string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return "oauth_error"
	}
	if len(value) > 60 {
		value = value[:60]
	}
	for _, r := range value {
		if !isAllowedErrorRune(r) {
			return "oauth_error"
		}
	}
	return value
}

func isAllowedErrorRune(r rune) bool {
	return r == '-' || r == '_' ||
		(r >= 'a' && r <= 'z') ||
		(r >= 'A' && r <= 'Z') ||
		(r >= '0' && r <= '9')
}

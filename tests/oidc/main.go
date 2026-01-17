package main

import (
	"crypto"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"math/big"
	"net/http"
	"net/url"
	"os"
	"strings"
	"sync"
	"time"
)

const (
	defaultIssuer       = "http://oidc:5555"
	defaultClientID     = "oidc-test"
	defaultClientSecret = "oidc-secret"
	defaultRedirectURI  = "http://app:8080/api/auth/google/callback"
)

type nextUser struct {
	Email         string `json:"email"`
	EmailVerified bool   `json:"email_verified"`
	Sub           string `json:"sub"`
}

type authCodeData struct {
	User        nextUser
	Nonce       string
	ClientID    string
	RedirectURI string
	IssuedAt    time.Time
}

type server struct {
	issuer       string
	clientID     string
	clientSecret string
	redirectURI  string
	privateKey   *rsa.PrivateKey
	keyID        string

	mu        sync.Mutex
	nextUser  *nextUser
	authCodes map[string]authCodeData
}

func main() {
	srv := newServer()

	http.HandleFunc("/.well-known/openid-configuration", srv.handleWellKnown)
	http.HandleFunc("/authorize", srv.handleAuthorize)
	http.HandleFunc("/token", srv.handleToken)
	http.HandleFunc("/keys", srv.handleKeys)
	http.HandleFunc("/test/next-user", srv.handleNextUser)

	addr := ":5555"
	server := &http.Server{
		Addr:              addr,
		ReadTimeout:       10 * time.Second,
		ReadHeaderTimeout: 5 * time.Second,
		WriteTimeout:      10 * time.Second,
		IdleTimeout:       30 * time.Second,
	}

	log.Printf("OIDC test server listening on %s", addr)
	if err := server.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
		log.Fatal(err)
	}
}

func newServer() *server {
	issuer := getEnv("OIDC_ISSUER_URL", defaultIssuer)
	clientID := getEnv("OIDC_CLIENT_ID", defaultClientID)
	clientSecret := getEnv("OIDC_CLIENT_SECRET", defaultClientSecret)
	redirectURI := getEnv("OIDC_REDIRECT_URI", defaultRedirectURI)

	privateKey, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		log.Fatalf("failed to generate RSA key: %v", err)
	}

	keyID, err := randomToken(8)
	if err != nil {
		log.Fatalf("failed to generate key id: %v", err)
	}

	return &server{
		issuer:       issuer,
		clientID:     clientID,
		clientSecret: clientSecret,
		redirectURI:  redirectURI,
		privateKey:   privateKey,
		keyID:        keyID,
		authCodes:    map[string]authCodeData{},
	}
}

func (s *server) handleWellKnown(w http.ResponseWriter, r *http.Request) {
	cfg := map[string]any{
		"issuer":                                s.issuer,
		"authorization_endpoint":                s.issuer + "/authorize",
		"token_endpoint":                        s.issuer + "/token",
		"jwks_uri":                              s.issuer + "/keys",
		"response_types_supported":              []string{"code"},
		"subject_types_supported":               []string{"public"},
		"id_token_signing_alg_values_supported": []string{"RS256"},
		"scopes_supported":                      []string{"openid", "email", "profile"},
		"token_endpoint_auth_methods_supported": []string{"client_secret_basic", "client_secret_post"},
	}
	writeJSON(w, cfg)
}

func (s *server) handleAuthorize(w http.ResponseWriter, r *http.Request) {
	query := r.URL.Query()
	if query.Get("response_type") != "code" {
		http.Error(w, "unsupported response_type", http.StatusBadRequest)
		return
	}
	clientID := query.Get("client_id")
	redirectURI := query.Get("redirect_uri")
	state := query.Get("state")
	nonce := query.Get("nonce")

	if clientID == "" || redirectURI == "" || state == "" {
		http.Error(w, "missing parameters", http.StatusBadRequest)
		return
	}
	if s.redirectURI != "" && redirectURI != s.redirectURI {
		http.Error(w, "invalid redirect_uri", http.StatusBadRequest)
		return
	}

	user := s.consumeNextUser()
	if user.Email == "" {
		http.Error(w, "missing test user", http.StatusBadRequest)
		return
	}
	if user.Sub == "" {
		user.Sub = randomSubject()
	}

	code, err := randomToken(16)
	if err != nil {
		http.Error(w, "failed to generate code", http.StatusInternalServerError)
		return
	}

	s.mu.Lock()
	s.authCodes[code] = authCodeData{
		User:        user,
		Nonce:       nonce,
		ClientID:    clientID,
		RedirectURI: redirectURI,
		IssuedAt:    time.Now(),
	}
	s.mu.Unlock()

	redirectTarget := s.redirectURI
	if redirectTarget == "" {
		http.Error(w, "redirect_uri not configured", http.StatusBadRequest)
		return
	}
	redirect, err := url.Parse(redirectTarget)
	if err != nil {
		http.Error(w, "invalid redirect_uri", http.StatusBadRequest)
		return
	}
	params := redirect.Query()
	params.Set("code", code)
	params.Set("state", state)
	redirect.RawQuery = params.Encode()
	http.Redirect(w, r, redirect.String(), http.StatusFound)
}

func (s *server) handleToken(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		http.Error(w, "invalid form", http.StatusBadRequest)
		return
	}
	if r.Form.Get("grant_type") != "authorization_code" {
		http.Error(w, "unsupported grant_type", http.StatusBadRequest)
		return
	}

	code := r.Form.Get("code")
	clientID := r.Form.Get("client_id")
	redirectURI := r.Form.Get("redirect_uri")

	if clientID == "" {
		if id, _, ok := parseBasicAuth(r.Header.Get("Authorization")); ok {
			clientID = id
		}
	}

	s.mu.Lock()
	data, ok := s.authCodes[code]
	if ok {
		delete(s.authCodes, code)
	}
	s.mu.Unlock()
	if !ok {
		http.Error(w, "invalid code", http.StatusBadRequest)
		return
	}

	expectedClientID := data.ClientID
	if expectedClientID == "" {
		expectedClientID = s.clientID
	}
	if clientID != "" && expectedClientID != "" && clientID != expectedClientID {
		http.Error(w, "invalid client_id", http.StatusUnauthorized)
		return
	}

	// For test flows, accept any client_secret as long as the auth code is valid.
	if data.RedirectURI != "" && redirectURI != "" && data.RedirectURI != redirectURI {
		http.Error(w, "redirect_uri mismatch", http.StatusBadRequest)
		return
	}

	idToken, err := s.issueIDToken(data)
	if err != nil {
		http.Error(w, "failed to issue token", http.StatusInternalServerError)
		return
	}

	accessToken, err := randomToken(16)
	if err != nil {
		http.Error(w, "failed to issue token", http.StatusInternalServerError)
		return
	}

	response := map[string]any{
		"access_token": accessToken,
		"token_type":   "Bearer",
		"expires_in":   600,
		"id_token":     idToken,
	}
	writeJSON(w, response)
}

func (s *server) handleKeys(w http.ResponseWriter, r *http.Request) {
	n := base64.RawURLEncoding.EncodeToString(s.privateKey.N.Bytes())
	e := base64.RawURLEncoding.EncodeToString(big.NewInt(int64(s.privateKey.PublicKey.E)).Bytes())
	keys := map[string]any{
		"keys": []map[string]any{
			{
				"kty": "RSA",
				"use": "sig",
				"alg": "RS256",
				"kid": s.keyID,
				"n":   n,
				"e":   e,
			},
		},
	}
	writeJSON(w, keys)
}

func (s *server) handleNextUser(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var user nextUser
	if err := json.NewDecoder(r.Body).Decode(&user); err != nil {
		http.Error(w, "invalid body", http.StatusBadRequest)
		return
	}
	user.Email = strings.TrimSpace(strings.ToLower(user.Email))
	if user.Email == "" {
		http.Error(w, "email required", http.StatusBadRequest)
		return
	}
	if user.Sub != "" {
		user.Sub = strings.TrimSpace(user.Sub)
	}

	s.mu.Lock()
	s.nextUser = &user
	s.mu.Unlock()

	w.WriteHeader(http.StatusNoContent)
}

func (s *server) issueIDToken(data authCodeData) (string, error) {
	now := time.Now()
	claims := map[string]any{
		"iss":            s.issuer,
		"sub":            data.User.Sub,
		"aud":            data.ClientID,
		"exp":            now.Add(10 * time.Minute).Unix(),
		"iat":            now.Unix(),
		"email":          data.User.Email,
		"email_verified": data.User.EmailVerified,
		"nonce":          data.Nonce,
	}

	header := map[string]any{
		"alg": "RS256",
		"typ": "JWT",
		"kid": s.keyID,
	}

	return signJWT(header, claims, s.privateKey)
}

func (s *server) consumeNextUser() nextUser {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.nextUser == nil {
		return nextUser{}
	}
	user := *s.nextUser
	s.nextUser = nil
	return user
}

func signJWT(header, claims map[string]any, key *rsa.PrivateKey) (string, error) {
	headerJSON, err := json.Marshal(header)
	if err != nil {
		return "", err
	}
	claimsJSON, err := json.Marshal(claims)
	if err != nil {
		return "", err
	}

	encodedHeader := base64.RawURLEncoding.EncodeToString(headerJSON)
	encodedClaims := base64.RawURLEncoding.EncodeToString(claimsJSON)
	signingInput := encodedHeader + "." + encodedClaims

	hash := sha256.Sum256([]byte(signingInput))
	signature, err := rsa.SignPKCS1v15(rand.Reader, key, crypto.SHA256, hash[:])
	if err != nil {
		return "", err
	}

	encodedSig := base64.RawURLEncoding.EncodeToString(signature)
	return signingInput + "." + encodedSig, nil
}

func parseBasicAuth(header string) (string, string, bool) {
	if header == "" {
		return "", "", false
	}
	parts := strings.SplitN(header, " ", 2)
	if len(parts) != 2 || !strings.EqualFold(parts[0], "Basic") {
		return "", "", false
	}
	decoded, err := base64.StdEncoding.DecodeString(parts[1])
	if err != nil {
		return "", "", false
	}
	creds := strings.SplitN(string(decoded), ":", 2)
	if len(creds) != 2 {
		return "", "", false
	}
	return creds[0], creds[1], true
}

func randomToken(length int) (string, error) {
	if length <= 0 {
		return "", errors.New("invalid length")
	}
	b := make([]byte, length)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(b), nil
}

func randomSubject() string {
	token, err := randomToken(12)
	if err != nil {
		return fmt.Sprintf("sub-%d", time.Now().UnixNano())
	}
	return "sub-" + token
}

func writeJSON(w http.ResponseWriter, payload any) {
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(payload)
}

func getEnv(key, fallback string) string {
	if value, ok := os.LookupEnv(key); ok {
		return value
	}
	return fallback
}

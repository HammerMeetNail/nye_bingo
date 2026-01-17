package services

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/coreos/go-oidc/v3/oidc"
	"golang.org/x/oauth2"
)

type Provider string

const (
	ProviderGoogle Provider = "google"
)

type IdentityClaims struct {
	Provider      Provider
	Subject       string
	Email         string
	EmailVerified bool
}

type OAuthProvider interface {
	Provider() Provider
	AuthCodeURL(state, nonce string) string
	ExchangeAndVerify(ctx context.Context, code, nonce string) (IdentityClaims, error)
}

type OIDCProviderConfig struct {
	Provider     Provider
	ClientID     string
	ClientSecret string
	RedirectURL  string
	IssuerURL    string
	Scopes       []string
}

type OIDCProvider struct {
	provider    Provider
	oidc        *oidc.Provider
	verifier    *oidc.IDTokenVerifier
	oauthConfig oauth2.Config
}

func NewOIDCProvider(ctx context.Context, cfg OIDCProviderConfig) (*OIDCProvider, error) {
	if cfg.Provider == "" {
		return nil, errors.New("provider is required")
	}
	if strings.TrimSpace(cfg.ClientID) == "" || strings.TrimSpace(cfg.ClientSecret) == "" {
		return nil, errors.New("client id and secret are required")
	}
	if strings.TrimSpace(cfg.RedirectURL) == "" || strings.TrimSpace(cfg.IssuerURL) == "" {
		return nil, errors.New("redirect url and issuer url are required")
	}

	oidcProvider, err := oidc.NewProvider(ctx, cfg.IssuerURL)
	if err != nil {
		return nil, fmt.Errorf("discovering oidc provider: %w", err)
	}

	oauthConfig := oauth2.Config{
		ClientID:     cfg.ClientID,
		ClientSecret: cfg.ClientSecret,
		RedirectURL:  cfg.RedirectURL,
		Endpoint:     oidcProvider.Endpoint(),
		Scopes:       cfg.Scopes,
	}

	verifier := oidcProvider.Verifier(&oidc.Config{ClientID: cfg.ClientID})

	return &OIDCProvider{
		provider:    cfg.Provider,
		oidc:        oidcProvider,
		verifier:    verifier,
		oauthConfig: oauthConfig,
	}, nil
}

func (p *OIDCProvider) Provider() Provider {
	return p.provider
}

func (p *OIDCProvider) AuthCodeURL(state, nonce string) string {
	return p.oauthConfig.AuthCodeURL(state, oidc.Nonce(nonce))
}

func (p *OIDCProvider) ExchangeAndVerify(ctx context.Context, code, nonce string) (IdentityClaims, error) {
	token, err := p.oauthConfig.Exchange(ctx, code)
	if err != nil {
		return IdentityClaims{}, fmt.Errorf("exchanging oauth code: %w", err)
	}

	rawIDToken, ok := token.Extra("id_token").(string)
	if !ok || rawIDToken == "" {
		return IdentityClaims{}, errors.New("missing id_token in oauth response")
	}

	idToken, err := p.verifier.Verify(ctx, rawIDToken)
	if err != nil {
		return IdentityClaims{}, fmt.Errorf("verifying id token: %w", err)
	}
	if idToken.Nonce != nonce {
		return IdentityClaims{}, errors.New("nonce mismatch")
	}

	var claims struct {
		Subject       string `json:"sub"`
		Email         string `json:"email"`
		EmailVerified bool   `json:"email_verified"`
	}
	if err := idToken.Claims(&claims); err != nil {
		return IdentityClaims{}, fmt.Errorf("parsing id token claims: %w", err)
	}

	return IdentityClaims{
		Provider:      p.provider,
		Subject:       claims.Subject,
		Email:         claims.Email,
		EmailVerified: claims.EmailVerified,
	}, nil
}

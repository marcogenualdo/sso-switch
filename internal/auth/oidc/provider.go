package oidc

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/coreos/go-oidc/v3/oidc"
	"github.com/google/uuid"
	"github.com/marcogenualdo/sso-switch/internal/auth"
	"github.com/marcogenualdo/sso-switch/internal/cache"
	"github.com/marcogenualdo/sso-switch/internal/config"
	"golang.org/x/oauth2"
)

type Provider struct {
	id             string
	name           string
	cfg            config.OIDCConfig
	headerMappings map[string]string
	cache          cache.Cache

	provider      *oidc.Provider
	oauth2Config  oauth2.Config
	verifier      *oidc.IDTokenVerifier
}

func NewProvider(ctx context.Context, providerCfg config.ProviderConfig, cache cache.Cache) (*Provider, error) {
	if providerCfg.OIDC == nil {
		return nil, fmt.Errorf("OIDC config is required")
	}

	provider, err := oidc.NewProvider(ctx, providerCfg.OIDC.Issuer)
	if err != nil {
		return nil, fmt.Errorf("failed to create OIDC provider: %w", err)
	}

	oauth2Config := oauth2.Config{
		ClientID:     providerCfg.OIDC.ClientID,
		ClientSecret: providerCfg.OIDC.ClientSecret,
		Endpoint:     provider.Endpoint(),
		Scopes:       providerCfg.OIDC.Scopes,
	}

	verifier := provider.Verifier(&oidc.Config{
		ClientID: providerCfg.OIDC.ClientID,
	})

	return &Provider{
		id:             providerCfg.ID,
		name:           providerCfg.Name,
		cfg:            *providerCfg.OIDC,
		headerMappings: providerCfg.HeaderMappings,
		cache:          cache,
		provider:       provider,
		oauth2Config:   oauth2Config,
		verifier:       verifier,
	}, nil
}

func (p *Provider) ID() string {
	return p.id
}

func (p *Provider) Name() string {
	return p.name
}

func (p *Provider) Type() string {
	return "oidc"
}

func (p *Provider) GetHeaderMappings() map[string]string {
	return p.headerMappings
}

func (p *Provider) InitiateAuth(ctx context.Context, redirectURL string) (*auth.AuthRedirect, error) {
	codeVerifier, err := generateCodeVerifier()
	if err != nil {
		return nil, fmt.Errorf("failed to generate code verifier: %w", err)
	}

	codeChallenge := generateCodeChallenge(codeVerifier)

	state := uuid.New().String()

	p.oauth2Config.RedirectURL = redirectURL

	authURL := p.oauth2Config.AuthCodeURL(
		state,
		oauth2.SetAuthURLParam("code_challenge", codeChallenge),
		oauth2.SetAuthURLParam("code_challenge_method", "S256"),
	)

	if p.cfg.HD != "" {
		authURL += "&hd=" + p.cfg.HD
	}

	oidcState := &auth.OIDCState{
		State:        state,
		ProviderID:   p.id,
		CodeVerifier: codeVerifier,
		RedirectURL:  redirectURL,
		CreatedAt:    time.Now(),
	}

	stateData, err := json.Marshal(oidcState)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal state: %w", err)
	}

	return &auth.AuthRedirect{
		URL:       authURL,
		Method:    "GET",
		CacheKey:  "oidc:state:" + state,
		CacheData: stateData,
		CacheTTL:  5 * time.Minute,
	}, nil
}

func (p *Provider) HandleCallback(ctx context.Context, req *http.Request) (*auth.Session, error) {
	code := req.URL.Query().Get("code")
	state := req.URL.Query().Get("state")

	if code == "" {
		return nil, fmt.Errorf("missing code parameter")
	}
	if state == "" {
		return nil, fmt.Errorf("missing state parameter")
	}

	stateData, err := p.cache.Get(ctx, "oidc:state:"+state)
	if err != nil {
		return nil, fmt.Errorf("invalid or expired state: %w", err)
	}

	var oidcState auth.OIDCState
	if err := json.Unmarshal(stateData, &oidcState); err != nil {
		return nil, fmt.Errorf("failed to unmarshal state: %w", err)
	}

	if oidcState.ProviderID != p.id {
		return nil, fmt.Errorf("provider mismatch")
	}

	p.cache.Delete(ctx, "oidc:state:"+state)

	p.oauth2Config.RedirectURL = oidcState.RedirectURL

	oauth2Token, err := p.oauth2Config.Exchange(
		ctx,
		code,
		oauth2.SetAuthURLParam("code_verifier", oidcState.CodeVerifier),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to exchange code: %w", err)
	}

	rawIDToken, ok := oauth2Token.Extra("id_token").(string)
	if !ok {
		return nil, fmt.Errorf("no id_token in token response")
	}

	idToken, err := p.verifier.Verify(ctx, rawIDToken)
	if err != nil {
		return nil, fmt.Errorf("failed to verify ID token: %w", err)
	}

	var claims map[string]interface{}
	if err := idToken.Claims(&claims); err != nil {
		return nil, fmt.Errorf("failed to parse claims: %w", err)
	}

	sessionID := uuid.New().String()
	session := &auth.Session{
		ID:           sessionID,
		ProviderID:   p.id,
		ProviderType: "oidc",
		UserInfo:     claims,
		CreatedAt:    time.Now(),
		ExpiresAt:    oauth2Token.Expiry,
		AccessToken:  oauth2Token.AccessToken,
		RefreshToken: oauth2Token.RefreshToken,
		IDToken:      rawIDToken,
		TokenExpiry:  oauth2Token.Expiry,
		CSRFToken:    uuid.New().String(),
	}

	return session, nil
}

func (p *Provider) ValidateSession(ctx context.Context, session *auth.Session) error {
	if session.ProviderID != p.id {
		return fmt.Errorf("provider mismatch")
	}

	if time.Now().After(session.ExpiresAt) {
		return fmt.Errorf("session expired")
	}

	return nil
}

func (p *Provider) RefreshSession(ctx context.Context, session *auth.Session) (*auth.Session, error) {
	if session.RefreshToken == "" {
		return nil, fmt.Errorf("no refresh token available")
	}

	tokenSource := p.oauth2Config.TokenSource(ctx, &oauth2.Token{
		RefreshToken: session.RefreshToken,
	})

	newToken, err := tokenSource.Token()
	if err != nil {
		return nil, fmt.Errorf("failed to refresh token: %w", err)
	}

	rawIDToken, ok := newToken.Extra("id_token").(string)
	if ok {
		idToken, err := p.verifier.Verify(ctx, rawIDToken)
		if err != nil {
			return nil, fmt.Errorf("failed to verify refreshed ID token: %w", err)
		}

		var claims map[string]interface{}
		if err := idToken.Claims(&claims); err != nil {
			return nil, fmt.Errorf("failed to parse refreshed claims: %w", err)
		}

		session.UserInfo = claims
		session.IDToken = rawIDToken
	}

	session.AccessToken = newToken.AccessToken
	if newToken.RefreshToken != "" {
		session.RefreshToken = newToken.RefreshToken
	}
	session.TokenExpiry = newToken.Expiry
	session.ExpiresAt = newToken.Expiry

	return session, nil
}

func generateCodeVerifier() (string, error) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(b), nil
}

func generateCodeChallenge(verifier string) string {
	hash := sha256.Sum256([]byte(verifier))
	return base64.RawURLEncoding.EncodeToString(hash[:])
}

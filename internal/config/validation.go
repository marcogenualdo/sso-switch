package config

import (
	"fmt"
	"net/url"
	"strings"
	"time"
)

func (c *Config) Validate() error {
	if err := c.validateServer(); err != nil {
		return fmt.Errorf("server config: %w", err)
	}

	if err := c.validateBackend(); err != nil {
		return fmt.Errorf("backend config: %w", err)
	}

	if err := c.validateCache(); err != nil {
		return fmt.Errorf("cache config: %w", err)
	}

	if err := c.validateProviders(); err != nil {
		return fmt.Errorf("providers config: %w", err)
	}

	if err := c.validateLogging(); err != nil {
		return fmt.Errorf("logging config: %w", err)
	}

	return nil
}

func (c *Config) validateServer() error {
	if c.Server.Port < 1 || c.Server.Port > 65535 {
		return fmt.Errorf("invalid port: %d", c.Server.Port)
	}

	if c.Server.BaseURL == "" {
		return fmt.Errorf("base_url is required")
	}

	if _, err := url.Parse(c.Server.BaseURL); err != nil {
		return fmt.Errorf("invalid base_url: %w", err)
	}

	sameSite := strings.ToLower(c.Server.CookieSameSite)
	if sameSite != "lax" && sameSite != "strict" && sameSite != "none" {
		return fmt.Errorf("invalid cookie_same_site: %s (must be lax, strict, or none)", c.Server.CookieSameSite)
	}

	if c.Server.SessionTTL < time.Minute {
		return fmt.Errorf("session_ttl must be at least 1 minute")
	}

	return nil
}

func (c *Config) validateBackend() error {
	if c.Backend.URL == "" {
		return fmt.Errorf("url is required")
	}

	if _, err := url.Parse(c.Backend.URL); err != nil {
		return fmt.Errorf("invalid url: %w", err)
	}

	if c.Backend.Timeout < 0 {
		return fmt.Errorf("timeout must be positive")
	}

	return nil
}

func (c *Config) validateCache() error {
	if c.Cache.Type != "memory" && c.Cache.Type != "redis" {
		return fmt.Errorf("invalid type: %s (must be memory or redis)", c.Cache.Type)
	}

	if c.Cache.Type == "redis" {
		if c.Cache.Redis == nil {
			return fmt.Errorf("redis config is required when type is redis")
		}
		if c.Cache.Redis.Address == "" {
			return fmt.Errorf("redis address is required")
		}
	}

	return nil
}

func (c *Config) validateProviders() error {
	if len(c.Providers) == 0 {
		return fmt.Errorf("at least one provider is required")
	}

	ids := make(map[string]bool)
	for i, provider := range c.Providers {
		if provider.ID == "" {
			return fmt.Errorf("provider %d: id is required", i)
		}

		if ids[provider.ID] {
			return fmt.Errorf("provider %d: duplicate id: %s", i, provider.ID)
		}
		ids[provider.ID] = true

		if provider.Name == "" {
			return fmt.Errorf("provider %s: name is required", provider.ID)
		}

		if provider.Type != "oidc" && provider.Type != "saml" {
			return fmt.Errorf("provider %s: invalid type: %s (must be oidc or saml)", provider.ID, provider.Type)
		}

		if provider.Type == "oidc" {
			if err := validateOIDCConfig(provider.ID, provider.OIDC); err != nil {
				return err
			}
		}

		if provider.Type == "saml" {
			if err := validateSAMLConfig(provider.ID, provider.SAML); err != nil {
				return err
			}
		}

		if len(provider.HeaderMappings) == 0 {
			return fmt.Errorf("provider %s: at least one header mapping is required", provider.ID)
		}
	}

	return nil
}

func validateOIDCConfig(providerID string, cfg *OIDCConfig) error {
	if cfg == nil {
		return fmt.Errorf("provider %s: oidc config is required", providerID)
	}

	if cfg.Issuer == "" {
		return fmt.Errorf("provider %s: issuer is required", providerID)
	}

	if _, err := url.Parse(cfg.Issuer); err != nil {
		return fmt.Errorf("provider %s: invalid issuer URL: %w", providerID, err)
	}

	if cfg.ClientID == "" {
		return fmt.Errorf("provider %s: client_id is required", providerID)
	}

	if cfg.ClientSecret == "" {
		return fmt.Errorf("provider %s: client_secret is required", providerID)
	}

	if len(cfg.Scopes) == 0 {
		return fmt.Errorf("provider %s: at least one scope is required", providerID)
	}

	hasOpenID := false
	for _, scope := range cfg.Scopes {
		if scope == "openid" {
			hasOpenID = true
			break
		}
	}
	if !hasOpenID {
		return fmt.Errorf("provider %s: 'openid' scope is required", providerID)
	}

	return nil
}

func validateSAMLConfig(providerID string, cfg *SAMLConfig) error {
	if cfg == nil {
		return fmt.Errorf("provider %s: saml config is required", providerID)
	}

	if cfg.IDPMetadataURL == "" && cfg.IDPMetadataXML == "" {
		return fmt.Errorf("provider %s: either idp_metadata_url or idp_metadata_xml is required", providerID)
	}

	if cfg.IDPMetadataURL != "" {
		if _, err := url.Parse(cfg.IDPMetadataURL); err != nil {
			return fmt.Errorf("provider %s: invalid idp_metadata_url: %w", providerID, err)
		}
	}

	if cfg.SPEntityID == "" {
		return fmt.Errorf("provider %s: sp_entity_id is required", providerID)
	}

	if cfg.ACSURL == "" {
		return fmt.Errorf("provider %s: acs_url is required", providerID)
	}

	if _, err := url.Parse(cfg.ACSURL); err != nil {
		return fmt.Errorf("provider %s: invalid acs_url: %w", providerID, err)
	}

	if cfg.CertificatePath == "" {
		return fmt.Errorf("provider %s: certificate_path is required", providerID)
	}

	if cfg.PrivateKeyPath == "" {
		return fmt.Errorf("provider %s: private_key_path is required", providerID)
	}

	return nil
}

func (c *Config) validateLogging() error {
	level := strings.ToLower(c.Logging.Level)
	if level != "debug" && level != "info" && level != "warn" && level != "error" {
		return fmt.Errorf("invalid level: %s (must be debug, info, warn, or error)", c.Logging.Level)
	}

	format := strings.ToLower(c.Logging.Format)
	if format != "json" && format != "text" {
		return fmt.Errorf("invalid format: %s (must be json or text)", c.Logging.Format)
	}

	return nil
}

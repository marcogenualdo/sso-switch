package config

import (
	"fmt"
	"os"
	"time"

	"gopkg.in/yaml.v3"
)

type Config struct {
	Server    ServerConfig      `yaml:"server"`
	Backend   BackendConfig     `yaml:"backend"`
	Cache     CacheConfig       `yaml:"cache"`
	Providers []ProviderConfig  `yaml:"providers"`
	Logging   LoggingConfig     `yaml:"logging"`
	UI        UIConfig          `yaml:"ui"`
}

type ServerConfig struct {
	Host           string        `yaml:"host"`
	Port           int           `yaml:"port"`
	BaseURL        string        `yaml:"base_url"`
	CookieName     string        `yaml:"cookie_name"`
	CookieDomain   string        `yaml:"cookie_domain"`
	CookieSecure   bool          `yaml:"cookie_secure"`
	CookieHTTPOnly bool          `yaml:"cookie_http_only"`
	CookieSameSite string        `yaml:"cookie_same_site"`
	SessionTTL     time.Duration `yaml:"session_ttl"`
}

type BackendConfig struct {
	URL          string        `yaml:"url"`
	Timeout      time.Duration `yaml:"timeout"`
	PreserveHost bool          `yaml:"preserve_host"`
}

type CacheConfig struct {
	Type  string       `yaml:"type"`
	Redis *RedisConfig `yaml:"redis,omitempty"`
}

type RedisConfig struct {
	Address    string `yaml:"address"`
	Password   string `yaml:"password"`
	DB         int    `yaml:"db"`
	PoolSize   int    `yaml:"pool_size"`
	MaxRetries int    `yaml:"max_retries"`
}

type ProviderConfig struct {
	ID             string            `yaml:"id"`
	Name           string            `yaml:"name"`
	Type           string            `yaml:"type"`
	OIDC           *OIDCConfig       `yaml:"oidc,omitempty"`
	SAML           *SAMLConfig       `yaml:"saml,omitempty"`
	HeaderMappings map[string]string `yaml:"header_mappings"`
}

type OIDCConfig struct {
	Issuer       string   `yaml:"issuer"`
	ClientID     string   `yaml:"client_id"`
	ClientSecret string   `yaml:"client_secret"`
	Scopes       []string `yaml:"scopes"`
	HD           string   `yaml:"hd,omitempty"`
}

type SAMLConfig struct {
	IDPMetadataURL  string `yaml:"idp_metadata_url,omitempty"`
	IDPMetadataXML  string `yaml:"idp_metadata_xml,omitempty"`
	SPEntityID      string `yaml:"sp_entity_id"`
	ACSURL          string `yaml:"acs_url"`
	CertificatePath string `yaml:"certificate_path"`
	PrivateKeyPath  string `yaml:"private_key_path"`
}

type LoggingConfig struct {
	Level  string `yaml:"level"`
	Format string `yaml:"format"`
	Output string `yaml:"output"`
}

type UIConfig struct {
	Enable           *bool  `yaml:"enable"`
	Title            string `yaml:"title"`
	GradientStart    string `yaml:"gradient_start"`
	GradientEnd      string `yaml:"gradient_end"`
	LogoPath         string `yaml:"logo_path"`
}

func Load(path string) (*Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("failed to read config file: %w", err)
	}

	var cfg Config
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("failed to parse config: %w", err)
	}

	if err := cfg.setDefaults(); err != nil {
		return nil, fmt.Errorf("failed to set defaults: %w", err)
	}

	if err := cfg.loadSecretsFromEnv(); err != nil {
		return nil, fmt.Errorf("failed to load secrets from environment: %w", err)
	}

	return &cfg, nil
}

func (c *Config) setDefaults() error {
	if c.Server.Host == "" {
		c.Server.Host = "0.0.0.0"
	}
	if c.Server.Port == 0 {
		c.Server.Port = 8080
	}
	if c.Server.CookieName == "" {
		c.Server.CookieName = "sso-switch-session"
	}
	if c.Server.CookieHTTPOnly == false {
		c.Server.CookieHTTPOnly = true
	}
	if c.Server.CookieSameSite == "" {
		c.Server.CookieSameSite = "lax"
	}
	if c.Server.SessionTTL == 0 {
		c.Server.SessionTTL = 24 * time.Hour
	}

	if c.Backend.Timeout == 0 {
		c.Backend.Timeout = 30 * time.Second
	}

	if c.Cache.Type == "" {
		c.Cache.Type = "memory"
	}

	if c.Cache.Type == "redis" && c.Cache.Redis != nil {
		if c.Cache.Redis.PoolSize == 0 {
			c.Cache.Redis.PoolSize = 10
		}
		if c.Cache.Redis.MaxRetries == 0 {
			c.Cache.Redis.MaxRetries = 3
		}
	}

	if c.Logging.Level == "" {
		c.Logging.Level = "info"
	}
	if c.Logging.Format == "" {
		c.Logging.Format = "json"
	}
	if c.Logging.Output == "" {
		c.Logging.Output = "stdout"
	}

	if c.UI.Enable == nil {
		defaultEnable := true
		c.UI.Enable = &defaultEnable
	}
	if c.UI.Title == "" {
		c.UI.Title = "Sign In"
	}
	if c.UI.GradientStart == "" {
		c.UI.GradientStart = "#667eea"
	}
	if c.UI.GradientEnd == "" {
		c.UI.GradientEnd = "#764ba2"
	}

	return nil
}

func (c *Config) loadSecretsFromEnv() error {
	for i := range c.Providers {
		provider := &c.Providers[i]

		if provider.OIDC != nil {
			if envClientID := os.Getenv(fmt.Sprintf("%s_CLIENT_ID", provider.ID)); envClientID != "" {
				provider.OIDC.ClientID = envClientID
			}
			if envClientSecret := os.Getenv(fmt.Sprintf("%s_CLIENT_SECRET", provider.ID)); envClientSecret != "" {
				provider.OIDC.ClientSecret = envClientSecret
			}
		}
	}

	if c.Cache.Type == "redis" && c.Cache.Redis != nil {
		if envPassword := os.Getenv("REDIS_PASSWORD"); envPassword != "" {
			c.Cache.Redis.Password = envPassword
		}
	}

	return nil
}

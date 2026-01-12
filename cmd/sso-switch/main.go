package main

import (
	"context"
	"flag"
	"fmt"
	"log/slog"
	"os"
	"strings"

	"github.com/marcogenualdo/sso-switch/internal/auth"
	"github.com/marcogenualdo/sso-switch/internal/auth/oidc"
	"github.com/marcogenualdo/sso-switch/internal/auth/saml"
	"github.com/marcogenualdo/sso-switch/internal/cache"
	"github.com/marcogenualdo/sso-switch/internal/config"
	"github.com/marcogenualdo/sso-switch/internal/server"
)

const version = "1.0.0"

func main() {
	configPath := flag.String("config", "/etc/sso-switch/config.yaml", "path to configuration file")
	configPathShort := flag.String("c", "/etc/sso-switch/config.yaml", "path to configuration file (short)")
	showVersion := flag.Bool("version", false, "show version and exit")
	showHelp := flag.Bool("help", false, "show help and exit")
	flag.Parse()

	if *showVersion {
		fmt.Printf("SSO Proxy v%s\n", version)
		os.Exit(0)
	}

	if *showHelp {
		fmt.Println("SSO Proxy - Multi-IdP SSO Proxy for OIDC and SAML")
		fmt.Println("\nUsage:")
		flag.PrintDefaults()
		os.Exit(0)
	}

	cfgPath := *configPath
	if *configPathShort != "/etc/sso-switch/config.yaml" {
		cfgPath = *configPathShort
	}

	if err := run(cfgPath); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}

func run(configPath string) error {
	cfg, err := config.Load(configPath)
	if err != nil {
		return fmt.Errorf("failed to load config: %w", err)
	}

	if err := cfg.Validate(); err != nil {
		return fmt.Errorf("invalid config: %w", err)
	}

	logger := setupLogger(cfg.Logging)
	logger.Info("starting sso-switch", "version", version)

	cacheInstance, err := cache.New(cfg.Cache)
	if err != nil {
		return fmt.Errorf("failed to create cache: %w", err)
	}
	logger.Info("cache initialized", "type", cfg.Cache.Type)

	ctx := context.Background()
	providers := make(map[string]auth.Provider)

	for _, providerCfg := range cfg.Providers {
		var provider auth.Provider
		var err error

		switch providerCfg.Type {
		case "oidc":
			provider, err = oidc.NewProvider(ctx, providerCfg, cacheInstance)
			if err != nil {
				return fmt.Errorf("failed to create OIDC provider %s: %w", providerCfg.ID, err)
			}

		case "saml":
			provider, err = saml.NewProvider(ctx, providerCfg, cacheInstance, cfg.Server.BaseURL)
			if err != nil {
				return fmt.Errorf("failed to create SAML provider %s: %w", providerCfg.ID, err)
			}

		default:
			return fmt.Errorf("unsupported provider type: %s", providerCfg.Type)
		}

		providers[providerCfg.ID] = provider
		logger.Info("provider initialized",
			"id", providerCfg.ID,
			"name", providerCfg.Name,
			"type", providerCfg.Type,
		)
	}

	srv, err := server.New(*cfg, cacheInstance, providers, logger)
	if err != nil {
		return fmt.Errorf("failed to create server: %w", err)
	}

	return srv.Start()
}

func setupLogger(cfg config.LoggingConfig) *slog.Logger {
	var level slog.Level
	switch strings.ToLower(cfg.Level) {
	case "debug":
		level = slog.LevelDebug
	case "info":
		level = slog.LevelInfo
	case "warn":
		level = slog.LevelWarn
	case "error":
		level = slog.LevelError
	default:
		level = slog.LevelInfo
	}

	opts := &slog.HandlerOptions{
		Level: level,
	}

	var handler slog.Handler
	if strings.ToLower(cfg.Format) == "json" {
		handler = slog.NewJSONHandler(os.Stdout, opts)
	} else {
		handler = slog.NewTextHandler(os.Stdout, opts)
	}

	return slog.New(handler)
}

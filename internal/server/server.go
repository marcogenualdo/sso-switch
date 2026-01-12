package server

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/marcogenualdo/sso-switch/internal/auth"
	"github.com/marcogenualdo/sso-switch/internal/cache"
	"github.com/marcogenualdo/sso-switch/internal/config"
)

type Server struct {
	cfg       config.Config
	cache     cache.Cache
	providers map[string]auth.Provider
	logger    *slog.Logger
	httpServer *http.Server
}

func New(cfg config.Config, cache cache.Cache, providers map[string]auth.Provider, logger *slog.Logger) (*Server, error) {
	return &Server{
		cfg:       cfg,
		cache:     cache,
		providers: providers,
		logger:    logger,
	}, nil
}

func (s *Server) Start() error {
	router, err := s.setupRoutes()
	if err != nil {
		return fmt.Errorf("failed to setup routes: %w", err)
	}

	s.httpServer = &http.Server{
		Addr:         fmt.Sprintf("%s:%d", s.cfg.Server.Host, s.cfg.Server.Port),
		Handler:      router,
		ReadTimeout:  15 * time.Second,
		WriteTimeout: 15 * time.Second,
		IdleTimeout:  60 * time.Second,
	}

	errChan := make(chan error, 1)
	go func() {
		s.logger.Info("starting server",
			"host", s.cfg.Server.Host,
			"port", s.cfg.Server.Port,
			"base_url", s.cfg.Server.BaseURL,
		)
		if err := s.httpServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			errChan <- err
		}
	}()

	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)

	select {
	case err := <-errChan:
		return err
	case sig := <-sigChan:
		s.logger.Info("received shutdown signal", "signal", sig)
		return s.Shutdown()
	}
}

func (s *Server) Shutdown() error {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	s.logger.Info("shutting down server")

	if err := s.httpServer.Shutdown(ctx); err != nil {
		s.logger.Error("error during server shutdown", "error", err)
		return err
	}

	if err := s.cache.Close(); err != nil {
		s.logger.Error("error closing cache", "error", err)
	}

	s.logger.Info("server shutdown complete")
	return nil
}

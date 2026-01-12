package handlers

import (
	"context"
	"encoding/json"
	"log/slog"
	"net/http"
	"time"

	"github.com/marcogenualdo/sso-switch/internal/auth"
	"github.com/marcogenualdo/sso-switch/internal/cache"
	"github.com/marcogenualdo/sso-switch/internal/config"
)

type HealthHandler struct {
	cfg       config.Config
	cache     cache.Cache
	providers map[string]auth.Provider
	logger    *slog.Logger
	startTime time.Time
}

func NewHealthHandler(cfg config.Config, cache cache.Cache, providers map[string]auth.Provider, logger *slog.Logger) *HealthHandler {
	return &HealthHandler{
		cfg:       cfg,
		cache:     cache,
		providers: providers,
		logger:    logger,
		startTime: time.Now(),
	}
}

type HealthResponse struct {
	Status    string              `json:"status"`
	Uptime    string              `json:"uptime"`
	Cache     CacheHealth         `json:"cache"`
	Backend   BackendHealth       `json:"backend"`
	Providers map[string]string   `json:"providers"`
}

type CacheHealth struct {
	Type   string `json:"type"`
	Status string `json:"status"`
}

type BackendHealth struct {
	URL    string `json:"url"`
	Status string `json:"status"`
}

func (h *HealthHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
	defer cancel()

	response := HealthResponse{
		Status:    "healthy",
		Uptime:    time.Since(h.startTime).String(),
		Providers: make(map[string]string),
	}

	response.Cache.Type = h.cfg.Cache.Type
	if err := h.cache.Set(ctx, "health:check", []byte("ok"), 1*time.Minute); err != nil {
		response.Cache.Status = "error: " + err.Error()
		response.Status = "degraded"
	} else {
		response.Cache.Status = "connected"
		h.cache.Delete(ctx, "health:check")
	}

	response.Backend.URL = h.cfg.Backend.URL
	backendResp, err := http.Get(h.cfg.Backend.URL)
	if err != nil {
		response.Backend.Status = "unreachable"
		response.Status = "degraded"
	} else {
		backendResp.Body.Close()
		response.Backend.Status = "reachable"
	}

	for id, provider := range h.providers {
		response.Providers[id] = provider.Name() + " (" + provider.Type() + ")"
	}

	w.Header().Set("Content-Type", "application/json")
	if response.Status != "healthy" {
		w.WriteHeader(http.StatusServiceUnavailable)
	}

	json.NewEncoder(w).Encode(response)
}

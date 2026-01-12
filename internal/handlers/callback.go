package handlers

import (
	"encoding/json"
	"log/slog"
	"net/http"
	"time"

	"github.com/google/uuid"
	"github.com/marcogenualdo/sso-switch/internal/auth"
	"github.com/marcogenualdo/sso-switch/internal/cache"
	"github.com/marcogenualdo/sso-switch/internal/config"
	"github.com/marcogenualdo/sso-switch/pkg/security"
)

type CallbackHandler struct {
	cfg       config.Config
	cache     cache.Cache
	providers map[string]auth.Provider
	logger    *slog.Logger
}

func NewCallbackHandler(cfg config.Config, cache cache.Cache, providers map[string]auth.Provider, logger *slog.Logger) *CallbackHandler {
	return &CallbackHandler{
		cfg:       cfg,
		cache:     cache,
		providers: providers,
		logger:    logger,
	}
}

func (h *CallbackHandler) HandleOIDCCallback(providerID string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		provider, exists := h.providers[providerID]
		if !exists {
			h.logger.Error("provider not found", "provider_id", providerID)
			http.Error(w, "Invalid provider", http.StatusBadRequest)
			return
		}

		session, err := provider.HandleCallback(r.Context(), r)
		if err != nil {
			h.logger.Error("callback failed", "provider", providerID, "error", err)
			http.Error(w, "Authentication failed: "+err.Error(), http.StatusUnauthorized)
			return
		}

		sessionID := uuid.New().String()
		session.ID = sessionID

		sessionData, err := json.Marshal(session)
		if err != nil {
			h.logger.Error("failed to marshal session", "error", err)
			http.Error(w, "Internal server error", http.StatusInternalServerError)
			return
		}

		ttl := time.Until(session.ExpiresAt)
		if err := h.cache.Set(r.Context(), "session:"+sessionID, sessionData, ttl); err != nil {
			h.logger.Error("failed to cache session", "error", err)
			http.Error(w, "Internal server error", http.StatusInternalServerError)
			return
		}

		cookie := security.CreateSessionCookie(h.cfg.Server, sessionID, ttl)
		http.SetCookie(w, cookie)

		h.logger.Info("authentication successful",
			"provider", providerID,
			"session_id", sessionID,
		)

		http.Redirect(w, r, "/", http.StatusFound)
	}
}

func (h *CallbackHandler) HandleSAMLCallback(providerID string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		provider, exists := h.providers[providerID]
		if !exists {
			h.logger.Error("provider not found", "provider_id", providerID)
			http.Error(w, "Invalid provider", http.StatusBadRequest)
			return
		}

		session, err := provider.HandleCallback(r.Context(), r)
		if err != nil {
			h.logger.Error("SAML callback failed", "provider", providerID, "error", err)
			http.Error(w, "Authentication failed: "+err.Error(), http.StatusUnauthorized)
			return
		}

		sessionID := uuid.New().String()
		session.ID = sessionID

		sessionData, err := json.Marshal(session)
		if err != nil {
			h.logger.Error("failed to marshal session", "error", err)
			http.Error(w, "Internal server error", http.StatusInternalServerError)
			return
		}

		ttl := time.Until(session.ExpiresAt)
		if err := h.cache.Set(r.Context(), "session:"+sessionID, sessionData, ttl); err != nil {
			h.logger.Error("failed to cache session", "error", err)
			http.Error(w, "Internal server error", http.StatusInternalServerError)
			return
		}

		cookie := security.CreateSessionCookie(h.cfg.Server, sessionID, ttl)
		http.SetCookie(w, cookie)

		h.logger.Info("SAML authentication successful",
			"provider", providerID,
			"session_id", sessionID,
		)

		relayState := r.FormValue("RelayState")
		if relayState != "" {
			http.Redirect(w, r, relayState, http.StatusFound)
		} else {
			http.Redirect(w, r, "/", http.StatusFound)
		}
	}
}

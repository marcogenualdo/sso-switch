package middleware

import (
	"context"
	"encoding/json"
	"log/slog"
	"net/http"
	"time"

	"github.com/marcogenualdo/sso-proxy/internal/auth"
	"github.com/marcogenualdo/sso-proxy/internal/cache"
	"github.com/marcogenualdo/sso-proxy/internal/config"
	"github.com/marcogenualdo/sso-proxy/pkg/security"
)

type contextKey string

const SessionContextKey contextKey = "session"

type AuthMiddleware struct {
	cfg       config.ServerConfig
	cache     cache.Cache
	providers map[string]auth.Provider
	logger    *slog.Logger
}

func NewAuthMiddleware(cfg config.ServerConfig, cache cache.Cache, providers map[string]auth.Provider, logger *slog.Logger) *AuthMiddleware {
	return &AuthMiddleware{
		cfg:       cfg,
		cache:     cache,
		providers: providers,
		logger:    logger,
	}
}

func (am *AuthMiddleware) RequireAuth(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		cookie, err := security.GetSessionCookie(r, am.cfg.CookieName)
		if err != nil {
			am.logger.Debug("no session cookie found", "path", r.URL.Path)
			http.Redirect(w, r, "/auth/select", http.StatusFound)
			return
		}

		sessionData, err := am.cache.Get(r.Context(), "session:"+cookie.Value)
		if err != nil {
			am.logger.Debug("session not found in cache", "session_id", cookie.Value)
			http.Redirect(w, r, "/auth/select", http.StatusFound)
			return
		}

		var session auth.Session
		if err := json.Unmarshal(sessionData, &session); err != nil {
			am.logger.Error("failed to unmarshal session", "error", err)
			http.Redirect(w, r, "/auth/select", http.StatusFound)
			return
		}

		provider, exists := am.providers[session.ProviderID]
		if !exists {
			am.logger.Error("provider not found", "provider_id", session.ProviderID)
			http.Redirect(w, r, "/auth/select", http.StatusFound)
			return
		}

		if err := provider.ValidateSession(r.Context(), &session); err != nil {
			am.logger.Debug("session validation failed", "error", err)

			if session.ProviderType == "oidc" && time.Until(session.TokenExpiry) < 5*time.Minute {
				newSession, err := provider.RefreshSession(r.Context(), &session)
				if err != nil {
					am.logger.Warn("token refresh failed", "error", err)
					http.Redirect(w, r, "/auth/select", http.StatusFound)
					return
				}

				sessionData, err := json.Marshal(newSession)
				if err != nil {
					am.logger.Error("failed to marshal refreshed session", "error", err)
					http.Redirect(w, r, "/auth/select", http.StatusFound)
					return
				}

				ttl := time.Until(newSession.ExpiresAt)
				if err := am.cache.Set(r.Context(), "session:"+cookie.Value, sessionData, ttl); err != nil {
					am.logger.Error("failed to update session in cache", "error", err)
				}

				session = *newSession
			} else {
				http.Redirect(w, r, "/auth/select", http.StatusFound)
				return
			}
		}

		ctx := context.WithValue(r.Context(), SessionContextKey, &session)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

func GetSession(ctx context.Context) (*auth.Session, bool) {
	session, ok := ctx.Value(SessionContextKey).(*auth.Session)
	return session, ok
}

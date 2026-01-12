package middleware

import (
	"context"
	"log/slog"
	"net/http"
	"time"

	"github.com/marcogenualdo/sso-switch/internal/cache"
	"github.com/marcogenualdo/sso-switch/pkg/security"
)

type CSRFMiddleware struct {
	cache  cache.Cache
	logger *slog.Logger
}

func NewCSRFMiddleware(cache cache.Cache, logger *slog.Logger) *CSRFMiddleware {
	return &CSRFMiddleware{
		cache:  cache,
		logger: logger,
	}
}

func (cm *CSRFMiddleware) ValidateCSRF(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == "POST" || r.Method == "PUT" || r.Method == "DELETE" {
			token := r.FormValue("csrf_token")
			if token == "" {
				token = r.Header.Get("X-CSRF-Token")
			}

			if token == "" {
				cm.logger.Warn("missing CSRF token", "path", r.URL.Path)
				http.Error(w, "Missing CSRF token", http.StatusForbidden)
				return
			}

			exists, err := cm.cache.Exists(r.Context(), "csrf:"+token)
			if err != nil {
				cm.logger.Error("failed to check CSRF token", "error", err)
				http.Error(w, "Internal server error", http.StatusInternalServerError)
				return
			}

			if !exists {
				cm.logger.Warn("invalid CSRF token", "path", r.URL.Path)
				http.Error(w, "Invalid or expired CSRF token", http.StatusForbidden)
				return
			}

			cm.cache.Delete(r.Context(), "csrf:"+token)
		}

		next.ServeHTTP(w, r)
	})
}

func (cm *CSRFMiddleware) GenerateCSRFToken(ctx context.Context) (string, error) {
	token, err := security.GenerateCSRFToken()
	if err != nil {
		return "", err
	}

	if err := cm.cache.Set(ctx, "csrf:"+token, []byte("1"), 10*time.Minute); err != nil {
		return "", err
	}

	return token, nil
}

package handlers

import (
	"log/slog"
	"net/http"

	"github.com/marcogenualdo/sso-switch/internal/cache"
	"github.com/marcogenualdo/sso-switch/internal/config"
	"github.com/marcogenualdo/sso-switch/pkg/security"
)

type LogoutHandler struct {
	cfg    config.Config
	cache  cache.Cache
	logger *slog.Logger
}

func NewLogoutHandler(cfg config.Config, cache cache.Cache, logger *slog.Logger) *LogoutHandler {
	return &LogoutHandler{
		cfg:    cfg,
		cache:  cache,
		logger: logger,
	}
}

func (h *LogoutHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if r.Method != "POST" {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	cookie, err := security.GetSessionCookie(r, h.cfg.Server.CookieName)
	if err == nil {
		if err := h.cache.Delete(r.Context(), "session:"+cookie.Value); err != nil {
			h.logger.Warn("failed to delete session from cache", "error", err)
		}
	}

	clearCookie := security.ClearSessionCookie(h.cfg.Server)
	http.SetCookie(w, clearCookie)

	h.logger.Info("user logged out")

	http.Redirect(w, r, "/auth/select", http.StatusFound)
}

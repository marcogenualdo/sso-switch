package proxy

import (
	"log/slog"
	"net/http"
	"net/http/httputil"
	"net/url"

	"github.com/marcogenualdo/sso-switch/internal/auth"
	"github.com/marcogenualdo/sso-switch/internal/config"
	"github.com/marcogenualdo/sso-switch/internal/middleware"
)

type ReverseProxy struct {
	proxy   *httputil.ReverseProxy
	cfg     config.BackendConfig
	logger  *slog.Logger
	providers map[string]auth.Provider
}

func NewReverseProxy(cfg config.BackendConfig, providers map[string]auth.Provider, logger *slog.Logger) (*ReverseProxy, error) {
	backendURL, err := url.Parse(cfg.URL)
	if err != nil {
		return nil, err
	}

	proxy := httputil.NewSingleHostReverseProxy(backendURL)

	originalDirector := proxy.Director
	proxy.Director = func(req *http.Request) {
		originalDirector(req)
		req.Host = backendURL.Host
		req.URL.Scheme = backendURL.Scheme
		req.URL.Host = backendURL.Host
	}

	proxy.ErrorHandler = func(w http.ResponseWriter, r *http.Request, err error) {
		logger.Error("proxy error",
			"error", err,
			"backend", backendURL.String(),
			"path", r.URL.Path,
		)
		http.Error(w, "Bad Gateway", http.StatusBadGateway)
	}

	return &ReverseProxy{
		proxy:     proxy,
		cfg:       cfg,
		logger:    logger,
		providers: providers,
	}, nil
}

func (rp *ReverseProxy) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	session, ok := middleware.GetSession(r.Context())
	if !ok {
		rp.logger.Error("no session in context")
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	provider, exists := rp.providers[session.ProviderID]
	if !exists {
		rp.logger.Error("provider not found", "provider_id", session.ProviderID)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}

	if err := InjectHeaders(r, session, provider); err != nil {
		rp.logger.Error("failed to inject headers", "error", err)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}

	if rp.cfg.PreserveHost {
		r.Host = r.Header.Get("X-Forwarded-Host")
		if r.Host == "" {
			r.Host = r.Header.Get("Host")
		}
	}

	rp.logger.Debug("proxying request",
		"path", r.URL.Path,
		"backend", rp.cfg.URL,
		"session_id", session.ID,
	)

	rp.proxy.ServeHTTP(w, r)
}

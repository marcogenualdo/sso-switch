package handlers

import (
	"embed"
	"encoding/json"
	"html/template"
	"log/slog"
	"net/http"

	"github.com/marcogenualdo/sso-switch/internal/auth"
	"github.com/marcogenualdo/sso-switch/internal/cache"
	"github.com/marcogenualdo/sso-switch/internal/config"
	"github.com/marcogenualdo/sso-switch/internal/middleware"
)

//go:embed templates/*
var templatesFS embed.FS

type SelectHandler struct {
	cfg       config.Config
	cache     cache.Cache
	providers map[string]auth.Provider
	csrf      *middleware.CSRFMiddleware
	logger    *slog.Logger
	template  *template.Template
}

func NewSelectHandler(cfg config.Config, cache cache.Cache, providers map[string]auth.Provider, csrf *middleware.CSRFMiddleware, logger *slog.Logger) (*SelectHandler, error) {
	tmpl, err := template.ParseFS(templatesFS, "templates/select.html")
	if err != nil {
		return nil, err
	}

	return &SelectHandler{
		cfg:       cfg,
		cache:     cache,
		providers: providers,
		csrf:      csrf,
		logger:    logger,
		template:  tmpl,
	}, nil
}

type SelectPageData struct {
	Providers     []ProviderInfo
	CSRFToken     string
	PageTitle     string
	GradientStart string
	GradientEnd   string
	LogoURL       string
}

type ProviderInfo struct {
	ID   string
	Name string
	Type string
}

func (h *SelectHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case "GET":
		h.handleGet(w, r)
	case "POST":
		h.handlePost(w, r)
	default:
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
	}
}

func (h *SelectHandler) initiateAuthForProvider(w http.ResponseWriter, r *http.Request, provider auth.Provider) {
	var redirectURL string
	if provider.Type() == "oidc" {
		redirectURL = h.cfg.Server.BaseURL + "/auth/oidc/" + provider.ID() + "/callback"
	} else {
		redirectURL = h.cfg.Server.BaseURL + "/auth/saml/" + provider.ID() + "/acs"
	}

	authRedirect, err := provider.InitiateAuth(r.Context(), redirectURL)
	if err != nil {
		h.logger.Error("failed to initiate auth", "provider", provider.ID(), "error", err)
		http.Error(w, "Failed to initiate authentication", http.StatusInternalServerError)
		return
	}

	if authRedirect.CacheKey != "" && authRedirect.CacheData != nil {
		var data []byte
		switch v := authRedirect.CacheData.(type) {
		case []byte:
			data = v
		default:
			var err error
			data, err = json.Marshal(v)
			if err != nil {
				h.logger.Error("failed to marshal cache data", "error", err)
				http.Error(w, "Internal server error", http.StatusInternalServerError)
				return
			}
		}

		if err := h.cache.Set(r.Context(), authRedirect.CacheKey, data, authRedirect.CacheTTL); err != nil {
			h.logger.Error("failed to cache auth state", "error", err)
			http.Error(w, "Internal server error", http.StatusInternalServerError)
			return
		}
	}

	http.Redirect(w, r, authRedirect.URL, http.StatusFound)
}

func (h *SelectHandler) handleGet(w http.ResponseWriter, r *http.Request) {
	// If only one provider and UI is enabled (default), redirect directly to the provider
	if len(h.providers) == 1 && h.cfg.UI.Enable != nil && *h.cfg.UI.Enable == false {
		for _, provider := range h.providers {
			h.initiateAuthForProvider(w, r, provider)
			return
		}
	}

	csrfToken, err := h.csrf.GenerateCSRFToken(r.Context())
	if err != nil {
		h.logger.Error("failed to generate CSRF token", "error", err)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}

	providers := make([]ProviderInfo, 0, len(h.providers))
	for _, provider := range h.providers {
		providers = append(providers, ProviderInfo{
			ID:   provider.ID(),
			Name: provider.Name(),
			Type: provider.Type(),
		})
	}

	logoURL := ""
	if h.cfg.UI.LogoPath != "" {
		logoURL = "/auth/select/logo"
	}

	data := SelectPageData{
		Providers:     providers,
		CSRFToken:     csrfToken,
		PageTitle:     h.cfg.UI.Title,
		GradientStart: h.cfg.UI.GradientStart,
		GradientEnd:   h.cfg.UI.GradientEnd,
		LogoURL:       logoURL,
	}

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	if err := h.template.Execute(w, data); err != nil {
		h.logger.Error("failed to render template", "error", err)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
	}
}

func (h *SelectHandler) handlePost(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		http.Error(w, "Invalid form data", http.StatusBadRequest)
		return
	}

	providerID := r.FormValue("provider")
	if providerID == "" {
		http.Error(w, "Provider is required", http.StatusBadRequest)
		return
	}

	provider, exists := h.providers[providerID]
	if !exists {
		http.Error(w, "Invalid provider", http.StatusBadRequest)
		return
	}

	h.initiateAuthForProvider(w, r, provider)
}

func (h *SelectHandler) ServeLogo(w http.ResponseWriter, r *http.Request) {
	if h.cfg.UI.LogoPath == "" {
		http.NotFound(w, r)
		return
	}

	http.ServeFile(w, r, h.cfg.UI.LogoPath)
}

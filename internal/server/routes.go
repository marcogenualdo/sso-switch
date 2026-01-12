package server

import (
	"encoding/xml"
	"net/http"

	"github.com/marcogenualdo/sso-switch/internal/auth/saml"
	"github.com/marcogenualdo/sso-switch/internal/handlers"
	"github.com/marcogenualdo/sso-switch/internal/middleware"
	"github.com/marcogenualdo/sso-switch/internal/proxy"
)

func (s *Server) setupRoutes() (http.Handler, error) {
	mux := http.NewServeMux()

	csrfMiddleware := middleware.NewCSRFMiddleware(s.cache, s.logger)
	authMiddleware := middleware.NewAuthMiddleware(s.cfg.Server, s.cache, s.providers, s.logger)

	selectHandler, err := handlers.NewSelectHandler(s.cfg, s.cache, s.providers, csrfMiddleware, s.logger)
	if err != nil {
		return nil, err
	}

	callbackHandler := handlers.NewCallbackHandler(s.cfg, s.cache, s.providers, s.logger)
	logoutHandler := handlers.NewLogoutHandler(s.cfg, s.cache, s.logger)
	healthHandler := handlers.NewHealthHandler(s.cfg, s.cache, s.providers, s.logger)

	reverseProxy, err := proxy.NewReverseProxy(s.cfg.Backend, s.providers, s.logger)
	if err != nil {
		return nil, err
	}

	mux.HandleFunc("/auth/select", selectHandler.ServeHTTP)

	for id, provider := range s.providers {
		if provider.Type() == "oidc" {
			loginPath := "/auth/oidc/" + id + "/login"
			callbackPath := "/auth/oidc/" + id + "/callback"

			mux.HandleFunc(loginPath, func(w http.ResponseWriter, r *http.Request) {
				http.Redirect(w, r, "/auth/select", http.StatusFound)
			})

			mux.HandleFunc(callbackPath, callbackHandler.HandleOIDCCallback(id))

		} else if provider.Type() == "saml" {
			loginPath := "/auth/saml/" + id + "/login"
			acsPath := "/auth/saml/" + id + "/acs"
			metadataPath := "/auth/saml/" + id + "/metadata"

			mux.HandleFunc(loginPath, func(w http.ResponseWriter, r *http.Request) {
				http.Redirect(w, r, "/auth/select", http.StatusFound)
			})

			mux.HandleFunc(acsPath, callbackHandler.HandleSAMLCallback(id))

			samlProvider, ok := provider.(*saml.Provider)
			if ok {
				mux.HandleFunc(metadataPath, func(w http.ResponseWriter, r *http.Request) {
					metadata, err := samlProvider.GetMetadata()
					if err != nil {
						http.Error(w, "Failed to generate metadata", http.StatusInternalServerError)
						return
					}

					w.Header().Set("Content-Type", "application/xml")
					xml.NewEncoder(w).Encode(metadata)
				})
			}
		}
	}

	mux.Handle("/auth/logout", csrfMiddleware.ValidateCSRF(logoutHandler))

	mux.HandleFunc("/health", healthHandler.ServeHTTP)

	mux.Handle("/", authMiddleware.RequireAuth(reverseProxy))

	handler := middleware.Recovery(s.logger)(
		middleware.Logging(s.logger)(
			addSecurityHeaders(mux),
		),
	)

	return handler, nil
}

func addSecurityHeaders(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("X-Frame-Options", "DENY")
		w.Header().Set("X-Content-Type-Options", "nosniff")
		w.Header().Set("X-XSS-Protection", "1; mode=block")
		w.Header().Set("Strict-Transport-Security", "max-age=31536000; includeSubDomains")
		w.Header().Set("Referrer-Policy", "strict-origin-when-cross-origin")

		next.ServeHTTP(w, r)
	})
}

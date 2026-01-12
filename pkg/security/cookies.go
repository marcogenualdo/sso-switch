package security

import (
	"net/http"
	"strings"
	"time"

	"github.com/marcogenualdo/sso-switch/internal/config"
)

func CreateSessionCookie(cfg config.ServerConfig, sessionID string, maxAge time.Duration) *http.Cookie {
	sameSite := http.SameSiteLaxMode
	switch strings.ToLower(cfg.CookieSameSite) {
	case "strict":
		sameSite = http.SameSiteStrictMode
	case "none":
		sameSite = http.SameSiteNoneMode
	}

	return &http.Cookie{
		Name:     cfg.CookieName,
		Value:    sessionID,
		Path:     "/",
		Domain:   cfg.CookieDomain,
		MaxAge:   int(maxAge.Seconds()),
		Secure:   cfg.CookieSecure,
		HttpOnly: cfg.CookieHTTPOnly,
		SameSite: sameSite,
	}
}

func ClearSessionCookie(cfg config.ServerConfig) *http.Cookie {
	cookie := CreateSessionCookie(cfg, "", 0)
	cookie.MaxAge = -1
	return cookie
}

func GetSessionCookie(req *http.Request, cookieName string) (*http.Cookie, error) {
	return req.Cookie(cookieName)
}

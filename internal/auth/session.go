package auth

import "time"

type Session struct {
	ID           string                 `json:"id"`
	ProviderID   string                 `json:"provider_id"`
	ProviderType string                 `json:"provider_type"`
	UserInfo     map[string]interface{} `json:"user_info"`
	CreatedAt    time.Time              `json:"created_at"`
	ExpiresAt    time.Time              `json:"expires_at"`

	AccessToken  string    `json:"access_token,omitempty"`
	RefreshToken string    `json:"refresh_token,omitempty"`
	IDToken      string    `json:"id_token,omitempty"`
	TokenExpiry  time.Time `json:"token_expiry,omitempty"`

	Assertion string `json:"assertion,omitempty"`

	CSRFToken string `json:"csrf_token"`
}

type OIDCState struct {
	State        string    `json:"state"`
	ProviderID   string    `json:"provider_id"`
	CodeVerifier string    `json:"code_verifier"`
	RedirectURL  string    `json:"redirect_url"`
	CreatedAt    time.Time `json:"created_at"`
}

type SAMLRequest struct {
	ID         string    `json:"id"`
	ProviderID string    `json:"provider_id"`
	RelayState string    `json:"relay_state"`
	CreatedAt  time.Time `json:"created_at"`
}

type AuthRedirect struct {
	URL       string
	Method    string
	FormData  map[string]string
	CacheKey  string
	CacheData interface{}
	CacheTTL  time.Duration
}

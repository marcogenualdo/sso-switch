package auth

import (
	"context"
	"net/http"
)

type Provider interface {
	ID() string
	Name() string
	Type() string

	InitiateAuth(ctx context.Context, redirectURL string) (*AuthRedirect, error)
	HandleCallback(ctx context.Context, req *http.Request) (*Session, error)
	ValidateSession(ctx context.Context, session *Session) error
	RefreshSession(ctx context.Context, session *Session) (*Session, error)

	GetHeaderMappings() map[string]string
}

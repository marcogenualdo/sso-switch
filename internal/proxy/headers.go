package proxy

import (
	"fmt"
	"net/http"
	"strings"

	"github.com/marcogenualdo/sso-switch/internal/auth"
)

func InjectHeaders(req *http.Request, session *auth.Session, provider auth.Provider) error {
	headerMappings := provider.GetHeaderMappings()

	for claim, header := range headerMappings {
		value, exists := session.UserInfo[claim]
		if !exists {
			continue
		}

		headerValue := formatHeaderValue(value)
		if headerValue != "" {
			req.Header.Set(header, headerValue)
		}
	}

	req.Header.Set("X-Auth-Provider", session.ProviderID)
	req.Header.Set("X-Auth-Provider-Type", session.ProviderType)
	req.Header.Set("X-Auth-Session-ID", session.ID)

	return nil
}

func formatHeaderValue(value interface{}) string {
	switch v := value.(type) {
	case string:
		return v
	case []string:
		return strings.Join(v, ",")
	case []interface{}:
		parts := make([]string, 0, len(v))
		for _, item := range v {
			if str, ok := item.(string); ok {
				parts = append(parts, str)
			} else {
				parts = append(parts, fmt.Sprintf("%v", item))
			}
		}
		return strings.Join(parts, ",")
	default:
		return fmt.Sprintf("%v", v)
	}
}

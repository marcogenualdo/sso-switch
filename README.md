# SSO Proxy

A multi-IdP SSO proxy for OIDC and SAML federated authentication, written in Go.

## Problem

You have an application with authorization implemented, but want to delegate authentication to a choice of external providers using SAML or OIDC. Existing solutions like oauth2-proxy don't support multiple IdP selection.

## Solution

This proxy intercepts HTTP requests and provides seamless authentication:

1. **Authenticated requests**: If a valid session exists, the proxy extracts user info from the cache, injects it as HTTP headers, and forwards the request to your backend application.

2. **Unauthenticated requests**: Users are redirected to an IdP selection page where they choose their identity provider. The proxy then initiates the appropriate authentication flow (OIDC with PKCE or SAML).

3. **After authentication**: The proxy validates tokens/assertions, creates a secure session, and caches the user information. Subsequent requests are automatically authenticated.

## Features

- **Multiple Identity Providers**: Support for multiple OIDC and SAML providers simultaneously
- **OIDC Support**: Full OpenID Connect implementation with PKCE and token refresh
- **SAML Support**: Complete SAML 2.0 Service Provider implementation
- **Flexible Caching**: In-memory cache or Redis for distributed deployments
- **Header Injection**: Configurable mapping of claims/attributes to HTTP headers
- **Security First**: CSRF protection, secure cookies, token validation, and HTTP security headers
- **Production Ready**: Graceful shutdown, health checks, structured logging
- **Easy Deployment**: Docker container with docker-compose for local development

## Quick Start

### Using Docker Compose

```bash
# Clone the repository
git clone https://github.com/marcogenualdo/sso-switch.git
cd sso-switch

# Edit configuration
cp examples/config.yaml config.yaml
# Edit config.yaml with your IdP settings

# Start services
docker-compose up -d

# View logs
docker-compose logs -f sso-switch
```

### Building from Source

```bash
# Install dependencies
go mod download

# Build
make build

# Run
./sso-switch --config config.yaml
```

## Configuration

### Basic Configuration Structure

```yaml
server:
  host: "0.0.0.0"
  port: 8080
  base_url: "https://sso.example.com"
  cookie_secure: true
  session_ttl: "24h"

backend:
  url: "http://backend-service:8000"
  timeout: "30s"

cache:
  type: "redis"  # or "memory"
  redis:
    address: "localhost:6379"

providers:
  - id: "azure"
    name: "Azure Entra ID"
    type: "oidc"
    oidc:
      issuer: "https://login.microsoftonline.com/{tenant}/v2.0"
      client_id: "your-client-id"
      client_secret: "your-client-secret"
      scopes: ["openid", "profile", "email"]
    header_mappings:
      email: "X-User-Email"
      name: "X-User-Name"

logging:
  level: "info"
  format: "json"
```

### Configuration Reference

#### Server Configuration

| Field | Type | Default | Description |
|-------|------|---------|-------------|
| `host` | string | `0.0.0.0` | Listen address |
| `port` | int | `8080` | Listen port |
| `base_url` | string | required | External URL for callbacks |
| `cookie_name` | string | `sso-switch-session` | Session cookie name |
| `cookie_domain` | string | - | Cookie domain (e.g., `.example.com`) |
| `cookie_secure` | bool | `false` | Require HTTPS for cookies |
| `cookie_http_only` | bool | `true` | HttpOnly cookie flag |
| `cookie_same_site` | string | `lax` | SameSite policy (lax/strict/none) |
| `session_ttl` | duration | `24h` | Session duration |

#### Provider Configuration (OIDC)

```yaml
providers:
  - id: "provider-id"
    name: "Provider Display Name"
    type: "oidc"
    oidc:
      issuer: "https://idp.example.com"
      client_id: "client-id"
      client_secret: "client-secret"
      scopes: ["openid", "profile", "email"]
      hd: "example.com"  # Optional: Google Workspace domain
    header_mappings:
      email: "X-User-Email"
      sub: "X-User-ID"
```

#### Provider Configuration (SAML)

```yaml
providers:
  - id: "provider-id"
    name: "Provider Display Name"
    type: "saml"
    saml:
      idp_metadata_url: "https://idp.example.com/metadata"
      # OR
      idp_metadata_xml: |
        <EntityDescriptor ...>
      sp_entity_id: "https://sso.example.com/saml/metadata"
      acs_url: "https://sso.example.com/auth/saml/provider-id/acs"
      certificate_path: "/etc/sso-switch/certs/sp-cert.pem"
      private_key_path: "/etc/sso-switch/certs/sp-key.pem"
    header_mappings:
      "urn:oid:0.9.2342.19200300.100.1.3": "X-User-Email"
```

### Environment Variables

Sensitive values can be overridden with environment variables:

```bash
# OIDC credentials
export azure_CLIENT_ID="your-client-id"
export azure_CLIENT_SECRET="your-client-secret"

# Redis password
export REDIS_PASSWORD="your-redis-password"
```

### Example Configurations

See the `examples/` directory for complete configuration examples:

- [`azure-entra.yaml`](examples/azure-entra.yaml) - Azure Entra ID (Azure AD) OIDC
- [`okta.yaml`](examples/okta.yaml) - Okta SAML
- [`google.yaml`](examples/google.yaml) - Google Workspace OIDC
- [`config.yaml`](examples/config.yaml) - Full example with multiple providers

## IdP Setup Guides

### Azure Entra ID (OIDC)

1. Go to Azure Portal > App registrations > New registration
2. Set redirect URI: `https://sso.example.com/auth/oidc/azure/callback`
3. Create a client secret
4. Configure token claims if needed
5. Use tenant-specific issuer URL in config

### Okta (SAML)

1. Create a new SAML 2.0 application in Okta
2. Set Single sign-on URL: `https://sso.example.com/auth/saml/okta/acs`
3. Set Audience URI: `https://sso.example.com/saml/metadata`
4. Configure attribute statements
5. Download or use metadata URL

### Google Workspace (OIDC)

1. Go to Google Cloud Console > APIs & Services > Credentials
2. Create OAuth 2.0 Client ID (Web application)
3. Add authorized redirect URI: `https://sso.example.com/auth/oidc/google/callback`
4. Optional: Configure domain restriction with `hd` parameter

## Deployment

### Docker

```bash
# Build image
docker build -t sso-switch:latest .

# Run container
docker run -d \
  -p 8080:8080 \
  -v $(pwd)/config.yaml:/etc/sso-switch/config.yaml:ro \
  --name sso-switch \
  sso-switch:latest
```

### Kubernetes

```yaml
apiVersion: apps/v1
kind: Deployment
metadata:
  name: sso-switch
spec:
  replicas: 2
  selector:
    matchLabels:
      app: sso-switch
  template:
    metadata:
      labels:
        app: sso-switch
    spec:
      containers:
      - name: sso-switch
        image: sso-switch:latest
        ports:
        - containerPort: 8080
        volumeMounts:
        - name: config
          mountPath: /etc/sso-switch
        env:
        - name: azure_CLIENT_SECRET
          valueFrom:
            secretKeyRef:
              name: sso-secrets
              key: azure-client-secret
      volumes:
      - name: config
        configMap:
          name: sso-config
```

### Behind a Reverse Proxy (nginx)

```nginx
server {
    listen 443 ssl http2;
    server_name sso.example.com;

    ssl_certificate /path/to/cert.pem;
    ssl_certificate_key /path/to/key.pem;

    location / {
        proxy_pass http://localhost:8080;
        proxy_set_header Host $host;
        proxy_set_header X-Real-IP $remote_addr;
        proxy_set_header X-Forwarded-For $proxy_add_x_forwarded_for;
        proxy_set_header X-Forwarded-Proto $scheme;
    }
}
```

## Architecture

### Authentication Flow

```
┌────────┐       ┌───────────┐       ┌──────────┐       ┌─────────┐
│ User   │──────▶│ SSO Proxy │──────▶│   IdP    │◀──────│ Backend │
└────────┘       └───────────┘       └──────────┘       └─────────┘
     │                 │                    │                  │
     │  1. Request     │                    │                  │
     ├────────────────▶│                    │                  │
     │                 │                    │                  │
     │  2. Select IdP  │                    │                  │
     │◀────────────────┤                    │                  │
     │                 │                    │                  │
     │  3. Auth Flow   │                    │                  │
     ├────────────────▶├───────────────────▶│                  │
     │                 │    4. Callback     │                  │
     │◀────────────────┤◀───────────────────┤                  │
     │                 │                    │                  │
     │  5. Set Cookie  │                    │                  │
     │◀────────────────┤                    │                  │
     │                 │                    │                  │
     │  6. Request     │  7. With Headers   │                  │
     ├────────────────▶├──────────────────────────────────────▶│
     │                 │                    │                  │
     │  8. Response    │                    │                  │
     │◀────────────────┤◀──────────────────────────────────────┤
```

### Components

- **Handlers**: Process authentication flows and serve IdP selection UI
- **Middleware**: Authentication checking, CSRF protection, logging
- **Providers**: Pluggable interface for OIDC and SAML implementations
- **Cache**: Session storage abstraction (memory or Redis)
- **Proxy**: Reverse proxy with header injection

## API Endpoints

| Endpoint | Method | Description |
|----------|--------|-------------|
| `/auth/select` | GET | IdP selection page |
| `/auth/select` | POST | Process IdP selection |
| `/auth/oidc/{id}/callback` | GET | OIDC callback |
| `/auth/saml/{id}/acs` | POST | SAML ACS endpoint |
| `/auth/saml/{id}/metadata` | GET | SAML SP metadata |
| `/auth/logout` | POST | Logout and clear session |
| `/health` | GET | Health check |
| `/*` | ANY | Proxy to backend (requires auth) |

## Security

- **CSRF Protection**: All state-changing operations are protected
- **PKCE**: OIDC flows use PKCE for enhanced security
- **Secure Cookies**: HttpOnly, Secure, SameSite flags
- **Token Validation**: Complete signature and claim validation
- **HTTP Security Headers**: HSTS, X-Frame-Options, CSP, etc.
- **No Client Secrets in Browser**: All auth flows are server-side

## Development

### Building

```bash
# Build binary
make build

# Run tests
make test

# Run with coverage
make test-coverage

# Format and lint
make fmt
make vet
make lint

# Run all checks
make check
```

### Project Structure

```
sso-switch/
├── cmd/sso-switch/          # Application entry point
├── internal/               # Private application code
│   ├── auth/               # Authentication providers
│   ├── cache/              # Cache implementations
│   ├── config/             # Configuration
│   ├── handlers/           # HTTP handlers
│   ├── middleware/         # HTTP middleware
│   ├── proxy/              # Reverse proxy
│   └── server/             # Server setup
├── pkg/security/           # Security utilities
├── examples/               # Example configurations
└── web/templates/          # HTML templates
```

## Contributing

Contributions are welcome! Please feel free to submit a Pull Request.

## License

This project is open source and available under the MIT License.

## Acknowledgments

Built with:
- [coreos/go-oidc](https://github.com/coreos/go-oidc) - OIDC client library
- [crewjam/saml](https://github.com/crewjam/saml) - SAML library
- [go-redis/redis](https://github.com/redis/go-redis) - Redis client

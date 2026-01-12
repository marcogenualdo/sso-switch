package saml

import (
	"context"
	"crypto/rsa"
	"crypto/x509"
	"encoding/json"
	"encoding/pem"
	"encoding/xml"
	"fmt"
	"net/http"
	"net/url"
	"os"
	"time"

	"github.com/crewjam/saml"
	"github.com/google/uuid"
	"github.com/marcogenualdo/sso-proxy/internal/auth"
	"github.com/marcogenualdo/sso-proxy/internal/cache"
	"github.com/marcogenualdo/sso-proxy/internal/config"
)

type Provider struct {
	id             string
	name           string
	cfg            config.SAMLConfig
	headerMappings map[string]string
	cache          cache.Cache

	sp          *saml.ServiceProvider
	idpMetadata *saml.EntityDescriptor
}

func NewProvider(ctx context.Context, providerCfg config.ProviderConfig, cache cache.Cache, baseURL string) (*Provider, error) {
	if providerCfg.SAML == nil {
		return nil, fmt.Errorf("SAML config is required")
	}

	certData, err := os.ReadFile(providerCfg.SAML.CertificatePath)
	if err != nil {
		return nil, fmt.Errorf("failed to read certificate: %w", err)
	}

	keyData, err := os.ReadFile(providerCfg.SAML.PrivateKeyPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read private key: %w", err)
	}

	certBlock, _ := pem.Decode(certData)
	if certBlock == nil {
		return nil, fmt.Errorf("failed to decode certificate PEM")
	}

	cert, err := x509.ParseCertificate(certBlock.Bytes)
	if err != nil {
		return nil, fmt.Errorf("failed to parse certificate: %w", err)
	}

	keyBlock, _ := pem.Decode(keyData)
	if keyBlock == nil {
		return nil, fmt.Errorf("failed to decode private key PEM")
	}

	key, err := x509.ParsePKCS1PrivateKey(keyBlock.Bytes)
	if err != nil {
		key8, err8 := x509.ParsePKCS8PrivateKey(keyBlock.Bytes)
		if err8 != nil {
			return nil, fmt.Errorf("failed to parse private key: %w (PKCS1: %v)", err8, err)
		}
		var ok bool
		key, ok = key8.(*rsa.PrivateKey)
		if !ok {
			return nil, fmt.Errorf("private key is not RSA")
		}
	}

	idpMetadata, err := fetchIDPMetadata(ctx, *providerCfg.SAML)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch IdP metadata: %w", err)
	}

	acsURL, err := url.Parse(providerCfg.SAML.ACSURL)
	if err != nil {
		return nil, fmt.Errorf("invalid ACS URL: %w", err)
	}

	metadataURL, err := url.Parse(baseURL + "/auth/saml/" + providerCfg.ID + "/metadata")
	if err != nil {
		return nil, fmt.Errorf("invalid metadata URL: %w", err)
	}

	sp := &saml.ServiceProvider{
		EntityID:          providerCfg.SAML.SPEntityID,
		Key:               key,
		Certificate:       cert,
		MetadataURL:       *metadataURL,
		AcsURL:            *acsURL,
		IDPMetadata:       idpMetadata,
		AllowIDPInitiated: true,
	}

	return &Provider{
		id:             providerCfg.ID,
		name:           providerCfg.Name,
		cfg:            *providerCfg.SAML,
		headerMappings: providerCfg.HeaderMappings,
		cache:          cache,
		sp:             sp,
		idpMetadata:    idpMetadata,
	}, nil
}

func (p *Provider) ID() string {
	return p.id
}

func (p *Provider) Name() string {
	return p.name
}

func (p *Provider) Type() string {
	return "saml"
}

func (p *Provider) GetHeaderMappings() map[string]string {
	return p.headerMappings
}

func (p *Provider) InitiateAuth(ctx context.Context, redirectURL string) (*auth.AuthRedirect, error) {
	requestID := uuid.New().String()

	authReq, err := p.sp.MakeAuthenticationRequest(
		p.sp.GetSSOBindingLocation(saml.HTTPRedirectBinding),
		saml.HTTPRedirectBinding,
		saml.HTTPPostBinding,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create authentication request: %w", err)
	}

	authReq.ID = requestID

	samlReq := &auth.SAMLRequest{
		ID:         requestID,
		ProviderID: p.id,
		RelayState: redirectURL,
		CreatedAt:  time.Now(),
	}

	reqData, err := json.Marshal(samlReq)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}

	redirectURLParsed, err := authReq.Redirect("", p.sp)
	if err != nil {
		return nil, fmt.Errorf("failed to create redirect: %w", err)
	}
	redirectURL = redirectURLParsed.String()

	return &auth.AuthRedirect{
		URL:       redirectURL,
		Method:    "GET",
		CacheKey:  "saml:request:" + requestID,
		CacheData: reqData,
		CacheTTL:  5 * time.Minute,
	}, nil
}

func (p *Provider) HandleCallback(ctx context.Context, req *http.Request) (*auth.Session, error) {
	err := req.ParseForm()
	if err != nil {
		return nil, fmt.Errorf("failed to parse form: %w", err)
	}

	samlResponse := req.PostForm.Get("SAMLResponse")
	if samlResponse == "" {
		return nil, fmt.Errorf("missing SAMLResponse")
	}

	assertion, err := p.sp.ParseResponse(req, []string{})
	if err != nil {
		return nil, fmt.Errorf("failed to parse SAML response: %w", err)
	}

	claims := make(map[string]interface{})

	if assertion.Subject != nil && assertion.Subject.NameID != nil {
		claims["name_id"] = assertion.Subject.NameID.Value
		claims["name_id_format"] = assertion.Subject.NameID.Format
	}

	for _, stmt := range assertion.AttributeStatements {
		for _, attr := range stmt.Attributes {
			if len(attr.Values) == 1 {
				claims[attr.Name] = attr.Values[0].Value
			} else if len(attr.Values) > 1 {
				values := make([]string, len(attr.Values))
				for i, v := range attr.Values {
					values[i] = v.Value
				}
				claims[attr.Name] = values
			}
		}
	}

	assertionData, err := xml.Marshal(assertion)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal assertion: %w", err)
	}

	expiresAt := time.Now().Add(24 * time.Hour)
	if assertion.Conditions != nil && !assertion.Conditions.NotOnOrAfter.IsZero() {
		expiresAt = assertion.Conditions.NotOnOrAfter
	}

	sessionID := uuid.New().String()
	session := &auth.Session{
		ID:           sessionID,
		ProviderID:   p.id,
		ProviderType: "saml",
		UserInfo:     claims,
		CreatedAt:    time.Now(),
		ExpiresAt:    expiresAt,
		Assertion:    string(assertionData),
		CSRFToken:    uuid.New().String(),
	}

	return session, nil
}

func (p *Provider) ValidateSession(ctx context.Context, session *auth.Session) error {
	if session.ProviderID != p.id {
		return fmt.Errorf("provider mismatch")
	}

	if time.Now().After(session.ExpiresAt) {
		return fmt.Errorf("session expired")
	}

	return nil
}

func (p *Provider) RefreshSession(ctx context.Context, session *auth.Session) (*auth.Session, error) {
	return nil, fmt.Errorf("SAML sessions cannot be refreshed")
}

func (p *Provider) GetMetadata() (*saml.EntityDescriptor, error) {
	return p.sp.Metadata(), nil
}

func fetchIDPMetadata(ctx context.Context, cfg config.SAMLConfig) (*saml.EntityDescriptor, error) {
	if cfg.IDPMetadataXML != "" {
		metadata := &saml.EntityDescriptor{}
		if err := xml.Unmarshal([]byte(cfg.IDPMetadataXML), metadata); err != nil {
			return nil, fmt.Errorf("failed to parse IdP metadata XML: %w", err)
		}
		return metadata, nil
	}

	if cfg.IDPMetadataURL != "" {
		req, err := http.NewRequestWithContext(ctx, "GET", cfg.IDPMetadataURL, nil)
		if err != nil {
			return nil, fmt.Errorf("failed to create metadata request: %w", err)
		}

		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			return nil, fmt.Errorf("failed to fetch metadata: %w", err)
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusOK {
			return nil, fmt.Errorf("metadata request returned status %d", resp.StatusCode)
		}

		metadata := &saml.EntityDescriptor{}
		if err := xml.NewDecoder(resp.Body).Decode(metadata); err != nil {
			return nil, fmt.Errorf("failed to decode metadata: %w", err)
		}

		return metadata, nil
	}

	return nil, fmt.Errorf("either idp_metadata_url or idp_metadata_xml must be provided")
}

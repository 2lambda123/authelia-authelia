package schema

import "time"

// IdentityProvidersConfiguration represents the IdentityProviders 2.0 configuration for Authelia.
type IdentityProvidersConfiguration struct {
	OIDC *OpenIDConnectConfiguration `mapstructure:"oidc"`
}

// OpenIDConnectConfiguration configuration for OpenID Connect.
type OpenIDConnectConfiguration struct {
	// This secret must be 32 bytes long
	HMACSecret       string `mapstructure:"hmac_secret"`
	IssuerPrivateKey string `mapstructure:"issuer_private_key"`

	IDTokenLifespan     time.Duration `mapstructure:"id_token_lifespan"`
	AccessTokenLifespan time.Duration `mapstructure:"access_token_lifespan"`

	Clients []OpenIDConnectClientConfiguration `mapstructure:"clients"`
}

// OpenIDConnectClientConfiguration configuration for an OpenID Connect client.
type OpenIDConnectClientConfiguration struct {
	ID            string   `mapstructure:"id"`
	Description   string   `mapstructure:"description"`
	Secret        string   `mapstructure:"secret"`
	RedirectURIs  []string `mapstructure:"redirect_uris"`
	Policy        string   `mapstructure:"authorization_policy"`
	Scopes        []string `mapstructure:"scopes"`
	GrantTypes    []string `mapstructure:"grant_types"`
	ResponseTypes []string `mapstructure:"response_types"`
}

var DefaultOpenIDConnectConfiguration = OpenIDConnectConfiguration{
	IDTokenLifespan:     time.Hour,
	AccessTokenLifespan: time.Hour,
}

// DefaultOpenIDConnectClientConfiguration contains defaults for OIDC AutheliaClients.
var DefaultOpenIDConnectClientConfiguration = OpenIDConnectClientConfiguration{
	Scopes:        []string{"openid", "groups", "profile", "email"},
	ResponseTypes: []string{"code"},
	GrantTypes:    []string{"refresh_token", "authorization_code"},
	Policy:        "two_factor",
}

package oidc

import (
	"context"
	"crypto/ecdsa"
	"crypto/rsa"
	"encoding/base64"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/go-crypt/crypt"
	"github.com/go-crypt/crypt/algorithm"
	"github.com/go-crypt/crypt/algorithm/plaintext"
	"github.com/golang-jwt/jwt/v5"
	"github.com/ory/fosite"
	"github.com/ory/x/errorsx"
	"github.com/pkg/errors"
	"github.com/valyala/fasthttp"
	"gopkg.in/square/go-jose.v2"
)

// NewHasher returns a new Hasher.
func NewHasher() (hasher *Hasher, err error) {
	hasher = &Hasher{}

	if hasher.decoder, err = crypt.NewDefaultDecoder(); err != nil {
		return nil, err
	}

	if err = plaintext.RegisterDecoderPlainText(hasher.decoder); err != nil {
		return nil, err
	}

	return hasher, nil
}

// Hasher implements the fosite.Hasher interface and adaptively compares hashes.
type Hasher struct {
	decoder algorithm.DecoderRegister
}

// Compare compares the hash with the data and returns an error if they don't match.
func (h Hasher) Compare(_ context.Context, hash, data []byte) (err error) {
	var digest algorithm.Digest

	if digest, err = h.decoder.Decode(string(hash)); err != nil {
		return err
	}

	if digest.MatchBytes(data) {
		return nil
	}

	return errPasswordsDoNotMatch
}

// Hash creates a new hash from data.
func (h Hasher) Hash(_ context.Context, data []byte) (hash []byte, err error) {
	return data, nil
}

// DefaultClientAuthenticationStrategy is a copy of fosite's with the addition of the client_secret_jwt method and some
// minor superficial changes.
//
//nolint:gocyclo // Complexity is necessary to remain in feature parity.
func (p *OpenIDConnectProvider) DefaultClientAuthenticationStrategy(ctx context.Context, r *http.Request, form url.Values) (client fosite.Client, err error) {
	switch assertionType := form.Get(FormParameterClientAssertionType); assertionType {
	case "":
		break
	case ClientAssertionJWTBearerType:
		return p.JWTBearerClientAuthenticationStrategy(ctx, r, form)
	default:
		return nil, errorsx.WithStack(fosite.ErrInvalidRequest.WithHintf("Unknown client_assertion_type '%s'.", assertionType))
	}

	clientID, clientSecret, err := clientCredentialsFromRequest(r, form)
	if err != nil {
		return nil, err
	}

	if client, err = p.Store.GetClient(ctx, clientID); err != nil {
		return nil, errorsx.WithStack(fosite.ErrInvalidClient.WithWrap(err).WithDebug(err.Error()))
	}

	if oidcClient, ok := client.(fosite.OpenIDConnectClient); ok {
		method := oidcClient.GetTokenEndpointAuthMethod()

		if form.Get(FormParameterClientID) != "" && form.Get(FormParameterClientSecret) != "" && method != ClientAuthMethodClientSecretPost {
			return nil, errorsx.WithStack(fosite.ErrInvalidClient.WithHintf("The OAuth 2.0 Client supports client authentication method '%s', but method 'client_secret_post' was requested. You must configure the OAuth 2.0 client's 'token_endpoint_auth_method' value to accept 'client_secret_post'.", method))
		} else if _, secret, basicOk := r.BasicAuth(); basicOk && secret != "" && method != ClientAuthMethodClientSecretBasic {
			return nil, errorsx.WithStack(fosite.ErrInvalidClient.WithHintf("The OAuth 2.0 Client supports client authentication method '%s', but method 'client_secret_basic' was requested. You must configure the OAuth 2.0 client's 'token_endpoint_auth_method' value to accept 'client_secret_basic'.", method))
		} else if method != ClientAuthMethodNone && client.IsPublic() {
			return nil, errorsx.WithStack(fosite.ErrInvalidClient.WithHintf("The OAuth 2.0 Client supports client authentication method '%s', but method 'none' was requested. You must configure the OAuth 2.0 client's 'token_endpoint_auth_method' value to accept 'none'.", method))
		}
	}

	if client.IsPublic() {
		return client, nil
	}

	if err = p.checkClientSecret(ctx, client, []byte(clientSecret)); err != nil {
		return nil, errorsx.WithStack(fosite.ErrInvalidClient.WithWrap(err).WithDebug(err.Error()))
	}

	return client, nil
}

func (p *OpenIDConnectProvider) JWTBearerClientAuthenticationStrategy(ctx context.Context, r *http.Request, form url.Values) (client fosite.Client, err error) {
	if form.Has(FormParameterClientSecret) {
		return nil, errorsx.WithStack(fosite.ErrInvalidRequest.WithHintf("The client_secret request parameter must not be set when using client_assertion_type of '%s'.", ClientAssertionJWTBearerType))
	}

	if value := r.Header.Get(fasthttp.HeaderAuthorization); len(value) != 0 {
		return nil, errorsx.WithStack(fosite.ErrInvalidRequest.WithHintf("The Authorization request header must not be set when using client_assertion_type of '%s'.", ClientAssertionJWTBearerType))
	}

	var claims jwt.MapClaims

	client, _, claims, err = p.parseJWTAssertion(ctx, form)

	switch {
	case err == nil:
		break
	default:
		var e *fosite.RFC6749Error

		if errors.As(err, &e) {
			return nil, e
		}

		rfc := fosite.ErrInvalidClient.
			WithHint("Unable to verify the integrity of the 'client_assertion' value.").
			WithWrap(err)

		switch {
		case errors.Is(err, jwt.ErrTokenMalformed):
			return nil, errorsx.WithStack(rfc.WithDebug("The token is malformed."))
		case errors.Is(err, jwt.ErrTokenUsedBeforeIssued):
			return nil, errorsx.WithStack(rfc.WithDebug("The token was used before it was issued."))
		case errors.Is(err, jwt.ErrTokenExpired):
			return nil, errorsx.WithStack(rfc.WithDebug("The token is expired."))
		case errors.Is(err, jwt.ErrTokenNotValidYet):
			return nil, errorsx.WithStack(rfc.WithDebug("The token isn't valid yet."))
		case errors.Is(err, jwt.ErrTokenSignatureInvalid):
			return nil, errorsx.WithStack(rfc.WithDebug("The signature is invalid."))
		case errors.Is(err, jwt.ErrTokenInvalidClaims):
			return nil, errorsx.WithStack(rfc.WithDebug("The token claims are invalid."))
		default:
			return nil, errorsx.WithStack(fosite.ErrInvalidClient.WithHint("Unable to verify the integrity of the 'client_assertion' value.").WithWrap(err).WithDebug(err.Error()))
		}
	}

	tokenURL := p.Config.GetTokenURL(ctx)

	var (
		iss, sub, jti string
		raw           any
		ok            bool
		exp           *jwt.NumericDate
	)

	if iss, err = claims.GetIssuer(); err != nil {
		return nil, errorsx.WithStack(fosite.ErrInvalidClient.WithHint("Claim 'iss' from 'client_assertion' must match the 'client_id' of the OAuth 2.0 Client.").WithWrap(err))
	}

	if sub, err = claims.GetSubject(); err != nil {
		return nil, errorsx.WithStack(fosite.ErrInvalidClient.WithHint("Claim 'sub' from 'client_assertion' must match the 'client_id' of the OAuth 2.0 Client.").WithWrap(err))
	}

	if raw, ok = claims[ClaimJWTID]; !ok {
		return nil, errorsx.WithStack(fosite.ErrInvalidClient.WithHint("Claim 'jti' from 'client_assertion' must be set but it is not."))
	}

	if jti, ok = raw.(string); !ok {
		return nil, errorsx.WithStack(fosite.ErrInvalidClient.WithHint("Claim 'jti' from 'client_assertion' must be a string but it is not."))
	}

	switch {
	case iss != client.GetID():
		return nil, errorsx.WithStack(fosite.ErrInvalidClient.WithHint("Claim 'iss' from 'client_assertion' must match the 'client_id' of the OAuth 2.0 Client."))
	case sub != client.GetID():
		return nil, errorsx.WithStack(fosite.ErrInvalidClient.WithHint("Claim 'sub' from 'client_assertion' must match the 'client_id' of the OAuth 2.0 Client."))
	case tokenURL == "":
		return nil, errorsx.WithStack(fosite.ErrMisconfiguration.WithHint("The authorization server's token endpoint URL has not been set."))
	case len(jti) == 0:
		return nil, errorsx.WithStack(fosite.ErrInvalidClient.WithHint("Claim 'jti' from 'client_assertion' must be set but is not."))
	case p.Store.ClientAssertionJWTValid(ctx, jti) != nil:
		return nil, errorsx.WithStack(fosite.ErrJTIKnown.WithHint("Claim 'jti' from 'client_assertion' MUST only be used once."))
	}

	if exp, err = claims.GetExpirationTime(); err != nil {
		var epoch int64

		if epoch, ok = claims[ClaimExpirationTime].(int64); ok {
			exp = jwt.NewNumericDate(time.Unix(epoch, 0))
		} else {
			return nil, errorsx.WithStack(err)
		}
	}

	if err = p.Store.SetClientAssertionJWT(ctx, jti, exp.Time); err != nil {
		return nil, err
	}

	var (
		found bool
	)

	if auds, ok := claims[ClaimAudience].([]any); ok {
		for _, aud := range auds {
			if audience, ok := aud.(string); ok && audience == tokenURL {
				found = true
				break
			}
		}
	}

	if !found {
		return nil, errorsx.WithStack(fosite.ErrInvalidClient.WithHintf("Claim 'audience' from 'client_assertion' must match the authorization server's token endpoint '%s'.", tokenURL))
	}

	return client, nil
}

func (p *OpenIDConnectProvider) parseJWTAssertion(ctx context.Context, form url.Values) (client fosite.Client, token *jwt.Token, claims jwt.MapClaims, err error) {
	assertion := form.Get(FormParameterClientAssertion)
	if len(assertion) == 0 {
		return nil, nil, nil, errorsx.WithStack(fosite.ErrInvalidRequest.WithHintf("The client_assertion request parameter must be set when using client_assertion_type of '%s'.", ClientAssertionJWTBearerType))
	}

	var (
		clientID string
	)

	clientID = form.Get(FormParameterClientID)

	token, err = jwt.ParseWithClaims(assertion, jwt.MapClaims{}, func(token *jwt.Token) (any, error) {
		var (
			ok bool
		)

		claims, ok = token.Claims.(jwt.MapClaims)
		if !ok {
			return nil, errorsx.WithStack(fosite.ErrInvalidClient.WithHint("The claims could not be parsed due to an unknown error."))
		}

		if clientID == "" {
			var sub string

			if sub, ok = claims[ClaimSubject].(string); !ok {
				return nil, errorsx.WithStack(fosite.ErrInvalidClient.WithHint("The claim 'sub' from the client_assertion JSON Web Token is undefined."))
			} else {
				clientID = sub
			}
		}

		if client, err = p.Store.GetClient(ctx, clientID); err != nil {
			return nil, errorsx.WithStack(fosite.ErrInvalidClient.WithWrap(err).WithDebug(err.Error()))
		}

		fclient, ok := client.(*FullClient)
		if !ok {
			return nil, errorsx.WithStack(fosite.ErrInvalidRequest.WithHint("The client configuration does not support OpenID Connect specific authentication methods."))
		}

		switch fclient.GetTokenEndpointAuthMethod() {
		case ClientAuthMethodPrivateKeyJWT:
			switch token.Method {
			case jwt.SigningMethodRS256, jwt.SigningMethodRS384, jwt.SigningMethodRS512,
				jwt.SigningMethodPS256, jwt.SigningMethodPS384, jwt.SigningMethodPS512,
				jwt.SigningMethodES256, jwt.SigningMethodES384, jwt.SigningMethodES512:
				break
			default:
				return nil, errorsx.WithStack(fosite.ErrInvalidClient.WithHintf("This requested OAuth 2.0 client supports client authentication method '%s', however the '%s' JWA is not supported with this method.", ClientAuthMethodPrivateKeyJWT, token.Header[JWTHeaderKeyAlgorithm]))
			}
		case ClientAuthMethodClientSecretJWT:
			switch token.Method {
			case jwt.SigningMethodHS256, jwt.SigningMethodHS384, jwt.SigningMethodHS512:
				break
			default:
				return nil, errorsx.WithStack(fosite.ErrInvalidClient.WithHintf("This requested OAuth 2.0 client supports client authentication method '%s', however the '%s' JWA is not supported with this method.", ClientAuthMethodClientSecretJWT, token.Header[JWTHeaderKeyAlgorithm]))
			}
		case ClientAuthMethodNone:
			return nil, errorsx.WithStack(fosite.ErrInvalidClient.WithHint("This requested OAuth 2.0 client does not support client authentication, however 'client_assertion' was provided in the request."))
		case ClientAuthMethodClientSecretPost:
			fallthrough
		case ClientAuthMethodClientSecretBasic:
			return nil, errorsx.WithStack(fosite.ErrInvalidClient.WithHintf("This requested OAuth 2.0 client only supports client authentication method '%s', however 'client_assertion' was provided in the request.", fclient.GetTokenEndpointAuthMethod()))
		default:
			return nil, errorsx.WithStack(fosite.ErrInvalidClient.WithHintf("This requested OAuth 2.0 client only supports client authentication method '%s', however that method is not supported by this server.", fclient.GetTokenEndpointAuthMethod()))
		}

		if fclient.GetTokenEndpointAuthSigningAlgorithm() != fmt.Sprintf("%s", token.Header[JWTHeaderKeyAlgorithm]) {
			return nil, errorsx.WithStack(fosite.ErrInvalidClient.WithHintf("The 'client_assertion' uses signing algorithm '%s' but the requested OAuth 2.0 Client enforces signing algorithm '%s'.", token.Header[JWTHeaderKeyAlgorithm], fclient.GetTokenEndpointAuthSigningAlgorithm()))
		}

		switch token.Method {
		case jwt.SigningMethodRS256, jwt.SigningMethodRS384, jwt.SigningMethodRS512:
			return p.findClientPublicJWK(ctx, fclient, token, true)
		case jwt.SigningMethodES256, jwt.SigningMethodES384, jwt.SigningMethodES512:
			return p.findClientPublicJWK(ctx, fclient, token, false)
		case jwt.SigningMethodPS256, jwt.SigningMethodPS384, jwt.SigningMethodPS512:
			return p.findClientPublicJWK(ctx, fclient, token, true)
		case jwt.SigningMethodHS256, jwt.SigningMethodHS384, jwt.SigningMethodHS512:
			var secret *plaintext.Digest

			if secret, ok = fclient.Secret.PlainText(); ok {
				return secret.Key(), nil
			}

			return nil, errorsx.WithStack(fosite.ErrInvalidClient.WithHint("This client does not support authentication method 'client_secret_jwt' as the client secret is not in plaintext."))
		default:
			return nil, errorsx.WithStack(fosite.ErrInvalidClient.WithHintf("The 'client_assertion' request parameter uses unsupported signing algorithm '%s'.", token.Header[JWTHeaderKeyAlgorithm]))
		}
	}, jwt.WithIssuedAt())

	return client, token, claims, err
}

func (p *OpenIDConnectProvider) checkClientSecret(ctx context.Context, client fosite.Client, clientSecret []byte) (err error) {
	if err = p.Config.GetSecretsHasher(ctx).Compare(ctx, client.GetHashedSecret(), clientSecret); err == nil {
		return nil
	}

	cc, ok := client.(fosite.ClientWithSecretRotation)
	if !ok {
		return err
	}

	for _, hash := range cc.GetRotatedHashes() {
		if err = p.Config.GetSecretsHasher(ctx).Compare(ctx, hash, clientSecret); err == nil {
			return nil
		}
	}

	return err
}

func resolveClientPublicJWK(ctx context.Context, oidcClient fosite.OpenIDConnectClient, fetcher fosite.JWKSFetcherStrategy, t *jwt.Token, expectsRSAKey bool) (any, error) {
	if set := oidcClient.GetJSONWebKeys(); set != nil {
		return findPublicKey(t, set, expectsRSAKey)
	}

	if location := oidcClient.GetJSONWebKeysURI(); len(location) > 0 {
		keys, err := fetcher.Resolve(ctx, location, false)
		if err != nil {
			return nil, err
		}

		if key, err := findPublicKey(t, keys, expectsRSAKey); err == nil {
			return key, nil
		}

		keys, err = fetcher.Resolve(ctx, location, true)
		if err != nil {
			return nil, err
		}

		return findPublicKey(t, keys, expectsRSAKey)
	}

	return nil, errorsx.WithStack(fosite.ErrInvalidClient.WithHint("The OAuth 2.0 Client has no JSON Web Keys set registered, but they are needed to complete the request."))
}

func (p *OpenIDConnectProvider) findClientPublicJWK(ctx context.Context, oidcClient fosite.OpenIDConnectClient, t *jwt.Token, expectsRSAKey bool) (any, error) {
	if set := oidcClient.GetJSONWebKeys(); set != nil {
		return findPublicKey(t, set, expectsRSAKey)
	}

	if location := oidcClient.GetJSONWebKeysURI(); len(location) > 0 {
		keys, err := p.Config.GetJWKSFetcherStrategy(ctx).Resolve(ctx, location, false)
		if err != nil {
			return nil, err
		}

		if key, err := findPublicKey(t, keys, expectsRSAKey); err == nil {
			return key, nil
		}

		keys, err = p.Config.GetJWKSFetcherStrategy(ctx).Resolve(ctx, location, true)
		if err != nil {
			return nil, err
		}

		return findPublicKey(t, keys, expectsRSAKey)
	}

	return nil, errorsx.WithStack(fosite.ErrInvalidClient.WithHint("The OAuth 2.0 Client has no JSON Web Keys set registered, but they are needed to complete the request."))
}

func findPublicKey(t *jwt.Token, set *jose.JSONWebKeySet, expectsRSAKey bool) (any, error) {
	keys := set.Keys
	if len(keys) == 0 {
		return nil, errorsx.WithStack(fosite.ErrInvalidRequest.WithHintf("The retrieved JSON Web Key Set does not contain any key."))
	}

	kid, ok := t.Header[JWTHeaderKeyIdentifier].(string)
	if ok {
		keys = set.Key(kid)
	}

	if len(keys) == 0 {
		return nil, errorsx.WithStack(fosite.ErrInvalidRequest.WithHintf("The JSON Web Token uses signing key with kid '%s', which could not be found.", kid))
	}

	for _, key := range keys {
		if key.Use != KeyUseSignature {
			continue
		}

		if expectsRSAKey {
			if k, ok := key.Key.(*rsa.PublicKey); ok {
				return k, nil
			}
		} else {
			if k, ok := key.Key.(*ecdsa.PublicKey); ok {
				return k, nil
			}
		}
	}

	if expectsRSAKey {
		return nil, errorsx.WithStack(fosite.ErrInvalidRequest.WithHintf("Unable to find RSA public key with use='sig' for kid '%s' in JSON Web Key Set.", kid))
	} else {
		return nil, errorsx.WithStack(fosite.ErrInvalidRequest.WithHintf("Unable to find ECDSA public key with use='sig' for kid '%s' in JSON Web Key Set.", kid))
	}
}

func clientCredentialsFromRequest(r *http.Request, form url.Values) (clientID, clientSecret string, err error) {
	var ok bool

	switch clientID, clientSecret, ok, err = clientCredentialsFromBasicAuth(r); {
	case err != nil:
		return "", "", errorsx.WithStack(fosite.ErrInvalidRequest.WithHint("The client credentials in the HTTP authorization header could not be parsed. Either the scheme was missing, the scheme was invalid, or the value had malformed data.").WithWrap(err).WithDebug(err.Error()))
	case ok:
		return clientID, clientSecret, nil
	default:
		return clientCredentialsFromRequestBody(form, true)
	}
}

func clientCredentialsFromBasicAuth(r *http.Request) (clientID, clientSecret string, ok bool, err error) {
	auth := r.Header.Get(fasthttp.HeaderAuthorization)

	if auth == "" {
		return "", "", false, nil
	}

	scheme, value, ok := strings.Cut(auth, " ")

	if !ok {
		return "", "", false, errors.New("failed to parse http authorization header: invalid scheme: the scheme was missing")
	}

	if !strings.EqualFold(scheme, httpAuthSchemeBasic) {
		return "", "", false, fmt.Errorf("failed to parse http authorization header: invalid scheme: expected the %s scheme but received %s", httpAuthSchemeBasic, scheme)
	}

	c, err := base64.StdEncoding.DecodeString(value)
	if err != nil {
		return "", "", false, fmt.Errorf("failed to parse http authorization header: invalid value: malformed base64 data: %w", err)
	}

	cs := string(c)

	clientID, clientSecret, ok = strings.Cut(cs, ":")
	if !ok {
		return "", "", false, errors.New("failed to parse http authorization header: invalid value: the basic scheme separator was missing")
	}

	return clientID, clientSecret, true, nil
}

func clientCredentialsFromRequestBody(form url.Values, forceID bool) (clientID, clientSecret string, err error) {
	clientID = form.Get(FormParameterClientID)
	clientSecret = form.Get(FormParameterClientSecret)

	if clientID == "" && forceID {
		return "", "", errorsx.WithStack(fosite.ErrInvalidRequest.WithHint("Client credentials missing or malformed in both HTTP Authorization header and HTTP POST body."))
	}

	return clientID, clientSecret, nil
}

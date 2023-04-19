package oidc

import (
	"context"
	"crypto/ecdsa"
	"crypto/rsa"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"time"

	"github.com/go-crypt/crypt/algorithm/plaintext"
	"github.com/ory/fosite"
	"github.com/ory/fosite/token/jwt"
	"github.com/ory/x/errorsx"
	"github.com/pkg/errors"
	"gopkg.in/square/go-jose.v2"
)

// DefaultClientAuthenticationStrategy is a copy of fosite's with the addition of the client_secret_jwt method and some
// minor superficial changes.
//
//nolint:gocyclo // Complexity is necessary to remain in feature parity.
func (p *OpenIDConnectProvider) DefaultClientAuthenticationStrategy(ctx context.Context, r *http.Request, form url.Values) (client fosite.Client, err error) {
	if assertionType := form.Get(FormParameterClientAssertionType); assertionType == ClientAssertionJWTBearerType {
		assertion := form.Get(FormParameterClientAssertion)
		if len(assertion) == 0 {
			return nil, errorsx.WithStack(fosite.ErrInvalidRequest.WithHintf("The client_assertion request parameter must be set when using client_assertion_type of '%s'.", ClientAssertionJWTBearerType))
		}

		var (
			token    *jwt.Token
			clientID string
		)

		token, err = jwt.ParseWithClaims(assertion, jwt.MapClaims{}, func(t *jwt.Token) (any, error) {
			clientID, _, err = clientCredentialsFromRequestBody(form, false)
			if err != nil {
				return nil, err
			}

			if clientID == "" {
				claims := t.Claims
				if sub, ok := claims[ClaimSubject].(string); !ok {
					return nil, errorsx.WithStack(fosite.ErrInvalidClient.WithHint("The claim 'sub' from the client_assertion JSON Web Token is undefined."))
				} else {
					clientID = sub
				}
			}

			client, err = p.Store.GetClient(ctx, clientID)
			if err != nil {
				return nil, errorsx.WithStack(fosite.ErrInvalidClient.WithWrap(err).WithDebug(err.Error()))
			}

			oidcClient, ok := client.(*FullClient)
			if !ok {
				return nil, errorsx.WithStack(fosite.ErrInvalidRequest.WithHint("The client configuration does not support OpenID Connect specific authentication methods."))
			}

			switch oidcClient.GetTokenEndpointAuthMethod() {
			case ClientAuthMethodPrivateKeyJWT:
				break
			case ClientAuthMethodNone:
				return nil, errorsx.WithStack(fosite.ErrInvalidClient.WithHint("This requested OAuth 2.0 client does not support client authentication, however 'client_assertion' was provided in the request."))
			case ClientAuthMethodClientSecretPost:
				fallthrough
			case ClientAuthMethodClientSecretBasic:
				return nil, errorsx.WithStack(fosite.ErrInvalidClient.WithHintf("This requested OAuth 2.0 client only supports client authentication method '%s', however 'client_assertion' was provided in the request.", oidcClient.GetTokenEndpointAuthMethod()))
			case ClientAuthMethodClientSecretJWT:
				fallthrough
			default:
				return nil, errorsx.WithStack(fosite.ErrInvalidClient.WithHintf("This requested OAuth 2.0 client only supports client authentication method '%s', however that method is not supported by this server.", oidcClient.GetTokenEndpointAuthMethod()))
			}

			if oidcClient.GetTokenEndpointAuthSigningAlgorithm() != fmt.Sprintf("%s", t.Header["alg"]) {
				return nil, errorsx.WithStack(fosite.ErrInvalidClient.WithHintf("The 'client_assertion' uses signing algorithm '%s' but the requested OAuth 2.0 Client enforces signing algorithm '%s'.", t.Header["alg"], oidcClient.GetTokenEndpointAuthSigningAlgorithm()))
			}
			switch t.Method {
			case jose.RS256, jose.RS384, jose.RS512:
				return p.findClientPublicJWK(ctx, oidcClient, t, true)
			case jose.ES256, jose.ES384, jose.ES512:
				return p.findClientPublicJWK(ctx, oidcClient, t, false)
			case jose.PS256, jose.PS384, jose.PS512:
				return p.findClientPublicJWK(ctx, oidcClient, t, true)
			case jose.HS256, jose.HS384, jose.HS512:
				switch secret := oidcClient.Secret.(type) {
				case *plaintext.Digest:
					return secret.Key(), nil
				default:
					return nil, errorsx.WithStack(fosite.ErrInvalidClient.WithHint("This client does not support authentication method 'client_secret_jwt' as the client secret is not in plaintext."))
				}
			default:
				return nil, errorsx.WithStack(fosite.ErrInvalidClient.WithHintf("The 'client_assertion' request parameter uses unsupported signing algorithm '%s'.", t.Header["alg"]))
			}
		})

		if err != nil {
			var e *jwt.ValidationError

			if errors.As(err, &e) {
				if e.Inner != nil {
					return nil, e.Inner
				}

				return nil, errorsx.WithStack(fosite.ErrInvalidClient.WithHint("Unable to verify the integrity of the 'client_assertion' value.").WithWrap(err).WithDebug(err.Error()))
			}

			return nil, err
		} else if err = token.Claims.Valid(); err != nil {
			return nil, errorsx.WithStack(fosite.ErrInvalidClient.WithHint("Unable to verify the request object because its claims could not be validated, check if the expiry time is set correctly.").WithWrap(err).WithDebug(err.Error()))
		}

		claims := token.Claims

		var jti string

		if !claims.VerifyIssuer(clientID, true) {
			return nil, errorsx.WithStack(fosite.ErrInvalidClient.WithHint("Claim 'iss' from 'client_assertion' must match the 'client_id' of the OAuth 2.0 Client."))
		} else if p.Config.GetTokenURL(ctx) == "" {
			return nil, errorsx.WithStack(fosite.ErrMisconfiguration.WithHint("The authorization server's token endpoint URL has not been set."))
		} else if sub, ok := claims[ClaimSubject].(string); !ok || sub != clientID {
			return nil, errorsx.WithStack(fosite.ErrInvalidClient.WithHint("Claim 'sub' from 'client_assertion' must match the 'client_id' of the OAuth 2.0 Client."))
		} else if jti, ok = claims[ClaimJWTID].(string); !ok || len(jti) == 0 {
			return nil, errorsx.WithStack(fosite.ErrInvalidClient.WithHint("Claim 'jti' from 'client_assertion' must be set but is not."))
		} else if p.Store.ClientAssertionJWTValid(ctx, jti) != nil {
			return nil, errorsx.WithStack(fosite.ErrJTIKnown.WithHint("Claim 'jti' from 'client_assertion' MUST only be used once."))
		}

		err = nil

		var expiry int64

		switch exp := claims[ClaimExpirationTime].(type) {
		case float64:
			expiry = int64(exp)
		case int64:
			expiry = exp
		case json.Number:
			expiry, err = exp.Int64()
		default:
			err = fosite.ErrInvalidClient.WithHint("Unable to type assert the expiry time from claims. This should not happen as we validate the expiry time already earlier with token.Claims.Valid()")
		}

		if err != nil {
			return nil, errorsx.WithStack(err)
		}

		if err = p.Store.SetClientAssertionJWT(ctx, jti, time.Unix(expiry, 0)); err != nil {
			return nil, err
		}

		if auds, ok := claims[ClaimAudience].([]any); !ok {
			if !claims.VerifyAudience(p.Config.GetTokenURL(ctx), true) {
				return nil, errorsx.WithStack(fosite.ErrInvalidClient.WithHintf("Claim 'audience' from 'client_assertion' must match the authorization server's token endpoint '%s'.", p.Config.GetTokenURL(ctx)))
			}
		} else {
			var found bool
			for _, aud := range auds {
				if a, ok := aud.(string); ok && a == p.Config.GetTokenURL(ctx) {
					found = true
					break
				}
			}

			if !found {
				return nil, errorsx.WithStack(fosite.ErrInvalidClient.WithHintf("Claim 'audience' from 'client_assertion' must match the authorization server's token endpoint '%s'.", p.Config.GetTokenURL(ctx)))
			}
		}

		return client, nil
	} else if len(assertionType) > 0 {
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

		if ok && form.Get(FormParameterClientID) != "" && form.Get(FormParameterClientSecret) != "" && method != ClientAuthMethodClientSecretPost {
			return nil, errorsx.WithStack(fosite.ErrInvalidClient.WithHintf("The OAuth 2.0 Client supports client authentication method '%s', but method 'client_secret_post' was requested. You must configure the OAuth 2.0 client's 'token_endpoint_auth_method' value to accept 'client_secret_post'.", method))
		} else if _, secret, basicOk := r.BasicAuth(); basicOk && ok && secret != "" && method != ClientAuthMethodClientSecretBasic {
			return nil, errorsx.WithStack(fosite.ErrInvalidClient.WithHintf("The OAuth 2.0 Client supports client authentication method '%s', but method 'client_secret_basic' was requested. You must configure the OAuth 2.0 client's 'token_endpoint_auth_method' value to accept 'client_secret_basic'.", method))
		} else if ok && oidcClient.GetTokenEndpointAuthMethod() != ClientAuthMethodNone && client.IsPublic() {
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
		if key.Use != "sig" {
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
	if id, secret, ok := r.BasicAuth(); !ok {
		return clientCredentialsFromRequestBody(form, true)
	} else if clientID, err = url.QueryUnescape(id); err != nil {
		return "", "", errorsx.WithStack(fosite.ErrInvalidRequest.WithHint("The client id in the HTTP authorization header could not be decoded from 'application/x-www-form-urlencoded'.").WithWrap(err).WithDebug(err.Error()))
	} else if clientSecret, err = url.QueryUnescape(secret); err != nil {
		return "", "", errorsx.WithStack(fosite.ErrInvalidRequest.WithHint("The client secret in the HTTP authorization header could not be decoded from 'application/x-www-form-urlencoded'.").WithWrap(err).WithDebug(err.Error()))
	}

	return clientID, clientSecret, nil
}

func clientCredentialsFromRequestBody(form url.Values, forceID bool) (clientID, clientSecret string, err error) {
	clientID = form.Get(FormParameterClientID)
	clientSecret = form.Get(FormParameterClientSecret)

	if clientID == "" && forceID {
		return "", "", errorsx.WithStack(fosite.ErrInvalidRequest.WithHint("Client credentials missing or malformed in both HTTP Authorization header and HTTP POST body."))
	}

	return clientID, clientSecret, nil
}
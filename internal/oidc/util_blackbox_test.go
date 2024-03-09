package oidc_test

import (
	"net/url"
	"sort"
	"testing"
	"time"

	oauthelia2 "authelia.com/provider/oauth2"
	fjwt "authelia.com/provider/oauth2/token/jwt"
	"github.com/go-jose/go-jose/v4"
	"github.com/stretchr/testify/assert"
	"golang.org/x/text/language"

	"github.com/authelia/authelia/v4/internal/oidc"
)

func TestSortedJSONWebKey(t *testing.T) {
	testCases := []struct {
		name     string
		have     []jose.JSONWebKey
		expected []jose.JSONWebKey
	}{
		{
			"ShouldOrderByKID",
			[]jose.JSONWebKey{
				{KeyID: abc},
				{KeyID: "123"},
			},
			[]jose.JSONWebKey{
				{KeyID: "123"},
				{KeyID: abc},
			},
		},
		{
			"ShouldOrderByAlg",
			[]jose.JSONWebKey{
				{Algorithm: "RS256"},
				{Algorithm: "HS256"},
			},
			[]jose.JSONWebKey{
				{Algorithm: "HS256"},
				{Algorithm: "RS256"},
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			sort.Sort(oidc.SortedJSONWebKey(tc.have))

			assert.Equal(t, tc.expected, tc.have)
		})
	}
}

func TestRFC6750Header(t *testing.T) {
	testCaes := []struct {
		name     string
		have     *oauthelia2.RFC6749Error
		realm    string
		scope    string
		expected string
	}{
		{
			"ShouldEncodeAll",
			&oauthelia2.RFC6749Error{
				ErrorField:       "invalid_example",
				DescriptionField: "A description",
			},
			"abc",
			"openid",
			`realm="abc",error="invalid_example",error_description="A description",scope="openid"`,
		},
		{
			"ShouldEncodeBasic",
			&oauthelia2.RFC6749Error{
				ErrorField:       "invalid_example",
				DescriptionField: "A description",
			},
			"",
			"",
			`error="invalid_example",error_description="A description"`,
		},
	}

	for _, tc := range testCaes {
		t.Run(tc.name, func(t *testing.T) {
			assert.Equal(t, tc.expected, oidc.RFC6750Header(tc.realm, tc.scope, tc.have))
		})
	}
}

func TestIsJWTProfileAccessToken(t *testing.T) {
	testCases := []struct {
		name     string
		have     *fjwt.Token
		expected bool
	}{
		{
			"ShouldReturnFalseOnNilToken",
			nil,
			false,
		},
		{
			"ShouldReturnFalseOnNilTokenHeader",
			&fjwt.Token{Header: nil},
			false,
		},
		{
			"ShouldReturnFalseOnEmptyHeader",
			&fjwt.Token{Header: map[string]any{}},
			false,
		},
		{
			"ShouldReturnFalseOnInvalidKeyTypeHeaderType",
			&fjwt.Token{Header: map[string]any{oidc.JWTHeaderKeyType: 123}},
			false,
		},
		{
			"ShouldReturnFalseOnInvalidKeyTypeHeaderValue",
			&fjwt.Token{Header: map[string]any{oidc.JWTHeaderKeyType: "JWT"}},
			false,
		},
		{
			"ShouldReturnTrue",
			&fjwt.Token{Header: map[string]any{oidc.JWTHeaderKeyType: oidc.JWTHeaderTypeValueAccessTokenJWT}},
			true,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			assert.Equal(t, tc.expected, oidc.IsJWTProfileAccessToken(tc.have))
		})
	}
}

func TestGetLangFromRequester(t *testing.T) {
	testCases := []struct {
		name     string
		have     oauthelia2.Requester
		expected language.Tag
	}{
		{
			"ShouldReturnDefault",
			&TestGetLangRequester{},
			language.English,
		},
		{
			"ShouldReturnEmpty",
			&oauthelia2.Request{},
			language.Tag{},
		},
		{
			"ShouldReturnValueFromRequest",
			&oauthelia2.Request{
				Lang: language.French,
			},
			language.French,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			assert.Equal(t, tc.expected, oidc.GetLangFromRequester(tc.have))
		})
	}
}

type TestGetLangRequester struct {
}

func (t TestGetLangRequester) SetID(id string) {}

func (t TestGetLangRequester) GetID() string {
	return ""
}

func (t TestGetLangRequester) GetRequestedAt() (requestedAt time.Time) {
	return time.Now().UTC()
}

func (t TestGetLangRequester) GetClient() (client oauthelia2.Client) {
	return nil
}

func (t TestGetLangRequester) GetRequestedScopes() (scopes oauthelia2.Arguments) {
	return nil
}

func (t TestGetLangRequester) GetRequestedAudience() (audience oauthelia2.Arguments) {
	return nil
}

func (t TestGetLangRequester) SetRequestedScopes(scopes oauthelia2.Arguments) {}

func (t TestGetLangRequester) SetRequestedAudience(audience oauthelia2.Arguments) {}

func (t TestGetLangRequester) AppendRequestedScope(scope string) {}

func (t TestGetLangRequester) GetGrantedScopes() (grantedScopes oauthelia2.Arguments) {
	return nil
}

func (t TestGetLangRequester) GetGrantedAudience() (grantedAudience oauthelia2.Arguments) {
	return nil
}

func (t TestGetLangRequester) GrantScope(scope string) {}

func (t TestGetLangRequester) GrantAudience(audience string) {}

func (t TestGetLangRequester) GetSession() (session oauthelia2.Session) {
	return nil
}

func (t TestGetLangRequester) SetSession(session oauthelia2.Session) {}

func (t TestGetLangRequester) GetRequestForm() url.Values {
	return nil
}

func (t TestGetLangRequester) Merge(requester oauthelia2.Requester) {}

func (t TestGetLangRequester) Sanitize(allowedParameters []string) oauthelia2.Requester {
	return nil
}

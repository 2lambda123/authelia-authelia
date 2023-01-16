package suites

import (
	"context"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/url"
	"path"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"
	"github.com/valyala/fasthttp"

	"github.com/authelia/authelia/v4/internal/storage"
	"github.com/authelia/authelia/v4/internal/utils"
)

type StandaloneWebDriverSuite struct {
	*RodSuite
}

func NewStandaloneWebDriverSuite() *StandaloneWebDriverSuite {
	return &StandaloneWebDriverSuite{RodSuite: new(RodSuite)}
}

func (s *StandaloneWebDriverSuite) SetupSuite() {
	browser, err := StartRod()

	if err != nil {
		log.Fatal(err)
	}

	s.RodSession = browser
}

func (s *StandaloneWebDriverSuite) TearDownSuite() {
	err := s.RodSession.Stop()

	if err != nil {
		log.Fatal(err)
	}
}

func (s *StandaloneWebDriverSuite) SetupTest() {
	s.Page = s.doCreateTab(s.T(), HomeBaseURL)
	s.verifyIsHome(s.T(), s.Page)
}

func (s *StandaloneWebDriverSuite) TearDownTest() {
	s.collectCoverage(s.Page)
	s.MustClose()
}

func (s *StandaloneWebDriverSuite) TestShouldLetUserKnowHeIsAlreadyAuthenticated() {
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer func() {
		cancel()
		s.collectScreenshot(ctx.Err(), s.Page)
	}()

	_ = s.doRegisterAndLogin2FA(s.T(), s.Context(ctx), "john", "password", false, "")

	// Visit home page to change context.
	s.doVisit(s.T(), s.Context(ctx), HomeBaseURL)
	s.verifyIsHome(s.T(), s.Context(ctx))

	// Visit the login page and wait for redirection to 2FA page with success icon displayed.
	s.doVisit(s.T(), s.Context(ctx), GetLoginBaseURL(BaseDomain))
	s.verifyIsAuthenticatedPage(s.T(), s.Context(ctx))
}

func (s *StandaloneWebDriverSuite) TestShouldRedirectAfterOneFactorOnAnotherTab() {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	targetURL := fmt.Sprintf("%s/secret.html", SingleFactorBaseURL)
	page2 := s.Browser().MustPage(targetURL)

	defer func() {
		cancel()
		s.collectScreenshot(ctx.Err(), s.Page)
		s.collectScreenshot(ctx.Err(), page2)
		page2.MustClose()
	}()

	// Open second tab with secret page.
	page2.MustWaitLoad()

	// Switch to first, visit the login page and wait for redirection to secret page with secret displayed.
	s.Page.MustActivate()
	s.doLoginOneFactor(s.T(), s.Context(ctx), "john", "password", false, BaseDomain, targetURL)
	s.verifySecretAuthorized(s.T(), s.Page)

	// Switch to second tab and wait for redirection to secret page with secret displayed.
	page2.MustActivate()
	s.verifySecretAuthorized(s.T(), page2.Context(ctx))
}

func (s *StandaloneWebDriverSuite) TestShouldRedirectAlreadyAuthenticatedUser() {
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer func() {
		cancel()
		s.collectScreenshot(ctx.Err(), s.Page)
	}()

	_ = s.doRegisterAndLogin2FA(s.T(), s.Context(ctx), "john", "password", false, "")

	// Visit home page to change context.
	s.doVisit(s.T(), s.Context(ctx), HomeBaseURL)
	s.verifyIsHome(s.T(), s.Context(ctx))

	// Visit the login page and wait for redirection to 2FA page with success icon displayed.
	s.doVisit(s.T(), s.Context(ctx), fmt.Sprintf("%s?rd=https://secure.example.com:8080", GetLoginBaseURL(BaseDomain)))

	_, err := s.Page.ElementR("h1", "Public resource")
	require.NoError(s.T(), err)
	s.verifyURLIs(s.T(), s.Context(ctx), "https://secure.example.com:8080/")
}

func (s *StandaloneWebDriverSuite) TestShouldNotRedirectAlreadyAuthenticatedUserToUnsafeURL() {
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer func() {
		cancel()
		s.collectScreenshot(ctx.Err(), s.Page)
	}()

	_ = s.doRegisterAndLogin2FA(s.T(), s.Context(ctx), "john", "password", false, "")

	// Visit home page to change context.
	s.doVisit(s.T(), s.Context(ctx), HomeBaseURL)
	s.verifyIsHome(s.T(), s.Context(ctx))

	// Visit the login page and wait for redirection to 2FA page with success icon displayed.
	s.doVisit(s.T(), s.Context(ctx), fmt.Sprintf("%s?rd=https://secure.example.local:8080", GetLoginBaseURL(BaseDomain)))
	s.verifyNotificationDisplayed(s.T(), s.Context(ctx), "Redirection was determined to be unsafe and aborted. Ensure the redirection URL is correct.")
}

func (s *StandaloneWebDriverSuite) TestShouldCheckUserIsAskedToRegisterDevice() {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer func() {
		cancel()
		s.collectScreenshot(ctx.Err(), s.Page)
	}()

	username := "john"
	password := "password"

	// Clean up any TOTP secret already in DB.
	provider := storage.NewSQLiteProvider(&storageLocalTmpConfig)

	require.NoError(s.T(), provider.DeleteTOTPConfiguration(ctx, username))

	// Login one factor.
	s.doLoginOneFactor(s.T(), s.Context(ctx), username, password, false, BaseDomain, "")

	// Check the user is asked to register a new device.
	s.WaitElementLocatedByClassName(s.T(), s.Context(ctx), "state-not-registered")

	// Then register the TOTP factor.
	s.doRegisterTOTP(s.T(), s.Context(ctx))
	// And logout.
	s.doLogout(s.T(), s.Context(ctx))

	// Login one factor again.
	s.doLoginOneFactor(s.T(), s.Context(ctx), username, password, false, BaseDomain, "")

	// now the user should be asked to perform 2FA.
	s.WaitElementLocatedByClassName(s.T(), s.Context(ctx), "state-method")
}

type StandaloneSuite struct {
	suite.Suite
}

func NewStandaloneSuite() *StandaloneSuite {
	return &StandaloneSuite{}
}

func (s *StandaloneSuite) TestShouldRespectMethodsACL() {
	req, err := http.NewRequest("GET", fmt.Sprintf("%s/api/verify?rd=%s", AutheliaBaseURL, GetLoginBaseURL(BaseDomain)), nil)
	s.Assert().NoError(err)
	req.Header.Set("X-Forwarded-Method", "GET")
	req.Header.Set("X-Forwarded-Proto", "https")
	req.Header.Set("X-Forwarded-Host", fmt.Sprintf("secure.%s", BaseDomain))
	req.Header.Set("X-Forwarded-URI", "/")
	req.Header.Set("Accept", "text/html; charset=utf8")

	client := NewHTTPClient()
	res, err := client.Do(req)
	s.Assert().NoError(err)
	s.Assert().Equal(res.StatusCode, 302)
	body, err := io.ReadAll(res.Body)
	s.Assert().NoError(err)

	urlEncodedAdminURL := url.QueryEscape(SecureBaseURL + "/")
	s.Assert().Equal(fmt.Sprintf("<a href=\"%s\">302 Found</a>", utils.StringHTMLEscape(fmt.Sprintf("%s/?rd=%s&rm=GET", GetLoginBaseURL(BaseDomain), urlEncodedAdminURL))), string(body))

	req.Header.Set("X-Forwarded-Method", "OPTIONS")

	res, err = client.Do(req)
	s.Assert().NoError(err)
	s.Assert().Equal(res.StatusCode, 200)
}

func (s *StandaloneSuite) TestShouldRespondWithCorrectStatusCode() {
	req, err := http.NewRequest("GET", fmt.Sprintf("%s/api/verify?rd=%s", AutheliaBaseURL, GetLoginBaseURL(BaseDomain)), nil)
	s.Assert().NoError(err)
	req.Header.Set("X-Forwarded-Method", "GET")
	req.Header.Set("X-Forwarded-Proto", "https")
	req.Header.Set("X-Forwarded-Host", fmt.Sprintf("secure.%s", BaseDomain))
	req.Header.Set("X-Forwarded-URI", "/")
	req.Header.Set("Accept", "text/html; charset=utf8")

	client := NewHTTPClient()
	res, err := client.Do(req)
	s.Assert().NoError(err)
	s.Assert().Equal(res.StatusCode, 302)
	body, err := io.ReadAll(res.Body)
	s.Assert().NoError(err)

	urlEncodedAdminURL := url.QueryEscape(SecureBaseURL + "/")
	s.Assert().Equal(fmt.Sprintf("<a href=\"%s\">302 Found</a>", utils.StringHTMLEscape(fmt.Sprintf("%s/?rd=%s&rm=GET", GetLoginBaseURL(BaseDomain), urlEncodedAdminURL))), string(body))

	req.Header.Set("X-Forwarded-Method", "POST")

	res, err = client.Do(req)
	s.Assert().NoError(err)
	s.Assert().Equal(res.StatusCode, 303)
	body, err = io.ReadAll(res.Body)
	s.Assert().NoError(err)

	urlEncodedAdminURL = url.QueryEscape(SecureBaseURL + "/")
	s.Assert().Equal(fmt.Sprintf("<a href=\"%s\">303 See Other</a>", utils.StringHTMLEscape(fmt.Sprintf("%s/?rd=%s&rm=POST", GetLoginBaseURL(BaseDomain), urlEncodedAdminURL))), string(body))
}

// Standard case using nginx.
func (s *StandaloneSuite) TestShouldVerifyAPIVerifyUnauthorized() {
	req, err := http.NewRequest("GET", fmt.Sprintf("%s/api/verify", AutheliaBaseURL), nil)
	s.Assert().NoError(err)
	req.Header.Set("X-Forwarded-Proto", "https")
	req.Header.Set("X-Original-URL", AdminBaseURL)
	req.Header.Set("Accept", "text/html; charset=utf8")

	client := NewHTTPClient()
	res, err := client.Do(req)
	s.Assert().NoError(err)
	s.Assert().Equal(res.StatusCode, 401)
	body, err := io.ReadAll(res.Body)
	s.Assert().NoError(err)
	s.Assert().Equal("401 Unauthorized", string(body))
}

// Standard case using Kubernetes.
func (s *StandaloneSuite) TestShouldVerifyAPIVerifyRedirectFromXOriginalURL() {
	req, err := http.NewRequest("GET", fmt.Sprintf("%s/api/verify?rd=%s", AutheliaBaseURL, GetLoginBaseURL(BaseDomain)), nil)
	s.Assert().NoError(err)
	req.Header.Set("X-Forwarded-Proto", "https")
	req.Header.Set("X-Original-URL", AdminBaseURL)
	req.Header.Set("Accept", "text/html; charset=utf8")

	client := NewHTTPClient()
	res, err := client.Do(req)
	s.Assert().NoError(err)
	s.Assert().Equal(res.StatusCode, 302)
	body, err := io.ReadAll(res.Body)
	s.Assert().NoError(err)

	urlEncodedAdminURL := url.QueryEscape(AdminBaseURL)
	s.Assert().Equal(fmt.Sprintf("<a href=\"%s\">302 Found</a>", utils.StringHTMLEscape(fmt.Sprintf("%s/?rd=%s&rm=GET", GetLoginBaseURL(BaseDomain), urlEncodedAdminURL))), string(body))
}

func (s *StandaloneSuite) TestShouldVerifyAPIVerifyRedirectFromXOriginalHostURI() {
	req, err := http.NewRequest("GET", fmt.Sprintf("%s/api/verify?rd=%s", AutheliaBaseURL, GetLoginBaseURL(BaseDomain)), nil)
	s.Assert().NoError(err)
	req.Header.Set("X-Forwarded-Proto", "https")
	req.Header.Set("X-Forwarded-Host", "secure.example.com:8080")
	req.Header.Set("X-Forwarded-URI", "/")
	req.Header.Set("Accept", "text/html; charset=utf8")

	client := NewHTTPClient()
	res, err := client.Do(req)
	s.Assert().NoError(err)
	s.Assert().Equal(res.StatusCode, 302)
	body, err := io.ReadAll(res.Body)
	s.Assert().NoError(err)

	urlEncodedAdminURL := url.QueryEscape(SecureBaseURL + "/")
	s.Assert().Equal(fmt.Sprintf("<a href=\"%s\">302 Found</a>", utils.StringHTMLEscape(fmt.Sprintf("%s/?rd=%s&rm=GET", GetLoginBaseURL(BaseDomain), urlEncodedAdminURL))), string(body))
}

func (s *StandaloneSuite) TestShouldVerifyAuthzResponseForAuthRequest() {
	client := NewHTTPClient()

	testCases := []struct {
		name           string
		originalMethod string
		originalURL    *url.URL
		status         int
		body           string
	}{
		{"ShouldDenyMethodPOST", http.MethodPost, MustParseURL("https://secure.example.com:8080/"), http.StatusUnauthorized, "abc"},
		{"ShouldAllowMethodPOST", http.MethodPost, MustParseURL("https://public.example.com:8080/"), http.StatusOK, "abc"},
	}

	for _, tc := range testCases {
		s.T().Run(tc.name, func(t *testing.T) {
			for _, method := range []string{http.MethodGet, http.MethodPost} {
				t.Run("Method"+method, func(t *testing.T) {
					req, err := http.NewRequest(method, fmt.Sprintf("%s/api/authz/auth-request?authelia_url=%s", AutheliaBaseURL, GetLoginBaseURL(BaseDomain)), nil)
					s.Assert().NoError(err)
					req.Header.Set("X-Original-Method", tc.originalMethod)
					req.Header.Set("X-Original-URL", tc.originalURL.String())
					req.Header.Set("Accept", "text/html; charset=utf8")

					res, err := client.Do(req)
					s.Assert().NoError(err)

					var body []byte

					switch method {
					case http.MethodGet, http.MethodHead:
						s.Assert().Equal(tc.status, res.StatusCode)

						body, err = io.ReadAll(res.Body)
						s.Assert().NoError(err)

						switch tc.status {
						case http.StatusFound, http.StatusMovedPermanently, http.StatusPermanentRedirect, http.StatusSeeOther, http.StatusTemporaryRedirect:
							s.Assert().Equal(fmt.Sprintf("<a href=\"%s\">%d %s</a>", utils.StringHTMLEscape(fmt.Sprintf("%s/?rd=%s&rm=%s", GetLoginBaseURL(BaseDomain), tc.originalURL.String(), tc.originalMethod)), tc.status, fasthttp.StatusMessage(tc.status)), string(body))
						default:
							s.Assert().Equal(fmt.Sprintf("%d %s", tc.status, fasthttp.StatusMessage(tc.status)), string(body))
						}
					default:
						s.Assert().Equal(http.StatusMethodNotAllowed, res.StatusCode)

						body, err = io.ReadAll(res.Body)
						s.Assert().NoError(err)

						s.Assert().Equal(fmt.Sprintf("%d %s", http.StatusMethodNotAllowed, fasthttp.StatusMessage(http.StatusMethodNotAllowed)), string(body))
					}
				})
			}
		})
	}
}

func MustParseURL(in string) *url.URL {
	if u, err := url.ParseRequestURI(in); err != nil {
		panic(err)
	} else {
		return u
	}
}

func (s *StandaloneSuite) TestShouldVerifyAuthzResponseForExtAuthz() {
	client := NewHTTPClient()

	testCases := []struct {
		name        string
		originalURL *url.URL
		success     bool
		body        string
	}{
		{"ShouldDeny", MustParseURL("https://secure.example.com:8080/"), false, "abc"},
		{"ShouldAllow", MustParseURL("https://public.example.com:8080/"), true, "abc"},
	}

	for _, tc := range testCases {
		s.T().Run(tc.name, func(t *testing.T) {
			for _, method := range []string{http.MethodHead, http.MethodGet, http.MethodPost, http.MethodDelete, http.MethodPut} {
				t.Run("Method"+method, func(t *testing.T) {
					reqURL, err := url.ParseRequestURI(AutheliaBaseURL)
					s.Assert().NoError(err)

					reqURL.Path = "/api/authz/ext-authz"
					if tc.originalURL.Path != "" && tc.originalURL.Path != "/" {
						reqURL.Path = path.Join(reqURL.Path, tc.originalURL.Path)
					}

					req, err := http.NewRequest(method, reqURL.String(), nil)
					s.Assert().NoError(err)
					req.Host = tc.originalURL.Host

					req.Header.Set("X-Authelia-URL", GetLoginBaseURL(BaseDomain))
					req.Header.Set("X-Forwarded-Proto", "https")
					req.Header.Set("Accept", "text/html; charset=utf8")

					res, err := client.Do(req)
					s.Assert().NoError(err)

					var status int

					if tc.success {
						status = http.StatusOK
					} else {
						switch method {
						case http.MethodGet, http.MethodOptions:
							status = http.StatusFound
						default:
							status = http.StatusSeeOther
						}
					}

					s.Assert().Equal(status, res.StatusCode)

					body, err := io.ReadAll(res.Body)
					s.Assert().NoError(err)

					if !tc.success {
						expected, err := url.ParseRequestURI(GetLoginBaseURL(BaseDomain))
						s.Assert().NoError(err)

						query := expected.Query()

						query.Set("rd", tc.originalURL.String())
						query.Set("rm", method)

						expected.RawQuery = query.Encode()

						switch method {
						case http.MethodHead:
							s.Assert().Equal("", string(body))
						default:
							s.Assert().Equal(fmt.Sprintf(`<a href="%s">%d %s</a>`, utils.StringHTMLEscape(expected.String()), status, fasthttp.StatusMessage(status)), string(body))
						}
					} else {
						s.Assert().Equal("200 OK", string(body))
					}
				})
			}
		})
	}
}

func (s *StandaloneSuite) TestShouldRecordMetrics() {
	client := NewHTTPClient()

	req, err := http.NewRequest("GET", fmt.Sprintf("%s/api/health", LoginBaseURL), nil)
	s.Require().NoError(err)

	res, err := client.Do(req)
	s.Require().NoError(err)
	s.Assert().Equal(res.StatusCode, 200)

	req, err = http.NewRequest("GET", fmt.Sprintf("%s/metrics", LoginBaseURL), nil)
	s.Require().NoError(err)

	res, err = client.Do(req)
	s.Require().NoError(err)
	s.Assert().Equal(res.StatusCode, 200)

	body, err := io.ReadAll(res.Body)
	s.Require().NoError(err)

	metrics := string(body)

	s.Assert().Contains(metrics, "authelia_request_duration_bucket{")
	s.Assert().Contains(metrics, "authelia_request_duration_sum{")
}

func (s *StandaloneSuite) TestStandaloneWebDriverScenario() {
	suite.Run(s.T(), NewStandaloneWebDriverSuite())
}

func (s *StandaloneSuite) Test1FAScenario() {
	suite.Run(s.T(), New1FAScenario())
}

func (s *StandaloneSuite) Test2FAScenario() {
	suite.Run(s.T(), New2FAScenario())
}

func (s *StandaloneSuite) TestBypassPolicyScenario() {
	suite.Run(s.T(), NewBypassPolicyScenario())
}

func (s *StandaloneSuite) TestBackendProtectionScenario() {
	suite.Run(s.T(), NewBackendProtectionScenario())
}

func (s *StandaloneSuite) TestResetPasswordScenario() {
	suite.Run(s.T(), NewResetPasswordScenario())
}

func (s *StandaloneSuite) TestAvailableMethodsScenario() {
	suite.Run(s.T(), NewAvailableMethodsScenario([]string{"TIME-BASED ONE-TIME PASSWORD", "SECURITY KEY - WEBAUTHN"}))
}

func (s *StandaloneSuite) TestRedirectionURLScenario() {
	suite.Run(s.T(), NewRedirectionURLScenario())
}

func (s *StandaloneSuite) TestRedirectionCheckScenario() {
	suite.Run(s.T(), NewRedirectionCheckScenario())
}

func TestStandaloneSuite(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping suite test in short mode")
	}

	suite.Run(t, NewStandaloneSuite())
}

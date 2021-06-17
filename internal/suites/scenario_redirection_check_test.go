package suites

import (
	"context"
	"log"
	"testing"
	"time"

	"github.com/stretchr/testify/suite"
)

type RedirectionCheckScenario struct {
	*SeleniumSuite
}

func NewRedirectionCheckScenario() *RedirectionCheckScenario {
	return &RedirectionCheckScenario{
		SeleniumSuite: new(SeleniumSuite),
	}
}

func (s *RedirectionCheckScenario) SetupSuite() {
	wds, err := StartWebDriver()

	if err != nil {
		log.Fatal(err)
	}

	s.WebDriverSession = wds
}

func (s *RedirectionCheckScenario) TearDownSuite() {
	err := s.WebDriverSession.Stop()

	if err != nil {
		log.Fatal(err)
	}
}

func (s *RedirectionCheckScenario) SetupTest() {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	s.doLogout(ctx, s.T())
	s.doVisit(s.T(), HomeBaseURL)
	s.verifyIsHome(ctx, s.T())
}

var redirectionAuthorizations = map[string]bool{
	// external website
	"https://www.google.fr": false,
	// Not the right domain
	"https://public.example.com.a:8080/secret.html": false,
	// Not https
	"http://secure.example.com:8080/secret.html": false,
	// Domain handled by Authelia
	"https://secure.example.com:8080/secret.html": true,
}

func (s *RedirectionCheckScenario) TestShouldRedirectOnLoginOnlyWhenDomainIsSafe() {
	ctx, cancel := context.WithTimeout(context.Background(), 35*time.Second)
	defer cancel()

	secret := s.doRegisterThenLogout(ctx, s.T(), "john", "password")

	for url, redirected := range redirectionAuthorizations {
		s.T().Run(url, func(t *testing.T) {
			s.doLoginTwoFactor(ctx, t, "john", "password", false, secret, url)

			// TODO: Remove this if it's not necessary.
			// time.Sleep(1000 * time.Millisecond)

			if redirected {
				s.verifySecretAuthorized(ctx, t)
			} else {
				s.verifyIsAuthenticatedPage(ctx, t)
			}

			s.doLogout(ctx, t)
		})
	}
}

var logoutRedirectionURLs = map[string]bool{
	// external website
	"https://www.google.fr": false,
	// Not the right domain
	"https://public.example-not-right.com:8080/index.html": false,
	// Not https
	"http://public.example.com:8080/index.html": false,
	// Domain handled by Authelia
	"https://public.example.com:8080/index.html": true,
}

func (s *RedirectionCheckScenario) TestShouldRedirectOnLogoutOnlyWhenDomainIsSafe() {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	for url, success := range logoutRedirectionURLs {
		s.T().Run(url, func(t *testing.T) {
			s.doLogoutWithRedirect(ctx, t, url, !success)
		})
	}
}

func TestRedirectionCheckScenario(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping suite test in short mode")
	}

	suite.Run(t, NewRedirectionCheckScenario())
}

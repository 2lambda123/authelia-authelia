package suites

import (
	"testing"

	"github.com/stretchr/testify/suite"
)

type OIDCSuite struct {
	*SeleniumSuite
}

func NewOIDCSuite() *OIDCSuite {
	return &OIDCSuite{SeleniumSuite: new(SeleniumSuite)}
}

func (s *OIDCSuite) TestOIDCScenario() {
	suite.Run(s.T(), NewOIDCScenario())
}

func TestOIDCSuite(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping suite test in short mode")
	}

	suite.Run(t, NewOIDCSuite())
}

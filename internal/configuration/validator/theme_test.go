package validator

import (
	"testing"

	"github.com/stretchr/testify/suite"

	"github.com/authelia/authelia/v4/internal/configuration/schema"
)

type Theme struct {
	suite.Suite
	configuration *schema.Configuration
	validator     *schema.StructValidator
}

func (suite *Theme) SetupTest() {
	suite.validator = schema.NewStructValidator()
	suite.configuration = &schema.Configuration{
		Theme: "light",
	}
}

func (suite *Theme) TestShouldValidateCompleteConfiguration() {
	ValidateTheme(suite.configuration, suite.validator)

	suite.Assert().False(suite.validator.HasWarnings())
	suite.Assert().False(suite.validator.HasErrors())
}

func (suite *Theme) TestShouldRaiseErrorWhenInvalidThemeProvided() {
	suite.configuration.Theme = "invalid"

	ValidateTheme(suite.configuration, suite.validator)

	suite.Assert().False(suite.validator.HasWarnings())
	suite.Require().Len(suite.validator.Errors(), 1)

	suite.Assert().EqualError(suite.validator.Errors()[0], "option 'theme' must be one of 'light', 'dark', 'grey', 'auto' but it is configured as 'invalid'")
}

func TestThemes(t *testing.T) {
	suite.Run(t, new(Theme))
}

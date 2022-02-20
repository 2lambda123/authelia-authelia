package validator

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/authelia/authelia/v4/internal/configuration/schema"
)

func newDefaultSessionConfig() schema.SessionConfiguration {
	config := schema.SessionConfiguration{}
	config.Secret = testJWTSecret
	config.Domain = "example.com"

	return config
}

func TestShouldSetDefaultSessionName(t *testing.T) {
	validator := schema.NewStructValidator()
	config := newDefaultSessionConfig()

	ValidateSession(&config, validator)

	assert.False(t, validator.HasWarnings())
	assert.False(t, validator.HasErrors())
	assert.Equal(t, schema.DefaultSessionConfiguration.Name, config.Name)
}

func TestShouldSetDefaultSessionInactivity(t *testing.T) {
	validator := schema.NewStructValidator()
	config := newDefaultSessionConfig()

	ValidateSession(&config, validator)

	assert.False(t, validator.HasWarnings())
	assert.False(t, validator.HasErrors())
	assert.Equal(t, schema.DefaultSessionConfiguration.Inactivity, config.Inactivity)
}

func TestShouldSetDefaultSessionExpiration(t *testing.T) {
	validator := schema.NewStructValidator()
	config := newDefaultSessionConfig()

	ValidateSession(&config, validator)

	assert.False(t, validator.HasWarnings())
	assert.False(t, validator.HasErrors())
	assert.Equal(t, schema.DefaultSessionConfiguration.Expiration, config.Expiration)
}

func TestShouldSetDefaultSessionSameSite(t *testing.T) {
	validator := schema.NewStructValidator()
	config := newDefaultSessionConfig()

	ValidateSession(&config, validator)

	assert.False(t, validator.HasWarnings())
	assert.False(t, validator.HasErrors())
	assert.Equal(t, schema.DefaultSessionConfiguration.SameSite, config.SameSite)
}

func TestShouldHandleRedisConfigSuccessfully(t *testing.T) {
	validator := schema.NewStructValidator()
	config := newDefaultSessionConfig()

	ValidateSession(&config, validator)

	assert.Len(t, validator.Errors(), 0)
	validator.Clear()

	// Set redis config because password must be set only when redis is used.
	config.Redis = &schema.RedisSessionConfiguration{
		Host:     "redis.localhost",
		Port:     6379,
		Password: "password",
	}

	ValidateSession(&config, validator)

	assert.False(t, validator.HasWarnings())
	assert.False(t, validator.HasErrors())

	assert.Equal(t, 8, config.Redis.MaximumActiveConnections)
}

func TestShouldRaiseErrorWithInvalidRedisPortLow(t *testing.T) {
	validator := schema.NewStructValidator()
	config := newDefaultSessionConfig()

	config.Redis = &schema.RedisSessionConfiguration{
		Host: "authelia-port-1",
		Port: -1,
	}

	ValidateSession(&config, validator)

	assert.False(t, validator.HasWarnings())
	require.Len(t, validator.Errors(), 1)

	assert.EqualError(t, validator.Errors()[0], fmt.Sprintf(errFmtSessionRedisPortRange, "redis"))
}

func TestShouldRaiseErrorWithInvalidRedisPortHigh(t *testing.T) {
	validator := schema.NewStructValidator()
	config := newDefaultSessionConfig()

	config.Redis = &schema.RedisSessionConfiguration{
		Host: "authelia-port-1",
		Port: 65536,
	}

	ValidateSession(&config, validator)

	assert.False(t, validator.HasWarnings())
	require.Len(t, validator.Errors(), 1)

	assert.EqualError(t, validator.Errors()[0], fmt.Sprintf(errFmtSessionRedisPortRange, "redis"))
}

func TestShouldRaiseErrorWhenRedisIsUsedAndSecretNotSet(t *testing.T) {
	validator := schema.NewStructValidator()
	config := newDefaultSessionConfig()
	config.Secret = ""

	ValidateSession(&config, validator)

	assert.Len(t, validator.Errors(), 0)
	validator.Clear()

	// Set redis config because password must be set only when redis is used.
	config.Redis = &schema.RedisSessionConfiguration{
		Host: "redis.localhost",
		Port: 6379,
	}

	ValidateSession(&config, validator)

	assert.False(t, validator.HasWarnings())
	assert.Len(t, validator.Errors(), 1)
	assert.EqualError(t, validator.Errors()[0], fmt.Sprintf(errFmtSessionSecretRedisProvider, "redis"))
}

func TestShouldRaiseErrorWhenRedisHasHostnameButNoPort(t *testing.T) {
	validator := schema.NewStructValidator()
	config := newDefaultSessionConfig()

	ValidateSession(&config, validator)

	assert.Len(t, validator.Errors(), 0)
	validator.Clear()

	// Set redis config because password must be set only when redis is used.
	config.Redis = &schema.RedisSessionConfiguration{
		Host: "redis.localhost",
		Port: 0,
	}

	ValidateSession(&config, validator)

	assert.False(t, validator.HasWarnings())
	assert.Len(t, validator.Errors(), 1)
	assert.EqualError(t, validator.Errors()[0], "A redis port different than 0 must be provided")
}

func TestShouldRaiseOneErrorWhenRedisHighAvailabilityHasNodesWithNoHost(t *testing.T) {
	validator := schema.NewStructValidator()
	config := newDefaultSessionConfig()

	config.Redis = &schema.RedisSessionConfiguration{
		Host: "redis",
		Port: 6379,
		HighAvailability: &schema.RedisHighAvailabilityConfiguration{
			SentinelName:     "authelia-sentinel",
			SentinelPassword: "abc123",
			Nodes: []schema.RedisNode{
				{
					Port: 26379,
				},
				{
					Port: 26379,
				},
			},
		},
	}

	ValidateSession(&config, validator)

	errors := validator.Errors()

	assert.False(t, validator.HasWarnings())
	require.Len(t, errors, 1)

	assert.EqualError(t, errors[0], "The redis sentinel nodes require a host set but you have not set the host for one or more nodes")
}

func TestShouldRaiseOneErrorWhenRedisHighAvailabilityDoesNotHaveSentinelName(t *testing.T) {
	validator := schema.NewStructValidator()
	config := newDefaultSessionConfig()

	config.Redis = &schema.RedisSessionConfiguration{
		Host: "redis",
		Port: 6379,
		HighAvailability: &schema.RedisHighAvailabilityConfiguration{
			SentinelPassword: "abc123",
		},
	}

	ValidateSession(&config, validator)

	errors := validator.Errors()

	assert.False(t, validator.HasWarnings())
	require.Len(t, errors, 1)

	assert.EqualError(t, errors[0], "Session provider redis is configured for high availability but doesn't have a sentinel_name which is required")
}

func TestShouldUpdateDefaultPortWhenRedisSentinelHasNodes(t *testing.T) {
	validator := schema.NewStructValidator()
	config := newDefaultSessionConfig()

	config.Redis = &schema.RedisSessionConfiguration{
		Host: "redis",
		Port: 6379,
		HighAvailability: &schema.RedisHighAvailabilityConfiguration{
			SentinelName:     "authelia-sentinel",
			SentinelPassword: "abc123",
			Nodes: []schema.RedisNode{
				{
					Host: "node-1",
					Port: 333,
				},
				{
					Host: "node-2",
				},
				{
					Host: "node-3",
				},
			},
		},
	}

	ValidateSession(&config, validator)

	assert.False(t, validator.HasWarnings())
	assert.False(t, validator.HasErrors())

	assert.Equal(t, 333, config.Redis.HighAvailability.Nodes[0].Port)
	assert.Equal(t, 26379, config.Redis.HighAvailability.Nodes[1].Port)
	assert.Equal(t, 26379, config.Redis.HighAvailability.Nodes[2].Port)
}

func TestShouldRaiseErrorsWhenRedisSentinelOptionsIncorrectlyConfigured(t *testing.T) {
	validator := schema.NewStructValidator()
	config := newDefaultSessionConfig()

	config.Secret = ""
	config.Redis = &schema.RedisSessionConfiguration{
		Port: 65536,
		HighAvailability: &schema.RedisHighAvailabilityConfiguration{
			SentinelName:     "sentinel",
			SentinelPassword: "abc123",
			Nodes: []schema.RedisNode{
				{
					Host: "node1",
					Port: 26379,
				},
			},
			RouteByLatency: true,
			RouteRandomly:  true,
		},
	}

	ValidateSession(&config, validator)

	errors := validator.Errors()

	assert.False(t, validator.HasWarnings())
	require.Len(t, errors, 2)

	assert.EqualError(t, errors[0], fmt.Sprintf(errFmtSessionRedisPortRange, "redis sentinel"))
	assert.EqualError(t, errors[1], fmt.Sprintf(errFmtSessionSecretRedisProvider, "redis sentinel"))

	validator.Clear()

	config.Redis.Port = -1

	ValidateSession(&config, validator)

	errors = validator.Errors()

	assert.False(t, validator.HasWarnings())
	require.Len(t, errors, 2)

	assert.EqualError(t, errors[0], fmt.Sprintf(errFmtSessionRedisPortRange, "redis sentinel"))
	assert.EqualError(t, errors[1], fmt.Sprintf(errFmtSessionSecretRedisProvider, "redis sentinel"))
}

func TestShouldNotRaiseErrorsAndSetDefaultPortWhenRedisSentinelPortBlank(t *testing.T) {
	validator := schema.NewStructValidator()
	config := newDefaultSessionConfig()

	config.Redis = &schema.RedisSessionConfiguration{
		Host: "mysentinelHost",
		Port: 0,
		HighAvailability: &schema.RedisHighAvailabilityConfiguration{
			SentinelName:     "sentinel",
			SentinelPassword: "abc123",
			Nodes: []schema.RedisNode{
				{
					Host: "node1",
					Port: 26379,
				},
			},
			RouteByLatency: true,
			RouteRandomly:  true,
		},
	}

	ValidateSession(&config, validator)

	assert.False(t, validator.HasWarnings())
	assert.False(t, validator.HasErrors())

	assert.Equal(t, 26379, config.Redis.Port)
}

func TestShouldRaiseErrorWhenRedisHostAndHighAvailabilityNodesEmpty(t *testing.T) {
	validator := schema.NewStructValidator()
	config := newDefaultSessionConfig()

	config.Redis = &schema.RedisSessionConfiguration{
		Port: 26379,
		HighAvailability: &schema.RedisHighAvailabilityConfiguration{
			SentinelName:     "sentinel",
			SentinelPassword: "abc123",
			RouteByLatency:   true,
			RouteRandomly:    true,
		},
	}

	ValidateSession(&config, validator)

	assert.False(t, validator.HasWarnings())
	require.Len(t, validator.Errors(), 1)

	assert.EqualError(t, validator.Errors()[0], fmt.Sprintf(errFmtSessionRedisHostOrNodesRequired, "redis sentinel"))
}

func TestShouldRaiseErrorsWhenRedisHostNotSet(t *testing.T) {
	validator := schema.NewStructValidator()
	config := newDefaultSessionConfig()

	config.Redis = &schema.RedisSessionConfiguration{
		Port: 6379,
	}

	ValidateSession(&config, validator)

	errors := validator.Errors()

	assert.False(t, validator.HasWarnings())
	require.Len(t, errors, 1)

	assert.EqualError(t, errors[0], fmt.Sprintf(errFmtSessionRedisHostRequired, "redis"))
}

func TestShouldRaiseErrorWhenDomainNotSet(t *testing.T) {
	validator := schema.NewStructValidator()
	config := newDefaultSessionConfig()
	config.Domain = ""

	ValidateSession(&config, validator)

	assert.False(t, validator.HasWarnings())
	assert.Len(t, validator.Errors(), 1)
	assert.EqualError(t, validator.Errors()[0], "Set domain of the session object")
}

func TestShouldRaiseErrorWhenDomainIsWildcard(t *testing.T) {
	validator := schema.NewStructValidator()
	config := newDefaultSessionConfig()
	config.Domain = "*.example.com"

	ValidateSession(&config, validator)

	assert.False(t, validator.HasWarnings())
	assert.Len(t, validator.Errors(), 1)
	assert.EqualError(t, validator.Errors()[0], "The domain of the session must be the root domain you're protecting instead of a wildcard domain")
}

func TestShouldRaiseErrorWhenSameSiteSetIncorrectly(t *testing.T) {
	validator := schema.NewStructValidator()
	config := newDefaultSessionConfig()
	config.SameSite = "NOne"

	ValidateSession(&config, validator)

	assert.False(t, validator.HasWarnings())
	assert.Len(t, validator.Errors(), 1)
	assert.EqualError(t, validator.Errors()[0], "session same_site is configured incorrectly, must be one of 'none', 'lax', or 'strict'")
}

func TestShouldNotRaiseErrorWhenSameSiteSetCorrectly(t *testing.T) {
	validator := schema.NewStructValidator()
	config := newDefaultSessionConfig()

	validOptions := []string{"none", "lax", "strict"}

	for _, opt := range validOptions {
		config.SameSite = opt

		ValidateSession(&config, validator)

		assert.False(t, validator.HasWarnings())
		assert.Len(t, validator.Errors(), 0)
	}
}

func TestShouldSetDefaultRememberMeDuration(t *testing.T) {
	validator := schema.NewStructValidator()
	config := newDefaultSessionConfig()

	ValidateSession(&config, validator)

	assert.False(t, validator.HasWarnings())
	assert.False(t, validator.HasErrors())
	assert.Equal(t, config.RememberMeDuration, schema.DefaultSessionConfiguration.RememberMeDuration)
}

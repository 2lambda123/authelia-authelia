package handlers

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/authelia/authelia/v4/internal/session"
)

func TestShouldDetectIfConsentIsMissing(t *testing.T) {
	var workflow *session.OIDCWorkflowSession

	requestedScopes := []string{"openid", "profile"}
	requestedAudience := []string{"https://authelia.com"}

	assert.True(t, isConsentMissing(workflow, requestedScopes, requestedAudience))

	workflow = &session.OIDCWorkflowSession{
		GrantedScopes:   []string{"openid", "profile"},
		GrantedAudience: []string{"https://authelia.com"},
	}

	assert.False(t, isConsentMissing(workflow, requestedScopes, requestedAudience))

	requestedScopes = []string{"openid", "profile", "group"}

	assert.True(t, isConsentMissing(workflow, requestedScopes, requestedAudience))

	requestedScopes = []string{"openid", "profile"}
	requestedAudience = []string{"https://not.authelia.com"}
	assert.True(t, isConsentMissing(workflow, requestedScopes, requestedAudience))
}

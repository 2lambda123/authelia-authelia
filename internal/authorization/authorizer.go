package authorization

import (
	"fmt"
	"net"
	"net/url"
	"strings"

	"github.com/authelia/authelia/internal/configuration/schema"
	"github.com/authelia/authelia/internal/logging"
	"github.com/authelia/authelia/internal/utils"
)

// Authorizer the component in charge of checking whether a user can access a given resource.
type Authorizer struct {
	configuration schema.AccessControlConfiguration
}

// NewAuthorizer create an instance of authorizer with a given access control configuration.
func NewAuthorizer(configuration schema.AccessControlConfiguration) *Authorizer {
	return &Authorizer{
		configuration: configuration,
	}
}

// Subject subject who to check access control for.
type Subject struct {
	Username string
	Groups   []string
	IP       net.IP
}

func (s Subject) String() string {
	return fmt.Sprintf("username=%s groups=%s ip=%s", s.Username, strings.Join(s.Groups, ","), s.IP.String())
}

// Object object to check access control for.
type Object struct {
	Scheme string
	Domain string
	Path   string
	Query  string
	Method string
}

func (o Object) String() string {
	return fmt.Sprintf("%s://%s%s", o.Scheme, o.Domain, o.Path)
}

// PolicyToLevel converts a string policy to int authorization level.
func PolicyToLevel(policy string) Level {
	switch policy {
	case "bypass":
		return Bypass
	case "one_factor":
		return OneFactor
	case "two_factor":
		return TwoFactor
	case "deny":
		return Denied
	}
	// By default the deny policy applies.
	return Denied
}

// getFirstMatchingRule returns the first rule that fully matches a given subject, url, and method.
func getFirstMatchingRule(rules []schema.ACLRule, networks []schema.ACLNetwork, subject Subject, object Object) (rule schema.ACLRule, err error) {
	for _, rule := range rules {
		if !isDomainMatching(object.Domain, rule.Domains) {
			continue
		}

		if !isPathMatching(object.Path + object.Query, rule.Resources) {
			continue
		}

		if len(rule.Methods) > 0 {
			if object.Method == "" || !utils.IsStringInSlice(object.Method, rule.Methods) {
				continue
			}
		}

		if len(rule.Networks) > 0 && !isIPMatching(subject.IP, rule.Networks, networks) {
			continue
		}

		if len(rule.Subjects) > 0 {
			for _, subjectRule := range rule.Subjects {
				if !isSubjectMatching(subject, subjectRule) {
					continue
				}
			}
		}

		return rule, nil
	}

	return rule, errNoMatchingRule
}

// IsSecondFactorEnabled return true if at least one policy is set to second factor.
func (p *Authorizer) IsSecondFactorEnabled() bool {
	if PolicyToLevel(p.configuration.DefaultPolicy) == TwoFactor {
		return true
	}

	for _, r := range p.configuration.Rules {
		if PolicyToLevel(r.Policy) == TwoFactor {
			return true
		}
	}

	return false
}

// GetRequiredLevel retrieve the required level of authorization to access the object.
func (p *Authorizer) GetRequiredLevel(subject Subject, object Object) Level {
	logger := logging.Logger()
	logger.Tracef("Check authorization of subject %s and url %s.", subject.String(), object.String())

	rule, err := getFirstMatchingRule(p.configuration.Rules, p.configuration.Networks, subject, object)

	if err != nil {
		if err == errNoMatchingRule {
			logger.Tracef("No matching rule for subject %s and url %s... Applying default policy.", subject.String(), object.String())
		} else {
			logger.Warnf("Error occurred matching ACL Rules for subject %s and url %s: %v", subject.String(), object.String(), err)
		}

		return PolicyToLevel(p.configuration.DefaultPolicy)
	}

	return PolicyToLevel(rule.Policy)
}

// IsURLMatchingRuleWithGroupSubjects returns true if the request has at least one
// matching ACL with a subject of type group attached to it, otherwise false.
func (p *Authorizer) IsURLMatchingRuleWithGroupSubjects(requestURL url.URL) (hasGroupSubjects bool) {
	for _, rule := range p.configuration.Rules {
		if isDomainMatching(requestURL.Hostname(), rule.Domains) && isPathMatching(requestURL.Path, rule.Resources) {
			for _, subjectRule := range rule.Subjects {
				for _, subject := range subjectRule {
					if strings.HasPrefix(subject, groupPrefix) {
						return true
					}
				}
			}
		}
	}

	return false
}

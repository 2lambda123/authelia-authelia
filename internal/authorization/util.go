package authorization

import (
	"net"
	"regexp"
	"strings"

	"github.com/authelia/authelia/internal/configuration/schema"
)

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

func schemaSubjectToACLSubject(subjectRule string) (subject AccessControlSubject) {
	if strings.HasPrefix(subjectRule, userPrefix) {
		user := strings.Trim(subjectRule[len(userPrefix):], " ")

		return AccessControlUser{Name: user}
	}

	if strings.HasPrefix(subjectRule, groupPrefix) {
		group := strings.Trim(subjectRule[len(groupPrefix):], " ")

		return AccessControlGroup{Name: group}
	}

	return nil
}

func schemaDomainsToACL(domainRules []string) (domains []AccessControlDomain) {
	for _, domainRule := range domainRules {
		domain := AccessControlDomain{}

		domainRule = strings.ToLower(domainRule)

		switch {
		case strings.HasPrefix(domainRule, "*."):
			domain.Wildcard = true
			domain.Name = domainRule[1:]
		case strings.HasPrefix(domainRule, "{user}"):
			domain.UserWildcard = true
			domain.Name = domainRule[7:]
		case strings.HasPrefix(domainRule, "{group}"):
			domain.GroupWildcard = true
			domain.Name = domainRule[8:]
		default:
			domain.Name = domainRule
		}

		domains = append(domains, domain)
	}

	return domains
}

func schemaResourcesToACL(resourceRules []string) (resources []AccessControlResource) {
	for _, resourceRule := range resourceRules {
		resources = append(resources, AccessControlResource{regexp.MustCompile(resourceRule)})
	}

	return resources
}

func schemaMethodsToACL(methodRules []string) (methods []string) {
	for _, method := range methodRules {
		methods = append(methods, strings.ToUpper(method))
	}

	return methods
}

func schemaNetworksToACL(networkRules []string, networksMap map[string][]*net.IPNet, networksCacheMap map[string]*net.IPNet) (networks []*net.IPNet) {
	for _, network := range networkRules {
		if _, ok := networksMap[network]; !ok {
			if _, ok := networksCacheMap[network]; ok {
				networks = append(networks, networksCacheMap[network])
			} else {
				cidr, err := parseNetwork(network)
				if err == nil {
					networks = append(networks, cidr)
					networksCacheMap[cidr.String()] = cidr

					if cidr.String() != network {
						networksCacheMap[network] = cidr
					}
				}
			}
		} else {
			networks = append(networks, networksMap[network]...)
		}
	}

	return networks
}

func parseSchemaNetworks(schemaNetworks []schema.ACLNetwork) (networksMap map[string][]*net.IPNet, networksCacheMap map[string]*net.IPNet) {
	// These maps store pointers to the net.IPNet values so we can reuse them efficiently.
	// The networksMap contains the named networks as keys, the networksCacheMap contains the CIDR notations as keys.
	networksMap = map[string][]*net.IPNet{}
	networksCacheMap = map[string]*net.IPNet{}

	for _, aclNetwork := range schemaNetworks {
		var networks []*net.IPNet

		for _, networkRule := range aclNetwork.Networks {
			cidr, err := parseNetwork(networkRule)
			if err == nil {
				networks = append(networks, cidr)
				networksCacheMap[cidr.String()] = cidr

				if cidr.String() != networkRule {
					networksCacheMap[networkRule] = cidr
				}
			}
		}

		if _, ok := networksMap[aclNetwork.Name]; len(networks) != 0 && !ok {
			networksMap[aclNetwork.Name] = networks
		}
	}

	return networksMap, networksCacheMap
}

func parseNetwork(networkRule string) (cidr *net.IPNet, err error) {
	if !strings.Contains(networkRule, "/") {
		ip := net.ParseIP(networkRule)
		if ip.To4() != nil {
			_, cidr, err = net.ParseCIDR(networkRule + "/32")
		} else {
			_, cidr, err = net.ParseCIDR(networkRule + "/128")
		}
	} else {
		_, cidr, err = net.ParseCIDR(networkRule)
	}

	return cidr, err
}

func schemaSubjectsToACL(subjectRules [][]string) (subjects []AccessControlSubjects) {
	for _, subjectRule := range subjectRules {
		subject := AccessControlSubjects{}

		for _, subjectRuleItem := range subjectRule {
			subject.AddSubject(subjectRuleItem)
		}

		if len(subject.Subjects) != 0 {
			subjects = append(subjects, subject)
		}
	}

	return subjects
}

func domainToPrefixSuffix(domain string) (prefix, suffix string) {
	parts := strings.Split(domain, ".")

	if len(parts) == 1 {
		return "", parts[0]
	}

	return parts[0], strings.Join(parts[1:], ".")
}

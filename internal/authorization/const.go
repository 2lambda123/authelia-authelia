package authorization

// Level is the type representing an authorization level.
type Level int

const (
	// Bypass bypass level.
	Bypass Level = iota
	// OneFactor one factor level.
	OneFactor Level = iota
	// TwoFactor two factor level.
	TwoFactor Level = iota
	// Denied denied level.
	Denied Level = iota
)

const (
	prefixUser  = "user:"
	prefixGroup = "group:"
)

const (
	bypass    = "bypass"
	oneFactor = "one_factor"
	twoFactor = "two_factor"
	deny      = "deny"
)

const (
	subexpNameUser  = "User"
	subexpNameGroup = "Group"
)

var (
	// IdentitySubexpNames is a list of valid regex subexp names.
	IdentitySubexpNames = []string{subexpNameUser, subexpNameGroup}
)

const traceFmtACLHitMiss = "ACL %s Position %d for subject %s and object %s (Method %s)"

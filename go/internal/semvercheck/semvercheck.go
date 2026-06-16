package semvercheck

import "regexp"

// semverRE matches X.Y.Z with numeric components only.
var semverRE = regexp.MustCompile(`^[0-9]+\.[0-9]+\.[0-9]+$`)

// IsSemver reports whether s is a valid semantic version of the form X.Y.Z
// with numeric components only (no leading "v", no pre-release suffix).
func IsSemver(s string) bool { return semverRE.MatchString(s) }

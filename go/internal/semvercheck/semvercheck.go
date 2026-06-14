package semvercheck

import "regexp"

// IsSemver reports whether s matches X.Y.Z with numeric components only.
var semverRE = regexp.MustCompile(`^[0-9]+\.[0-9]+\.[0-9]+$`)

func IsSemver(s string) bool { return semverRE.MatchString(s) }

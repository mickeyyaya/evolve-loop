package semvercheck

import "regexp"

var semverRE = regexp.MustCompile(`^[0-9]+\.[0-9]+\.[0-9]+$`)

func IsSemver(s string) bool { return semverRE.MatchString(s) }

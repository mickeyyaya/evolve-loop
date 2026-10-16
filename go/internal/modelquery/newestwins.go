package modelquery

import (
	"regexp"
	"strconv"
	"strings"
)

var versionToken = regexp.MustCompile(`[0-9]+(?:\.[0-9]+)*`)

func NewestInLineage(ids []string) string {
	if len(ids) == 0 {
		return ""
	}
	best := ids[0]
	for _, id := range ids[1:] {
		if newer(id, best) {
			best = id
		}
	}
	return best
}

func newer(a, b string) bool {
	va, vb := parseVersion(withoutDate(a)), parseVersion(withoutDate(b))
	if newerVersion(va, vb) {
		return true
	}
	if newerVersion(vb, va) {
		return false
	}
	da, db := parseDate(a), parseDate(b)
	return da.ok && db.ok && da.value > db.value
}

type version struct {
	ok    bool
	parts []int
}

func parseVersion(id string) version {
	tok := versionToken.FindString(id)
	if tok == "" {
		return version{}
	}
	fields := strings.Split(tok, ".")
	parts := make([]int, 0, len(fields))
	for _, f := range fields {
		n, err := strconv.Atoi(f)
		if err != nil {
			return version{}
		}
		parts = append(parts, n)
	}
	return version{ok: true, parts: parts}
}

func newerVersion(a, b version) bool {
	if a.ok != b.ok {
		return a.ok
	}
	if !a.ok {
		return false
	}
	return compareParts(a.parts, b.parts) > 0
}

type dateVersion struct {
	ok    bool
	value int
}

var parseDateAtoi = strconv.Atoi

func parseDate(id string) dateVersion {
	tok := dateRun.FindString(id)
	if tok == "" {
		return dateVersion{}
	}
	fields := strings.Split(tok, "-")
	year, err := parseDateAtoi(fields[0])
	if err != nil {
		return dateVersion{}
	}
	month, day := 0, 0
	if len(fields) > 1 {
		if month, err = parseDateAtoi(fields[1]); err != nil {
			return dateVersion{}
		}
	}
	if len(fields) > 2 {
		if day, err = parseDateAtoi(fields[2]); err != nil {
			return dateVersion{}
		}
	}
	return dateVersion{ok: true, value: year*10000 + month*100 + day}
}

func compareParts(a, b []int) int {
	n := len(a)
	if len(b) > n {
		n = len(b)
	}
	for i := 0; i < n; i++ {
		var ai, bi int
		if i < len(a) {
			ai = a[i]
		}
		if i < len(b) {
			bi = b[i]
		}
		if ai != bi {
			if ai > bi {
				return 1
			}
			return -1
		}
	}
	return 0
}

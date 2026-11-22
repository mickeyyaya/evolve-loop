package stageevidence

import (
	"strconv"
	"strings"
)

type ScopeKind string

const (
	ScopeCycle   ScopeKind = "cycle"
	ScopePool    ScopeKind = "pool"
	ScopeFleet   ScopeKind = "fleet"
	ScopeCurator ScopeKind = "curator"
)

type ScopeKey struct {
	Kind ScopeKind
	ID   int
}

func CycleScope(id int) ScopeKey {
	return ScopeKey{Kind: ScopeCycle, ID: id}
}

func (k ScopeKey) String() string {
	return string(k.Kind) + "-" + strconv.Itoa(k.ID)
}

func normalizeScope(kind ScopeKind, id int) ScopeKey {
	if kind == "" {
		kind = ScopeCycle
	}
	return ScopeKey{Kind: kind, ID: id}
}

func (r Record) ScopeKey() ScopeKey {
	return normalizeScope(r.Scope, r.ID)
}

func (o Outcome) ScopeKey() ScopeKey {
	return normalizeScope(o.Scope, o.ID)
}

func ParseScope(label string) (ScopeKey, bool) {
	kind, id, ok := strings.Cut(label, "-")
	n, err := strconv.Atoi(id)
	if !ok || err != nil || n < 0 || !knownScopeKind(ScopeKind(kind)) {
		return ScopeKey{}, false
	}
	return ScopeKey{Kind: ScopeKind(kind), ID: n}, true
}

func knownScopeKind(k ScopeKind) bool {
	return k == ScopeCycle || k == ScopePool || k == ScopeFleet || k == ScopeCurator
}

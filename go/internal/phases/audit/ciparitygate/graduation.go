package ciparitygate

import (
	"fmt"

	"github.com/mickeyyaya/evolve-loop/go/internal/ciparity"
)

// ApicoverGraduation flags changed go/internal/<pkg> packages that are NEW
// this cycle and absent from .apicover-enforce — the blind spot
// ApicoverEnforce's IntersectEnforced silently drops (a package new this
// cycle cannot yet be in the enforce list, so the touched∩enforced scoping
// never inspects it). This is the deterministic, fail-fast half of the
// recurring warnship_apicover_ci_gap: each ungraduated package must gain an
// .apicover-enforce entry + an apicover_named_test.go before audit can PASS.
// Shares ApicoverEnforce's prologue (inputs); a no-op (nil, nil) when there is
// no module, no enforce list, or nothing ungraduated. go/cmd/... changes are
// never flagged (out of apicover's scope). A test-only package is deferred,
// not flagged (GRADUATION_DEFERRED — the stderr line it replaces, D2).
func (g *Gates) ApicoverGraduation(req Request) ([]string, error) {
	in, offenders, done := g.inputs(gateGraduation, req, underivableGraduation)
	if done {
		return offenders, nil
	}
	ungraduated := ciparity.NewUngraduatedPackages(in.changed, in.enforce)
	if len(ungraduated) == 0 {
		return nil, nil
	}
	offenders, deferred := graduationOffenders(in.dir, ungraduated)
	for _, pkg := range deferred {
		// Never silently skip: the deferred obligation must be visible in the
		// stream — enrollment re-raises when the production half lands (this
		// seam has no new-this-cycle filter, so ANY later change re-flags it).
		g.warn(gateGraduation, req, CodeGraduationDeferred, "graduation deferred: "+pkg+" has no production .go surface (test-only/absent) — enrollment obligation re-raises when production code lands", "pkg", pkg, "dir", in.dir)
	}
	if len(offenders) > 0 {
		return g.failed(gateGraduation, req, causeUngraduated, offenders), nil
	}
	return offenders, nil // empty but non-nil when everything is deferred (Q13): the host keys off len, gates.go applyCIGate
}

// graduationOffenders splits the ungraduated packages into offenders (a
// prescriptive fix each) and deferred test-only/absent packages — the same
// predicate as the build-entry seam: a test-only package has no production
// surface for the gate to inspect, and flagging it here after the build seam
// stopped doing so would just move the vacuous FAIL one phase later. The
// offender list is empty-but-non-nil when everything is deferred, as before.
func graduationOffenders(dir string, ungraduated []string) (offenders, deferred []string) {
	offenders = make([]string, 0, len(ungraduated))
	for _, pkg := range ungraduated {
		if !ciparity.PackageDirHasProductionGoFiles(dir, pkg) {
			deferred = append(deferred, pkg)
			continue
		}
		offenders = append(offenders, fmt.Sprintf("%s: new package absent from go/.apicover-enforce — the repo-wide apicover unnamed-export gate never inspects it. Make EXACTLY these edits:\n%s", pkg, ciparity.GraduationPrescription([]string{pkg})))
	}
	return offenders, deferred
}

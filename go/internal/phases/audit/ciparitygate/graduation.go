package ciparitygate

import (
	"fmt"

	"github.com/mickeyyaya/evolve-loop/go/internal/ciparity"
)

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
		g.warn(gateGraduation, req, CodeGraduationDeferred, "graduation deferred: "+pkg+" has no production .go surface (test-only/absent) — enrollment obligation re-raises when production code lands", "pkg", pkg, "dir", in.dir)
	}
	if len(offenders) > 0 {
		return g.failed(gateGraduation, req, causeUngraduated, offenders), nil
	}
	return offenders, nil
}

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

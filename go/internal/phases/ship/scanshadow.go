package ship

import (
	"context"
	"fmt"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/core"
	"github.com/mickeyyaya/evolve-loop/go/internal/overlap"
	"github.com/mickeyyaya/evolve-loop/go/internal/policy"
	"github.com/mickeyyaya/evolve-loop/go/internal/repocontract"
	"github.com/mickeyyaya/evolve-loop/go/internal/signalcenter"
	"github.com/mickeyyaya/evolve-loop/go/internal/sysexec"
	"github.com/mickeyyaya/evolve-loop/go/internal/testkey"
	"github.com/mickeyyaya/evolve-loop/go/internal/testverdict"
)

const (
	CodeScanShadowEscape signalcenter.Code = "SHIP_SCAN_SHADOW_ESCAPE"
	CodeScanShadowFault  signalcenter.Code = "SHIP_SCAN_SHADOW_FAULT"

	shipGateID   = "ship"
	floorGateID  = "build-floor"
	stageEnforce = "enforce"
	stageOff     = "off"
)

func init() {
	signalcenter.RegisterCode(signalcenter.ModuleShip, CodeScanShadowFault, "the scan shadow panicked and recovered; the ship scan and its verdict were not affected, and the shadow records for this scan are incomplete")
	signalcenter.RegisterCode(signalcenter.ModuleShip, CodeScanShadowEscape, "the scan shadow found a stored green result for the content key of a package, and that package then failed in the real run; the content key misses an input of the test, and the scan cache cannot be enforced until the miss is closed")
}

type scanShadow struct {
	store   *testverdict.Store
	run     sysexec.RunFunc
	signals *signalcenter.Center
	cycle   int
	limits  policy.ScanCacheConfig

	gate       string
	enforce    bool
	recordOnly bool

	root      string
	treeID    string
	goVersion string
	buildEnv  map[string]string
	degraded  string
	faulted   bool
	dropped   int

	settled     map[string]bool
	hits        []string
	packages    int
	escapes     int
	recorded    int
	unkeyed     int
	storeErrors int
	skipped     int
	uncacheable int
}

func newScanShadow(store *testverdict.Store, run sysexec.RunFunc, signals *signalcenter.Center, cycle int, limits policy.ScanCacheConfig) *scanShadow {
	return &scanShadow{store: store, run: run, signals: signals, cycle: cycle, limits: limits, gate: shipGateID, enforce: limits.Gate == stageEnforce, settled: map[string]bool{}}
}

func newFloorRecorder(store *testverdict.Store, run sysexec.RunFunc, limits policy.ScanCacheConfig) *scanShadow {
	return &scanShadow{store: store, run: run, limits: limits, gate: floorGateID, recordOnly: true, settled: map[string]bool{}}
}

func verdictStore(projectRoot string, now func() time.Time) *testverdict.Store {
	return testverdict.NewStore(testverdict.DirFor(projectRoot), now)
}

func floorRecorderFor(projectRoot string) *scanShadow {
	if projectRoot == "" {
		return nil
	}
	cfg := policy.ScanCacheFor(projectRoot)
	if cfg.Gate == stageOff {
		return nil
	}
	return newFloorRecorder(verdictStore(projectRoot, time.Now), sysexec.DefaultRunner, cfg)
}

type scanShadowKey struct{}

func withScanShadow(ctx context.Context, s *scanShadow) context.Context {
	return context.WithValue(ctx, scanShadowKey{}, s)
}

func scanShadowFrom(ctx context.Context) *scanShadow {
	s, _ := ctx.Value(scanShadowKey{}).(*scanShadow)
	return s
}

func (p *Phase) scanShadowFor(req core.PhaseRequest) *scanShadow {
	if req.ProjectRoot == "" {
		return nil
	}
	cfg := policy.ScanCacheFor(req.ProjectRoot)
	if cfg.Gate == stageOff {
		return nil
	}
	now := p.nowFn
	if now == nil {
		now = time.Now
	}
	return newScanShadow(verdictStore(req.ProjectRoot, now), sysexec.DefaultRunner, p.signals, req.Cycle, cfg)
}

func (s *scanShadow) begin(ctx context.Context, root string) {
	if s == nil {
		return
	}
	defer s.guard("begin")
	ctx, cancel := context.WithTimeout(ctx, s.limits.Budget)
	defer cancel()
	s.root, s.treeID, s.goVersion, s.buildEnv = root, "", "", nil
	id, err := treeID(ctx, s.run, root, s.limits.MaxBytes)
	if err != nil {
		s.degrade(err.Error())
		return
	}
	s.treeID = id
	if s.goVersion, s.buildEnv, err = goEnv(ctx, s.run, repocontract.ModuleDir(root)); err != nil {
		s.degrade(err.Error())
	}
}

func (s *scanShadow) degrade(cause string) {
	if s.degraded != "" {
		cause = s.degraded + "; " + cause
	}
	s.degraded = cause
}

func (s *scanShadow) guard(stage string) {
	v := recover()
	if v == nil {
		return
	}
	s.degrade(fmt.Sprintf("panic in %s: %v", stage, v))
	if s.faulted {
		return
	}
	s.faulted = true
	s.signals.Emit(signalcenter.Event{
		Cycle: s.cycle, Phase: phaseName,
		Module: signalcenter.ModuleShip, Origin: "scanShadow.guard", Kind: signalcenter.KindShipWarning,
		Severity: signalcenter.SeverityWarn, Code: CodeScanShadowFault,
		Reason: fmt.Sprintf("the scan shadow panicked in %s and recovered; the scan is unaffected: %v", stage, v),
		Fields: map[string]string{"stage": stage},
	})
}

func (s *scanShadow) observe(o packOutcome) {
	if s == nil {
		return
	}
	defer s.guard("observe")
	s.dropped += o.droppedLines
	for _, r := range o.pkgs {
		s.settle(r)
	}
}

func (s *scanShadow) settle(r pkgResult) {
	defer s.guard("settle")
	s.packages++
	if !cacheable(r) {
		s.uncacheable++
		return
	}
	key, err := s.keyOf(r)
	if err != nil {
		s.unkeyed++
		return
	}
	if s.settled[key.Full] {
		return
	}
	s.settled[key.Full] = true
	if s.recordOnly {
		s.recordGreen(r, key)
		return
	}
	stored, hit := s.store.Lookup(key.File, nil)
	switch {
	case hit && r.Passed:
		s.hits = append(s.hits, r.Package)
	case hit:
		s.escape(r, key, stored.Gate)
	case r.Passed:
		s.record(r, key)
	}
}

func (s *scanShadow) recordGreen(r pkgResult, key testkey.Key) {
	if r.Passed {
		s.record(r, key)
	}
}

func cacheable(r pkgResult) bool {
	return len(r.Tags) == 0 && !slices.ContainsFunc(repoContractSelections, func(sel repocontract.TestSelection) bool {
		return strings.HasSuffix(r.Package, strings.TrimPrefix(sel.Package, "."))
	})
}

func (s *scanShadow) keyOf(r pkgResult) (testkey.Key, error) {
	return testkey.Compute(testkey.Input{
		Package:     r.Package,
		Module:      overlap.Module{Packages: []overlap.Package{{ImportPath: r.Package}}},
		Toolchain:   testkey.Toolchain{GoVersion: s.goVersion, Tags: r.Tags, Flags: r.Flags, BuildEnv: s.buildEnv},
		WholeTreeID: s.treeID,
		Log:         testkey.Log{Unknown: true},
		RepoRoot:    s.root,
		WorkDir:     s.root,
	})
}

func (s *scanShadow) record(r pkgResult, key testkey.Key) {
	rec := testverdict.Record{Package: r.Package, Outcome: testverdict.Pass, Key: key, Log: testkey.Log{Unknown: true}, Gate: s.gate}
	if err := s.store.Put(rec); err != nil {
		s.storeErrors++
		if s.storeErrors == 1 {
			s.degrade("store: " + err.Error())
		}
		return
	}
	s.recorded++
}

func (s *scanShadow) escape(r pkgResult, key testkey.Key, gate string) {
	s.escapes++
	s.signals.Emit(signalcenter.Event{
		Cycle: s.cycle, Phase: phaseName,
		Module: signalcenter.ModuleShip, Origin: "scanShadow.escape", Kind: signalcenter.KindShipWarning,
		Severity: signalcenter.SeverityWarn, Code: CodeScanShadowEscape,
		Reason: "a stored green result matched the content key of " + r.Package + ", and the package failed in the real run",
		Fields: map[string]string{
			"package": r.Package,
			"key":     key.File,
			"tags":    strings.Join(r.Tags, ","),
			"flags":   strings.Join(r.Flags, " "),
			"gate":    gate,
		},
	})
}

func (s *scanShadow) finish() {
	if s == nil {
		return
	}
	defer s.guard("finish")
	fields := map[string]string{
		"packages":      strconv.Itoa(s.packages),
		"hits":          strconv.Itoa(len(s.hits)),
		"escapes":       strconv.Itoa(s.escapes),
		"recorded":      strconv.Itoa(s.recorded),
		"unkeyed":       strconv.Itoa(s.unkeyed),
		"store_errors":  strconv.Itoa(s.storeErrors),
		"dropped_lines": strconv.Itoa(s.dropped),
		"hit_packages":  strings.Join(sortedCopy(s.hits), ","),
		"stage":         s.stage(),
		"skipped":       strconv.Itoa(s.skipped),
		"uncacheable":   strconv.Itoa(s.uncacheable),
	}
	if s.degraded != "" {
		fields["degraded"] = s.degraded
	}
	s.signals.Emit(signalcenter.Event{
		Cycle: s.cycle, Phase: phaseName,
		Module: signalcenter.ModuleShip, Origin: "scanShadow.finish", Kind: signalcenter.KindShipWarning,
		Severity: signalcenter.SeverityInfo,
		Reason:   fmt.Sprintf("scan shadow: %d package result(s), %d could have been skipped, %d escape(s)", s.packages, len(s.hits), s.escapes),
		Fields:   fields,
	})
}

func sortedCopy(xs []string) []string {
	out := slices.Clone(xs)
	slices.Sort(out)
	return out
}

func (s *scanShadow) stage() string {
	if s.enforce {
		return stageEnforce
	}
	return "shadow"
}

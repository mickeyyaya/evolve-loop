package core

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/dispositionrouter"
	"github.com/mickeyyaya/evolve-loop/go/internal/explanationdocs"
	"github.com/mickeyyaya/evolve-loop/go/internal/inboxbatch"
	"github.com/mickeyyaya/evolve-loop/go/internal/policy"
)

const classSegment = "|class-"

var classCountedLayers = map[string]bool{dispositionrouter.LayerPipelineCode: true, dispositionrouter.LayerHarness: true, dispositionrouter.LayerInfra: true}

var floorFailureCount = regexp.MustCompile(regexp.QuoteMeta(floorReasonHead) + `(\d+)` + regexp.QuoteMeta(floorReasonCount))

func failingChecks(reasons []string) []string {
	set := map[string]bool{}
	for _, r := range reasons {
		m := floorFailureCount.FindStringSubmatch(r)
		if m == nil {
			continue
		}
		total, _ := strconv.Atoi(m[1])
		explained := strings.Count(r, floorFailureSeparator+explanationdocs.FailurePrefix)
		set[explanationFloorCheckName] = set[explanationFloorCheckName] || explained > 0
		set[composedFloorCheckName] = set[composedFloorCheckName] || total > explained
	}
	var out []string
	for name, failed := range set {
		if failed {
			out = append(out, name)
		}
	}
	sort.Strings(out)
	return out
}

type failureClassKey struct {
	phase, preClass, layer string
	checks                 []string
}

func (k failureClassKey) fingerprint() string {
	sum := sha256.Sum256([]byte(k.phase + "\x00" + k.preClass + "\x00" + strings.Join(k.checks, ",") + "\x00" + k.layer))
	return k.phase + "|" + k.preClass + classSegment + hex.EncodeToString(sum[:])[:12]
}

func recordFailureClass(workspace string, now func() time.Time) (FailureDigest, error) {
	record, ok := failureDigestRecordPath(workspace)
	if !ok {
		return FailureDigest{}, nil
	}
	var disp disposition
	if err := readJSONFile(filepath.Join(workspace, "disposition.json"), &disp); err != nil {
		return FailureDigest{}, fmt.Errorf("read the disposition: %w", err)
	}
	var d FailureDigest
	if err := readJSONFile(record, &d); err != nil {
		return d, fmt.Errorf("read the harness digest record: %w", err)
	}
	if !classCountedLayers[disp.RootCause.Layer] {
		return d, nil
	}
	phase, _, _ := strings.Cut(d.Fingerprint, "|")
	d.ClassFingerprint, d.ClassLayer = failureClassKey{phase: phase, preClass: d.PreClass, layer: disp.RootCause.Layer, checks: d.Checks}.fingerprint(), disp.RootCause.Layer
	prior, err := priorClassCount(record, d.ClassFingerprint)
	if err != nil {
		return d, err
	}
	d.ClassRecurrence, d.ClassRecordedAt = prior+1, now().UTC().Format(time.RFC3339Nano)
	dec := dispositionrouter.Decide(d.PreClass, d.ClassRecurrence, disp.Routing)
	d.ClassRoute, d.ClassRouteReason = dec.Route, dec.Reason
	if err := persistFailureDigest(workspace, d); err != nil {
		return d, err
	}
	return d, stageClassEscalation(filepath.Dir(filepath.Dir(record)), d)
}

func classItemID(class string) string {
	_, hash, _ := strings.Cut(class, classSegment)
	return "failure-class-" + hash
}

func stageClassEscalation(evolveDir string, d FailureDigest) error {
	if d.ClassRoute != dispositionrouter.RouteConsole {
		return nil
	}
	pol, perr := policy.Load(filepath.Join(evolveDir, "policy.json"))
	if perr != nil {
		perr = fmt.Errorf("load the policy for the autofile weight: %w", perr)
	}
	_, serr := dispositionrouter.StageIntent(filepath.Join(evolveDir, "escalations"), dispositionrouter.Intent{
		Action: dispositionrouter.ActionAutofile, Route: dispositionrouter.RouteConsole,
		ItemID:  classItemID(d.ClassFingerprint),
		Pattern: d.ClassFingerprint, Cycle: d.Cycle, Recurrence: d.ClassRecurrence, Weight: pol.RetroAutofileDefaultWeight(),
		Reason: d.ClassRouteReason, PriorityClass: inboxbatch.ClassCorrectness,
	})
	return errors.Join(perr, serr)
}

func withRecordedClass(workspace string, d FailureDigest) FailureDigest {
	record, ok := failureDigestRecordPath(workspace)
	var prior FailureDigest
	if !ok || readJSONFile(record, &prior) != nil || prior.Fingerprint != d.Fingerprint {
		return d
	}
	d.ClassFingerprint, d.ClassLayer, d.ClassRecurrence, d.ClassRecordedAt = prior.ClassFingerprint, prior.ClassLayer, prior.ClassRecurrence, prior.ClassRecordedAt
	d.ClassRoute, d.ClassRouteReason = prior.ClassRoute, prior.ClassRouteReason
	return d
}

func priorClassCount(record, class string) (int, error) {
	digestsDir := filepath.Dir(record)
	resolved, err := loadResolvedFingerprintTimes(filepath.Dir(digestsDir))
	if err != nil {
		return 0, err
	}
	entries, err := os.ReadDir(digestsDir)
	if err != nil {
		return 0, fmt.Errorf("list the digest records: %w", err)
	}
	count := 0
	for _, e := range entries {
		path := filepath.Join(digestsDir, e.Name())
		if path == record || e.IsDir() || filepath.Ext(e.Name()) != ".json" {
			continue
		}
		var prior FailureDigest
		if err := readJSONFile(path, &prior); err != nil {
			return 0, fmt.Errorf("read the digest record %s: %w", e.Name(), err)
		}
		if prior.ClassFingerprint == class && classRecordedAfter(prior, resolved[class]) {
			count++
		}
	}
	return count, nil
}

func classRecordedAfter(d FailureDigest, resolvedAt time.Time) bool {
	at, err := time.Parse(time.RFC3339Nano, d.ClassRecordedAt)
	return err == nil && at.After(resolvedAt)
}

func readJSONFile(path string, into any) error {
	raw, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	return json.Unmarshal(raw, into)
}

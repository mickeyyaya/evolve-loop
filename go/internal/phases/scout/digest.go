package scout

import (
	"sort"
	"strings"
)

// digest.go — scout map-reduce REDUCE-step inputs (pure). Each MAP worker emits a
// ScanDigest for its slice; mergeDigests folds them into MergedFindings the scout
// synthesizer consumes in place of scanning the codebase itself.

// ScanDigest is one MAP worker's structured findings for its slice.
type ScanDigest struct {
	SliceID           string          `json:"slice_id"`
	Findings          []Finding       `json:"findings"`
	CandidateTasks    []CandidateTask `json:"candidate_tasks"`
	CrossSliceSignals []CrossSignal   `json:"cross_slice_signals,omitempty"`
}

// Finding is one gap/hotspot/debt observation within a slice.
type Finding struct {
	File     string `json:"file"`
	Kind     string `json:"kind"`     // gap | hotspot | debt
	Severity string `json:"severity"` // LOW | MEDIUM | HIGH
	Note     string `json:"note"`
}

// CandidateTask is a slice-local task proposal the synthesizer ranks + selects.
type CandidateTask struct {
	Slug       string   `json:"slug"`
	Type       string   `json:"type"`
	Complexity string   `json:"complexity"` // S | M | L
	Files      []string `json:"files"`
	Confidence float64  `json:"confidence"`
	// Occurrences is set by mergeDigests = how many slices proposed an
	// equivalent task (its dedup boost). 1 for a single-slice task.
	Occurrences int `json:"occurrences,omitempty"`
}

// CrossSignal is a worker's flag that a pattern may span slices.
type CrossSignal struct {
	Pattern string   `json:"pattern"`
	Slices  []string `json:"slices,omitempty"`
}

// MergedFindings is the reduce-ready view: deduped+ranked tasks, all findings,
// and the confirmed cross-cutting signals (a pattern flagged by >=2 slices).
type MergedFindings struct {
	Tasks        []CandidateTask `json:"tasks"`
	Findings     []Finding       `json:"findings"`
	CrossCutting []CrossSignal   `json:"cross_cutting,omitempty"`
}

// mergeDigests folds per-slice digests deterministically: candidate tasks are
// deduped by fingerprint (slug + sorted files), keeping the highest confidence
// and counting Occurrences across slices; tasks are ranked by Occurrences then
// confidence (cross-slice agreement wins). A CrossSignal pattern flagged by >=2
// slices becomes a confirmed cross-cutting item (the synthesizer prioritizes it).
// Pure.
func mergeDigests(digests []ScanDigest) MergedFindings {
	taskByFP := map[string]*CandidateTask{}
	var order []string
	var findings []Finding
	patternSlices := map[string]map[string]bool{}

	for _, d := range digests {
		findings = append(findings, d.Findings...)
		for _, t := range d.CandidateTasks {
			fp := taskFingerprint(t)
			if cur, ok := taskByFP[fp]; ok {
				cur.Occurrences++
				if t.Confidence > cur.Confidence {
					cur.Confidence = t.Confidence
				}
				continue
			}
			cp := t
			cp.Occurrences = 1
			taskByFP[fp] = &cp
			order = append(order, fp)
		}
		for _, s := range d.CrossSliceSignals {
			if patternSlices[s.Pattern] == nil {
				patternSlices[s.Pattern] = map[string]bool{}
			}
			patternSlices[s.Pattern][d.SliceID] = true
		}
	}

	tasks := make([]CandidateTask, 0, len(order))
	for _, fp := range order {
		tasks = append(tasks, *taskByFP[fp])
	}
	sort.SliceStable(tasks, func(i, j int) bool {
		if tasks[i].Occurrences != tasks[j].Occurrences {
			return tasks[i].Occurrences > tasks[j].Occurrences
		}
		return tasks[i].Confidence > tasks[j].Confidence
	})

	var cross []CrossSignal
	for pat, slices := range patternSlices {
		if len(slices) < 2 {
			continue
		}
		ids := make([]string, 0, len(slices))
		for id := range slices {
			ids = append(ids, id)
		}
		sort.Strings(ids)
		cross = append(cross, CrossSignal{Pattern: pat, Slices: ids})
	}
	sort.SliceStable(cross, func(i, j int) bool { return cross[i].Pattern < cross[j].Pattern })

	return MergedFindings{Tasks: tasks, Findings: findings, CrossCutting: cross}
}

func taskFingerprint(t CandidateTask) string {
	f := append([]string(nil), t.Files...)
	sort.Strings(f)
	return t.Slug + "\x00" + strings.Join(f, ",")
}

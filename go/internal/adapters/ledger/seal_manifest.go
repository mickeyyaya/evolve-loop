package ledger

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"path/filepath"
	"sort"

	"github.com/mickeyyaya/evolve-loop/go/internal/atomicwrite"
	"github.com/mickeyyaya/evolve-loop/go/internal/core"
)

const (
	manifestFileName = "manifest.jsonl"
	summaryVersion   = 1
	undecodableKind  = "_undecodable"
)

var errManifestMismatch = errors.New("ledger segment manifest does not match the hash chain")

var manifestWrite = atomicwrite.Durable

type segmentSummary struct {
	Version        int            `json:"version"`
	Lines          int            `json:"lines"`
	SeqFirst       int            `json:"seq_first"`
	SeqLast        int            `json:"seq_last"`
	TSFirst        string         `json:"ts_first"`
	TSLast         string         `json:"ts_last"`
	Cycles         []int          `json:"cycles"`
	KindCounts     map[string]int `json:"kind_counts"`
	RawSHA256      string         `json:"raw_sha256"`
	GzSHA256       string         `json:"gz_sha256"`
	LastLineSHA256 string         `json:"last_line_sha256"`
}

type sealExtension struct {
	Summary        json.RawMessage `json:"summary"`
	ManifestSHA256 string          `json:"manifest_sha256"`
}

type sealRecord struct {
	core.LedgerEntry
	sealExtension
}

type manifestWire struct {
	Segment string          `json:"segment"`
	Summary json.RawMessage `json:"summary"`
}

type manifestLine struct {
	segment string
	raw     []byte
}

func chainSealInto(rec *sealRecord) func(seq int, prevHash string) any {
	return func(seq int, prevHash string) any {
		rec.EntrySeq = seq
		rec.PrevHash = prevHash
		return *rec
	}
}

func decodeSealRecord(line []byte) (sealRecord, bool) {
	var rec sealRecord
	if json.Unmarshal(line, &rec.LedgerEntry) != nil || rec.Kind != SealKind || json.Unmarshal(line, &rec.sealExtension) != nil {
		return sealRecord{}, false
	}
	return rec, true
}

func decodeSeals(lines [][]byte) []sealRecord {
	var seals []sealRecord
	for _, line := range lines {
		if rec, ok := decodeSealRecord(line); ok {
			seals = append(seals, rec)
		}
	}
	return seals
}

func summarizeSegment(lines [][]byte, rawSHA, gzSHA string) segmentSummary {
	s := segmentSummary{Version: summaryVersion, Lines: len(lines), Cycles: []int{}, KindCounts: map[string]int{}, RawSHA256: rawSHA, GzSHA256: gzSHA}
	seen := map[int]bool{}
	for i, line := range lines {
		e := decodeSummaryLine(line)
		if i == 0 {
			s.SeqFirst = e.EntrySeq
		}
		s.SeqLast = e.EntrySeq
		if e.TS != "" {
			if s.TSFirst == "" {
				s.TSFirst = e.TS
			}
			s.TSLast = e.TS
		}
		if e.Cycle != 0 && !seen[e.Cycle] {
			seen[e.Cycle] = true
			s.Cycles = append(s.Cycles, e.Cycle)
		}
		s.KindCounts[e.Kind]++
		s.LastLineSHA256 = sha256Hex(line)
	}
	sort.Ints(s.Cycles)
	return s
}

func decodeSummaryLine(line []byte) core.LedgerEntry {
	var e core.LedgerEntry
	if err := json.Unmarshal(line, &e); err != nil {
		return core.LedgerEntry{Kind: undecodableKind}
	}
	return e
}

func encodeSummary(sum segmentSummary) (json.RawMessage, error) {
	raw, err := hooks.marshal(sum)
	if err != nil {
		return nil, fmt.Errorf("ledger manifest: marshal summary: %w", err)
	}
	return raw, nil
}

func newManifestLine(rel string, summary json.RawMessage) (manifestLine, error) {
	raw, err := hooks.marshal(manifestWire{Segment: rel, Summary: summary})
	if err != nil {
		return manifestLine{}, fmt.Errorf("ledger manifest: marshal line: %w", err)
	}
	return manifestLine{segment: rel, raw: raw}, nil
}

func manifestBytes(lines []manifestLine) []byte {
	var buf bytes.Buffer
	for _, ml := range lines {
		buf.Write(ml.raw)
		buf.WriteByte('\n')
	}
	return buf.Bytes()
}

func readManifest(path string) []manifestLine {
	data, err := hooks.readF(path)
	if err != nil {
		return nil
	}
	var out []manifestLine
	for _, line := range splitLines(data) {
		var w manifestWire
		if json.Unmarshal(line, &w) != nil {
			return nil
		}
		out = append(out, manifestLine{segment: w.Segment, raw: line})
	}
	return out
}

func keepSegments(lines []manifestLine, rels []string) []manifestLine {
	wanted := map[string]bool{}
	for _, r := range rels {
		wanted[r] = true
	}
	var kept []manifestLine
	for _, ml := range lines {
		if wanted[ml.segment] {
			kept = append(kept, ml)
		}
	}
	return kept
}

func segmentGzSHA(path string) (string, error) {
	gz, err := hooks.readF(path)
	if err != nil {
		return "", fmt.Errorf("ledger manifest: read segment %s: %w", filepath.Base(path), err)
	}
	return sha256Hex(gz), nil
}

func segmentRels(evolveDir string, paths []string) ([]string, error) {
	rels := make([]string, len(paths))
	for i, p := range paths {
		rel, err := relSegment(evolveDir, p)
		if err != nil {
			return nil, fmt.Errorf("ledger manifest: rel: %w", err)
		}
		rels[i] = rel
	}
	return rels, nil
}

func (l *FileLedger) manifestPath() string {
	return filepath.Join(filepath.Dir(l.ledgerPath), segmentsDirName, manifestFileName)
}

func (l *FileLedger) writeManifestAndSealRecord(anchor *core.LedgerEntry) (*sealRecord, error) {
	evolveDir := filepath.Dir(l.ledgerPath)
	segs, err := segmentFiles(filepath.Join(evolveDir, segmentsDirName))
	if err != nil {
		return nil, err
	}
	segPath := filepath.Join(evolveDir, filepath.FromSlash(anchor.ArtifactPath))
	lines, rawSHA, err := readSegment(segPath)
	if err != nil {
		return nil, err
	}
	gzSHA, err := segmentGzSHA(segPath)
	if err != nil {
		return nil, err
	}
	liveRaw, err := hooks.readF(l.ledgerPath)
	if err != nil {
		return nil, fmt.Errorf("ledger manifest: read live: %w", err)
	}
	olderRels, err := segmentRels(evolveDir, segs[:sort.SearchStrings(segs, segPath)])
	if err != nil {
		return nil, err
	}
	prefix, err := l.healedPrefix(olderRels, append(lines[:len(lines):len(lines)], splitLines(liveRaw)...))
	if err != nil {
		return nil, err
	}
	summary, err := encodeSummary(summarizeSegment(lines, rawSHA, gzSHA))
	if err != nil {
		return nil, err
	}
	line, err := newManifestLine(anchor.ArtifactPath, summary)
	if err != nil {
		return nil, err
	}
	body := manifestBytes(append(prefix, line))
	if err := manifestWrite(l.manifestPath(), body, segmentMode); err != nil {
		return nil, fmt.Errorf("ledger manifest: write: %w", err)
	}
	return &sealRecord{LedgerEntry: *anchor, sealExtension: sealExtension{Summary: summary, ManifestSHA256: sha256Hex(body)}}, nil
}

func (l *FileLedger) healedPrefix(rels []string, tail [][]byte) ([]manifestLine, error) {
	if len(rels) == 0 {
		return nil, nil
	}
	for _, s := range decodeSeals(tail) {
		if s.ArtifactPath == rels[len(rels)-1] {
			return l.prefixBoundTo(rels, s.ManifestSHA256)
		}
	}
	return nil, fmt.Errorf("%w: no segment_seal for %s in the live tail", errManifestMismatch, rels[len(rels)-1])
}

func (l *FileLedger) prefixBoundTo(rels []string, bound string) ([]manifestLine, error) {
	if bound != "" {
		if kept := keepSegments(readManifest(l.manifestPath()), rels); sha256Hex(manifestBytes(kept)) == bound {
			return kept, nil
		}
	}
	rebuilt, err := l.rebuildLines(len(rels))
	if err != nil {
		return nil, err
	}
	if got := sha256Hex(manifestBytes(rebuilt)); bound != "" && got != bound {
		return nil, fmt.Errorf("%w: rebuilt manifest sha256 %s, the previous seal binds %s", errManifestMismatch, got, bound)
	}
	return rebuilt, nil
}

func (l *FileLedger) rebuildLines(upto int) ([]manifestLine, error) {
	segs, live, err := l.readChain()
	if err != nil {
		return nil, err
	}
	seals := map[string]sealRecord{}
	for _, s := range decodeSeals(chainOrder(segs, live)) {
		seals[s.ArtifactPath] = s
	}
	evolveDir := filepath.Dir(l.ledgerPath)
	out := make([]manifestLine, 0, upto)
	for _, seg := range segs[:upto] {
		gzSHA, err := segmentGzSHA(filepath.Join(evolveDir, filepath.FromSlash(seg.rel)))
		if err != nil {
			return nil, err
		}
		ml, err := manifestLineFor(seg, gzSHA, seals[seg.rel])
		if err != nil {
			return nil, err
		}
		out = append(out, ml)
	}
	return out, nil
}

func manifestLineFor(seg sealedSegment, gzSHA string, seal sealRecord) (manifestLine, error) {
	computed := summarizeSegment(seg.lines, seg.sha, gzSHA)
	if len(seal.Summary) == 0 {
		raw, err := encodeSummary(computed)
		if err != nil {
			return manifestLine{}, err
		}
		return newManifestLine(seg.rel, raw)
	}
	var recorded segmentSummary
	if err := json.Unmarshal(seal.Summary, &recorded); err != nil {
		return manifestLine{}, fmt.Errorf("%w: summary of %s does not decode: %v", errManifestMismatch, seg.rel, err)
	}
	if recorded.RawSHA256 != seal.ArtifactSHA256 || recorded.RawSHA256 != computed.RawSHA256 {
		return manifestLine{}, fmt.Errorf("%w: summary raw_sha256 of %s disagrees with its anchor or segment", errManifestMismatch, seg.rel)
	}
	if recorded.GzSHA256 != computed.GzSHA256 || recorded.Lines != computed.Lines {
		return manifestLine{}, fmt.Errorf("%w: summary gz_sha256 or lines of %s disagrees with its segment", errManifestMismatch, seg.rel)
	}
	return newManifestLine(seg.rel, seal.Summary)
}

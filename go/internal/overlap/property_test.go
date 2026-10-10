package overlap

import (
	"testing"

	"pgregory.net/rapid"
)

var pathPool = []string{
	src("a"), src("b"), src("c"), src("leaf"), src("top"),
	"go/internal/a/assets/x.txt", "go/internal/a/testdata/g.json", "go/internal/a/a_test.go",
	"docs/guide.md", "solutions/s/plan.md", "docs/architecture/control-flags.md",
	"go/go.sum", "go/Makefile", ".github/workflows/go.yml",
	".evolve/inbox/a.json", "knowledge-base/cycles/cycle-1.md", "go/acs/cycle1/p_test.go",
	"go/internal/a/notes.txt", "skills/x/SKILL.md", "docs/research/r.md", "docs/café.md",
}

var unknownPool = []string{"go/internal/a/notes.txt", "skills/x/SKILL.md", "docs/research/r.md", "docs/café.md", "README.md"}

func drawPaths(rt *rapid.T, label string) []string {
	return rapid.SliceOfDistinct(rapid.SampledFrom(pathPool), rapid.ID[string]).Draw(rt, label)
}

func shuffled[T any](rt *rapid.T, label string, xs []T) []T {
	out := append([]T(nil), xs...)
	for i := len(out) - 1; i > 0; i-- {
		j := rapid.IntRange(0, i).Draw(rt, label)
		out[i], out[j] = out[j], out[i]
	}
	return out
}

func TestProof_UnknownIsNeverT1(t *testing.T) {
	rapid.Check(t, func(rt *rapid.T) {
		in := baseInput(drawPaths(rt, "lane"), drawPaths(rt, "peer"))
		in.Peer = append(in.Peer, rapid.SampledFrom([]string{src("c"), "docs/guide.md", "go/Makefile"}).Draw(rt, "non-bookkeeping peer path"))
		switch rapid.IntRange(0, 2).Draw(rt, "unknown source") {
		case 0:
			in.Lane = append(in.Lane, rapid.SampledFrom(unknownPool).Draw(rt, "unknown lane path"))
		case 1:
			in.Peer = append(in.Peer, rapid.SampledFrom(unknownPool).Draw(rt, "unknown peer path"))
		default:
			in.Failures = []string{"go list: exit status 1"}
		}

		got := Prove(in)

		if got.Tier == T1 || got.Tier == T2 {
			rt.Fatalf("tier = %s with rules %v, want T3 or T4 for unknown %v", got.Tier, got.Rules, got.Evidence.Unknown)
		}
		if len(got.Evidence.Unknown) == 0 {
			rt.Fatalf("unknown evidence is empty for lane %v peer %v failures %v", in.Lane, in.Peer, in.Failures)
		}
	})
}

func TestEvidenceDigest_DoesNotDependOnInputOrder(t *testing.T) {
	rapid.Check(t, func(rt *rapid.T) {
		in := baseInput(drawPaths(rt, "lane"), drawPaths(rt, "peer"))
		in.Failures = rapid.SliceOfDistinct(rapid.SampledFrom([]string{"f1", "f2", "f3"}), rapid.ID[string]).Draw(rt, "failures")
		in.Blobs = map[string]BlobPair{}
		for _, p := range pathPool {
			in.Blobs[p] = BlobPair{Lane: "l-" + p, Peer: "p-" + p}
		}
		in.Catalogs = fakeCatalogs{dataReads: map[string]bool{"docs/guide.md": true}}

		reordered := in
		reordered.Lane = shuffled(rt, "lane order", in.Lane)
		reordered.Peer = shuffled(rt, "peer order", in.Peer)
		reordered.Failures = shuffled(rt, "failure order", in.Failures)
		reordered.Module.Packages = shuffled(rt, "package order", in.Module.Packages)
		for i := range reordered.Module.Packages {
			p := reordered.Module.Packages[i]
			p.Files = shuffled(rt, "file order", p.Files)
			p.Deps = shuffled(rt, "dep order", p.Deps)
			reordered.Module.Packages[i] = p
		}

		first, second := Prove(in), Prove(reordered)

		if len(first.EvidenceDigest) != 64 {
			rt.Fatalf("digest %q is not a hex SHA-256", first.EvidenceDigest)
		}
		if first.EvidenceDigest != second.EvidenceDigest {
			rt.Fatalf("digest %s != %s for evidence %+v and %+v", first.EvidenceDigest, second.EvidenceDigest, first.Evidence, second.Evidence)
		}
		if first.Tier != second.Tier {
			rt.Fatalf("tier %s != %s", first.Tier, second.Tier)
		}
	})
}

func TestEvidenceDigest_BindsTheBlobsOfEachEvidencePath(t *testing.T) {
	in := baseInput([]string{"docs/guide.md"}, []string{"docs/guide.md"})
	in.Blobs = map[string]BlobPair{"docs/guide.md": {Lane: "aaa", Peer: "bbb"}, "docs/other.md": {Lane: "x"}}
	before := Prove(in).EvidenceDigest

	in.Blobs = map[string]BlobPair{"docs/guide.md": {Lane: "aaa", Peer: "ccc"}, "docs/other.md": {Lane: "x"}}
	changed := Prove(in).EvidenceDigest

	in.Blobs = map[string]BlobPair{"docs/guide.md": {Lane: "aaa", Peer: "bbb"}, "docs/other.md": {Lane: "y"}}
	unrelated := Prove(in).EvidenceDigest

	if len(before) != 64 {
		t.Fatalf("digest %q is not a hex SHA-256", before)
	}
	if before == changed {
		t.Fatalf("digest %s did not change when the peer blob of an evidence path changed", before)
	}
	if before != unrelated {
		t.Fatalf("digest changed from %s to %s for a path outside the evidence", before, unrelated)
	}
}

func TestEvidenceDigest_BindsThePeerContentBehindAnEdge(t *testing.T) {
	digest := func(edgeBlob, otherBlob string) string {
		in := baseInput([]string{src("top")}, []string{src("a"), src("b")})
		in.Blobs = map[string]BlobPair{src("a"): {Peer: edgeBlob}, src("b"): {Peer: otherBlob}, src("top"): {Lane: "t"}}
		return Prove(in).EvidenceDigest
	}
	reverse := func(edgeBlob string) string {
		in := baseInput([]string{src("a")}, []string{src("top")})
		in.Blobs = map[string]BlobPair{src("top"): {Peer: edgeBlob}}
		return Prove(in).EvidenceDigest
	}

	before := digest("p1", "o1")

	if after := digest("p2", "o1"); after == before {
		t.Fatalf("digest %s did not change when the peer content of the edge package a changed", before)
	}
	if outside := digest("p1", "o2"); outside != before {
		t.Fatalf("digest changed from %s to %s for a peer path outside every edge package", before, outside)
	}
	if reverse("r1") == reverse("r2") {
		t.Fatal("digest did not change when the peer content behind a peer-to-lane edge changed")
	}
}

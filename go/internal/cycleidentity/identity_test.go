package cycleidentity

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/phasecontract"
)

func fixtureRoot(t *testing.T, registry, policy string) string {
	t.Helper()
	root := t.TempDir()
	if registry != "" {
		path := filepath.Join(root, "docs", "architecture", "phase-registry.json")
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(registry), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	if policy != "" {
		path := filepath.Join(root, ".evolve", "policy.json")
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(policy), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	return root
}

func contractsOf(version string) []phasecontract.Contract {
	return []phasecontract.Contract{{Phase: "scout", ArtifactName: version}, {Phase: "build", ArtifactName: "b.md"}}
}

func TestCompute_SameSourcesGiveTheSameIdentity(t *testing.T) {
	t.Parallel()
	root := fixtureRoot(t, `{"phases":["scout","build"]}`, `{"a":1,"b":2}`)
	first, err := Compute(Sources{Root: root, Contracts: contractsOf("v1")})
	if err != nil {
		t.Fatal(err)
	}
	reordered := fixtureRoot(t, "{ \"phases\": [\"scout\", \"build\"] }\n", `{"b":2,"a":1}`)
	second, err := Compute(Sources{Root: reordered, Contracts: []phasecontract.Contract{{Phase: "build", ArtifactName: "b.md"}, {Phase: "scout", ArtifactName: "v1"}}})
	if err != nil {
		t.Fatal(err)
	}
	if first != second {
		t.Fatalf("identity changed with spacing, key order or contract order: %+v vs %+v", first, second)
	}
	if first.Graph == "" || first.Contract == "" || first.Policy == "" {
		t.Fatalf("an identity part is empty: %+v", first)
	}
}

func TestCompute_EachSourceChangesOnlyItsPart(t *testing.T) {
	t.Parallel()
	base := Sources{Root: fixtureRoot(t, `{"phases":["scout","build"]}`, `{"a":1}`), Contracts: contractsOf("v1")}
	baseID, err := Compute(base)
	if err != nil {
		t.Fatal(err)
	}
	tests := []struct {
		name string
		src  Sources
		want []Part
	}{
		{"graph", Sources{Root: fixtureRoot(t, `{"phases":["build","scout"]}`, `{"a":1}`), Contracts: contractsOf("v1")}, []Part{PartGraph}},
		{"contract", Sources{Root: base.Root, Contracts: contractsOf("v2")}, []Part{PartContract}},
		{"policy", Sources{Root: fixtureRoot(t, `{"phases":["scout","build"]}`, `{"a":2}`), Contracts: contractsOf("v1")}, []Part{PartPolicy}},
	}
	for _, tc := range tests {
		got, err := Compute(tc.src)
		if err != nil {
			t.Fatalf("%s: %v", tc.name, err)
		}
		if parts := Diff(baseID, got); len(parts) != 1 || parts[0] != tc.want[0] {
			t.Errorf("%s: Diff = %v, want %v", tc.name, parts, tc.want)
		}
	}
}

func TestCompute_TheResumeBlockOfThePolicyIsOutsideThePolicyPart(t *testing.T) {
	t.Parallel()
	registry := `{"phases":["scout","build"]}`
	shadow, err := Compute(Sources{Root: fixtureRoot(t, registry, `{"a":1,"resume":{"identity":{"stage":"shadow"}}}`), Contracts: contractsOf("v1")})
	if err != nil {
		t.Fatal(err)
	}
	enforce, err := Compute(Sources{Root: fixtureRoot(t, registry, `{"a":1,"resume":{"max_checkpoint_age_h":24,"identity":{"stage":"enforce"}}}`), Contracts: contractsOf("v1")})
	if err != nil {
		t.Fatal(err)
	}
	if got := Diff(shadow, enforce); len(got) != 0 {
		t.Errorf("a resume-block edit changed %v, want no part", got)
	}
	other, err := Compute(Sources{Root: fixtureRoot(t, registry, `{"a":2,"resume":{"identity":{"stage":"shadow"}}}`), Contracts: contractsOf("v1")})
	if err != nil {
		t.Fatal(err)
	}
	if got := Diff(shadow, other); len(got) != 1 || got[0] != PartPolicy {
		t.Errorf("a pipeline key edit changed %v, want policy", got)
	}
}

func TestCompute_AMissingFileHashesAsEmptyAndRawBytesHashWhenNotJSON(t *testing.T) {
	t.Parallel()
	empty, err := Compute(Sources{Root: t.TempDir(), Contracts: contractsOf("v1")})
	if err != nil {
		t.Fatal(err)
	}
	raw, err := Compute(Sources{Root: fixtureRoot(t, "not json", "also not json"), Contracts: contractsOf("v1")})
	if err != nil {
		t.Fatal(err)
	}
	if got := Diff(empty, raw); len(got) != 2 || got[0] != PartGraph || got[1] != PartPolicy {
		t.Fatalf("Diff = %v, want graph and policy", got)
	}
}

func TestCompute_AnUnreadableSourceIsAnError(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, "docs", "architecture", "phase-registry.json"), 0o755); err != nil {
		t.Fatal(err)
	}
	if _, err := Compute(Sources{Root: root}); err == nil {
		t.Fatal("a registry that is a directory must be an error")
	}
	root = t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, ".evolve", "policy.json"), 0o755); err != nil {
		t.Fatal(err)
	}
	if _, err := Compute(Sources{Root: root}); err == nil {
		t.Fatal("a policy that is a directory must be an error")
	}
}

func TestLive_ReadsTheRealContracts(t *testing.T) {
	t.Parallel()
	src := Live(t.TempDir())
	if len(src.Contracts) == 0 {
		t.Fatal("Live carries no phase contract")
	}
}

func TestEncodeDecode_RoundTrip(t *testing.T) {
	t.Parallel()
	id := Identity{Graph: "g", Contract: "c", Policy: "p"}
	got, err := Decode(Encode(id))
	if err != nil || got != id {
		t.Fatalf("Decode(Encode) = %+v, %v", got, err)
	}
	if _, err := Decode("{"); err == nil {
		t.Fatal("a malformed record must be an error")
	}
	if _, err := Decode(""); !errors.Is(err, ErrNoIdentity) {
		t.Fatalf("an empty record = %v, want ErrNoIdentity", err)
	}
}

func TestCheck_NamesEveryChangedPart(t *testing.T) {
	t.Parallel()
	a := Identity{Graph: "g", Contract: "c", Policy: "p"}
	if err := Check(a, a); err != nil {
		t.Fatalf("same identity: %v", err)
	}
	err := Check(a, Identity{Graph: "g2", Contract: "c", Policy: "p2"})
	var mm *MismatchError
	if !errors.As(err, &mm) || !errors.Is(err, ErrMismatch) {
		t.Fatalf("Check = %v, want a MismatchError", err)
	}
	if len(mm.Parts) != 2 || mm.Parts[0] != PartGraph || mm.Parts[1] != PartPolicy {
		t.Fatalf("Parts = %v", mm.Parts)
	}
	if got := fmt.Sprint(mm); got == "" || JoinParts(mm.Parts) != "graph, policy" {
		t.Fatalf("message %q parts %q", got, JoinParts(mm.Parts))
	}
}

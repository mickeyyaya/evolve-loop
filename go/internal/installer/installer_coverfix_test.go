package installer

import (
	"bytes"
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

func TestValidateManifestFields_UnreadableManifestAddsNoVerdict(t *testing.T) {
	t.Parallel()
	var out bytes.Buffer
	var res ValidateResult
	validateManifestFields(filepath.Join(t.TempDir(), "missing-plugin.json"), &res, &out)
	if res.Errors != 0 || out.Len() != 0 {
		t.Fatalf("errors=%d out=%q, want no verdict for an unreadable manifest", res.Errors, out.String())
	}
}

func TestFrontmatterMissingKeys_UnreadableFileMissesEveryKey(t *testing.T) {
	t.Parallel()
	got := frontmatterMissingKeys(filepath.Join(t.TempDir(), "absent.md"), "name", "description")
	if want := []string{"name", "description"}; !reflect.DeepEqual(got, want) {
		t.Fatalf("missing = %v, want %v", got, want)
	}
}

func TestFrontmatterMissingKeys_BodyTextNeverSatisfiesAKey(t *testing.T) {
	t.Parallel()
	path := filepath.Join(t.TempDir(), "agent.md")
	if err := os.WriteFile(path, []byte("---\nname: x\n---\ndescription: in the body\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	got := frontmatterMissingKeys(path, "name", "description")
	if want := []string{"description"}; !reflect.DeepEqual(got, want) {
		t.Fatalf("missing = %v, want %v", got, want)
	}
}

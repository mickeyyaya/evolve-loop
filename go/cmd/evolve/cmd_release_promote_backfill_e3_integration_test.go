//go:build integration

package main

import (
	"bytes"
	"strings"
	"testing"
)

const e3GoreleaserFull = e3GoreleaserNoRepo + `
release:
  github:
    owner: e3owner
    name: e3repo
`

func TestRunReleasePromote_DrivesTheRealGHBinaryAgainstTheMatrixRepo(t *testing.T) {
	e3GoreleaserRoot(t, e3GoreleaserFull)
	record := bfStub(t, "gh", `case "$1 $2" in
"run list") echo '[{"databaseId":91}]' ;;
"run view") echo '{"status":"completed","conclusion":"success"}' ;;
"api repos/e3owner/e3repo/releases/tags/v5.0.0") echo '{"id":17,"assets":[{"name":"evolve_darwin_amd64.tar.gz"},{"name":"checksums.txt"}]}' ;;
"api -X") exit 0 ;;
*) echo "unexpected: $*" >&2; exit 9 ;;
esac`)
	var out, errb bytes.Buffer

	code := runReleasePromote([]string{"v5.0.0"}, nil, &out, &errb)

	if code != 0 || !strings.HasSuffix(out.String(), "release-promote: v5.0.0 promoted (prerelease cleared)\n") {
		t.Fatalf("code %d stdout %q stderr %q", code, out.String(), errb.String())
	}
	calls := bfCalls(t, record)
	for _, want := range []string{
		"run list --workflow release.yml --branch v5.0.0 --limit 1 --json " + runFields + " -R e3owner/e3repo",
		"run view 91 --json " + runFields + " -R e3owner/e3repo",
		"api -X PATCH repos/e3owner/e3repo/releases/17 -F prerelease=false",
	} {
		if !strings.Contains(calls, want) {
			t.Errorf("gh never ran %q; calls:\n%s", want, calls)
		}
	}
}

package landing

import (
	"strings"
	"testing"
)

func TestClassifyPush_PolicyIsFinalTransportBacksOffEverythingElseIsARace(t *testing.T) {
	for stderr, want := range map[string]pushFailure{
		serverError:   pushTransport,
		policyRefusal: pushPolicy,
		raceRejection: pushRace,
		"":            pushRace,
		"remote: error: GH006: Protected branch update failed for refs/heads/main.":                pushPolicy,
		"remote: Permission to o/r.git denied to bot.":                                             pushPolicy,
		"fatal: unable to access 'https://github.com/o/r/': The requested URL returned error: 503": pushTransport,
		"fatal: unable to access 'https://github.com/o/r/': The requested URL returned error: 403": pushPolicy,
		"error: RPC failed; HTTP 502 curl 22":                                                      pushTransport,
		"ssh: Could not resolve host: github.com":                                                  pushTransport,
		"fatal: the remote end hung up unexpectedly":                                               pushTransport,
		" ! [remote rejected] main -> main (pre-receive hook declined)":                            pushPolicy,
	} {
		if got := classifyPush(stderr); got != want {
			t.Errorf("classifyPush(%q) = %d, want %d", stderr, got, want)
		}
	}
}

func TestClassifyPush_TheRealStderrTextsOfGitAndGitHub(t *testing.T) {
	for _, tc := range []struct {
		stderr string
		want   pushFailure
	}{
		{"To github.com:o/r.git\n ! [remote rejected] main -> main (refusing to allow a Personal Access Token to create or update workflow `.github/workflows/ci.yml` without `workflow` scope)\nerror: failed to push some refs to 'github.com:o/r.git'\n", pushPolicy},
		{"To https://github.com/o/r.git\n ! [remote rejected] main -> main (refusing to allow an OAuth App to create or update workflow `.github/workflows/ci.yml` without `workflow` scope)\n", pushPolicy},
		{"remote: This repository was archived so it is read-only.\nfatal: unable to access 'https://github.com/o/r.git/': The requested URL returned error: 403\n", pushPolicy},
		{"remote: Write access to repository not granted.\nfatal: unable to access 'https://github.com/o/r.git/': The requested URL returned error: 403\n", pushPolicy},
		{"remote: Invalid username or token. Password authentication is not supported for Git operations.\nfatal: Authentication failed for 'https://github.com/o/r.git/'\n", pushPolicy},
		{"git@github.com: Permission denied (publickey).\nfatal: Could not read from remote repository.\n\nPlease make sure you have the correct access rights\nand the repository exists.\n", pushPolicy},
		{"remote: error: GH013: Repository rule violations found for refs/heads/main.\n ! [remote rejected] main -> main (push declined due to repository rule violations)\n", pushPolicy},
		{"error: RPC failed; HTTP 500 curl 22 The requested URL returned error: 500\nsend-pack: unexpected disconnect while reading sideband packet\nfatal: the remote end hung up unexpectedly\n", pushTransport},
		{"fatal: unable to access 'https://github.com/o/r.git/': Failed to connect to github.com port 443 after 75003 ms: Couldn't connect to server\n", pushTransport},
		{"kex_exchange_identification: Connection closed by remote host\nConnection closed by 140.82.121.4 port 22\nfatal: Could not read from remote repository.\n", pushTransport},
		{"error: RPC failed; curl 92 HTTP/2 stream 7 was not closed cleanly: CANCEL (err 8)\n", pushTransport},
		{"remote: Internal Server Error\nTo github.com:o/r.git\n ! [remote rejected] main -> main (Internal Server Error)\n", pushTransport},
		{" ! [remote rejected] main -> main (cannot lock ref 'refs/heads/main': is at 1111 but expected 2222)\n", pushRace},
		{"fatal: unable to access 'https://github.com/o/r.git/': The requested URL returned error: 429\n", pushTransport},
		{"remote: fatal error in commit_refs\n ! [remote rejected] main -> main (failure)\n", pushRace},
	} {
		if got := classifyPush(tc.stderr); got != tc.want {
			t.Errorf("classifyPush(%q) = %d, want %d", tc.stderr, got, tc.want)
		}
	}
}

func TestStderrDetail_JoinsGitsLinesAndBoundsThem(t *testing.T) {
	if got := stderrDetail("  first line \n\n second line\n"); got != "first line | second line" {
		t.Errorf("stderrDetail = %q", got)
	}
	long := stderrDetail(strings.Repeat("x", 2*maxStderrDetail))
	if len(long) != maxStderrDetail+len("…") || !strings.HasSuffix(long, "…") {
		t.Errorf("a long stderr is cut at %d bytes and marked: %d bytes", maxStderrDetail, len(long))
	}
}

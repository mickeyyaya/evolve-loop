package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"io/fs"
	"os"
	"regexp"
	"runtime"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/bridge"
	"github.com/mickeyyaya/evolve-loop/go/internal/runlease"
	"github.com/mickeyyaya/evolve-loop/go/internal/sysexec"
)

const (
	authUsage          = "usage: evolve auth store-times [--json]\n       evolve auth seat-token set --project-root <root>   (the token comes on stdin only; do not set it before the readers PR lands, because the post-mortem and the spend readers do not see the transcripts of a seat until then)\n       evolve auth seat-token clear --project-root <root>\n  store-times: print the time of the credentials file and of the Keychain item of the Claude Code login; it reads no credential\n  seat-token set: read the token of claude setup-token on stdin and write it to the holder at mode 0600; each claude seat then runs isolated on it; seat-token clear: remove it, and each claude seat uses the stored login; both refuse while a loop lane is live\n  run set in your own terminal, never paste a token into a chat or an agent session; a ! command in a session lands in the transcript\n  exit 0 = done, 2 = home directory unreadable or write failed, 3 = refused (a lane is live), 10 = usage, 11 = the value is not a seat token"
	authPrefix         = "evolve auth: "
	authStoreTimesName = "store-times"
	authSchema         = "evolve.auth.store-times/1"
	authKeychainWait   = 3 * time.Second
	authNoKeychain     = "no keychain"
	keychainNotFound   = 44
)

var keychainMdat = regexp.MustCompile(`"mdat"<timedate>=0x[0-9A-Fa-f]+\s+"(\d{14})Z`)

type storeState string

const (
	storePresent    storeState = "present"
	storeAbsent     storeState = "absent"
	storeUnreadable storeState = "unreadable"
)

type keychainTimeFunc func(ctx context.Context, service string) (time.Time, storeState, error)

type authEnv struct {
	Home     string
	Stat     func(string) (fs.FileInfo, error)
	Keychain keychainTimeFunc
	GOOS     string
	Stdin    io.Reader
	Live     func(string) []runlease.LiveRun
	Now      func() time.Time
}

type authStore struct {
	State  string `json:"state"`
	Time   string `json:"time,omitempty"`
	Detail string `json:"detail,omitempty"`
}

type authReport struct {
	Schema          string    `json:"schema"`
	CredentialsFile authStore `json:"credentials_file"`
	Keychain        authStore `json:"keychain"`
}

func (s authStore) text() string {
	if s.Time != "" {
		return s.Time
	}
	return s.State
}

func parseKeychainMdat(out string) (time.Time, bool) {
	m := keychainMdat.FindStringSubmatch(out)
	if m == nil {
		return time.Time{}, false
	}
	t, err := time.Parse("20060102150405", m[1])
	return t, err == nil
}

func newKeychainTime(run sysexec.RunFunc, timeout time.Duration) keychainTimeFunc {
	return func(ctx context.Context, service string) (time.Time, storeState, error) {
		ctx, cancel := context.WithTimeout(ctx, timeout)
		defer cancel()
		var out bytes.Buffer
		rc, err := run(ctx, "security", "", bridge.KeychainAttributesArgs(service), nil, nil, &out, io.Discard)
		switch {
		case ctx.Err() != nil:
			return time.Time{}, storeUnreadable, fmt.Errorf("read the Keychain item time: %w", ctx.Err())
		case err != nil:
			return time.Time{}, storeUnreadable, fmt.Errorf("read the Keychain item time: %w", err)
		case rc == keychainNotFound:
			return time.Time{}, storeAbsent, nil
		case rc != 0:
			return time.Time{}, storeUnreadable, fmt.Errorf("security exited %d", rc)
		}
		t, ok := parseKeychainMdat(out.String())
		if !ok {
			return time.Time{}, storeUnreadable, errors.New("no mdat in the Keychain item")
		}
		return t, storePresent, nil
	}
}

func fileStore(env authEnv) authStore {
	info, err := env.Stat(bridge.ClaudeCredentialsPath(env.Home))
	switch {
	case errors.Is(err, fs.ErrNotExist):
		return authStore{State: string(storeAbsent)}
	case err != nil:
		return authStore{State: string(storeUnreadable), Detail: err.Error()}
	}
	return authStore{State: string(storePresent), Time: info.ModTime().UTC().Format(time.RFC3339)}
}

func keychainStore(ctx context.Context, env authEnv) authStore {
	if env.GOOS != "darwin" {
		return authStore{State: authNoKeychain}
	}
	t, state, err := env.Keychain(ctx, bridge.ClaudeKeychainService)
	if err != nil {
		return authStore{State: string(storeUnreadable), Detail: err.Error()}
	}
	if state != storePresent {
		return authStore{State: string(state)}
	}
	return authStore{State: string(state), Time: t.UTC().Format(time.RFC3339)}
}

func reportDetail(w io.Writer, name string, s authStore) {
	if s.Detail != "" {
		fmt.Fprintf(w, "%s%s: %s\n", authPrefix, name, s.Detail)
	}
}

func runAuthEnv(env authEnv, args []string, stdout, stderr io.Writer) int {
	if len(args) > 0 && (args[0] == "--help" || args[0] == "-h") {
		fmt.Fprintln(stdout, authUsage)
		return 0
	}
	if len(args) > 0 && args[0] == authSeatTokenName {
		return runSeatToken(env, args[1:], stdout, stderr)
	}
	if len(args) == 0 || args[0] != authStoreTimesName {
		fmt.Fprintf(stderr, "%sname the sub-verb %s\n%s\n", authPrefix, authStoreTimesName, authUsage)
		return exitUsage
	}
	var asJSON bool
	flags := flag.NewFlagSet("auth "+authStoreTimesName, flag.ContinueOnError)
	flags.SetOutput(io.Discard)
	flags.BoolVar(&asJSON, "json", false, "")
	err := flags.Parse(args[1:])
	if errors.Is(err, flag.ErrHelp) {
		fmt.Fprintln(stdout, authUsage)
		return 0
	}
	if err == nil && flags.NArg() > 0 {
		err = fmt.Errorf("unexpected argument %q", flags.Arg(0))
	}
	if err != nil {
		fmt.Fprintf(stderr, "%s%v\n%s\n", authPrefix, err, authUsage)
		return exitUsage
	}
	report := authReport{Schema: authSchema, CredentialsFile: fileStore(env), Keychain: keychainStore(context.Background(), env)}
	if asJSON {
		b, _ := json.Marshal(report)
		fmt.Fprintf(stdout, "%s\n", b)
		return 0
	}
	fmt.Fprintf(stdout, "credentials file: %s\nkeychain item: %s\n", report.CredentialsFile.text(), report.Keychain.text())
	reportDetail(stderr, "credentials file", report.CredentialsFile)
	reportDetail(stderr, "keychain item", report.Keychain)
	return 0
}

func productionAuthEnv(home string) authEnv {
	return authEnv{
		Home:     home,
		Stat:     os.Stat,
		Keychain: newKeychainTime(sysexec.DefaultRunner, authKeychainWait),
		GOOS:     runtime.GOOS,
		Live:     liveRunsOf,
		Now:      time.Now,
	}
}

func runAuthAt(getHome func() (string, error), stdin io.Reader, args []string, stdout, stderr io.Writer) int {
	home, err := getHome()
	if err != nil {
		fmt.Fprintf(stderr, "%sread the home directory: %v\n", authPrefix, err)
		return exitIO
	}
	env := productionAuthEnv(home)
	env.Stdin = stdin
	return runAuthEnv(env, args, stdout, stderr)
}

func runAuth(args []string, stdin io.Reader, stdout, stderr io.Writer) int {
	return runAuthAt(os.UserHomeDir, stdin, args, stdout, stderr)
}

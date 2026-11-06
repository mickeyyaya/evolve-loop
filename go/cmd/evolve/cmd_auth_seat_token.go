package main

import (
	"errors"
	"flag"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/atomicwrite"
	"github.com/mickeyyaya/evolve-loop/go/internal/paths"
	"github.com/mickeyyaya/evolve-loop/go/internal/seatlogin"
	"github.com/mickeyyaya/evolve-loop/go/internal/signalcenter"
)

const (
	roleAuth           = "auth"
	authSeatTokenName  = "seat-token"
	seatTokenSet       = "set"
	seatTokenClear     = "clear"
	seatTokenMaxBytes  = seatlogin.MaxTokenBytes
	seatTokenDirMode   = 0o700
	seatTokenFileMode  = seatlogin.TokenMode
	codeSeatTokenMoved = signalcenter.Code("CLI_SEAT_TOKEN_MOVED")
)

func init() {
	signalcenter.RegisterCode(signalcenter.ModuleCLI, codeSeatTokenMoved, "the operator ran evolve auth seat-token set or clear; fields.verb names the verb and fields.at the time; the token is never in the signal")
}

type seatTokenCall struct {
	verb string
	root string
	env  authEnv
	out  io.Writer
	err  io.Writer
}

func (c seatTokenCall) prefix() string {
	return "evolve auth " + authSeatTokenName + " " + c.verb + ": "
}

func runSeatToken(env authEnv, args []string, stdout, stderr io.Writer) int {
	if len(args) == 0 || (args[0] != seatTokenSet && args[0] != seatTokenClear) {
		return authUsageError(stderr, fmt.Errorf("name the sub-verb %s set or %s clear", authSeatTokenName, authSeatTokenName))
	}
	verb := args[0]
	var root string
	flags := flag.NewFlagSet("auth "+authSeatTokenName+" "+verb, flag.ContinueOnError)
	flags.SetOutput(io.Discard)
	flags.StringVar(&root, "project-root", "", "")
	switch {
	case flags.Parse(args[1:]) != nil:
		return authUsageError(stderr, errors.New("a flag is not known; the verb takes only --project-root"))
	case flags.NArg() > 0:
		return authUsageError(stderr, errors.New("the verb takes no argument: the token comes on stdin"))
	case root == "":
		return authUsageError(stderr, errors.New("--project-root is required: name the root of the loop"))
	}
	if info, err := os.Stat(paths.EvolveDirOf(root)); err != nil || !info.IsDir() {
		return authUsageError(stderr, errors.New("--project-root is not a loop root: it has no .evolve directory"))
	}
	call := seatTokenCall{verb: verb, root: root, env: env, out: stdout, err: stderr}
	if live := env.Live(root); len(live) > 0 {
		fmt.Fprintf(stderr, "%srefused — a loop lane is live (%s, pid %d); run it at a wave boundary (`evolve loop-stop --wait`)\n", call.prefix(), live[0].Dir, live[0].Lease.OwnerPID)
		return cliExitRefused
	}
	path := paths.SeatTokenPath(env.Home)
	if path == "" {
		fmt.Fprintf(stderr, "%sthe home directory is empty\n", call.prefix())
		return cliExitIO
	}
	if verb == seatTokenSet {
		return call.set(path)
	}
	return call.clear(path)
}

func authUsageError(stderr io.Writer, err error) int {
	fmt.Fprintf(stderr, "%s%v\n%s\n", authPrefix, err, authUsage)
	return cliExitUsage
}

func (c seatTokenCall) set(path string) int {
	raw, err := io.ReadAll(io.LimitReader(c.env.Stdin, seatTokenMaxBytes+2))
	if err != nil {
		fmt.Fprintf(c.err, "%sread stdin: %v\n", c.prefix(), err)
		return cliExitIO
	}
	token, reason := seatlogin.ParseToken(string(raw))
	if reason != "" {
		fmt.Fprintf(c.err, "%sthe value on stdin was refused: %s. The holder at %s is unchanged. Next: run claude setup-token, then evolve auth seat-token set again.\n", c.prefix(), reason, path)
		return cliExitInput
	}
	if err := os.MkdirAll(filepath.Dir(path), seatTokenDirMode); err != nil {
		fmt.Fprintf(c.err, "%screate the holder directory: %v\n", c.prefix(), err)
		return cliExitIO
	}
	if err := atomicwrite.Durable(path, []byte(token+"\n"), seatTokenFileMode); err != nil {
		fmt.Fprintf(c.err, "%swrite the token: %v\n", c.prefix(), err)
		return cliExitIO
	}
	c.signal()
	fmt.Fprintf(c.out, "seat-token set: wrote %s at mode 0600; each claude seat now runs isolated on it\n", path)
	return cliExitOK
}

func (c seatTokenCall) clear(path string) int {
	if err := os.Remove(path); err != nil && !errors.Is(err, fs.ErrNotExist) {
		fmt.Fprintf(c.err, "%sremove the token: %v\n", c.prefix(), err)
		return cliExitIO
	}
	if err := os.Remove(paths.SeatSettingsPath(c.env.Home)); err != nil && !errors.Is(err, fs.ErrNotExist) {
		fmt.Fprintf(c.err, "%sremove the seat settings: %v\n", c.prefix(), err)
		return cliExitIO
	}
	c.signal()
	fmt.Fprintf(c.out, "seat-token clear: %s is absent; each claude seat uses the stored login\n", path)
	return cliExitOK
}

func (c seatTokenCall) signal() {
	signals, closeSignals := newRoleSignalCenter(roleAuth, c.root, paths.EvolveDirOf(c.root), c.err)
	defer closeSignals()
	signals.Emit(signalcenter.Event{Module: signalcenter.ModuleCLI, Origin: "evolve.seatToken", Kind: signalcenter.KindConfigWarning,
		Severity: signalcenter.SeverityInfo, Code: codeSeatTokenMoved,
		Reason: "the seat token holder moved",
		Fields: map[string]string{"verb": c.verb, "at": c.env.Now().UTC().Format(time.RFC3339)}})
}

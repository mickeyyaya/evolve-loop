package main

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"flag"
	"fmt"
	"io"
	"path/filepath"
	"slices"
	"sort"

	"github.com/mickeyyaya/evolve-loop/go/internal/adapters/flock"
	"github.com/mickeyyaya/evolve-loop/go/internal/adapters/ledger"
	"github.com/mickeyyaya/evolve-loop/go/internal/gatecontract"
	"github.com/mickeyyaya/evolve-loop/go/internal/gateregistry"
	"github.com/mickeyyaya/evolve-loop/go/internal/policy"
	"github.com/mickeyyaya/evolve-loop/go/internal/stageevidence"
)

const (
	stageMigrateLockName = "stage-migrate.lock"
	grandfatherReason    = "enforced before the contract"
)

var (
	errRestoreReason    = errors.New("--restore-genesis needs --reason")
	errNothingToRestore = errors.New("no grandfather flip record exists, so there is no genesis to restore")
)

type unresolvedKeyError struct{ Key string }

func (e *unresolvedKeyError) Error() string {
	return fmt.Sprintf("%v: %s", stageevidence.ErrUnresolvedStageKey, e.Key)
}

func (e *unresolvedKeyError) Unwrap() error { return stageevidence.ErrUnresolvedStageKey }

type migrateStore interface {
	stageevidence.FlipRecords
	AppendStageFlip(ctx context.Context, f stageevidence.Flip) error
	AppendStageGenesis(ctx context.Context, g stageevidence.Genesis) error
}

type migrateRequest struct {
	Effective       map[string]string
	Digest          string
	Reason          string
	GenesisRequired bool
	DryRun          bool
	Restore         bool
}

type migrateResult struct {
	Existing *stageevidence.Genesis
	Genesis  *stageevidence.Genesis
	Planned  []string
	DryRun   bool
}

type migratePlan struct {
	flips   []string
	genesis stageevidence.Genesis
}

func migrateStages(ctx context.Context, store migrateStore, req migrateRequest) (migrateResult, error) {
	set, err := store.FlipRecords(ctx)
	if err != nil {
		return migrateResult{}, err
	}
	if set.Genesis != nil {
		return migrateResult{Existing: set.Genesis}, nil
	}
	plan, err := planMigration(set, req)
	if err != nil {
		return migrateResult{}, err
	}
	res := migrateResult{Genesis: &plan.genesis, Planned: plan.flips, DryRun: req.DryRun}
	if req.DryRun {
		return res, nil
	}
	return res, writeMigration(ctx, store, req, plan)
}

func planMigration(set stageevidence.FlipSet, req migrateRequest) (migratePlan, error) {
	if req.Restore {
		return planRestore(set, req)
	}
	if req.GenesisRequired {
		return migratePlan{}, stageevidence.ErrGenesisRequired
	}
	for key, value := range req.Effective {
		if value == "" {
			return migratePlan{}, &unresolvedKeyError{Key: key}
		}
	}
	keys := stageevidence.EnforcedKeys(req.Effective)
	var flips []string
	for _, key := range keys {
		if !slices.Contains(grandfatherKeys(set), key) {
			flips = append(flips, key)
		}
	}
	return migratePlan{flips: flips, genesis: genesisOfKeys(keys, req)}, nil
}

func planRestore(set stageevidence.FlipSet, req migrateRequest) (migratePlan, error) {
	if req.Reason == "" {
		return migratePlan{}, errRestoreReason
	}
	keys := grandfatherKeys(set)
	if len(keys) == 0 {
		return migratePlan{}, errNothingToRestore
	}
	return migratePlan{genesis: genesisOfKeys(keys, req)}, nil
}

func genesisOfKeys(keys []string, req migrateRequest) stageevidence.Genesis {
	return stageevidence.Genesis{V: stageevidence.FlipVersion, Keys: keys, PolicyDigest: req.Digest, Count: len(keys), Reason: req.Reason}
}

func grandfatherKeys(set stageevidence.FlipSet) []string {
	var keys []string
	for _, f := range set.Flips {
		if f.Origin == stageevidence.OriginGrandfather && f.Value == gatecontract.StageEnforce && !slices.Contains(keys, f.Key) {
			keys = append(keys, f.Key)
		}
	}
	sort.Strings(keys)
	return keys
}

func writeMigration(ctx context.Context, store migrateStore, req migrateRequest, plan migratePlan) error {
	for _, key := range plan.flips {
		flip := stageevidence.Flip{V: stageevidence.FlipVersion, Key: key, Value: gatecontract.StageEnforce, Origin: stageevidence.OriginGrandfather, PolicyDigest: req.Digest, Reason: grandfatherReason}
		if err := store.AppendStageFlip(ctx, flip); err != nil {
			return err
		}
	}
	return store.AppendStageGenesis(ctx, plan.genesis)
}

type stageMigrateEnv struct {
	genesisRequired func() (bool, error)
	lock            func(path string) (func(), error)
	effective       func(policy.Policy) (map[string]string, error)
}

func defaultStageMigrateEnv() stageMigrateEnv {
	return stageMigrateEnv{
		genesisRequired: func() (bool, error) {
			ratchet, err := gateregistry.StageRatchet()
			return ratchet.GenesisRequired, err
		},
		lock:      flock.Lock,
		effective: policy.Policy.EffectiveStages,
	}
}

type stageMigrateCmd struct {
	env    stageMigrateEnv
	stdout io.Writer
	stderr io.Writer
}

type migrateFlags struct {
	projectRoot string
	reason      string
	dryRun      bool
	restore     bool
}

func runStageMigrate(args []string, stdout, stderr io.Writer) int {
	return stageMigrateCmd{env: defaultStageMigrateEnv(), stdout: stdout, stderr: stderr}.run(args)
}

func (c stageMigrateCmd) parse(args []string) (migrateFlags, int) {
	fs := flag.NewFlagSet("evolve stage migrate", flag.ContinueOnError)
	fs.SetOutput(c.stderr)
	var f migrateFlags
	fs.StringVar(&f.projectRoot, "project-root", "", "project root (default: $EVOLVE_PROJECT_ROOT or cwd)")
	fs.StringVar(&f.reason, "reason", "", "operator reason, required with --restore-genesis")
	fs.BoolVar(&f.dryRun, "dry-run", false, "print the keys that the migration would record and write nothing")
	fs.BoolVar(&f.restore, "restore-genesis", false, "write the genesis record from the existing grandfather records and grandfather no new key")
	if err := fs.Parse(args); err != nil {
		return f, cliExitUsage
	}
	if fs.NArg() != 0 || (f.reason != "" && !f.restore) {
		fmt.Fprintln(c.stderr, "evolve stage migrate: usage: evolve stage migrate [--dry-run] [--restore-genesis --reason TEXT] [--project-root P]")
		return f, cliExitUsage
	}
	return f, cliExitOK
}

func (c stageMigrateCmd) run(args []string) int {
	flags, rc := c.parse(args)
	if rc != cliExitOK {
		return rc
	}
	root, policyBytes, rc := loadStagePolicy(flags.projectRoot, c.stderr)
	if rc != cliExitOK {
		return rc
	}
	req, err := c.request(flags, policyBytes)
	if err != nil {
		return c.fail(err)
	}
	evolveDir := stageEvolveDir(root)
	release, err := c.env.lock(filepath.Join(evolveDir, stageMigrateLockName))
	if err != nil {
		return c.fail(fmt.Errorf("lock: %w", err))
	}
	defer release()
	res, err := migrateStages(context.Background(), ledger.New(evolveDir), req)
	if err != nil {
		return c.fail(err)
	}
	writeMigrateResult(c.stdout, res)
	return cliExitOK
}

func (c stageMigrateCmd) request(flags migrateFlags, policyBytes []byte) (migrateRequest, error) {
	required, err := c.env.genesisRequired()
	if err != nil {
		return migrateRequest{}, err
	}
	p, err := policy.Parse(policyBytes)
	if err != nil {
		return migrateRequest{}, fmt.Errorf("parse policy.json: %w", err)
	}
	effective, err := c.env.effective(p)
	if err != nil {
		return migrateRequest{}, fmt.Errorf("%w: %v", stageevidence.ErrUnresolvedStageKey, err)
	}
	sum := sha256.Sum256(policyBytes)
	return migrateRequest{Effective: effective, Digest: hex.EncodeToString(sum[:]), Reason: flags.reason, GenesisRequired: required, DryRun: flags.dryRun, Restore: flags.restore}, nil
}

func (c stageMigrateCmd) fail(err error) int {
	fmt.Fprintf(c.stderr, "evolve stage migrate: %v\n", err)
	switch {
	case errors.Is(err, stageevidence.ErrGenesisRequired):
		return cliExitRefused
	case errors.Is(err, errRestoreReason):
		return cliExitUsage
	case errors.Is(err, errNothingToRestore), errors.Is(err, stageevidence.ErrUnresolvedStageKey):
		return cliExitFailed
	}
	return cliExitIO
}

func writeMigrateResult(w io.Writer, res migrateResult) {
	switch {
	case res.Existing != nil:
		fmt.Fprintf(w, "stage migrate: genesis present: %d keys, policy digest %s\n", res.Existing.Count, res.Existing.PolicyDigest)
		writeMigrateKeys(w, res.Existing.Keys)
	case res.DryRun:
		fmt.Fprintf(w, "stage migrate: dry run: would write %d flip record(s) and one genesis record for %d key(s)\n", len(res.Planned), res.Genesis.Count)
		writeMigrateKeys(w, res.Genesis.Keys)
	default:
		fmt.Fprintf(w, "stage migrate: wrote %d flip record(s) and one genesis record for %d key(s)\n", len(res.Planned), res.Genesis.Count)
		writeMigrateKeys(w, res.Genesis.Keys)
	}
}

func writeMigrateKeys(w io.Writer, keys []string) {
	for _, key := range keys {
		fmt.Fprintf(w, "  %s\n", key)
	}
}

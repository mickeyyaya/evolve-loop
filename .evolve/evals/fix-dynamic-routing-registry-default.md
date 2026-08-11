# Eval: fix-dynamic-routing-registry-default

## Task
Update the `EVOLVE_DYNAMIC_ROUTING` entry in `go/internal/flagregistry/registry_table.go` to:
1. Set `Default: "advisory"` (the actual code default since cycle-108; currently missing)
2. Update the `Cluster` field to remove the stale `default-off` annotation (was true at v13.0.0/PR#4 but not since cycle-108)

Then re-run `evolve flags generate` so the Generated Flag Index in `docs/architecture/control-flags.md`
shows the correct default.

## Acceptance Criteria

### AC-1: Registry entry has Default="advisory" [code]
```bash
# registry_table.go must have Default:"advisory" for EVOLVE_DYNAMIC_ROUTING
grep -A 3 '"EVOLVE_DYNAMIC_ROUTING"' go/internal/flagregistry/registry_table.go | grep -q 'Default.*advisory'
echo "exit: $?"
# exit 0 = PASS
```

### AC-2: Cluster field no longer contains "default-off" [code]
```bash
grep -A 3 '"EVOLVE_DYNAMIC_ROUTING"' go/internal/flagregistry/registry_table.go | grep -q 'default-off'
if [ $? -eq 0 ]; then
  echo "FAIL: stale 'default-off' annotation still present in Cluster field"
  exit 1
fi
echo "PASS: stale annotation removed"
```

### AC-3: Generated Flag Index reflects advisory default [code]
```bash
# After 'evolve flags generate', the Generated Flag Index section must show "advisory"
# as the Default for EVOLVE_DYNAMIC_ROUTING
awk '/^## Generated Flag Index/{found=1}found' docs/architecture/control-flags.md \
  | grep 'EVOLVE_DYNAMIC_ROUTING' \
  | grep -q 'advisory'
echo "exit: $?"
# exit 0 = PASS
```

### AC-4: evolve flags check exits 0 [code]
```bash
cd go && go build -o /tmp/evolve-flagcheck ./cmd/evolve/ && \
  /tmp/evolve-flagcheck flags check 2>&1
# exit code must be 0
```

### AC-5: All flagregistry tests pass [code]
```bash
cd go && go test ./internal/flagregistry/... -count=1 -v 2>&1 | grep -E "^(ok|FAIL|---)"
# Must show "ok" and no FAIL lines
```

### AC-5 (Negative — gaming guard): code default in config.go must be StageAdvisory [code]
The registry default annotation must match the code. Verify config.go still hard-codes advisory.
```bash
grep -A 5 'RolloutStages:' go/internal/config/config.go | head -10
grep 'Stage:.*StageAdvisory' go/internal/config/config.go
echo "exit: $?"
# exit 0 = config.go still sets advisory (registry annotation matches reality)
```

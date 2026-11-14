# Build Explanation — Cycle 2379

## Build Binding
- Cycle: 2379
- Base SHA: d4af8528106710209f9b2543759b4cec4eda6a16

## Summary
`cmd/hostsampler` writes one host-class NDJSON header line on startup. `cmd/testlatency` reads that header with `-samples` and prints the host class in CI job summaries.

## Rationale
Recording CPU count and memory size inside `runner-samples.ndjson` makes 3-core versus 5-core macOS runner variance visible. Rendering them beside the package wall report keeps `Sample` parsing and `citelemetry` intact.

## Changed Areas
- `.github/workflows/go.yml` — passes `-samples "$GITHUB_WORKSPACE/go/runner-samples.ndjson"` to both ubuntu and macOS `testlatency -budget` summary steps.
- `docs/architecture/packages/cmd-hostsampler.md` — documents the startup host-class header (`hw.ncpu`, `hw.memsize`, `nproc`, `MemTotal`) in ASD-STE100.
- `docs/architecture/packages/cmd-testlatency.md` — documents `-samples` and the `Host class` overview line in ASD-STE100.
- `go/cmd/hostsampler/sample.go` — adds `HostInfo` and `HostRecord` structs for the startup header.
- `go/cmd/hostsampler/main.go` — adds `probeHost` hook and writes `HostRecord` once at the start of `sampleLoop`.
- `go/cmd/hostsampler/macos.go` — adds `readMacOSHost` querying `sysctl -n hw.ncpu` and `sysctl -n hw.memsize`.
- `go/cmd/hostsampler/linux.go` — adds `readLinuxHost` and `parseMemTotalBytes` querying `nproc` and `/proc/meminfo` `MemTotal`.
- `go/cmd/hostsampler/source_darwin.go` — implements `readHost` using `readMacOSHost(execOutput)`.
- `go/cmd/hostsampler/source_linux.go` — implements `readHost` using `readLinuxHost(procRoot, execOutput)`.
- `go/cmd/hostsampler/source_other.go` — implements fallback `readHost` returning `HostInfo{OS: runtime.GOOS}`.
- `go/cmd/hostsampler/main_test.go` — updates `readSamples` to assert the line-1 `HostInfo` header before sample records.
- `go/cmd/hostsampler/macos_test.go` — adds unit tests for `readMacOSHost` success and malformed probe handling.
- `go/cmd/hostsampler/linux_test.go` — adds unit tests for `readLinuxHost` success and failed probe handling.
- `go/cmd/testlatency/main.go` — adds `-samples` flag and passes `HasSamples` and `HostClass` to `rep.Markdown`.
- `go/cmd/testlatency/report.go` — adds `HostClass`, `readHostClass`, and `Host class` overview formatting (including `not recorded` fallback).
- `go/cmd/testlatency/report_test.go` — adds unit tests for `Host class` rendering with host data, missing header, and omitted flag.

## Design Decisions
Using `{"host":{"os":...,"cpus":...,"mem_bytes":...}}` with no `ts` field avoids colliding with `Sample` unmarshaling in `citelemetry`. Scanning every line in `readHostClass` tolerates sample files where the header is absent or not on line 1.

## Verification
`cd go && go test -count=1 ./cmd/hostsampler/... ./cmd/testlatency/... ./internal/citelemetry/...` and `cd go && go test -tags acs -count=1 ./acs/cycle2379` all pass, and `evolve docs ste-lint --strict` reports 0 findings.

## Compatibility
Existing `testlatency` invocations without `-samples` produce identical output, and `-samples` with a missing, empty, or headerless file prints `not recorded` and exits 0.

## Limitations
`hostsampler` probes host class once at startup and ignores failed `sysctl` or `nproc` probes by leaving fields zero rather than failing the CI job.

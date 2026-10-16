# internal/doctor

## Purpose

`internal/doctor` implements `evolve doctor probe <tool>`, the Go port of `scripts/utility/probe-tool.sh`. It finds a CLI tool and shows the trail of places it checked.

## Design

- **A trail, not one check.** An earlier insights report described a failure pattern: an agent declares a tool unavailable after one check. The probe walks `PATH` with `exec.LookPath`, then the explicit fallback locations, and returns the full `Checked` list.
- **Fallback order matches the bash script.** `/usr/local/bin`, `/opt/homebrew/bin` (Homebrew Intel and Apple silicon), `~/.local/bin`, `~/bin`, `/usr/bin`. Home-prefixed paths are skipped when the home directory is unknown, and the error is logged in `Checked`.
- **Wire shape.** `EmitJSON` writes `{tool, found, path, method, checked:[...]}`. `Path` serializes as `null` when empty, so `Result` carries a custom wire type. `Method` is `path` or `fallback`.
- **Seams.** `probeHooks` holds the injectable functions (look-path, candidate builder, home directory, marshal) so tests drive the PATH-miss, fallback-hit and home-error branches without the host.
- **Executable check.** A file counts when any mode bit is executable. On Windows the check degrades to "is a regular file", since NTFS ACLs do not carry the bit the same way.

## Invariants

- **The error return is reserved.** A missing tool is `Found=false`, never an error. Pinned by `TestProbe_NotFound` and `TestRealProbe_Missing`.
- **The real-host test needs `go` on `PATH`**, because the test binary was built with it. `TestRealProbe_Go` fails loudly when that breaks.

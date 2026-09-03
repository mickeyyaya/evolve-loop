## Explanation Documentation

- Status: NEEDS_CORRECTION
- Build status: required
- Document: docs/explain/builds/cycle-1605-01m1fd689e6f9m7sz7gj985tz1.md
- Document SHA256: 15da26236753cadb54feab7d1955cdacfb2b5853d5a5ddcbe42e8e8da343b820
- Evidence: the host handoff integrity check passes — `git show :docs/explain/builds/cycle-1605-01m1fd689e6f9m7sz7gj985tz1.md | shasum -a 256` and the worktree copy both equal the typed handoff `document_sha256`, and all three `material_paths` map to real Changed Areas (`docs/explain/builds/cycle-1605-01m1fd689e6f9m7sz7gj985tz1.md:14` ↔ `go/.apicover-enforce:66`; `:15` ↔ `go/internal/decisionsample/sampler.go:23-48`; `:16` ↔ `go/internal/policy/policy.go:426-427`). Summary (`:8`), Rationale (`:11`), Compatibility (`:31`) and Limitations (`:34`) are accurate and reproduce under `go test -tags acs -count=1 ./acs/cycle1605` (5/5 PASS). Correction required at `docs/explain/builds/cycle-1605-01m1fd689e6f9m7sz7gj985tz1.md:17`, which lists `go/internal/policy/decision_sampling_test.go` as a Changed Area although that path is in neither the base-bound diff nor the index (M2); and at `:15`, whose "Changed Areas" framing of `go/internal/decisionsample/sampler.go` omits that the exported seam it adds has no production caller (H1).


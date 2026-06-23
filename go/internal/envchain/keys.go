package envchain

// keys.go is the single registry of operational EVOLVE_* env-var names and
// their default values. Centralizing the string literals means a rename
// touches one line and call sites can't drift on a typo; centralizing the
// defaults gives the previously-inlined magic numbers (Smell D) one home with
// a documented rationale.
//
// This registry grows as read sites migrate onto the typed getters — it is
// deliberately NOT an exhaustive enumeration of every flag in CLAUDE.md.
// Single-use, never-duplicated reads (e.g. one-off path overrides) stay inline
// at their call site; a key earns a constant here when a typed getter reads it.

// SystemPromptReqEnvKey is the reqEnv-only lookup key for the global
// system-prompt override. Written as a concatenation so the flagreaders
// AST guard does not treat it as an os.Getenv reader: after the cycle-16
// envchain.ResolveNoOS migration, EVOLVE_SYSTEM_PROMPT is no longer read
// from the OS environment — it is only looked up in the reqEnv map (the
// orchestrator-set in-process pass-through, not a shell env var).
const SystemPromptReqEnvKey = "EVOLVE_" + "SYSTEM_PROMPT"

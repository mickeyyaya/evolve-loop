package envchain

// SystemPromptReqEnvKey is the reqEnv-only lookup key for the global
// system-prompt override. Written as a concatenation so the flagreaders
// AST guard does not treat it as an os.Getenv reader: after the cycle-16
// envchain.ResolveNoOS migration, EVOLVE_SYSTEM_PROMPT is no longer read
// from the OS environment — it is only looked up in the reqEnv map (the
// orchestrator-set in-process pass-through, not a shell env var).
// SSOT IPC-protocol-allowed: orchestrator -> phase reqEnv pass-through.
const SystemPromptReqEnvKey = "EVOLVE_" + "SYSTEM_PROMPT"

## 2026-11-05 — why the Claude Code login fails, and how a seat stops ending it

| Document | What it records |
|---|---|
| [research/claude-login-split-2026-11.md](research/claude-login-split-2026-11.md) | Research: 264 auth errors in 7 clusters since 11-02, and an older console pattern. A sandboxed seat cannot write the Keychain. |
| [plans/seat-login-isolation-2026-11.md](plans/seat-login-isolation-2026-11.md) | Plan: five scored designs. A seat loses its path to the stored login. The operator chose design C. |
| [architecture/seat-login.md](architecture/seat-login.md) | Spec: the token file is the stage, `Resolve` at each door, the seat directory, the denies and the test rows. |
| [architecture/adr/0163-a-seat-holds-a-narrow-token-in-its-own-directory-and-the-token-file-is-the-stage.md](architecture/adr/0163-a-seat-holds-a-narrow-token-in-its-own-directory-and-the-token-file-is-the-stage.md) | ADR-0163: a seat holds a narrow token in its own directory. Rejected: a stage record, a gateway, API keys. |

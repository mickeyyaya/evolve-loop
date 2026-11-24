# Ollama Runtime

> How the pipeline reaches local models through Ollama. Ollama is a routing target for local models. It has no skill or plugin surface of its own.

## Driver

The ollama tmux-REPL bridge driver (`go/internal/bridge/driver_ollamatmux.go`) drives Ollama. The driver starts `ollama run <model>` in a tmux pane. It reuses the shared REPL completion-detection machinery.

## Instructions

Ollama reads no instruction file. The manifest `ollama-tmux` sets `instruction_file` to `inject`. The bridge puts the L0 rules and the pointer line at the start of each prompt, above the cycle context. The text is the same in each cycle, so the prompt cache holds it.

## Limits

Plain `ollama run` has no tool use. The bridge refuses to give it a phase that writes source files. It runs reasoning and review phases.

## See also

- [docs/research/ollama-control-surface-2026.md](../../../docs/research/ollama-control-surface-2026.md): the research record.
- [generic-runtime.md](generic-runtime.md): a CLI with no runtime file of its own.

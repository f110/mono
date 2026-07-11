---
description: Load Build (in-house CI tool) project context, then wait for the task instruction
---

# Goal

Prime the context for the **Build** project (the in-house CI tool) so the *next* message can be a task instruction against an already-loaded context. This is stage 1 of a two-stage flow: load context now, receive the actual work instruction in the following message.

# Steps

1. Read `docs/build.md` in full. This is the cross-cutting architecture doc covering `go/build` (backend), `ts/apps/build` (frontend), and `proto/build` (schema).

2. Do **not** start any work: don't read source files, run builds, or make changes yet.

3. Confirm briefly (one or two lines) that the Build context is loaded and you're ready, then stop and wait for the task in the next message.

# Notes

- Keep stage 1 minimal: only `docs/build.md`. Pull in specific source under `go/build`, `proto/build`, or `ts/apps/build` on demand once the task arrives.
- `docs/build.md` is the single source of truth; don't copy its content elsewhere.

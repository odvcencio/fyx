# Contributing to Fyx

Fyx is trying to be a first-class language layer for Fyrox, not a toy transpiler. Changes should keep that bar in mind: preserve generated Rust quality, keep diagnostics honest, and avoid syntax that only works in demos.

## Development Loop

```bash
go test ./...
cd runtime && cargo test
go run ./cmd/fyxc check testdata --cargo-check
```

`fyxc check --cargo-check` is part of the contract. If a compiler change regresses generated Rust validity, it is not done yet.

## Repo Shape

- `grammar/`: Fyx grammar definitions.
- `ast/`: CST-to-AST extraction and source-preserving parsing helpers.
- `transpiler/`: Rust codegen, source maps, and sugar rewrites.
- `cmd/fyxc/`: build/check CLI and validation harness.
- `runtime/`: small Rust runtime for ECS-side support.
- `queries/` and `editors/`: editor-facing highlighting assets.
- `testdata/`: fixtures and golden outputs.

## Contribution Rules

- Keep `.fyx` syntax additions source-mapped and test-covered.
- Add or update golden files when output shape changes.
- Prefer generated Rust that looks hand-written and debuggable.
- Preserve escape hatches. Top-level Rust passthrough is a feature, not a loophole.
- For roadmap features that depend on upstream Fyrox changes, land the in-repo seam cleanly and document the external dependency explicitly.

## Branching

- Default branch is `main`.
- CI runs on pushes to `main` and on pull requests.

## Still Needs Maintainer Choice

- The repo does not yet declare an open-source license. That needs an explicit maintainer decision rather than an accidental default.

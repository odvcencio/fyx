# Fyx Architecture

Fyx is a source-preserving language layer over Rust, tuned for Fyrox-style gameplay code.

## Pipeline

1. `.fyx` source is parsed with the grammar in `grammar/`.
2. `ast.BuildAST` extracts a source-aware AST and preserves line information.
3. `transpiler/` rewrites Fyx sugar into ordinary Rust:
   - script lifecycle handlers
   - signals and targeted emits
   - reactive/derived/watch plumbing
   - Fyx ECS queries, despawn, and script-side `ecs.spawn(...)`
   - node/resource resolution
   - top-level import/module wiring
   - rust-only helper modules preserved as authored Rust
4. `fyxc build` writes:
   - generated `.rs`
   - `.fyxmap.json` line maps
   - `.arb` sidecars for preserved Arbiter declarations
   - with `--watch`, rebuilds or re-checks when `.fyx` files change
5. `fyxc check --cargo-check` validates generated Rust in a synthetic harness and maps compiler diagnostics back to `.fyx` lines.

## Design Constraints

- Generated Rust should remain readable and cargo-native.
- Fyx sugar must lower deterministically to explicit Rust, not hidden runtime magic.
- Errors should point back to authoring lines, not just generated code.
- Cross-file projects must compile as normal Rust module trees.
- A `.fyx` file that is pure Rust helper code should remain a first-class module, not a special case.

## Current Boundaries

### In repo today

- Script authoring surface
- Signal/reaction model
- Fyx ECS runtime crate with sparse-set storage
- Source-mapped validation
- CLI watch loop for rebuild/check on `.fyx` changes
- Preserved Arbiter declarations and sidecar emission
- Basic editor assets

### External or upstream work

- `fyrox-template --lang fyx`
- Fyroxed-native plugin integration
- LSP
- Full Arbiter runtime execution wiring beyond preserved bundles

## Runtime Split

- `cmd/fyxc/validate.go` contains a synthetic Rust prelude used only for cargo-check validation.
- `runtime/` contains the actual reusable runtime crate surface.

Those two should stay conceptually aligned. If script contexts or ECS APIs change in one place, they should be updated in the other in the same change.

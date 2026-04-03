# Fyx

Fyx is a ferrous scripting language for Fyrox.

It keeps the Rust escape hatch fully open, but adds engine-native authoring sugar for scripts, signals, reactive state, and ECS. `.fyx` files transpile to ordinary Rust modules, so the output still flows through the normal cargo toolchain instead of hiding inside a bespoke runtime.

## Why it’s interesting

- `script` blocks collapse Fyrox script boilerplate into one unit.
- `signal`, `emit`, and `connect` give you typed message wiring.
- `reactive`, `derived`, and `watch` let UI and gameplay state update with minimal noise.
- `component`, `system`, and `query` add a lightweight ECS lane next to the scene graph.
- Any valid top-level Rust item passes through unchanged.

## Status

Phase 1 is implemented and tested:

- Tree-sitter grammar via `grammargen`
- CST to AST builder
- Full `.fyx` to `.rs` transpiler pipeline
- Script lifecycle handlers
- Signals and targeted emits
- Reactive fields and watches
- ECS components, systems, queries, and despawn
- Node/resource field resolution
- Rust passthrough
- `fyxc build`

Phase 2 work now landed in this repo:

- Multi-file module output with preserved relative paths
- Top-level `import` support for cross-file modules
- Recursive `mod.rs` generation
- Source-map sidecars (`.fyxmap.json`)
- `cargo check` validation of generated Rust through `fyxc check --cargo-check`
- Source-aware diagnostics mapped back to `.fyx` lines
- Script-side `dt` shorthand in update handlers
- Codegen fixes for compile-safe defaults, signal binding, node field rewrites, and ECS single-query destructuring

Still on the roadmap:

- Arbiter integration
- `fyrox-template --lang fyx`
- Editor plugin support in Fyroxed
- Script-to-ECS `ecs.spawn(...)` bridge
- Hot reload/watch mode
- LSP

## Example

```rust
script Weapon {
    inspect fire_rate: f32 = 0.1
    node muzzle: Node = "MuzzlePoint"

    reactive ammo: i32 = 30
    derived can_fire: bool = self.ammo > 0

    signal fired(position: Vector3, direction: Vector3)

    on update(ctx) {
        self.cooldown -= dt;
    }
}
```

## Commands

```bash
go test ./...
cd runtime && cargo test
go run ./cmd/fyxc check testdata --cargo-check
go run ./cmd/fyxc build testdata --out generated
```

## Naming

The public-facing name is `Fyx`.

The codebase still uses a few internal `fyroxscript` identifiers for the grammar package and historical docs. That can be normalized later without changing the external story: Fyx is the language, `fyxc` is the compiler, and `fyx-runtime` is the runtime crate.

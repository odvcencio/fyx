# Fyx

[![CI](https://github.com/odvcencio/fyx/actions/workflows/ci.yml/badge.svg?branch=main)](https://github.com/odvcencio/fyx/actions/workflows/ci.yml)

Fyx is a ferrous scripting language for Fyrox.

It keeps the Rust escape hatch fully open, but adds engine-native authoring sugar for scripts, signals, reactive state, and ECS. `.fyx` files transpile to ordinary Rust modules, so the output still flows through the normal cargo toolchain instead of hiding inside a bespoke runtime.

## Quick Start

```bash
git clone git@github.com:odvcencio/fyx.git
cd fyx
go install github.com/odvcencio/fyx/cmd/fyxc@latest
go test ./...
cd runtime && cargo test && cd ..
go run ./cmd/fyxc check testdata --cargo-check
go run ./cmd/fyxc build testdata --out generated
```

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
- Script-side `ecs.spawn(...)` rewritten into tuple-bundle ECS spawns
- First-pass Arbiter authoring surface: top-level `source` / `worker` / `rule` / `arbiter` blocks are preserved into generated Rust and emitted as `.arb` sidecars
- Editor artifacts: `queries/highlights.scm` plus a minimal VS Code grammar under `editors/vscode/`
- Codegen fixes for compile-safe defaults, signal binding, node field rewrites, and ECS single-query destructuring

Still on the roadmap:

- `fyrox-template --lang fyx`
- Deeper Arbiter runtime wiring beyond preserved bundles
- Native Fyroxed plugin support beyond the shipped highlighting assets
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

```rust
source npc_senses {
    path sensor://vision
}

worker decide_directive {
    input ThreatOutcome
    output NpcDirective
    exec "npc-directive"
}

arbiter npc_brain {
    poll every_frame
    use_worker decide_directive
}

script Spawner {
    on update(ctx) {
        let spawned = ecs.spawn(
            Projectile { damage: 10.0, lifetime: 1.0 },
            Velocity { linear: Vector3::default(), angular: Vector3::default() },
        );
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

`fyxc build` now also writes `.arb` sidecars for any preserved Arbiter declarations alongside the generated `.rs` and `.fyxmap.json` files.

## Project Layout

- `grammar/`: syntax definition
- `ast/`: CST-to-AST extraction and source preservation
- `transpiler/`: Rust lowering and sugar rewrites
- `cmd/fyxc/`: compiler CLI and validation harness
- `runtime/`: Rust runtime crate
- `examples/`: small authoring examples
- `queries/` and `editors/`: editor-facing assets

## Docs

- [Architecture](docs/architecture.md)
- [Contributing](CONTRIBUTING.md)
- [Examples](examples/README.md)

## License

MIT, matching Fyrox.

## Naming

The public-facing name is `Fyx`.

The public surface is `Fyx`, `fyxc`, and `fyx-runtime`. The grammar package now exposes `grammar.FyxGrammar()` and keeps the older `FyroxScriptGrammar()` name only as a compatibility alias.

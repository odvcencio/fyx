<div align="center">
  <a href="https://fyrox.rs/">
    <img src="https://raw.githubusercontent.com/FyroxEngine/Fyrox/master/pics/logo.png" width="112" height="112" alt="Fyrox logo" />
  </a>
  <h1>Fyx</h1>
  <p><strong>A cargo-native scripting language for Fyrox.</strong></p>
</div>

[![CI](https://github.com/odvcencio/fyx/actions/workflows/ci.yml/badge.svg?branch=main)](https://github.com/odvcencio/fyx/actions/workflows/ci.yml)
[![License: MIT](https://img.shields.io/badge/license-MIT-informational)](LICENSE)

Fyx is a scripting language built specifically for the [Fyrox](https://github.com/FyroxEngine/Fyrox) engine. It removes the repetitive glue around scripts, signals, reactive state, node binding, and ECS, without hiding the real engine behind a VM or an invented type system.

`.fyx` files transpile to ordinary Rust modules. Top-level Rust items pass through unchanged. Cargo still checks the result. The goal is simple: convenient authoring on day one, real depth on day one hundred.

## The Standard

The real test for a Fyrox scripting language is not whether it can transpile toy fixtures. It is whether it can take a real Fyrox demo and recreate it as a `.fyx` project.

Fyx now does that.

[`proofs/fyrox_2d/fyx/demo.fyx`](proofs/fyrox_2d/fyx/demo.fyx) recreates Fyrox's official [`examples/2d.rs`](https://github.com/FyroxEngine/Fyrox/blob/master/examples/2d.rs) as a Fyx-authored project. The proof crate compiles the generated Rust against the real engine with `fyrox = 1.0.1`, using a normal Cargo build:

```bash
cargo check --manifest-path proofs/fyrox_2d/Cargo.toml
```

That matters more than any slogan. It means Fyx is already being forced through real Fyrox APIs, real scene construction, real scripts, and real Cargo validation.

## Why This Exists

Fyrox is already a serious engine. A first-class scripting layer for it should feel the same way:

- Rust-native, not sandboxed away from the real engine.
- Friendly for gameplay authoring, not only engine internals.
- Deep enough for real gameplay architecture, not only toy samples.
- Testable and inspectable, not magical.
- Compatible with normal cargo builds, normal Rust errors, and normal source control.

That is the bar for Fyx.

## Not A Toy DSL

Fyx is intentionally two-layered:

- Use the language sugar for the parts of Fyrox gameplay code that are noisy over and over again: script boilerplate, node lookup, signals, reactive state, ECS queries, and script-side spawning.
- Keep ordinary Rust available whenever the sugar stops helping: helper functions, custom structs, impl blocks, imports, multi-file modules, and full handler bodies all remain normal Rust territory.

That matters because it changes what “depth” means:

- A simple `.fyx` file can stay small and ergonomic.
- A large `.fyx` project can grow into multiple modules and Rust helper files without abandoning the language.
- The escape hatch is not a rewrite. It is the same file and the same toolchain.

If you want pure sugar, it is there. If you want to drop into raw Fyrox and Rust in the middle of a gameplay file, that is there too.

## What It Looks Like

Small scripts stay compact:

```rust
script Weapon {
    inspect fire_rate: f32 = 0.1
    inspect damage: f32 = 25.0

    node muzzle: Node = "MuzzlePoint"
    node flash: Light = "MuzzleFlash"

    reactive ammo: i32 = 30
    derived can_fire: bool = self.ammo > 0

    signal fired(position: Vector3, direction: Vector3)
    signal emptied()

    cooldown: f32

    on update(ctx) {
        self.cooldown -= dt;

        if self.cooldown <= 0.0 && self.can_fire {
            emit fired(self.muzzle.global_position(), self.forward());
            self.cooldown = self.fire_rate;
            self.ammo -= 1;
        }

        if self.ammo <= 0 {
            emit emptied();
        }

        self.flash.set_visibility(false);
    }
}
```

Larger gameplay modules can mix imports, raw Rust, multiple scripts, and ECS in one surface:

```rust
import support.helpers

use fyrox::prelude::*;

fn target_visible(scene: &Scene, origin: Vector3, direction: Vector3, range: f32) -> bool {
    scene.physics.raycast(origin, direction, range).next().is_some()
}

script TurretController {
    inspect range: f32 = 18.0
    node muzzle: Node = "Turret/Muzzle"

    reactive heat: f32 = 0.0
    derived overheated: bool = self.heat >= 1.0

    signal fired(origin: Vector3, direction: Vector3)

    plan: ShotPlan

    on update(ctx) {
        self.heat = cool_heat(self.heat, dt);
        self.plan = ShotPlan::for_heat(self.heat);

        let origin = self.muzzle.global_position();
        let direction = aim_direction(&ctx.scene.graph[self.muzzle]);

        if target_visible(&ctx.scene, origin, direction, self.range) {
            emit fired(origin, direction);
            let _trail = ecs.spawn(
                HeatTrail { heat: self.heat, ttl: 0.5 },
                ShotOwner { node: self.muzzle },
            );
        }
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

rule PlayerDetected {
    when distance_to_player < 8.0
    then threat_high
}

arbiter npc_brain {
    poll every_frame
    use_worker decide_directive
}

component ThreatMarker {
    active: bool
}

script NpcBrain {
    on update(ctx) {
        let spawned = ecs.spawn(
            ThreatMarker { active: true },
        );
    }
}
```

## What Depth Means In Practice

Fyx is trying to be the scripting language for Fyrox, not a side experiment. Concretely, that means:

- `.fyx` files transpile to ordinary `.rs` modules instead of living inside a bespoke VM.
- The generated output still runs through cargo, so type errors stay real.
- `fyxc check --cargo-check` validates generated Rust and maps diagnostics back to `.fyx` lines.
- Multi-file projects compile as normal Rust module trees with `import` support and generated `mod.rs`.
- Rust-only helper modules are preserved too, so a support file can be pure Rust and still live in the same `.fyx` project tree.
- Script authoring includes node fields, resource fields, lifecycle handlers, signals, reactive state, ECS, and raw Rust passthrough.
- Script-side `ecs.spawn(...)` lowers into tuple-bundle ECS spawns instead of inventing separate runtime semantics.
- Arbiter declarations can live alongside scripts and are preserved into generated Rust plus `.arb` sidecars.

If you outgrow the sugar, you do not leave Fyx. You lean harder on Rust.

## Proof, Not Hype

The current repo tests the pipeline in layers, not only through small examples:

- `116` Go tests across grammar, AST construction, transpiler codegen, CLI validation, and end-to-end fixtures.
- `9` Rust runtime tests in [`runtime/`](runtime/).
- A real proof crate in [`proofs/fyrox_2d/`](proofs/fyrox_2d/) that compiles a Fyx-authored port of Fyrox's 2D demo against the real engine.
- Coverage from the current run:
  - `grammar`: `97.9%`
  - `transpiler`: `92.0%`
  - `ast`: `81.9%`
  - `cmd/fyxc`: `62.5%`

The flagship proof fixture is [`testdata/depth.fyx`](testdata/depth.fyx), backed by [`testdata/support/helpers.fyx`](testdata/support/helpers.fyx). That corpus exercises:

- imports and generated module trees
- rust-only helper modules
- top-level Rust passthrough in authored gameplay files
- lifecycle handlers
- signals and connect-driven message dispatch
- reactive and derived fields with watches
- script-side `ecs.spawn(...)`
- standalone ECS components and systems
- cargo-backed validation of the generated output

There are also targeted tests for:

- Grammar parsing in [`grammar/`](grammar/)
- CST-to-AST extraction in [`ast/`](ast/)
- Golden-file transpilation in [`testdata/golden/`](testdata/golden/)
- Sugar rewrites for scripts, signals, reactive state, and ECS in [`transpiler/`](transpiler/)
- CLI build/check behavior and source-map diagnostics in [`cmd/fyxc/`](cmd/fyxc/)

The practical confidence check is this:

```bash
go test ./...
cd runtime && cargo test && cd ..
go run ./cmd/fyxc check testdata --cargo-check
cargo check --manifest-path proofs/fyrox_2d/Cargo.toml
```

If those pass, the grammar, AST, transpiler, runtime bridge, generated Rust validation, and real-engine proof crate all agree with each other.

For a local smoke run on Linux:

```bash
cargo run --manifest-path proofs/fyrox_2d/Cargo.toml
```

That launches the same ported scene as a normal Fyrox executable.

## Quick Start

```bash
git clone git@github.com:odvcencio/fyx.git
cd fyx
go install github.com/odvcencio/fyx/cmd/fyxc@latest

go test ./...
cd runtime && cargo test && cd ..
go run ./cmd/fyxc check testdata --cargo-check
cargo check --manifest-path proofs/fyrox_2d/Cargo.toml
go run ./cmd/fyxc build testdata --out generated
```

`fyxc build` writes:

- generated Rust modules
- `.fyxmap.json` source-map sidecars
- `.arb` sidecars for preserved Arbiter declarations

## Examples And Ecosystem Fit

Fyx should feel at home next to Fyrox's own examples and demo projects, not like a parallel ecosystem.

- Fyx examples live in [`examples/`](examples/)
- Fyx proof projects live in [`proofs/`](proofs/)
- Official Fyrox examples live in [`Fyrox/examples`](https://github.com/FyroxEngine/Fyrox/tree/master/examples)
- Official Fyrox demo projects are listed at [fyrox.rs/examples.html](https://fyrox.rs/examples.html)
- The Fyrox editor and engine repo provide the visual and workflow standard Fyx is aiming to match

The local examples are intentionally split into two categories:

- Small readable examples in [`examples/`](examples/) for authoring surface and syntax feel:
  - [`examples/weapon/weapon.fyx`](examples/weapon/weapon.fyx)
  - [`examples/npc_brain/brain.fyx`](examples/npc_brain/brain.fyx)
- A larger CI-backed gameplay fixture in [`testdata/depth.fyx`](testdata/depth.fyx) plus [`testdata/support/helpers.fyx`](testdata/support/helpers.fyx) for “can this scale?” proof
- A real-engine proof crate in [`proofs/fyrox_2d/`](proofs/fyrox_2d/) for “can this recreate an actual Fyrox demo?” proof

## Current Status

### Shipped

- Tree-sitter grammar via `grammargen`
- CST-to-AST builder
- Full `.fyx` to `.rs` transpiler pipeline
- Lifecycle handlers
- Signals, emits, and `connect`
- Reactive fields, derived fields, and watches
- ECS components, systems, queries, and despawn
- Script-side `ecs.spawn(...)`
- Node/resource field resolution
- Rust passthrough
- Rust-only helper module preservation
- Multi-file `import` support
- Source maps and mapped cargo diagnostics
- `fyxc build` and `fyxc check`
- First-pass Arbiter preservation and `.arb` sidecars
- Basic editor assets for highlighting
- Real-engine proof crate recreating Fyrox's 2D demo in `.fyx`

### Next To Truly Feel Native

- `fyrox-template --lang fyx`
- Deeper Fyroxed integration beyond shipped highlighting assets
- More flagship Fyrox sample projects authored in Fyx
- Hot reload / watch mode
- LSP
- Deeper Arbiter runtime wiring than preserved bundles

## Project Layout

- `grammar/`: syntax definition
- `ast/`: CST-to-AST extraction and source preservation
- `transpiler/`: Rust lowering and sugar rewrites
- `cmd/fyxc/`: compiler CLI and validation harness
- `runtime/`: Rust runtime crate
- `examples/`: small authoring examples
- `proofs/`: real-engine proof projects
- `queries/` and `editors/`: editor-facing assets

## Docs

- [Architecture](docs/architecture.md)
- [Contributing](CONTRIBUTING.md)
- [Examples](examples/README.md)
- [Proofs](proofs/README.md)
- [Fyrox Book](https://fyrox-book.github.io/)

## License

MIT, matching Fyrox.

## Naming

The public surface is `Fyx`, `fyxc`, and `fyx-runtime`. The grammar package exposes `grammar.FyxGrammar()`. `FyroxScriptGrammar()` remains as a compatibility alias for the initial API.

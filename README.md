<div align="center">
  <a href="https://fyrox.rs/">
    <img src="https://raw.githubusercontent.com/FyroxEngine/Fyrox/master/pics/logo.png" width="112" height="112" alt="Fyrox logo" />
  </a>
  <h1>Fyx</h1>
  <p><strong>A cargo-native scripting language for Fyrox.</strong></p>
</div>

[![CI](https://github.com/odvcencio/fyx/actions/workflows/ci.yml/badge.svg?branch=main)](https://github.com/odvcencio/fyx/actions/workflows/ci.yml)
[![License: MIT](https://img.shields.io/badge/license-MIT-informational)](LICENSE)

Fyx is a scripting language built specifically for the [Fyrox](https://github.com/FyroxEngine/Fyrox) engine. It keeps the Rust escape hatch wide open, but removes a lot of the repetitive glue around scripts, signals, reactive state, and ECS.

The point is not to hide Fyrox behind another runtime. The point is to make gameplay code feel lighter while still compiling down to ordinary Rust modules that fit the engine's normal workflow.

## Why This Exists

Fyrox is already a serious engine. A first-class scripting layer for it should feel the same way:

- Rust-native, not sandboxed away from the real engine.
- Friendly for gameplay authoring, not only engine internals.
- Testable and inspectable, not magical.
- Compatible with normal cargo builds, normal Rust errors, and normal source control.

That is the bar for Fyx.

## What It Looks Like

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

## What Makes It First-Class

Fyx is trying to be the scripting language for Fyrox, not a side experiment. Concretely, that means:

- `.fyx` files transpile to ordinary `.rs` modules instead of living inside a bespoke VM.
- The generated output still runs through cargo, so type errors stay real.
- `fyxc check --cargo-check` validates generated Rust and maps diagnostics back to `.fyx` lines.
- Multi-file projects compile as normal Rust module trees with `import` support and generated `mod.rs`.
- Script authoring includes node fields, resource fields, lifecycle handlers, signals, reactive state, ECS, and raw Rust passthrough.
- Script-side `ecs.spawn(...)` lowers into tuple-bundle ECS spawns instead of inventing separate runtime semantics.
- Arbiter declarations can live alongside scripts and are preserved into generated Rust plus `.arb` sidecars.

## Proof, Not Hype

The current repo already tests the pipeline in layers:

- Grammar parsing in [`grammar/`](grammar/)
- CST-to-AST extraction in [`ast/`](ast/)
- Golden-file transpilation in [`testdata/golden/`](testdata/golden/)
- Sugar rewrites for scripts, signals, reactive state, and ECS in [`transpiler/`](transpiler/)
- CLI build/check behavior and source-map diagnostics in [`cmd/fyxc/`](cmd/fyxc/)
- Runtime ECS behavior in [`runtime/`](runtime/)

The practical confidence check is this:

```bash
go test ./...
cd runtime && cargo test && cd ..
go run ./cmd/fyxc check testdata --cargo-check
```

If those pass, the grammar, AST, transpiler, runtime bridge, and generated Rust validation all agree with each other.

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

`fyxc build` writes:

- generated Rust modules
- `.fyxmap.json` source-map sidecars
- `.arb` sidecars for preserved Arbiter declarations

## Examples And Ecosystem Fit

Fyx should feel at home next to Fyrox's own examples and demo projects, not like a parallel ecosystem.

- Fyx examples live in [`examples/`](examples/)
- Official Fyrox examples live in [`Fyrox/examples`](https://github.com/FyroxEngine/Fyrox/tree/master/examples)
- Official Fyrox demo projects are listed at [fyrox.rs/examples.html](https://fyrox.rs/examples.html)
- The Fyrox editor and engine repo provide the visual and workflow standard Fyx is aiming to match

The current local examples are intentionally small and gameplay-shaped:

- [`examples/weapon/weapon.fyx`](examples/weapon/weapon.fyx)
- [`examples/npc_brain/brain.fyx`](examples/npc_brain/brain.fyx)

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
- Multi-file `import` support
- Source maps and mapped cargo diagnostics
- `fyxc build` and `fyxc check`
- First-pass Arbiter preservation and `.arb` sidecars
- Basic editor assets for highlighting

### Next To Truly Feel Native

- `fyrox-template --lang fyx`
- Deeper Fyroxed integration beyond shipped highlighting assets
- Real flagship Fyrox sample projects authored in Fyx
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
- `queries/` and `editors/`: editor-facing assets

## Docs

- [Architecture](docs/architecture.md)
- [Contributing](CONTRIBUTING.md)
- [Examples](examples/README.md)
- [Fyrox Book](https://fyrox-book.github.io/)

## License

MIT, matching Fyrox.

## Naming

The public surface is `Fyx`, `fyxc`, and `fyx-runtime`. The grammar package exposes `grammar.FyxGrammar()`. `FyroxScriptGrammar()` remains as a compatibility alias for the initial API.

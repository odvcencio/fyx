<div align="center">
  <a href="https://fyrox.rs/">
    <img src="https://raw.githubusercontent.com/FyroxEngine/Fyrox/master/pics/logo.png" width="112" height="112" alt="Fyrox logo" />
  </a>
  <h1>Fyx</h1>
  <p><strong>A cargo-native scripting language for Fyrox.</strong></p>
</div>

[![CI](https://github.com/odvcencio/fyx/actions/workflows/ci.yml/badge.svg?branch=main)](https://github.com/odvcencio/fyx/actions/workflows/ci.yml)
[![License: MIT](https://img.shields.io/badge/license-MIT-informational)](LICENSE)

Fyx is a scripting language built specifically for [Fyrox](https://github.com/FyroxEngine/Fyrox). It removes the repetitive glue around scripts, signals, scene bindings, reactive state, the Fyx ECS surface, and Arbiter-driven gameplay orchestration without hiding the real engine behind a VM or a fake type system.

`.fyx` files transpile to ordinary Rust modules. Cargo still checks the result. Raw Rust can live right beside the sugar. The goal is not to build a parallel ecosystem. The goal is to make Fyrox gameplay authoring dramatically nicer while staying firmly in Rust-world.

## The Bar

The standard for Fyx is not "can it parse some cute syntax." The standard is whether it feels native inside a normal Fyrox workflow:

- create a project
- attach a `.fyx` script from the editor
- see `inspect` fields in the inspector
- bind into real scenes and prefabs without friction
- press play and get errors on the original `.fyx` line

If Fyx nails that loop, it matters more than another pile of language features.

## Proof It Can Carry Real Fyrox Code

Fyx already has a real-engine proof crate.

[`proofs/fyrox_2d/fyx/demo.fyx`](proofs/fyrox_2d/fyx/demo.fyx) recreates Fyrox's official [`examples/2d.rs`](https://github.com/FyroxEngine/Fyrox/blob/master/examples/2d.rs) as a Fyx-authored project. The proof crate compiles the generated Rust against the real engine with a normal Cargo build:

```bash
cargo check --manifest-path proofs/fyrox_2d/Cargo.toml
```

That matters more than any slogan. It means Fyx is already being pushed through real Fyrox APIs, real scene construction, real scripts, and real Cargo validation.

## What Fyx Is Trying To Be

Fyx is aiming to be the most ergonomic way to write Fyrox gameplay code without leaving Rust-world.

- Small at the core, not a giant alternate language.
- Explicit in how it lowers, not magical.
- Friendly for gameplay code, not only engine internals.
- Deep enough for real projects, not only syntax demos.
- Compatible with normal Cargo builds, normal Rust modules, and normal source control.
- Owning scripts, ECS, and Arbiter as one gameplay surface instead of treating ECS like borrowed syntax.
- Inclusive of Arbiter as a native scripting primitive, not a bolt-on.

That last point matters. If the sugar stops helping, the escape hatch is not a rewrite. It is the same project, the same toolchain, and the same Rust underneath.

## Grow With Fyrox

Fyx should grow up alongside Fyrox until it can comfortably carry the full V1 gameplay surface: scripts, scenes, UI wiring, ECS, spawning, and the ordinary Rust modules that real projects accumulate over time.

That does not require turning Fyx into a second universe. It requires staying close to how Fyrox already works and smoothing the parts of gameplay authoring that are repetitive enough to deserve a better surface.

## Keep The Core Small

Fyx should not try to become a giant alternate Rust. The core language should stay focused on the parts of Fyrox gameplay code that are repetitive over and over again: script boilerplate, scene bindings, signals, reactive state, ECS queries, script-side spawning, and Arbiter-authored decision flow.

Everything else should stay honest:

- generated Rust should remain readable
- lowering should stay deterministic
- raw Rust should remain a first-class escape hatch

Higher-level batteries can live above the core language instead of bloating it.

That keeps the language sharp without turning it into soup.

## Arbiter Is Native

Arbiter belongs in the Fyx surface beside `script`, `component`, and `system`.

It is not meant to live off to the side as an optional side language. It should author cleanly in `.fyx`, travel through the same cargo-native build flow, map diagnostics back to authored lines, and integrate as deeply as the rest of the gameplay surface.

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

Larger gameplay modules can still mix imports, raw Rust, scripts, and ECS in one surface:

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

Arbiter-oriented authoring lives in that same surface:

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
```

## Current Status

Shipped in repo today:

- lifecycle handlers
- signals, `emit`, and `connect`
- reactive fields, derived fields, and `watch`
- node and resource bindings
- Fyx ECS components, systems, queries, despawn, and script-side `ecs.spawn(...)`
- raw Rust passthrough
- multi-file `import` support and Rust helper modules
- source maps and mapped Cargo diagnostics
- `fyxc build` and `fyxc check`
- basic editor assets
- a real-engine proof crate recreating Fyrox's 2D demo in `.fyx`
- Arbiter-authored declarations with generated bundle emission and `.arb` sidecars

## Examples And Proofs

The repo is intentionally split into a few different kinds of proof:

- Small readable examples in [`examples/`](examples/) for syntax and authoring feel.
- A larger gameplay fixture in [`testdata/depth.fyx`](testdata/depth.fyx) plus [`testdata/support/helpers.fyx`](testdata/support/helpers.fyx) for multi-file depth.
- A real-engine proof crate in [`proofs/fyrox_2d/`](proofs/fyrox_2d/) for "can this recreate an actual Fyrox example?" proof.

## Quick Start

```bash
git clone git@github.com:odvcencio/fyx.git
cd fyx
go install github.com/odvcencio/fyx/cmd/fyxc@latest

fyxc check testdata --cargo-check
fyxc build testdata --out generated
cargo check --manifest-path proofs/fyrox_2d/Cargo.toml
```

`fyxc build` writes:

- generated Rust modules
- `.fyxmap.json` source-map sidecars
- `.arb` sidecars for Arbiter-authored bundles

## Project Layout

- `grammar/`: language surface
- `ast/`: source-aware extraction and preservation
- `transpiler/`: lowering into Rust
- `cmd/fyxc/`: CLI build and check commands
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

The public surface is `Fyx`, `fyxc`, and `fyx-runtime`.

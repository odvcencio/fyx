# Fyrox 2D In Fyx

This proof crate recreates Fyrox's official [`examples/2d.rs`](https://github.com/FyroxEngine/Fyrox/blob/master/examples/2d.rs) as a `.fyx`-authored project.

The point is not to show that Fyx can transpile a toy script. The point is to force it through a real Fyrox example and make Cargo accept the result.

## What this proves

- A `.fyx` file can author a real Fyrox plugin and scene setup.
- Generated Rust from `fyxc` compiles against the real engine, not only the synthetic validation harness.
- Script authoring features like lifecycle handlers, `self.node`, and `dt` can live inside a normal Fyrox application.

## How it works

`cargo check --manifest-path proofs/fyrox_2d/Cargo.toml` runs the real pipeline:

1. `build.rs` invokes `fyxc build` on [`fyx/demo.fyx`](./fyx/demo.fyx)
2. `fyxc` generates Rust into Cargo's `OUT_DIR`
3. the generated Rust is compiled against `fyrox = 1.0.1`

To launch the port locally:

```bash
cargo run --manifest-path proofs/fyrox_2d/Cargo.toml
```

The crate includes the official demo texture asset in [`data/Crate.png`](./data/Crate.png).

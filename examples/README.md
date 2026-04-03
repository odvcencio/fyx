# Examples

These examples are intentionally small and readable. They are meant to show the authoring surface, not to hide it behind massive fixture files.

- `weapon/weapon.fyx`: script, signals, reactive state, and node fields.
- `npc_brain/brain.fyx`: preserved Arbiter declarations plus script-side ECS spawning.

For the larger CI-backed gameplay sample that proves module imports, raw Rust passthrough, lifecycle handlers, reactive state, message wiring, and ECS together, see:

- `../testdata/depth.fyx`
- `../testdata/support/helpers.fyx`

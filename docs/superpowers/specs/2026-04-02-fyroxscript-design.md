# FyroxScript Language Design

A scripting language for the Fyrox game engine that extends Rust with game-specific syntax. FyroxScript transpiles to idiomatic Fyrox Rust — every valid Rust program is a valid FyroxScript program.

## Design Principles

1. **More Fyrox-y** — Syntax mirrors Fyrox's own vocabulary: `inspect` for editor-visible fields, `node` for scene graph handles, `resource` for asset references.
2. **More nicey** — Common patterns collapse to one line. The ten-line boilerplate becomes a keyword.
3. **Rust is the escape hatch** — Any valid Rust passes through the transpiler unchanged. Zero ceiling on capability.
4. **Work with Fyrox, not against it** — The compiler emits standard Rust that flows through cargo, the editor, and hot-reload without modification.

## Compilation Model

```
.fyx source → grammargen parser → FyroxScript AST → transpiler → .rs files → cargo build
```

### Pipeline

FyroxScript adds a pre-compilation step to the standard Fyrox build. The transpiler reads `.fyx` files from `game/src/` and emits `.rs` files into `game/src/generated/`. These generated files join the cargo workspace like any other Rust source.

### Integration with Fyrox Tooling

- **fyrox-template** gains `--lang fyrox-script` for scaffolding `.fyx` projects.
- **Hot-reload** works unchanged: `.fyx` edit → transpile → `cargo build` dylib → editor reloads.
- **Plugin registration** is automatic — the transpiler scans all `script` declarations and generates `Plugin::register()`.
- **UUIDs** are deterministic — derived from script name and module path, stored in `.fyx.meta` sidecars. Stable across rebuilds.

### Grammar

The FyroxScript grammar extends the Rust grammar (already at full parity in gotreesitter's grammargen). New productions add `script`, `signal`, `emit`, `connect`, `component`, `system`, `query`, `inspect`, `node`, `resource`, `reactive`, `derived`, `watch`, `spawn`, and `nodes` as context-sensitive keywords. They are identifiers everywhere except at declaration position.

## Scripts

A `script` block declares a Fyrox script: struct fields, lifecycle handlers, signals, and reactive state in one unit.

### Declaration

```rust
script Player {
    inspect speed: f32 = 10.0
    inspect jump_force: f32 = 5.0
    node camera: Camera3D = "Camera3D"
    resource footstep: SoundBuffer = "res://audio/footstep.wav"
    
    move_dir: Vector3  // internal state — hidden from editor, not serialized

    on init(ctx) { }
    on start(ctx) { }
    on update(ctx) { }
    on deinit(ctx) { }
    on event(ev: KeyboardInput, ctx) { }
    on message(msg: DamageMsg, ctx) { }
}
```

### Field Modifiers

| Modifier | Meaning | Transpiles to |
|----------|---------|---------------|
| `inspect` | Visible and editable in the Fyrox Inspector panel | `pub` + `#[reflect(expand)]` |
| `node` | A typed `Handle<Node>` resolved from the scene graph by path | `Handle<Node>` resolved in `on_start` |
| `nodes` | Multiple node handles matching a wildcard pattern | `Vec<Handle<Node>>` populated in `on_start` |
| `resource` | An asset handle loaded via Fyrox's `ResourceManager` | Resource handle field + load in `on_start` |
| *(bare)* | Internal state — hidden from editor, not serialized | `#[reflect(hidden)]` + `#[visit(skip)]` |

### Lifecycle Handlers

Each `on` block maps directly to a `ScriptTrait` method:

| Handler | ScriptTrait method | Called when |
|---------|-------------------|-------------|
| `on init(ctx)` | `on_init` | Script first spawns (skipped for deserialized) |
| `on start(ctx)` | `on_start` | All scripts initialized |
| `on update(ctx)` | `on_update` | Every frame (default 60 Hz) |
| `on deinit(ctx)` | `on_deinit` | Script or node destroyed |
| `on event(ev: T, ctx)` | `on_os_event` | OS event matching type `T` |
| `on message(msg: T, ctx)` | `on_message` | Message matching type `T` received |

The `ctx` parameter is always `&mut ScriptContext`. Its type is inferred — never written out.

### Self-Node Shortcuts

Inside a script, `self.node` refers to the node the script is attached to. Common operations lift to the script:

```rust
self.position()     // global position of the node
self.forward()      // forward direction vector
self.parent()       // parent node handle
self.node.rotate_y(angle)
```

### Transpiled Output

A `script Player` block generates:

1. A struct with `#[derive(Visit, Reflect, Default, Debug, Clone, TypeUuidProvider, ComponentProvider)]`
2. A `#[type_uuid(id = "...")]` attribute with a deterministic UUID
3. A `Default` impl incorporating field initializers
4. A `ScriptTrait` impl with each `on` block as the corresponding method
5. A `Plugin::register()` entry: `context.serialization_context.script_constructors.add::<Player>("Player")`

### Scene Manipulation

```rust
// spawn a prefab as a child node
let goblin = spawn self.prefab at Vector3::new(0.0, 1.0, 0.0)

// configure the spawned instance
goblin.script::<Enemy>().health = 50.0
```

`spawn` transpiles to `resource_manager.request::<Model>(path).instantiate()` with position set on the result.

## Signals

FyroxScript provides two signal systems for different purposes.

### Event Signals

Event signals announce that something happened. They decouple the sender from its listeners.

**Declare and emit:**

```rust
script Enemy {
    inspect health: f32 = 100.0
    
    signal died(position: Vector3)
    signal damaged(amount: f32, source: Handle<Node>)
    
    on update(ctx) {
        if self.health <= 0.0 {
            emit died(self.position())
        }
    }
}
```

**Connect and respond:**

```rust
script ScoreTracker {
    inspect score: i32 = 0
    
    connect Enemy::died(pos) {
        self.score += 100
        spawn_particles(pos)
    }
}
```

**Targeted emit:** To send a signal to a specific node rather than broadcasting, use `emit ... to`:

```rust
emit Health::damaged(amount: 10.0, source: ctx.handle) to target_node
```

This transpiles to `ctx.message_sender.send_to_target(target_node, ...)` — a direct message to one node. The target must have a `connect` block for the signal, or handle it via `on message`. This form is available in both scripts and ECS systems.

**Transpilation:** `signal` becomes a message struct. `emit` becomes `ctx.message_sender.send()` (broadcast) or `ctx.message_sender.send_to_target()` (targeted). `connect` becomes `subscribe_to::<T>()` in `on_start` and a dispatch branch in `on_message`. All compile-time — the type checker verifies signal names and argument types.

### Reactive Signals

Reactive signals track values and propagate changes through a dependency chain.

```rust
script PlayerHUD {
    node health_bar: ProgressBar = "UI/HealthBar"
    
    reactive health: f32 = 100.0
    derived health_pct: f32 = self.health / 100.0
    derived is_critical: bool = self.health < 20.0
    
    watch self.is_critical {
        if self.is_critical {
            self.health_bar.set_color(Color::RED)
        }
    }
    
    watch self.health_pct {
        self.health_bar.set_value(self.health_pct)
    }
}
```

| Keyword | Purpose | Transpiles to |
|---------|---------|---------------|
| `reactive` | A value whose changes propagate to dependents | Field + shadow `_prev` field for dirty-tracking |
| `derived` | Recomputes when its dependencies change | Conditional recompute in `on_update` |
| `watch` | Runs a side-effect block when a value changes | Conditional execution in `on_update` |

`watch` blocks observe `self` fields only. For cross-script data flow, use event signals — the emitter broadcasts a value change, and any listener's `connect` block responds. This keeps the reactive system local to one script and avoids hidden cross-script coupling.

**Transpilation:** The transpiler analyzes dependency chains at compile time. No runtime dependency tracking. `reactive` fields get shadow `_prev` fields; `derived` fields recompute when dependencies are dirty; `watch` blocks execute conditionally. All generated code runs inside `on_update`.

### When to Use Which

| | Event signals | Reactive signals |
|---|---|---|
| **Purpose** | "something happened" | "this value changed" |
| **Direction** | broadcast to subscribers | push through dependency chain |
| **Coupling** | loose — emitter ignores listeners | tight — explicit dependency graph |
| **Use case** | game events, inter-script communication | UI binding, derived state, property chains |
| **Runtime cost** | Fyrox message dispatch | dirty-check per frame |

## ECS Layer

FyroxScript provides an Entity Component System that runs alongside the scene graph. The ECS handles high-volume, data-oriented workloads: thousands of bullets, flocking agents, spatial queries.

### Components

```rust
component Velocity {
    linear: Vector3
    angular: Vector3
}

component Projectile {
    damage: f32
    owner: Handle<Node>
    lifetime: f32
}
```

Components are plain data. No derives, no UUIDs, no lifecycle hooks.

### Systems

```rust
system move_projectiles(dt: f32) {
    query(pos: &mut Transform, vel: &Velocity) {
        pos.translate(vel.linear * dt)
    }
}

system expire_projectiles(dt: f32) {
    query(entity: Entity, proj: &mut Projectile) {
        proj.lifetime -= dt
        if proj.lifetime <= 0.0 {
            despawn(entity)
        }
    }
}
```

Systems run every frame from `Plugin::update()` in declaration order within a file. Cross-file order follows alphabetical file name. Each `query` iterates all entities matching the requested component types.

System-level parameters (e.g., `dt: f32`) are injected values provided by the runtime, not components. `dt` is always delta time. The `query` parameter list contains only component references and the optional `entity: Entity` handle.

### Scene Graph Bridge

Scripts spawn ECS entities. ECS systems emit signals to scene graph nodes.

**Script to ECS:**

```rust
script Weapon {
    on event(ev: MouseButton::Left, ctx) {
        ecs.spawn(
            Projectile { damage: 25.0, owner: ctx.handle, lifetime: 2.0 },
            Velocity { linear: self.forward() * 200.0, angular: Vector3::ZERO },
            Transform::from(self.position()),
        )
    }
}
```

**ECS to script:**

```rust
system projectile_hits {
    query(entity: Entity, pos: &Transform, vel: &Velocity, proj: &Projectile) {
        for hit in scene.physics.raycast(pos.position(), vel.linear.normalized(), 0.5) {
            emit Health::damaged(amount: proj.damage, source: proj.owner) to hit.node
            despawn(entity)
        }
    }
}
```

### When to Use Which

| | Scene Graph Scripts | ECS |
|---|---|---|
| **Count** | tens to hundreds | thousands to millions |
| **Identity** | named nodes in the editor | anonymous entities |
| **Editor visibility** | inspector, scene tree | not in scene tree |
| **Lifecycle** | init / start / update / deinit | spawn / despawn |
| **Communication** | signals, node paths | queries, batch processing |
| **Use case** | player, enemies, doors, UI | bullets, particles, AI agents |

### Transpilation

- `component` → plain Rust struct (`Clone` only)
- `system` → a function called from `Plugin::update()`
- `query` → typed iteration over component storage
- `ecs.spawn` / `despawn` → arena insert / remove
- `scene` in systems → access via `PluginContext`

The ECS storage (sparse set or archetype) is an implementation detail internal to the generated runtime. No external ECS crate required.

## Type System

FyroxScript uses static types with local inference. Types must appear on:

- Script fields
- Function parameters and return types

Types may be omitted on local variables — the transpiler infers them from context, matching Rust's own `let` inference:

```rust
let speed = 10.0          // f32
let name = "Player"       // &str
let pos = self.position() // Vector3
```

Since FyroxScript transpiles to Rust, every FyroxScript type maps to a Rust type. No `Variant`, no dynamic dispatch, no runtime type checks.

## Syntax Summary

FyroxScript extends Rust's grammar with context-sensitive keywords. These are identifiers in all positions except their declaration context — existing Rust code that uses `inspect` or `signal` as variable names compiles without conflict.

| Keyword | Context | Purpose |
|---------|---------|---------|
| `script` | top-level | Declare a Fyrox script |
| `inspect` | script field | Editor-visible field |
| `node` | script field | Typed scene graph handle |
| `nodes` | script field | Multiple scene graph handles (wildcard) |
| `resource` | script field | Asset handle |
| `reactive` | script field | Change-tracked value |
| `derived` | script field | Auto-recomputed value |
| `signal` | script body | Event signal declaration |
| `emit` | any block | Emit an event signal |
| `connect` | script body | Subscribe to an event signal |
| `watch` | script body | React to a value change |
| `component` | top-level | ECS component (plain data) |
| `system` | top-level | ECS system (runs each frame) |
| `query` | system body | Iterate matching ECS entities |
| `spawn` | any block | Instantiate a prefab or ECS entity |
| `despawn` | system body | Remove an ECS entity |
| `on` | script body | Lifecycle handler |

## Complete Example

A weapon system demonstrating scripts, signals, ECS, and scene graph access working together:

```rust
// weapon.fyx

script Weapon {
    inspect damage: f32 = 25.0
    inspect fire_rate: f32 = 0.1
    
    node muzzle: Node = "MuzzlePoint"
    node flash: Light = "MuzzleFlash"
    resource fire_sound: SoundBuffer = "res://audio/rifle_fire.wav"
    
    reactive ammo: i32 = 30
    derived can_fire: bool = self.ammo > 0
    derived ammo_display: String = format!("{} / 30", self.ammo)
    
    signal fired(position: Vector3, direction: Vector3)
    signal emptied
    
    cooldown: f32

    on update(ctx) {
        self.cooldown -= ctx.dt
        self.flash.set_visibility(false)
    }

    on event(ev: MouseButton::Left, ctx) {
        if !self.can_fire || self.cooldown > 0.0 {
            return
        }

        self.ammo -= 1
        self.cooldown = self.fire_rate
        self.flash.set_visibility(true)

        ecs.spawn(
            Projectile { damage: self.damage, owner: ctx.handle, lifetime: 2.0 },
            Velocity { linear: self.forward() * 200.0, angular: Vector3::ZERO },
            Transform::from(self.muzzle.global_position()),
        )

        emit fired(self.muzzle.global_position(), self.forward())

        if self.ammo <= 0 {
            emit emptied
        }
    }
}

// hud.fyx

script WeaponHUD {
    node ammo_label: Text = "UI/AmmoCount"
    node crosshair: Sprite = "UI/Crosshair"

    connect Weapon::fired(pos, dir) {
        self.crosshair.animate("kick")
    }

    connect Weapon::emptied {
        self.ammo_label.set_color(Color::RED)
    }

    connect Weapon::fired(pos, dir) {
        // update ammo display via the weapon script on the same node or parent
        if let Some(weapon) = self.parent().script::<Weapon>() {
            self.ammo_label.set_text(weapon.ammo_display)
        }
    }
}

// projectiles.fyx

component Projectile {
    damage: f32
    owner: Handle<Node>
    lifetime: f32
}

component Velocity {
    linear: Vector3
    angular: Vector3
}

system move_projectiles(dt: f32) {
    query(pos: &mut Transform, vel: &Velocity) {
        pos.translate(vel.linear * dt)
    }
}

system expire_projectiles(dt: f32) {
    query(entity: Entity, proj: &mut Projectile) {
        proj.lifetime -= dt
        if proj.lifetime <= 0.0 {
            despawn(entity)
        }
    }
}

system projectile_hits {
    query(entity: Entity, pos: &Transform, vel: &Velocity, proj: &Projectile) {
        for hit in scene.physics.raycast(pos.position(), vel.linear.normalized(), 0.5) {
            emit Health::damaged(amount: proj.damage, source: proj.owner) to hit.node
            despawn(entity)
        }
    }
}
```

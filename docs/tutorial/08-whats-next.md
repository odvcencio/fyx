# What's Next

You have written a script, moved a character, handled input, used states, and connected scripts with signals. Here is what else Fyx can do.

## Reactive and Derived Fields

Mark a field `reactive` and Fyx tracks changes to it automatically. Combine that with `derived` to compute values that stay in sync, and `watch` to run code when they change:

```fyx
script HUD {
    reactive health: f32 = 100.0
    derived is_critical: bool = self.health < 20.0

    watch self.is_critical {
        println!("critical!");
    }
}
```

When `health` drops below 20, `is_critical` becomes true and the `watch` block runs. You do not need to check it yourself every frame.

## Timer Fields

Use `timer` to create cooldowns without tracking elapsed time manually:

```fyx
script Weapon {
    inspect fire_rate: f32 = 0.1

    timer fire_cooldown = self.fire_rate

    on update(ctx) {
        if fire_cooldown.ready {
            fire_cooldown.reset();
        }
    }
}
```

The timer counts down automatically. Check `.ready` and call `.reset()` when you use it.

## ECS: Components, Systems, and Queries

When you have lots of similar entities -- bullets, particles, pickups -- the ECS (Entity Component System) is more efficient than one script per object. Fyx has a built-in ECS surface:

```fyx
component Velocity {
    linear: Vector3
    angular: Vector3
}

component Projectile {
    damage: f32
    lifetime: f32
}

system move_things(dt: f32) {
    query(pos: &mut Transform, vel: &Velocity) {
        pos.translate(vel.linear * dt);
    }
}

system expire(dt: f32) {
    query(entity: Entity, proj: &mut Projectile) {
        proj.lifetime -= dt;
        if proj.lifetime <= 0.0 {
            despawn(entity);
        }
    }
}
```

Components hold data. Systems contain queries that run on every entity with matching components.

## Spawning Entities

Inside a script, use `ecs.spawn(...)` to create ECS entities at runtime:

```fyx
script Weapon {
    inspect damage: f32 = 25.0

    node muzzle: Node = "MuzzlePoint"

    signal fired(position: Vector3, direction: Vector3)

    on update(ctx) {
        let origin = self.muzzle.position();
        let direction = self.muzzle.forward();
        let _bullet = ecs.spawn(
            Projectile { damage: self.damage, lifetime: 2.0 },
            Velocity { linear: direction * 24.0, angular: Vector3::default() },
        );
        emit fired(origin, direction);
    }
}
```

## Raw Rust

When you need something Fyx does not cover, you can write raw Rust in the same file. Fyx generates Rust, so there is no wall between them. A `use` import or a plain `fn` at the top level passes straight through to the generated output:

```fyx
use fyrox::prelude::*;

fn clamp_health(value: f32) -> f32 {
    value.clamp(0.0, 100.0)
}

script Player {
    inspect health: f32 = 100.0

    on update(ctx) {
        self.health = clamp_health(self.health);
    }
}
```

This is the escape hatch. When Fyx's sugar does not reach far enough, drop into Rust for that one piece and keep everything else in Fyx.

## Where to Go From Here

- [README](../../README.md) -- project overview and quick start
- [examples/](../../examples/) -- small, focused example scripts
- [proofs/](../../proofs/) -- real Fyrox projects built with Fyx
- [Fyrox Book](https://fyrox-book.github.io/) -- the engine's own documentation

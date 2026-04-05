package lsp

// hoverDocs maps Fyx keywords to beginner-friendly markdown documentation.
var hoverDocs = map[string]string{

	"script": `## script

A **script** is the main building block in Fyx. It attaches behavior to a game object (node) in your scene. Think of it as a class that controls one thing in your game -- a player, an enemy, a door.

` + "```fyx" + `
script Player {
    inspect speed: f32 = 5.0

    on update(ctx) {
        // move the player every frame
    }
}
` + "```" + `

Every script can have fields (data), handlers (behavior), signals (events), and states (state machines).`,

	"inspect": `## inspect

An **inspect** field is visible and editable in the Fyrox editor's Inspector panel. Use it for values you want designers to tweak without touching code -- speed, health, damage, colors.

` + "```fyx" + `
script Enemy {
    inspect health: f32 = 100.0
    inspect speed: f32 = 3.0
    inspect color: Color = Color::RED
}
` + "```" + `

The type and default value are required. Supported types include ` + "`f32`" + `, ` + "`i32`" + `, ` + "`bool`" + `, ` + "`String`" + `, ` + "`Vector3`" + `, and any Fyrox type.`,

	"node": `## node

A **node** field holds a reference to a single scene node by its path in the scene tree. The engine resolves the path at runtime.

` + "```fyx" + `
script Weapon {
    node muzzle: Node = "MuzzlePoint"
    node flash: Light = "MuzzleFlash"
}
` + "```" + `

The path must be a quoted string. The type (` + "`Node`" + `, ` + "`Light`" + `, ` + "`Camera`" + `, etc.) tells Fyrox what kind of node to expect.`,

	"nodes": `## nodes

A **nodes** field holds a collection of scene nodes matching a path pattern. Use it when you need to reference multiple children.

` + "```fyx" + `
script Spawner {
    nodes spawn_points: Node = "SpawnPoint*"
}
` + "```" + `

Works like ` + "`node`" + ` but returns all matching nodes instead of one.`,

	"resource": `## resource

A **resource** field loads an external asset (sound, model, texture) by its resource path. Fyrox loads and caches it automatically.

` + "```fyx" + `
script Weapon {
    resource fire_sound: SoundBuffer = "res://audio/rifle_fire.wav"
    resource projectile: Model = "res://models/bullet.rgs"
}
` + "```" + `

Resource paths start with ` + "`res://`" + ` and point to files in your project.`,

	"timer": `## timer

A **timer** field creates a countdown timer. It ticks down each frame and you can check ` + "`.ready`" + ` to see if it has elapsed, then call ` + "`.reset()`" + ` to restart it.

` + "```fyx" + `
script Weapon {
    timer fire_cooldown = 0.5

    on update(ctx) {
        if fire_cooldown.ready {
            // fire!
            fire_cooldown.reset();
        }
    }
}
` + "```" + `

The default value is the duration in seconds (f32).`,

	"reactive": `## reactive

A **reactive** field automatically triggers updates when its value changes. Any ` + "`derived`" + ` field or ` + "`watch`" + ` block that depends on it will re-evaluate.

` + "```fyx" + `
script Player {
    reactive health: i32 = 100

    watch self.health {
        if self.health <= 0 {
            // handle death
        }
    }
}
` + "```" + `

Use reactive fields for values that drive UI or trigger side effects when they change.`,

	"derived": `## derived

A **derived** field is a computed value that updates automatically when its reactive dependencies change. You never assign to it directly -- it recalculates itself.

` + "```fyx" + `
script Player {
    reactive health: i32 = 100
    derived is_alive: bool = self.health > 0
    derived health_pct: f32 = self.health as f32 / 100.0
}
` + "```" + `

Derived fields are read-only. They are perfect for UI bindings and conditional logic.`,

	"signal": `## signal

A **signal** declares a named event that this script can broadcast. Other scripts can listen using ` + "`connect`" + `.

` + "```fyx" + `
script Weapon {
    signal fired(position: Vector3, direction: Vector3)
    signal emptied()
}
` + "```" + `

Signals are like custom events. They decouple scripts so they don't need to know about each other directly.`,

	"emit": `## emit

**emit** fires a signal, notifying all connected listeners. Use it inside handlers to broadcast events.

` + "```fyx" + `
script Weapon {
    signal fired(position: Vector3)

    on update(ctx) {
        emit fired(self.position());
    }
}
` + "```" + `

The arguments must match the signal's declared parameter types.`,

	"connect": `## connect

**connect** listens for a signal from another script. When that signal fires, the code block runs.

` + "```fyx" + `
script HUD {
    connect Weapon::fired(pos) {
        // show muzzle flash effect at pos
    }

    connect Player::health_changed(hp) {
        // update health bar
    }
}
` + "```" + `

The format is ` + "`connect ScriptName::signal_name(params) { ... }`" + `. The parameter names bind to the emitted values.`,

	"watch": `## watch

**watch** runs a code block whenever a reactive field changes value.

` + "```fyx" + `
script Player {
    reactive ammo: i32 = 30

    watch self.ammo {
        if self.ammo <= 5 {
            // show low ammo warning
        }
    }
}
` + "```" + `

Watch blocks only trigger on actual value changes, not every frame. The watched field must be ` + "`reactive`" + `.`,

	"state": `## state

A **state** defines a named state in a finite state machine. Each state can have ` + "`on enter`" + `, ` + "`on update`" + `, and ` + "`on exit`" + ` handlers.

` + "```fyx" + `
script Enemy {
    state idle {
        on enter {
            // play idle animation
        }
        on update {
            if self.sees_player() {
                go chase;
            }
        }
    }

    state chase {
        on enter {
            // play run animation
        }
        on update {
            self.move_toward(player);
        }
    }
}
` + "```" + `

Use ` + "`go state_name;`" + ` to transition between states.`,

	"component": `## component

A **component** declares a custom ECS data type. Components hold data, not behavior -- they get attached to entities.

` + "```fyx" + `
component Velocity {
    linear: Vector3
    angular: Vector3
}

component Health {
    current: f32
    max: f32
}
` + "```" + `

Components are used with ` + "`system`" + ` and ` + "`query`" + ` to build data-driven gameplay.`,

	"system": `## system

A **system** runs logic on all entities that match a query. Systems are the "behavior" side of ECS -- they process components every frame.

` + "```fyx" + `
system apply_velocity(dt) {
    query(pos: &mut Transform, vel: &Velocity) {
        pos.translate(vel.linear * dt);
    }
}
` + "```" + `

Parameters like ` + "`dt`" + ` are injected automatically. Use ` + "`query`" + ` blocks inside to iterate over matching entities.`,

	"query": `## query

A **query** iterates over all entities that have the specified components. Use ` + "`&`" + ` for read-only access and ` + "`&mut`" + ` for write access.

` + "```fyx" + `
system physics(dt) {
    query(pos: &mut Transform, vel: &Velocity) {
        pos.translate(vel.linear * dt);
    }
}
` + "```" + `

Queries run inside systems. Each iteration gives you one entity's components.`,

	"spawn": `## spawn

**spawn** creates a new entity or instantiates a prefab in the scene.

` + "```fyx" + `
let bullet = spawn projectile_prefab at muzzle_pos lifetime 2.0;
` + "```" + `

You can also spawn raw ECS entities with components:

` + "```fyx" + `
let e = ecs.spawn(
    Velocity { linear: dir * 10.0, angular: Vector3::default() },
) lifetime 1.0;
` + "```" + `

The ` + "`lifetime`" + ` parameter is optional and auto-destroys the entity after the given seconds.`,
}

// HoverInfo returns the markdown documentation for a Fyx keyword,
// or an empty string if the keyword is not recognized.
func HoverInfo(keyword string) string {
	return hoverDocs[keyword]
}

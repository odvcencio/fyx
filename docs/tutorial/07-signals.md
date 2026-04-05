# Talking Between Scripts

In this chapter you will learn how scripts communicate with each other using signals.

## Why Signals?

In a game, different scripts need to know about things that happen elsewhere. When the player takes damage, the HUD should update. When an enemy dies, the score tracker should add points. But you do not want scripts calling each other directly -- that creates a tangle of dependencies.

Signals solve this. A script announces that something happened. Other scripts listen for that announcement. The sender does not know or care who is listening.

## Declaring a Signal

Add a `signal` line to your script to declare what it can announce:

```fyx
script Player {
    inspect speed: f32 = 10.0
    inspect health: f32 = 100.0

    signal took_damage(amount: f32)

    on update(ctx) {
        let offset = Vector3::new(0.0, 0.0, self.speed * dt);
        self.node
            .local_transform_mut()
            .offset(offset);
    }
}
```

`signal took_damage(amount: f32)` declares that `Player` can emit a signal called `took_damage`, carrying a decimal number for the damage amount.

## Emitting a Signal

Use `emit` to send the signal. Here the player emits it when health drops:

```fyx
script Player {
    inspect speed: f32 = 10.0
    inspect health: f32 = 100.0

    signal took_damage(amount: f32)
    signal died(position: Vector3)

    on update(ctx) {
        let offset = Vector3::new(0.0, 0.0, self.speed * dt);
        self.node
            .local_transform_mut()
            .offset(offset);

        if self.health <= 0.0 {
            emit died(self.node.position());
        }
    }
}
```

`emit died(self.node.position());` sends the `died` signal with the player's current position. Any script listening for `Player::died` will receive it.

## Connecting to a Signal

A separate script uses `connect` to listen for signals from another script type:

```fyx
script GameHUD {
    node health_label: Text = "UI/HealthLabel"
    node score_label: Text = "UI/ScoreLabel"

    inspect score: i32 = 0

    connect Player::took_damage(amount) {
        let remaining = 100.0 - amount;
        self.health_label.set_text(format!("HP: {}", remaining));
    }

    connect Player::died(pos) {
        self.score_label.set_text("GAME OVER");
    }
}
```

`connect Player::took_damage(amount)` runs the block whenever any `Player` script emits `took_damage`. The `amount` variable holds whatever value was passed to `emit`.

## A Two-Script Example

Here are both scripts together. Put them in the same `.fyx` file or in separate files in the same directory.

**player.fyx:**

```fyx
script Player {
    inspect speed: f32 = 10.0
    inspect health: f32 = 100.0

    signal took_damage(amount: f32)
    signal died(position: Vector3)

    move_forward: bool
    move_backward: bool

    on key(code, pressed) {
        match code {
            KeyCode::KeyW => self.move_forward = pressed,
            KeyCode::KeyS => self.move_backward = pressed,
            _ => (),
        }
    }

    on update(ctx) {
        let mut offset = Vector3::default();
        if self.move_forward {
            offset.z += 1.0;
        }
        if self.move_backward {
            offset.z -= 1.0;
        }

        if let Some(offset) = offset.try_normalize(f32::EPSILON) {
            self.node
                .local_transform_mut()
                .offset(offset.scale(self.speed * dt));
        }

        if self.health <= 0.0 {
            emit died(self.node.position());
        }
    }
}
```

**hud.fyx:**

```fyx
script GameHUD {
    node health_label: Text = "UI/HealthLabel"
    node score_label: Text = "UI/ScoreLabel"

    inspect score: i32 = 0

    connect Player::took_damage(amount) {
        let remaining = 100.0 - amount;
        self.health_label.set_text(format!("HP: {}", remaining));
    }

    connect Player::died(pos) {
        self.score_label.set_text("GAME OVER");
    }
}
```

The `Player` script knows nothing about `GameHUD`. If you later add a sound-effects script that also listens to `Player::died`, the player does not need to change.

Next: [What's Next](08-whats-next.md)

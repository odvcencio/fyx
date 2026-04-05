# Handling Input

In this chapter you will make your player respond to keyboard input so you can move it around with WASD.

## The Key Handler

Fyx provides an `on key` handler that fires whenever a key is pressed or released. It gives you two values: the key code and whether the key is currently pressed.

```fyx
script Player {
    inspect speed: f32 = 10.0

    move_forward: bool
    move_backward: bool
    move_left: bool
    move_right: bool

    on key(code, pressed) {
        match code {
            KeyCode::KeyW => self.move_forward = pressed,
            KeyCode::KeyS => self.move_backward = pressed,
            KeyCode::KeyA => self.move_left = pressed,
            KeyCode::KeyD => self.move_right = pressed,
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
        if self.move_left {
            offset.x += 1.0;
        }
        if self.move_right {
            offset.x -= 1.0;
        }

        if let Some(offset) = offset.try_normalize(f32::EPSILON) {
            self.node
                .local_transform_mut()
                .offset(offset.scale(self.speed * dt));
        }
    }
}
```

## How It Works

The `on key` handler stores which keys are held down into plain `bool` fields. These fields are not marked `inspect` because they are internal state -- you do not need to see them in the editor.

The `on update` handler builds a movement direction from the active keys, normalizes it so diagonal movement is not faster, and then applies it to the node's position.

**`match`** works like a switch statement. Each `KeyCode::KeyW =>` arm runs when that key is involved. The `_ => ()` arm catches every other key and does nothing.

**`try_normalize`** returns `Some(...)` if the vector has a meaningful direction, or `None` if the player is not pressing any movement keys. This prevents dividing by zero.

**`offset.scale(self.speed * dt)`** scales the normalized direction by your speed and frame time, giving smooth, frame-independent movement.

## The Complete File

Here is the full `player.fyx` with WASD movement:

```fyx
script Player {
    inspect speed: f32 = 10.0

    move_forward: bool
    move_backward: bool
    move_left: bool
    move_right: bool

    on key(code, pressed) {
        match code {
            KeyCode::KeyW => self.move_forward = pressed,
            KeyCode::KeyS => self.move_backward = pressed,
            KeyCode::KeyA => self.move_left = pressed,
            KeyCode::KeyD => self.move_right = pressed,
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
        if self.move_left {
            offset.x += 1.0;
        }
        if self.move_right {
            offset.x -= 1.0;
        }

        if let Some(offset) = offset.try_normalize(f32::EPSILON) {
            self.node
                .local_transform_mut()
                .offset(offset.scale(self.speed * dt));
        }
    }
}
```

Next: [States](06-state-machines.md)

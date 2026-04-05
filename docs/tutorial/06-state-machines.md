# States

In this chapter you will learn how to organize your script into states so different behaviors run at different times.

## What Is a State Machine?

As your script grows, you end up with a lot of "if I am doing X, then do Y" checks. States clean that up. Instead of one big `on update` with flags everywhere, you split your logic into named states. Only one state is active at a time.

## Declaring States

Use the `state` keyword inside a script. Each state gets its own set of handlers:

```fyx
script Player {
    inspect speed: f32 = 10.0
    inspect jump_height: f32 = 2.0

    move_forward: bool
    move_backward: bool

    on key(code, pressed) {
        match code {
            KeyCode::KeyW => self.move_forward = pressed,
            KeyCode::KeyS => self.move_backward = pressed,
            _ => (),
        }
    }

    state idle {
        on update {
            if self.move_forward || self.move_backward {
                go walking;
            }
        }
    }

    state walking {
        on update {
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

            if !self.move_forward && !self.move_backward {
                go idle;
            }
        }
    }
}
```

## Switching States

`go walking;` transitions to the `walking` state. `go idle;` transitions back. The switch happens at the end of the current frame. The first state declared in the script is the starting state.

## Enter and Exit

You can run code when entering or leaving a state:

```fyx
script Player {
    inspect speed: f32 = 10.0
    inspect jump_height: f32 = 2.0

    move_forward: bool
    move_backward: bool
    jump_pressed: bool

    on key(code, pressed) {
        match code {
            KeyCode::KeyW => self.move_forward = pressed,
            KeyCode::KeyS => self.move_backward = pressed,
            KeyCode::Space => self.jump_pressed = pressed,
            _ => (),
        }
    }

    state idle {
        on enter {
            self.jump_pressed = false;
        }

        on update {
            if self.move_forward || self.move_backward {
                go walking;
            }
            if self.jump_pressed {
                go jumping;
            }
        }
    }

    state walking {
        on update {
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

            if !self.move_forward && !self.move_backward {
                go idle;
            }
            if self.jump_pressed {
                go jumping;
            }
        }
    }

    state jumping {
        on enter {
            self.node
                .local_transform_mut()
                .offset(Vector3::new(0.0, self.jump_height, 0.0));
        }

        on update {
            go idle;
        }

        on exit {
            self.jump_pressed = false;
        }
    }
}
```

`on enter` runs once when the state becomes active. `on exit` runs once when leaving the state. `on update` runs every frame while the state is active.

This keeps each behavior isolated. The `idle` state only worries about detecting input. The `walking` state only worries about movement. The `jumping` state only worries about the jump.

Next: [Talking Between Scripts](07-signals.md)

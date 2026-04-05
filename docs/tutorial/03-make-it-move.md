# Making It Move

In this chapter you will add an update handler to your script so your player moves every frame.

## The Update Handler

Games run in a loop. Every frame, the engine gives each script a chance to do something. In Fyx, you respond to that with `on update`:

```fyx
script Player {
    inspect speed: f32 = 5.0

    on update(ctx) {

    }
}
```

The `ctx` parameter gives you access to the scene and engine context. Inside the handler, you also get `dt` automatically -- it is the number of seconds since the last frame. A typical value is something like `0.016` (about 60 frames per second).

## Why dt Matters

If you move your player by a fixed amount every frame, the movement speed depends on the frame rate. A faster computer would make the player move faster. Multiplying by `dt` makes the movement frame-independent: your player moves the same speed no matter the frame rate.

## Moving the Node

Every script is attached to a scene node. You access it through `self.node`. To move the node, you modify its local transform:

```fyx
script Player {
    inspect speed: f32 = 5.0

    on update(ctx) {
        let offset = Vector3::new(self.speed * dt, 0.0, 0.0);
        self.node
            .local_transform_mut()
            .offset(offset);
    }
}
```

This moves the player to the right every frame. `Vector3::new(x, y, z)` creates a 3D direction. Here the `x` component is `speed * dt`, so the player drifts rightward at a steady pace.

## Moving in a Direction You Choose

Let us make it move forward along the Z axis instead, and a bit faster:

```fyx
script Player {
    inspect speed: f32 = 10.0

    on update(ctx) {
        let offset = Vector3::new(0.0, 0.0, self.speed * dt);
        self.node
            .local_transform_mut()
            .offset(offset);
    }
}
```

Change the `Vector3` components to control which direction the player moves. The three axes are:

- **x** -- left and right
- **y** -- up and down
- **z** -- forward and backward

## The Complete File

Here is what `player.fyx` looks like now:

```fyx
script Player {
    inspect speed: f32 = 10.0

    on update(ctx) {
        let offset = Vector3::new(0.0, 0.0, self.speed * dt);
        self.node
            .local_transform_mut()
            .offset(offset);
    }
}
```

Run `fyxc check .` to make sure everything is correct before moving on.

Next: [Tweaking in the Editor](04-editor-fields.md)

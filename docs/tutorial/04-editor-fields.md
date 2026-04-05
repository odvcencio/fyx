# Tweaking in the Editor

In this chapter you will learn how to expose fields to the Fyrox editor and bind your script to other objects in the scene.

## Inspect Fields

You have already seen `inspect`. Any field marked with `inspect` appears in the Fyrox editor's inspector panel when you select the node your script is attached to. You can change the value without recompiling.

Add a few more fields to your player:

```fyx
script Player {
    inspect speed: f32 = 10.0
    inspect health: f32 = 100.0
    inspect jump_height: f32 = 2.0

    on update(ctx) {
        let offset = Vector3::new(0.0, 0.0, self.speed * dt);
        self.node
            .local_transform_mut()
            .offset(offset);
    }
}
```

Now in the editor you can adjust speed, health, and jump height per-instance. One enemy could be fast with low health. Another could be slow with high health. Same script, different values.

## Node Fields

Sometimes your script needs to reference other nodes in the scene -- a camera, a label, a light. Use a `node` field to bind to them by path:

```fyx
script Player {
    inspect speed: f32 = 10.0
    inspect health: f32 = 100.0
    inspect jump_height: f32 = 2.0

    node camera: Node = "CameraRig/Camera"

    on update(ctx) {
        let offset = Vector3::new(0.0, 0.0, self.speed * dt);
        self.node
            .local_transform_mut()
            .offset(offset);
    }
}
```

The `node camera: Node = "CameraRig/Camera"` line tells Fyx to look for a scene node at the path `CameraRig/Camera` relative to your script's node. You can then use `self.camera` in your handlers just like `self.node`.

## Node Types

The type after the colon tells Fyx what kind of node to expect:

- `Node` -- a generic scene node
- `Light` -- a light source
- `Text` -- a UI text label
- `Sprite` -- a 2D sprite
- `ProgressBar` -- a UI progress bar

For example, a HUD script might look like this:

```fyx
script PlayerHUD {
    node health_bar: ProgressBar = "UI/HealthBar"
    node score_label: Text = "UI/ScoreLabel"

    on update(ctx) {
        self.health_bar.set_value(0.75);
        self.score_label.set_text("Score: 100");
    }
}
```

Next: [Handling Input](05-input.md)

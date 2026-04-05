# Your First Script

In this chapter you will write a Fyx script, understand its parts, and compile it for the first time.

## What Is a Script?

In Fyrox, a script is a piece of gameplay logic that you attach to something in your scene -- a character, a door, a camera. When the game runs, the engine calls your script every frame so it can do its thing.

In Fyx, you declare a script like this:

```fyx
script Player {
    inspect speed: f32 = 5.0
}
```

That is a complete, valid `.fyx` file. Put it in `player.fyx`.

## Breaking It Down

**`script Player`** -- This declares a new script called `Player`. The name is how the Fyrox editor will list it when you attach it to a scene node.

**`inspect speed: f32 = 5.0`** -- This creates a field called `speed`.

- `inspect` means it will show up in the Fyrox editor's inspector panel, so you can tweak it without touching code.
- `f32` is the type. It means a decimal number (like `5.0`, `3.14`, or `100.0`). You will see `f32` a lot in game code -- it stands for a 32-bit floating-point number.
- `= 5.0` is the default value. If you do not change it in the editor, the player's speed starts at 5.

## Build It

Run the Fyx compiler to generate Rust output:

```bash
fyxc build .
```

You will see a generated `.rs` file appear. That file is real Rust code that Fyrox can compile and run. You do not need to open it or understand it -- `fyxc` handles that translation for you.

## Check It

You can also run a quick syntax check without generating files:

```bash
fyxc check .
```

This is useful while you are editing. It tells you about mistakes before you do a full build.

Next: [Making It Move](03-make-it-move.md)

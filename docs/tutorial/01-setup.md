# Setting Up

In this chapter you will install the Fyx toolchain and create your first project directory.

## Install Go

Fyx is built with Go. Download and install it from [go.dev/dl](https://go.dev/dl/). Once installed, confirm it works:

```bash
go version
```

You should see something like `go version go1.25.1`.

## Install fyxc

`fyxc` is the Fyx compiler. It reads your `.fyx` files and generates Rust code that the Fyrox engine can use. Install it with:

```bash
go install github.com/odvcencio/fyx/cmd/fyxc@latest
```

Verify the install:

```bash
fyxc --help
```

## Editor Setup

Install [VS Code](https://code.visualstudio.com/) if you do not already have it. Then install the Fyx extension from the VS Code marketplace. It gives you syntax highlighting for `.fyx` files.

## Create Your Project

Make a directory for your game and create an empty Fyx file inside it:

```bash
mkdir my-game
cd my-game
touch player.fyx
```

## Verify Everything Works

Run the Fyx checker on your project directory:

```bash
fyxc check .
```

You should see no errors. The checker parsed your empty file and found nothing wrong. You are ready to write your first script.

Next: [Your First Script](02-first-script.md)

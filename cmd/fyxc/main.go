package main

import (
	"errors"
	"flag"
	"fmt"
	"os"
	"strings"
)

type runOptions struct {
	CheckOnly      bool
	OutDir         string
	CargoCheck     bool
	WriteSourceMap bool
}

func main() {
	cmd, flagArgs, posArgs := parseArgs(os.Args[1:])
	if cmd == "" {
		usage()
		os.Exit(1)
	}

	fs := flag.NewFlagSet(cmd, flag.ExitOnError)
	checkOnly := fs.Bool("check", cmd == "check", "Parse/transpile validation only")
	outDir := fs.String("out", "generated", "Output directory")
	cargoCheck := fs.Bool("cargo-check", false, "Validate generated Rust with cargo check")
	writeSourceMap := fs.Bool("emit-source-map", true, "Write .fyxmap.json sidecars when output files are written")
	watch := fs.Bool("watch", false, "Re-run build/check when .fyx files change")
	fs.Parse(flagArgs)

	inputDir := "."
	if len(posArgs) > 0 {
		inputDir = posArgs[0]
	}

	opts := runOptions{
		CheckOnly:      *checkOnly,
		OutDir:         *outDir,
		CargoCheck:     *cargoCheck,
		WriteSourceMap: *writeSourceMap,
	}

	if *watch {
		if err := watchAndRun(inputDir, opts); err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(1)
		}
		return
	}

	if err := runOnce(inputDir, opts); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func runOnce(inputDir string, opts runOptions) error {
	result, err := compileProject(inputDir)
	if err != nil {
		return err
	}

	printSummary(result, opts.OutDir)

	if !opts.CheckOnly {
		if err := writeOutputTree(opts.OutDir, result.Files, opts.WriteSourceMap); err != nil {
			return fmt.Errorf("write output: %w", err)
		}
	}

	if opts.CargoCheck {
		diagnostics, err := validateGeneratedFiles(result.Files)
		if err != nil {
			return fmt.Errorf("cargo check failed: %w", err)
		}
		if len(diagnostics) > 0 {
			var b strings.Builder
			for _, diag := range diagnostics {
				fmt.Fprintf(&b, "%s:%d:%d: %s\n", diag.SourcePath, diag.SourceLine, diag.Column, diag.Message)
			}
			return errors.New(strings.TrimRight(b.String(), "\n"))
		}
		fmt.Println("  cargo check passed")
	}

	if opts.CheckOnly {
		fmt.Printf("\n  Check passed: %d files, %d scripts, %d components, %d systems, %d arbiter decls\n",
			len(result.Files), result.TotalScripts, result.TotalComponents, result.TotalSystems, result.TotalArbiter)
		return nil
	}

	fmt.Printf("\n  Summary: %d files, %d scripts, %d components, %d systems, %d arbiter decls\n",
		len(result.Files), result.TotalScripts, result.TotalComponents, result.TotalSystems, result.TotalArbiter)
	return nil
}

func parseArgs(args []string) (cmd string, flagArgs []string, posArgs []string) {
	if len(args) == 0 {
		return "", nil, nil
	}

	cmd = args[0]
	if cmd != "build" && cmd != "check" {
		return "", nil, nil
	}

	raw := args[1:]
	for i := 0; i < len(raw); i++ {
		arg := raw[i]
		switch {
		case arg == "--check" || arg == "--cargo-check" || arg == "--emit-source-map" || arg == "--watch":
			flagArgs = append(flagArgs, arg)
		case arg == "--out" && i+1 < len(raw):
			flagArgs = append(flagArgs, arg, raw[i+1])
			i++
		case strings.HasPrefix(arg, "--out="):
			flagArgs = append(flagArgs, arg)
		default:
			posArgs = append(posArgs, arg)
		}
	}

	return cmd, flagArgs, posArgs
}

func usage() {
	fmt.Fprintln(os.Stderr, "Usage: fyxc build [dir] [--check] [--cargo-check] [--watch] [--out <dir>]")
	fmt.Fprintln(os.Stderr, "       fyxc check [dir] [--cargo-check] [--watch] [--out <dir>]")
}

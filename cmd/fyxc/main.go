package main

import (
	"flag"
	"fmt"
	"os"
	"strings"
)

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
	fs.Parse(flagArgs)

	inputDir := "."
	if len(posArgs) > 0 {
		inputDir = posArgs[0]
	}

	result, err := compileProject(inputDir)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}

	printSummary(result, *outDir)

	if !*checkOnly {
		if err := writeOutputTree(*outDir, result.Files, *writeSourceMap); err != nil {
			fmt.Fprintf(os.Stderr, "write output: %v\n", err)
			os.Exit(1)
		}
	}

	if *cargoCheck {
		diagnostics, err := validateGeneratedFiles(result.Files)
		if err != nil {
			fmt.Fprintf(os.Stderr, "cargo check failed: %v\n", err)
			os.Exit(1)
		}
		if len(diagnostics) > 0 {
			for _, diag := range diagnostics {
				fmt.Fprintf(os.Stderr, "%s:%d:%d: %s\n", diag.SourcePath, diag.SourceLine, diag.Column, diag.Message)
			}
			os.Exit(1)
		}
		fmt.Println("  cargo check passed")
	}

	if *checkOnly {
		fmt.Printf("\n  Check passed: %d files, %d scripts, %d components, %d systems, %d arbiter decls\n",
			len(result.Files), result.TotalScripts, result.TotalComponents, result.TotalSystems, result.TotalArbiter)
		return
	}

	fmt.Printf("\n  Summary: %d files, %d scripts, %d components, %d systems, %d arbiter decls\n",
		len(result.Files), result.TotalScripts, result.TotalComponents, result.TotalSystems, result.TotalArbiter)
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
		case arg == "--check" || arg == "--cargo-check" || arg == "--emit-source-map":
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
	fmt.Fprintln(os.Stderr, "Usage: fyxc build [dir] [--check] [--cargo-check] [--out <dir>]")
	fmt.Fprintln(os.Stderr, "       fyxc check [dir] [--cargo-check] [--out <dir>]")
}

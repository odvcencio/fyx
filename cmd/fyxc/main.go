package main

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/odvcencio/fyrox-lang/ast"
	"github.com/odvcencio/fyrox-lang/grammar"
	"github.com/odvcencio/fyrox-lang/transpiler"
	gotreesitter "github.com/odvcencio/gotreesitter"
	"github.com/odvcencio/gotreesitter/grammargen"
)

func main() {
	if len(os.Args) < 2 || os.Args[1] != "build" {
		fmt.Fprintln(os.Stderr, "Usage: fyxc build [dir] [--check] [--out <dir>]")
		os.Exit(1)
	}

	// Separate flags from positional args so flags can appear in any position.
	buildArgs := os.Args[2:]
	var flagArgs, posArgs []string
	for i := 0; i < len(buildArgs); i++ {
		if buildArgs[i] == "--check" {
			flagArgs = append(flagArgs, buildArgs[i])
		} else if buildArgs[i] == "--out" && i+1 < len(buildArgs) {
			flagArgs = append(flagArgs, buildArgs[i], buildArgs[i+1])
			i++
		} else if strings.HasPrefix(buildArgs[i], "--out=") {
			flagArgs = append(flagArgs, buildArgs[i])
		} else {
			posArgs = append(posArgs, buildArgs[i])
		}
	}

	fs := flag.NewFlagSet("build", flag.ExitOnError)
	check := fs.Bool("check", false, "Parse and validate only, no output")
	outDir := fs.String("out", "generated", "Output directory")
	fs.Parse(flagArgs)

	inputDir := "."
	if len(posArgs) > 0 {
		inputDir = posArgs[0]
	}

	// Collect .fyx files.
	var fyxFiles []string
	err := filepath.Walk(inputDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if !info.IsDir() && strings.HasSuffix(path, ".fyx") {
			fyxFiles = append(fyxFiles, path)
		}
		return nil
	})
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error walking directory %s: %v\n", inputDir, err)
		os.Exit(1)
	}

	if len(fyxFiles) == 0 {
		fmt.Fprintf(os.Stderr, "No .fyx files found in %s\n", inputDir)
		os.Exit(1)
	}

	// Generate the language once.
	g := grammar.FyroxScriptGrammar()
	lang, err := grammargen.GenerateLanguage(g)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error generating language: %v\n", err)
		os.Exit(1)
	}

	// Process each file.
	type fileResult struct {
		basename   string
		rustOutput string
		scripts    int
		components int
		systems    int
	}

	var results []fileResult
	var totalScripts, totalComponents, totalSystems int

	for _, fpath := range fyxFiles {
		source, err := os.ReadFile(fpath)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error reading %s: %v\n", fpath, err)
			os.Exit(1)
		}

		file, err := buildAST(lang, source)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error parsing %s: %v\n", fpath, err)
			os.Exit(1)
		}

		basename := strings.TrimSuffix(filepath.Base(fpath), ".fyx")
		nScripts := len(file.Scripts)
		nComponents := len(file.Components)
		nSystems := len(file.Systems)

		totalScripts += nScripts
		totalComponents += nComponents
		totalSystems += nSystems

		rustOutput := transpiler.TranspileFile(*file)

		results = append(results, fileResult{
			basename:   basename,
			rustOutput: rustOutput,
			scripts:    nScripts,
			components: nComponents,
			systems:    nSystems,
		})

		rsPath := filepath.Join(*outDir, basename+".rs")
		fmt.Printf("  Transpiled %s.fyx -> %s (%d scripts, %d components, %d systems)\n",
			basename, rsPath, nScripts, nComponents, nSystems)
	}

	if *check {
		fmt.Printf("\n  Check passed: %d files, %d scripts, %d components, %d systems\n",
			len(results), totalScripts, totalComponents, totalSystems)
		return
	}

	// Create output directory.
	if err := os.MkdirAll(*outDir, 0o755); err != nil {
		fmt.Fprintf(os.Stderr, "Error creating output directory %s: %v\n", *outDir, err)
		os.Exit(1)
	}

	// Write .rs files.
	for _, r := range results {
		outPath := filepath.Join(*outDir, r.basename+".rs")
		if err := os.WriteFile(outPath, []byte(r.rustOutput), 0o644); err != nil {
			fmt.Fprintf(os.Stderr, "Error writing %s: %v\n", outPath, err)
			os.Exit(1)
		}
	}

	// Generate mod.rs.
	var modLines []string
	for _, r := range results {
		modLines = append(modLines, fmt.Sprintf("pub mod %s;", r.basename))
	}
	modContent := strings.Join(modLines, "\n") + "\n"
	modPath := filepath.Join(*outDir, "mod.rs")
	if err := os.WriteFile(modPath, []byte(modContent), 0o644); err != nil {
		fmt.Fprintf(os.Stderr, "Error writing %s: %v\n", modPath, err)
		os.Exit(1)
	}
	fmt.Printf("  Generated %s\n", filepath.Join(*outDir, "mod.rs"))

	fmt.Printf("\n  Summary: %d files, %d scripts, %d components, %d systems\n",
		len(results), totalScripts, totalComponents, totalSystems)
}

// buildAST wraps ast.BuildAST for use by the CLI.
func buildAST(lang *gotreesitter.Language, source []byte) (*ast.File, error) {
	return ast.BuildAST(lang, source)
}

package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"

	"github.com/odvcencio/fyx/ast"
	"github.com/odvcencio/fyx/grammar"
	"github.com/odvcencio/fyx/transpiler"
	gotreesitter "github.com/odvcencio/gotreesitter"
	"github.com/odvcencio/gotreesitter/grammargen"
)

type compiledFile struct {
	SourcePath string
	ModulePath []string
	File       ast.File
	Output     transpiler.GeneratedFile
}

type compileResult struct {
	Files           []compiledFile
	TotalScripts    int
	TotalComponents int
	TotalSystems    int
	TotalArbiter    int
}

func compileProject(inputDir string) (*compileResult, error) {
	paths, err := collectFyxFiles(inputDir)
	if err != nil {
		return nil, err
	}
	if len(paths) == 0 {
		return nil, fmt.Errorf("no .fyx files found in %s", inputDir)
	}

	lang, err := generateLanguage()
	if err != nil {
		return nil, err
	}

	files := make([]compiledFile, 0, len(paths))
	astFiles := make([]ast.File, 0, len(paths))
	for _, path := range paths {
		source, err := os.ReadFile(path)
		if err != nil {
			return nil, fmt.Errorf("read %s: %w", path, err)
		}
		file, err := buildAST(lang, source)
		if err != nil {
			return nil, fmt.Errorf("parse %s: %w", path, err)
		}
		rel, err := filepath.Rel(inputDir, path)
		if err != nil {
			return nil, fmt.Errorf("rel path %s: %w", path, err)
		}
		rel = filepath.ToSlash(rel)
		files = append(files, compiledFile{
			SourcePath: rel,
			ModulePath: transpiler.ModulePathFromRelative(rel),
			File:       *file,
		})
		astFiles = append(astFiles, *file)
	}

	signalIndex := transpiler.BuildSignalIndex(astFiles)
	componentHandleIndex := transpiler.BuildComponentHandleIndex(astFiles)
	result := &compileResult{Files: files}
	for i := range result.Files {
		file := &result.Files[i]
		file.Output = transpiler.TranspileFileResult(file.File, transpiler.Options{
			CurrentModule:        file.ModulePath,
			SignalIndex:          signalIndex,
			ComponentHandleIndex: componentHandleIndex,
			SourcePath:           file.SourcePath,
		})
		result.TotalScripts += len(file.File.Scripts)
		result.TotalComponents += len(file.File.Components)
		result.TotalSystems += len(file.File.Systems)
		result.TotalArbiter += len(file.File.ArbiterDecls)
	}

	return result, nil
}

func collectFyxFiles(root string) ([]string, error) {
	var paths []string
	err := filepath.Walk(root, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if info.IsDir() {
			return nil
		}
		if strings.HasSuffix(path, ".fyx") {
			paths = append(paths, path)
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	slices.Sort(paths)
	return paths, nil
}

func generateLanguage() (*gotreesitter.Language, error) {
	return grammargen.GenerateLanguage(grammar.FyxGrammar())
}

func buildAST(lang *gotreesitter.Language, source []byte) (*ast.File, error) {
	return ast.BuildAST(lang, source)
}

func printSummary(result *compileResult, outDir string) {
	for _, file := range result.Files {
		target := filepath.ToSlash(filepath.Join(outDir, strings.TrimSuffix(file.SourcePath, ".fyx")+".rs"))
		fmt.Printf("  Transpiled %s -> %s (%d scripts, %d components, %d systems, %d arbiter decls)\n",
			file.SourcePath, target, len(file.File.Scripts), len(file.File.Components), len(file.File.Systems), len(file.File.ArbiterDecls))
	}
}

func writeOutputTree(outDir string, files []compiledFile, writeSourceMap bool) error {
	if err := os.MkdirAll(outDir, 0o755); err != nil {
		return err
	}

	type dirEntry struct {
		Files []string
		Dirs  []string
	}
	tree := map[string]*dirEntry{
		".": &dirEntry{},
	}

	ensureDir := func(dir string) *dirEntry {
		entry, ok := tree[dir]
		if !ok {
			entry = &dirEntry{}
			tree[dir] = entry
		}
		return entry
	}

	addUnique := func(items []string, value string) []string {
		for _, existing := range items {
			if existing == value {
				return items
			}
		}
		return append(items, value)
	}

	for _, file := range files {
		outPath := filepath.Join(outDir, strings.TrimSuffix(file.SourcePath, ".fyx")+".rs")
		if err := os.MkdirAll(filepath.Dir(outPath), 0o755); err != nil {
			return err
		}
		if err := os.WriteFile(outPath, []byte(file.Output.Code), 0o644); err != nil {
			return err
		}
		if writeSourceMap {
			mapPath := strings.TrimSuffix(outPath, ".rs") + ".fyxmap.json"
			payload, err := json.MarshalIndent(transpiler.SourceMap{
				SourcePath: file.SourcePath,
				Lines:      file.Output.LineMap,
			}, "", "  ")
			if err != nil {
				return err
			}
			if err := os.WriteFile(mapPath, payload, 0o644); err != nil {
				return err
			}
		}
		if file.Output.ArbiterBundle != "" {
			arbPath := strings.TrimSuffix(outPath, ".rs") + ".arb"
			if err := os.WriteFile(arbPath, []byte(file.Output.ArbiterBundle+"\n"), 0o644); err != nil {
				return err
			}
		}

		dirParts := file.ModulePath[:max(0, len(file.ModulePath)-1)]
		dirKey := "."
		if len(dirParts) > 0 {
			dirKey = filepath.Join(dirParts...)
		}
		ensureDir(dirKey).Files = addUnique(ensureDir(dirKey).Files, file.ModulePath[len(file.ModulePath)-1])

		for i := 0; i < len(dirParts); i++ {
			parent := "."
			if i > 0 {
				parent = filepath.Join(dirParts[:i]...)
			}
			ensureDir(parent).Dirs = addUnique(ensureDir(parent).Dirs, dirParts[i])
			ensureDir(filepath.Join(dirParts[:i+1]...))
		}
	}

	for dir, entry := range tree {
		slices.Sort(entry.Files)
		slices.Sort(entry.Dirs)
		var lines []string
		for _, child := range entry.Dirs {
			lines = append(lines, fmt.Sprintf("pub mod %s;", child))
		}
		for _, mod := range entry.Files {
			lines = append(lines, fmt.Sprintf("pub mod %s;", mod))
		}
		if len(lines) == 0 {
			continue
		}

		modPath := filepath.Join(outDir, dir, "mod.rs")
		if dir == "." {
			modPath = filepath.Join(outDir, "mod.rs")
		}
		if err := os.MkdirAll(filepath.Dir(modPath), 0o755); err != nil {
			return err
		}
		if err := os.WriteFile(modPath, []byte(strings.Join(lines, "\n")+"\n"), 0o644); err != nil {
			return err
		}
	}

	return nil
}

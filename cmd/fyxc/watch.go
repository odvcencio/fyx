package main

import (
	"fmt"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"syscall"
	"time"
)

type fileSnapshot struct {
	Size    int64
	ModTime int64
}

type projectSnapshot map[string]fileSnapshot

func watchAndRun(inputDir string, opts runOptions) error {
	snapshot, err := scanProjectSnapshot(inputDir)
	if err != nil {
		return err
	}

	reportRunResult(runOnce(inputDir, opts))
	fmt.Printf("\nWatching %s for .fyx changes. Press Ctrl+C to stop.\n", inputDir)

	ticker := time.NewTicker(500 * time.Millisecond)
	defer ticker.Stop()

	signals := make(chan os.Signal, 1)
	signal.Notify(signals, os.Interrupt, syscall.SIGTERM)
	defer signal.Stop(signals)

	for {
		select {
		case <-signals:
			fmt.Println("\nStopping watch.")
			return nil
		case <-ticker.C:
			next, err := scanProjectSnapshot(inputDir)
			if err != nil {
				fmt.Fprintf(os.Stderr, "watch scan failed: %v\n", err)
				continue
			}
			if sameProjectSnapshot(snapshot, next) {
				continue
			}

			snapshot = next
			fmt.Print("\nChange detected. Rebuilding...\n\n")
			reportRunResult(runOnce(inputDir, opts))
		}
	}
}

func reportRunResult(err error) {
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
	}
}

func scanProjectSnapshot(root string) (projectSnapshot, error) {
	files := make(projectSnapshot)
	err := filepath.Walk(root, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if info.IsDir() {
			return nil
		}
		if !strings.HasSuffix(path, ".fyx") {
			return nil
		}

		rel, err := filepath.Rel(root, path)
		if err != nil {
			return err
		}
		files[filepath.ToSlash(rel)] = fileSnapshot{
			Size:    info.Size(),
			ModTime: info.ModTime().UnixNano(),
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	return files, nil
}

func sameProjectSnapshot(a, b projectSnapshot) bool {
	if len(a) != len(b) {
		return false
	}
	for path, state := range a {
		other, ok := b[path]
		if !ok {
			return false
		}
		if state != other {
			return false
		}
	}
	return true
}

// Command speccheck walks the repository and fails if the anchoring between
// specs and tests is broken (constitution §3).
package main

import (
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	"github.com/zetesis-labs/postik/internal/speccheck"
)

func main() {
	root := "."
	if len(os.Args) > 1 {
		root = os.Args[1]
	}
	src, err := collect(root)
	if err != nil {
		fmt.Fprintln(os.Stderr, "speccheck:", err)
		os.Exit(2)
	}
	report, err := speccheck.Check(src)
	if err != nil {
		fmt.Fprintln(os.Stderr, "speccheck:", err)
		os.Exit(2)
	}
	for _, id := range report.Untested {
		fmt.Printf("%s has no test\n", id)
	}
	for _, c := range report.Unknown {
		fmt.Printf("%s is cited by %s but no spec defines it\n", c.Case, c.File)
	}
	if !report.OK() {
		os.Exit(1)
	}
	fmt.Println("speccheck: every case has a test")
}

func collect(root string) (speccheck.Sources, error) {
	src := speccheck.Sources{Specs: map[string]string{}, GoTests: map[string]string{}, E2E: map[string]string{}}
	specs, err := filepath.Glob(filepath.Join(root, "specs", "S*.md"))
	if err != nil {
		return src, err
	}
	for _, path := range specs {
		if err := read(src.Specs, path); err != nil {
			return src, err
		}
	}
	err = filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			switch d.Name() {
			case "node_modules", "testdata", ".git", "dist":
				return filepath.SkipDir
			}
			return nil
		}
		switch {
		case strings.HasSuffix(path, "_test.go"):
			return read(src.GoTests, path)
		case strings.Contains(filepath.ToSlash(path), "web/e2e/") && strings.HasSuffix(path, ".spec.ts"):
			return read(src.E2E, path)
		}
		return nil
	})
	return src, err
}

func read(into map[string]string, path string) error {
	data, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	into[filepath.ToSlash(path)] = string(data)
	return nil
}

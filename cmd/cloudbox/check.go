// check (X3): the offline compatibility report — the first command in every
// onboarding path. It is the one deliberate offline copy of intake analysis
// (CP2); the server-side run at apply time remains authoritative.
package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"cloudbox/internal/core"
)

func runCheck(args []string) int {
	dir := ""
	for i := 0; i < len(args); i++ {
		if args[i] == "-f" && i+1 < len(args) {
			dir = args[i+1]
			i++
		}
	}
	if dir == "" {
		fmt.Fprintln(os.Stderr, "usage: cloudbox check -f <dir>")
		return 2
	}

	var docs []string
	entries, err := os.ReadDir(dir)
	if err != nil {
		fmt.Fprintf(os.Stderr, "check: %v\n", err)
		return 2
	}
	for _, e := range entries {
		ext := filepath.Ext(e.Name())
		if e.IsDir() || (ext != ".yaml" && ext != ".yml") {
			continue
		}
		content, err := os.ReadFile(filepath.Join(dir, e.Name()))
		if err != nil {
			fmt.Fprintf(os.Stderr, "check: %v\n", err)
			return 2
		}
		docs = append(docs, string(content))
	}

	objects, err := core.ParseManifests(strings.Join(docs, "\n---\n"))
	if err != nil {
		fmt.Fprintf(os.Stderr, "check: %v\n", err)
		return 1
	}

	// No boundary contract is available offline: contract-dependent rules
	// (undeclared secrets) are exactly why server-side intake stays
	// authoritative.
	analysis := core.AnalyzeManifests(objects, nil)

	blockers := 0
	for _, f := range analysis.Findings {
		if f.Level == "blocker" {
			blockers++
		}
		fmt.Printf("%s  %s  %s\n    %s\n", strings.ToUpper(f.Level), f.Code, f.Manifest, f.Message)
		if f.Suggestion != "" {
			fmt.Printf("    fix: %s\n", f.Suggestion)
		}
	}
	for _, t := range analysis.Transforms {
		fmt.Printf("NOTE  intake will apply a recorded %s transform: %s\n", t.Kind, t.Detail)
	}

	if blockers > 0 {
		fmt.Printf("\n%d blocker(s): fix the list above, then re-run check\n", blockers)
		return 1
	}
	fmt.Println("compatible: no intake blockers")
	return 0
}

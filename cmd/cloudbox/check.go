// check (X3): the offline compatibility report — the first command in every
// onboarding path. It is the one deliberate offline copy of intake analysis
// (CP2), linked from package intake rather than the control plane; the
// server-side run at apply time remains authoritative.
package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"cloudbox/internal/intake"
)

func runCheck(c *client, args []string) int {
	dir := ""
	for i := 0; i < len(args); i++ {
		if args[i] == "-f" && i+1 < len(args) {
			dir = args[i+1]
			i++
		}
	}
	if dir == "" {
		return c.usage("check", "cloudbox check -f <dir>")
	}

	var docs []string
	entries, err := os.ReadDir(dir)
	if err != nil {
		return c.fail("check", &failure{exit: exitUsage, message: err.Error()})
	}
	for _, e := range entries {
		ext := filepath.Ext(e.Name())
		if e.IsDir() || (ext != ".yaml" && ext != ".yml") {
			continue
		}
		content, err := os.ReadFile(filepath.Join(dir, e.Name()))
		if err != nil {
			return c.fail("check", &failure{exit: exitUsage, message: err.Error()})
		}
		docs = append(docs, string(content))
	}

	objects, err := intake.ParseManifests(strings.Join(docs, "\n---\n"))
	if err != nil {
		return c.fail("check", &failure{exit: exitRefused, message: err.Error(), reason: string(intake.Rejected)})
	}

	// No boundary contract is available offline: contract-dependent rules
	// (undeclared secrets) are exactly why server-side intake stays
	// authoritative.
	analysis := intake.AnalyzeManifests(objects, nil)
	blockers := len(analysis.Blockers())

	if c.output == "json" {
		if blockers > 0 {
			return c.fail("check", &failure{exit: exitRefused, reason: string(intake.Rejected),
				message:  fmt.Sprintf("%d blocker(s): fix the findings, then re-run check", blockers),
				findings: analysis.Findings})
		}
		return c.emit((&view{}).add("compatible", true).
			add("findings", analysis.Findings).add("transforms", analysis.Transforms))
	}

	for _, f := range analysis.Findings {
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

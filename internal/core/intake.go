// Intake: parsing, analysis, and repair of incoming bundles (B1–B5, C2, and
// the determinism gate from G7). `check` (X3) is the offline advisory copy of
// this same analysis; the server-side run remains authoritative (CP2).
package core

import (
	"cloudbox/internal/cluster"
	"cloudbox/internal/intake"
)

// Intake analysis lives in package intake so the CLI's offline check can run
// it without linking the control plane (ADR 0004). The server-side run at
// apply time remains authoritative.
type (
	Finding      = intake.Finding
	Transform    = intake.Transform
	IntakeResult = intake.IntakeResult
)

func ParseManifests(yamlText string) ([]cluster.Object, error) {
	return intake.ParseManifests(yamlText)
}

func AnalyzeManifests(objects []cluster.Object, contract *Contract) IntakeResult {
	return intake.AnalyzeManifests(objects, contract)
}

func manifestID(obj cluster.Object) string  { return intake.ManifestID(obj) }
func collectSecretRefs(v any) []string      { return intake.CollectSecretRefs(v) }
func walkStrings(v any, visit func(string)) { intake.WalkStrings(v, visit) }

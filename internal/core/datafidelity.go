// Data fidelity (D1–D8): the seal is closed-world, data is open-world — so
// the product verifies shape, declares grade, and bounds the rest downstream.
// No production value leaves production by default.
package core

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"sort"
	"strings"
	"unicode"
)

// DeclaredDatastore is one datastore an application declares (D1).
type DeclaredDatastore struct {
	Name      string `json:"name"`
	Type      string `json:"type"` // e.g. "postgresql"
	Branching bool   `json:"branching,omitempty"` // vendor supports thin clones (D7)
	Endpoint  string `json:"endpoint,omitempty"`
}

// SimDatabase is production's database as the sim models it: schema plus rows
// whose VALUES must never appear outside profiling output.
type SimDatabase struct {
	Schema map[string][]string `json:"schema"` // table → columns
	Rows   []map[string]string `json:"rows"`
}

// ColumnProfile is the statistical shape of one column (D1).
type ColumnProfile struct {
	NullRate    float64 `json:"nullRate"`
	Cardinality int     `json:"cardinality"`
	MaxLength   int     `json:"maxLength"`
	CharClasses string  `json:"charClasses"` // e.g. "alpha,digit"
}

// DataProfile is the content-addressed data profile lockfile (D1).
type DataProfile struct {
	App          string                   `json:"app"`
	Datastore    string                   `json:"datastore"`
	SchemaDigest string                   `json:"schemaDigest"`
	Columns      map[string]ColumnProfile `json:"columns"`
	RowCount     int                      `json:"rowCount"`
	Digest       string                   `json:"digest"`
}

func dsKey(app, ds string) string { return app + "/" + ds }

// SeedDatabase is sim arrangement: production's database content.
func (c *Core) SeedDatabase(appName, datastore string, db SimDatabase) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	if _, ok := c.apps[appName]; !ok {
		return errf(404, "application %q is not known", appName)
	}
	c.databases[dsKey(appName, datastore)] = &db
	return nil
}

// ProfileDatastore profiles production's database: schema digest plus
// per-column statistics, content-addressed. Values never leave (D1).
func (c *Core) ProfileDatastore(appName, datastore string) (*DataProfile, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.profileLocked(appName, datastore)
}

func (c *Core) profileLocked(appName, datastore string) (*DataProfile, error) {
	app, ok := c.apps[appName]
	if !ok {
		return nil, errf(404, "application %q is not known", appName)
	}
	declared := false
	for _, ds := range app.Datastores {
		if ds.Name == datastore {
			declared = true
		}
	}
	if !declared {
		return nil, errf(404, "datastore %q is not declared by application %q", datastore, appName)
	}
	db, ok := c.databases[dsKey(appName, datastore)]
	if !ok {
		return nil, errf(409, "no production database reachable for %q", datastore)
	}

	schemaBlob, _ := json.Marshal(db.Schema)
	schemaSum := sha256.Sum256(schemaBlob)

	profile := &DataProfile{
		App: appName, Datastore: datastore,
		SchemaDigest: "sha256:" + hex.EncodeToString(schemaSum[:]),
		Columns:      map[string]ColumnProfile{},
		RowCount:     len(db.Rows),
	}
	var columns []string
	for _, cols := range db.Schema {
		columns = append(columns, cols...)
	}
	sort.Strings(columns)
	for _, col := range columns {
		profile.Columns[col] = profileColumn(db.Rows, col)
	}
	blob, _ := json.Marshal(profile)
	sum := sha256.Sum256(blob)
	profile.Digest = "sha256:" + hex.EncodeToString(sum[:])
	c.profiles[dsKey(appName, datastore)] = profile
	return profile, nil
}

// profileColumn reduces values to shape: rates, cardinalities, lengths,
// character classes — never the values themselves (D1).
func profileColumn(rows []map[string]string, col string) ColumnProfile {
	distinct := map[string]bool{}
	nulls, maxLen := 0, 0
	hasAlpha, hasDigit := false, false
	for _, row := range rows {
		value, present := row[col]
		if !present || value == "" {
			nulls++
			continue
		}
		distinct[value] = true
		if len(value) > maxLen {
			maxLen = len(value)
		}
		for _, r := range value {
			if unicode.IsLetter(r) {
				hasAlpha = true
			}
			if unicode.IsDigit(r) {
				hasDigit = true
			}
		}
	}
	var classes []string
	if hasAlpha {
		classes = append(classes, "alpha")
	}
	if hasDigit {
		classes = append(classes, "digit")
	}
	nullRate := 0.0
	if len(rows) > 0 {
		nullRate = float64(nulls) / float64(len(rows))
	}
	return ColumnProfile{
		NullRate: nullRate, Cardinality: len(distinct),
		MaxLength: maxLen, CharClasses: strings.Join(classes, ","),
	}
}

// EnableRealData is the admin opt-in for masked-snapshot and live-clone (D8):
// never default, audited, and separately gated for agent-owned sandboxes.
func (c *Core) EnableRealData(appName, admin string, forAgents bool) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	app, ok := c.apps[appName]
	if !ok {
		return errf(404, "application %q is not known", appName)
	}
	app.RealDataEnabled = true
	app.RealDataForAgents = forAgents
	c.recordAudit(admin, "real-data-enabled", appName, fmt.Sprintf("forAgents=%t", forAgents))
	return nil
}

// ProvisionDatastore provisions one declared datastore in a sandbox at a
// fidelity level, enforcing the real-data gates (D8) and wiring thin clones
// through the boundary contract (D7).
func (c *Core) ProvisionDatastore(sandboxName, datastore, level string) (map[string]any, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	sb, ok := c.sandboxes[sandboxName]
	if !ok {
		return nil, errf(404, "sandbox %q is not known", sandboxName)
	}
	if _, ok := fidelityRank[level]; !ok {
		return nil, errf(422, "unknown fidelity level %q", level)
	}
	app := c.apps[sb.App]

	if fidelityRank[level] >= fidelityRank["masked-snapshot"] {
		if app == nil || !app.RealDataEnabled {
			return nil, errf(403,
				"real-data levels are not enabled for application %q: masked-snapshot and live-clone are admin-enabled per application, never default (D8)",
				sb.App)
		}
		if sb.AgentOwned && !app.RealDataForAgents {
			return nil, errf(403,
				"sandbox %q is agent-owned: real-data levels are unavailable to agent-owned sandboxes pending explicit policy (D8)",
				sandboxName)
		}
	}

	result := map[string]any{"datastore": datastore, "fidelity": level}

	if level == "live-clone" {
		var declared *DeclaredDatastore
		if app != nil {
			for i := range app.Datastores {
				if app.Datastores[i].Name == datastore {
					declared = &app.Datastores[i]
				}
			}
		}
		if declared == nil || !declared.Branching {
			return nil, errf(409, "datastore %q has no branching-capable service: live-clone is a thin-clone integration (D7)", datastore)
		}
		// D7: the branch endpoint enters the sandbox through the boundary
		// contract — a per-sandbox secret plus an allowlist entry.
		branch := fmt.Sprintf("branch-%s.%s", sandboxName, declared.Endpoint)
		secret := fmt.Sprintf("clone-%s-%s", datastore, sandboxName)
		if sb.CloneEndpoints == nil {
			sb.CloneEndpoints = map[string]string{}
			sb.CloneSecrets = map[string]string{}
		}
		sb.CloneEndpoints[datastore] = branch
		sb.CloneSecrets[datastore] = secret
		if host, ok := c.driver.Cluster(sb.Cluster); ok && app != nil {
			host.SealNamespace(sb.Namespace, append(append([]string{}, app.Contract.EgressAllowlist...), branch))
		}
		result["branchEndpoint"] = branch
		result["perSandboxSecret"] = secret
	}

	// A shape-claiming level pins the profile digest the sandbox was
	// provisioned from, so drift can stale it (D5).
	if fidelityRank[level] >= fidelityRank["schema-replay"] {
		if profile, ok := c.profiles[dsKey(sb.App, datastore)]; ok {
			if sb.ProfileDigests == nil {
				sb.ProfileDigests = map[string]string{}
			}
			sb.ProfileDigests[datastore] = profile.Digest
		}
	}

	if sb.Datastores == nil {
		sb.Datastores = map[string]string{}
	}
	sb.Datastores[datastore] = level
	return result, nil
}

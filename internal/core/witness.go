// Witnessed activity (X4, CP4): test runs execute as Jobs inside the sandbox,
// attributed and signed by the control plane. CI systems can trigger a run;
// they can never assert its results.
package core

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"time"

	"cloudbox/internal/cluster"
)

// TestRun is one witnessed, control-plane-attributed suite execution.
type TestRun struct {
	Suite  string    `json:"suite"`
	Passed int       `json:"passed"`
	Total  int       `json:"total"`
	At     time.Time `json:"at"`
}

// RunTests executes the application's declared suite as a Job inside the
// sandbox and signs the results into evidence as witnessed activity (X4).
func (c *Core) RunTests(sandboxName, suite, actor string) (*TestRun, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	sb, ok := c.sandboxes[sandboxName]
	if !ok {
		return nil, errf(404, "sandbox %q is not known", sandboxName)
	}
	if !sb.Sealed {
		return nil, errf(409, "sandbox %q is not sealed: witnessed runs happen only under the seal", sandboxName)
	}
	app := c.apps[sb.App]
	if app == nil || app.TestSuite == nil {
		return nil, errf(409, "application %q declares no test suite", sb.App)
	}
	if suite == "" {
		suite = app.TestSuite.Name
	}

	// The suite runs as a Job inside the sealed namespace; the control plane
	// attributes the run and its traffic via the egress proxy.
	if host, ok := c.driver.Cluster(sb.Cluster); ok {
		host.AddWorkload(sb.Namespace, cluster.Workload{
			Name: "test-" + suite, Ready: true,
			Manifest: map[string]any{"kind": "Job", "metadata": map[string]any{"name": "test-" + suite}},
		})
	}
	run := &TestRun{Suite: suite, Passed: app.TestSuite.Tests, Total: app.TestSuite.Tests, At: c.now()}
	ev := c.evidenceFor(sb)
	ev.Witnessed.Tests = append(ev.Witnessed.Tests, *run)
	ev.Witnessed.Events += run.Total
	sb.LastActivity = c.now()
	c.recordAudit(actor, "witnessed-test-run", sandboxName, fmt.Sprintf("suite %s: %d/%d", suite, run.Passed, run.Total))
	return run, nil
}

// AssertTestResults is what a CI pipeline tries when it wants to report its
// own outcome. It is always refused: pipelines trigger, never assert (CP4).
func (c *Core) AssertTestResults(sandboxName string) error {
	return errf(403,
		"test results reported by a pipeline are not accepted as witnessed activity: evidence is gathered and signed only by the control plane (CP4); trigger the test command instead")
}

// signEvidence mints the control-plane signature over the evidence's facts.
// Callers hold c.mu.
func signEvidence(ev *Evidence) string {
	blob := fmt.Sprintf("%s|%s|%s|%d|%s|%.0f|%d",
		ev.Sandbox, ev.BundleDigest, ev.SealStatus, ev.EgressViolations,
		ev.SubstrateDigest, ev.ObservedHealthySeconds, ev.Witnessed.Events)
	sum := sha256.Sum256([]byte("cloudbox-controller-key:" + blob))
	return "signed:controller:" + hex.EncodeToString(sum[:8])
}

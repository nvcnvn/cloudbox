// Package sim is the simulated cluster driver (ADR 0007): an in-process model
// of the Kubernetes semantics the specs exercise. The acceptance suite boots
// cloudboxd against it; the kube driver is the production path behind the same
// interfaces.
package sim

import "sync"

// World holds every simulated cluster and all control-plane state for one
// cloudboxd process. State grows per capability as rule tasks land.
type World struct {
	mu sync.Mutex
}

func NewWorld() *World {
	return &World{}
}

// Reset returns the world to a pristine state. Only the sim driver has this;
// the acceptance suite calls it between scenarios via /simctl/reset.
func (w *World) Reset() {
	w.mu.Lock()
	defer w.mu.Unlock()
}

// Capacity transforms (S7): recorded, digest-preserving admission transforms
// that make prod-sized bundles schedulable under sandbox quotas. CPU is
// compressible (starvation slows, never kills), memory is not (an OOM-killed
// quorum member proves nothing), and replica topology is where N>1 bugs live.
package core

import (
	"fmt"
	"strconv"
	"strings"

	"cloudbox/internal/cluster"
)

const (
	cpuScaleFactor   = 0.25
	cpuFloorMilli    = 10
	memScaleFactor   = 0.5
	memFloorMi       = 64
)

// parseCPUMilli parses "500m" or "2" into millicores.
func parseCPUMilli(s string) (int64, bool) {
	if s == "" {
		return 0, false
	}
	if strings.HasSuffix(s, "m") {
		v, err := strconv.ParseInt(strings.TrimSuffix(s, "m"), 10, 64)
		return v, err == nil
	}
	v, err := strconv.ParseFloat(s, 64)
	return int64(v * 1000), err == nil
}

// parseMemMi parses "512Mi" or "2Gi" into Mi.
func parseMemMi(s string) (int64, bool) {
	switch {
	case strings.HasSuffix(s, "Gi"):
		v, err := strconv.ParseInt(strings.TrimSuffix(s, "Gi"), 10, 64)
		return v * 1024, err == nil
	case strings.HasSuffix(s, "Mi"):
		v, err := strconv.ParseInt(strings.TrimSuffix(s, "Mi"), 10, 64)
		return v, err == nil
	}
	return 0, false
}

// applyCapacityTransform builds the admitted manifest for one workload object
// under a capacity mode. Bundle bytes are untouched — this shapes only what
// gets admitted. Workload-internal sizing (env vars like -Xmx) is never
// rewritten (S7).
func applyCapacityTransform(obj cluster.Object, mode string) map[string]any {
	admitted := deepCopy(obj.Manifest)
	if mode == "full" {
		return admitted
	}
	spec, _ := admitted["spec"].(map[string]any)
	if spec == nil {
		return admitted
	}
	if mode == "minimal" {
		if _, ok := spec["replicas"]; ok {
			spec["replicas"] = 1
		}
	}
	// squeezed and minimal both scale container requests.
	scaleContainers(spec)
	return admitted
}

func scaleContainers(v any) {
	switch t := v.(type) {
	case map[string]any:
		if containers, ok := t["containers"].([]any); ok {
			for _, item := range containers {
				container, _ := item.(map[string]any)
				if container == nil {
					continue
				}
				resources, _ := container["resources"].(map[string]any)
				if resources == nil {
					continue
				}
				for _, section := range []string{"requests", "limits"} {
					entries, _ := resources[section].(map[string]any)
					if entries == nil {
						continue
					}
					if cpu, ok := entries["cpu"].(string); ok {
						if milli, ok := parseCPUMilli(cpu); ok {
							scaled := int64(float64(milli) * cpuScaleFactor)
							if scaled < cpuFloorMilli {
								scaled = cpuFloorMilli
							}
							entries["cpu"] = fmt.Sprintf("%dm", scaled)
						}
					}
					if mem, ok := entries["memory"].(string); ok {
						if mi, ok := parseMemMi(mem); ok {
							scaled := int64(float64(mi) * memScaleFactor)
							if scaled < memFloorMi {
								scaled = memFloorMi
							}
							entries["memory"] = fmt.Sprintf("%dMi", scaled)
						}
					}
				}
			}
		}
		for _, val := range t {
			scaleContainers(val)
		}
	case []any:
		for _, item := range t {
			scaleContainers(item)
		}
	}
}

// totalCPUCores sums container CPU requests (in cores) across a manifest —
// what the quota check meters after transforms (S5).
func totalCPUCores(manifest map[string]any) float64 {
	var milli int64
	var walk func(any)
	walk = func(v any) {
		switch t := v.(type) {
		case map[string]any:
			if requests, ok := t["requests"].(map[string]any); ok {
				if cpu, ok := requests["cpu"].(string); ok {
					if m, ok := parseCPUMilli(cpu); ok {
						milli += m
					}
				}
			}
			for _, val := range t {
				walk(val)
			}
		case []any:
			for _, item := range t {
				walk(item)
			}
		}
	}
	walk(manifest)
	replicas := int64(1)
	if spec, ok := manifest["spec"].(map[string]any); ok {
		switch r := spec["replicas"].(type) {
		case int:
			replicas = int64(r)
		case float64:
			replicas = int64(r)
		}
	}
	return float64(milli*replicas) / 1000
}

func deepCopy(m map[string]any) map[string]any {
	out := make(map[string]any, len(m))
	for k, v := range m {
		out[k] = deepCopyValue(v)
	}
	return out
}

func deepCopyValue(v any) any {
	switch t := v.(type) {
	case map[string]any:
		return deepCopy(t)
	case []any:
		items := make([]any, len(t))
		for i, item := range t {
			items[i] = deepCopyValue(item)
		}
		return items
	default:
		return v
	}
}

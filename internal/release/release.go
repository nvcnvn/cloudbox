// Package release holds what the release process stamps into a build
// (release-integrity): the version that identifies the build, and the pinned
// egress proxy image that build deploys. Both are set with -ldflags -X by
// `make release-binaries`; a build made any other way keeps the defaults and
// says it is unreleased.
package release

import (
	"fmt"
	"os"
	"runtime/debug"
)

// Stamped by the release process.
var (
	Version    = "unreleased"
	ProxyImage = "cloudbox-egress-proxy:dev" // loaded locally by scripts/kind-cluster.sh
)

// ProxyName names the egress proxy Deployment and Service in every sealed
// namespace, on both drivers.
const ProxyName = "cloudbox-egress-proxy"

// Released reports whether the release process produced this build.
func Released() bool { return Version != "unreleased" }

// Describe is the version report: a released build names its version; any
// other build says it is unreleased and names the commit it came from, so it
// can never be mistaken for a release.
func Describe() string {
	if Released() {
		return Version
	}
	rev, modified := "unknown commit", false
	if info, ok := debug.ReadBuildInfo(); ok {
		for _, s := range info.Settings {
			switch s.Key {
			case "vcs.revision":
				if len(s.Value) > 12 {
					rev = "commit " + s.Value[:12]
				} else if s.Value != "" {
					rev = "commit " + s.Value
				}
			case "vcs.modified":
				modified = s.Value == "true"
			}
		}
	}
	if modified {
		rev += ", with uncommitted changes"
	}
	return fmt.Sprintf("unreleased build (%s) — not a released version", rev)
}

// EgressProxyImage is the reference the drivers deploy. CLOUDBOX_PROXY_IMAGE
// overrides it (a mirror for an air-gapped cluster, or a local build).
func EgressProxyImage() string {
	if img := os.Getenv("CLOUDBOX_PROXY_IMAGE"); img != "" {
		return img
	}
	return ProxyImage
}

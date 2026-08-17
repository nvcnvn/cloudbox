// cloudbox-proxy is the product-managed egress proxy (ADR 0001): the only
// path out of a sealed namespace. It enforces the FQDN allowlist that
// NetworkPolicy v1 cannot express, and records the attempts it sees — the
// recorded, attributed denial is N4's raw material.
//
// The record is bounded (a single replica holding an unbounded slice loses
// everything the day it runs out of memory) and the bound reports what it
// discarded, because a truncated record served as a complete one under-counts
// the violations in a run's evidence.
//
// WHAT PROTECTS THE ADMIN SURFACE. /attempts requires the per-namespace
// credential the control plane creates at seal time, and NOT the seal's
// NetworkPolicy. The seal's cloudbox-egress-proxy-admin rule admits ingress on
// the admin port from any source, deliberately: collection arrives through the
// API server's service proxy, whose source address is a cluster-topology
// detail, and pinning it into the seal would make the seal cluster-specific and
// quietly breakable by a topology change. So any pod on the cluster can still
// open a connection to this port — it just cannot read anything without the
// credential. Anyone hardening this later should know the policy was never the
// control (ADR 0001: residual channels are published, not papered over).
//
// One proxy runs per sealed namespace. The allowlist arrives via the
// cloudbox-egress-allowlist ConfigMap, mounted as a file and re-read per
// request so a re-seal (allowlist change) needs no restart. Attempts are
// served on the admin port for the control plane to collect; attribution to
// a workload happens control-plane-side by resolving the recorded source IP.
package main

import (
	"bufio"
	"crypto/rand"
	"crypto/subtle"
	"encoding/hex"
	"encoding/json"
	"flag"
	"io"
	"log"
	"net"
	"net/http"
	"os"
	"strings"
	"sync"
	"time"
)

// defaultMaxRetainedAttempts bounds the in-memory attempt record. An
// unbounded record in a single replica with no memory limit fails by losing
// every attempt at once — the silent under-count N4 cannot tolerate. A bound
// plus a reported drop count keeps the recent picture accurate and the loss
// visible. 512 raw records is tens of kilobytes, far under the container's
// memory limit, and wide enough for a sandbox's ordinary behaviour; the
// workload that retries one blocked destination in a loop is what aggregating
// by destination would fix, and that is deliberately deferred because it
// would redefine what evidence.EgressViolations counts.
const defaultMaxRetainedAttempts = 512

type attempt struct {
	Destination string    `json:"destination"`
	SourceIP    string    `json:"sourceIp"`
	At          time.Time `json:"at"`
	Allowed     bool      `json:"allowed"`
}

// attemptRecord is what /attempts serves: the attempts still retained, how
// many the retention bound discarded, and which process is answering, so
// neither a truncated record nor a restarted one is presented as complete.
type attemptRecord struct {
	Attempts    []attempt `json:"attempts"`
	Dropped     int       `json:"dropped"`
	Incarnation string    `json:"incarnation"`
}

type proxy struct {
	mu          sync.Mutex
	attempts    []attempt
	dropped     int // monotonic: attempts the bound discarded, never reset
	maxRetained int

	// incarnation identifies this process. The record lives in memory, so a
	// restart loses whatever was not collected; a stateless pod cannot prove
	// it lost nothing, and this is what lets the control plane tell a restart
	// from a quiet stretch instead of reading a short count as the truth.
	incarnation string

	allowlistPath string
	// adminTokenPath is the per-namespace credential the control plane
	// presents to read /attempts. Read per request, like the allowlist, so a
	// re-seal needs no restart.
	adminTokenPath string
}

// adminTokenHeader carries the credential on /attempts. A header rather than a
// query parameter: the collection goes through the API server's service proxy,
// which forwards headers to the backend but records request URIs in its audit
// log, and a credential does not belong in an audit log.
const adminTokenHeader = "X-Cloudbox-Egress-Token"

// authorized reports whether a request may read the attempt record. Fails
// closed, like the allowlist: an unreadable credential authorizes nobody.
func (p *proxy) authorized(r *http.Request) bool {
	want, err := os.ReadFile(p.adminTokenPath)
	if err != nil {
		log.Printf("cloudbox-proxy: reading the admin token: %v", err)
		return false
	}
	expected := strings.TrimSpace(string(want))
	if expected == "" {
		return false
	}
	got := strings.TrimSpace(r.Header.Get(adminTokenHeader))
	return subtle.ConstantTimeCompare([]byte(got), []byte(expected)) == 1
}

// newIncarnation is generated once per process start.
func newIncarnation() string {
	buf := make([]byte, 16)
	if _, err := rand.Read(buf); err != nil {
		// Without a distinguishable identity the control plane cannot detect a
		// restart, and would report a possibly-short record as complete.
		log.Fatalf("cloudbox-proxy: generating the startup incarnation: %v", err)
	}
	return hex.EncodeToString(buf)
}

func (p *proxy) allowed(host string) bool {
	data, err := os.ReadFile(p.allowlistPath)
	if err != nil {
		return false // no allowlist readable → nothing is allowed
	}
	for _, line := range strings.Split(string(data), "\n") {
		if fqdn := strings.TrimSpace(line); fqdn != "" && fqdn == host {
			return true
		}
	}
	return false
}

func (p *proxy) record(destination, remoteAddr string, allowed bool) {
	ip := remoteAddr
	if host, _, err := net.SplitHostPort(remoteAddr); err == nil {
		ip = host
	}
	p.mu.Lock()
	defer p.mu.Unlock()
	p.attempts = append(p.attempts, attempt{
		Destination: destination, SourceIP: ip, At: time.Now().UTC(), Allowed: allowed,
	})
	// Oldest-first: keeping the first N instead would let a workload looping
	// on one blocked destination mask a later, different one. What the bound
	// costs is counted, not hidden.
	if excess := len(p.attempts) - p.maxRetained; excess > 0 {
		p.attempts = append(p.attempts[:0], p.attempts[excess:]...)
		p.dropped += excess
	}
}

func (p *proxy) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if r.Method == http.MethodConnect {
		p.connect(w, r)
		return
	}
	host := r.URL.Hostname()
	ok := p.allowed(host)
	p.record(host, r.RemoteAddr, ok)
	if !ok {
		http.Error(w, "egress to "+host+" is not on the application allowlist", http.StatusForbidden)
		return
	}
	outbound := r.Clone(r.Context())
	outbound.RequestURI = ""
	resp, err := http.DefaultTransport.RoundTrip(outbound)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadGateway)
		return
	}
	defer resp.Body.Close()
	for key, values := range resp.Header {
		for _, v := range values {
			w.Header().Add(key, v)
		}
	}
	w.WriteHeader(resp.StatusCode)
	_, _ = io.Copy(w, resp.Body)
}

func (p *proxy) connect(w http.ResponseWriter, r *http.Request) {
	host, _, err := net.SplitHostPort(r.Host)
	if err != nil {
		host = r.Host
	}
	ok := p.allowed(host)
	p.record(host, r.RemoteAddr, ok)
	if !ok {
		http.Error(w, "egress to "+host+" is not on the application allowlist", http.StatusForbidden)
		return
	}
	upstream, err := net.DialTimeout("tcp", r.Host, 15*time.Second)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadGateway)
		return
	}
	hijacker, ok2 := w.(http.Hijacker)
	if !ok2 {
		upstream.Close()
		http.Error(w, "cannot hijack", http.StatusInternalServerError)
		return
	}
	client, _, err := hijacker.Hijack()
	if err != nil {
		upstream.Close()
		return
	}
	_, _ = client.Write([]byte("HTTP/1.1 200 Connection Established\r\n\r\n"))
	go pipe(upstream, client)
	go pipe(client, upstream)
}

func pipe(dst io.WriteCloser, src io.ReadCloser) {
	defer dst.Close()
	defer src.Close()
	_, _ = io.Copy(dst, bufio.NewReader(src))
}

func (p *proxy) serveAttempts(w http.ResponseWriter, r *http.Request) {
	if !p.authorized(r) {
		http.Error(w, "the attempt record is readable only by the control plane",
			http.StatusUnauthorized)
		return
	}
	p.mu.Lock()
	defer p.mu.Unlock()
	retained := p.attempts
	if retained == nil {
		retained = []attempt{}
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(attemptRecord{
		Attempts: retained, Dropped: p.dropped, Incarnation: p.incarnation,
	})
}

func main() {
	proxyAddr := flag.String("proxy-addr", ":3128", "egress proxy listen address")
	adminAddr := flag.String("admin-addr", ":3129", "attempt-record listen address")
	allowlist := flag.String("allowlist", "/etc/cloudbox/allowlist", "allowlist file path")
	maxRetained := flag.Int("max-attempts", defaultMaxRetainedAttempts,
		"how many attempts to retain; beyond this the oldest are discarded and counted")
	adminToken := flag.String("admin-token", "/etc/cloudbox/admin/token",
		"file holding the credential required to read /attempts")
	flag.Parse()

	p := &proxy{
		allowlistPath:  *allowlist,
		maxRetained:    *maxRetained,
		incarnation:    newIncarnation(),
		adminTokenPath: *adminToken,
	}

	admin := http.NewServeMux()
	admin.HandleFunc("GET /attempts", p.serveAttempts)
	admin.HandleFunc("GET /healthz", func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte("ok"))
	})
	go func() { log.Fatal(http.ListenAndServe(*adminAddr, admin)) }()

	log.Printf("cloudbox-proxy listening on %s (admin %s), incarnation %s, retaining %d attempts",
		*proxyAddr, *adminAddr, p.incarnation, p.maxRetained)
	log.Fatal(http.ListenAndServe(*proxyAddr, p))
}

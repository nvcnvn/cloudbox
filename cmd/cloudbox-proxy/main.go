// cloudbox-proxy is the product-managed egress proxy (ADR 0001): the only
// path out of a sealed namespace. It enforces the FQDN allowlist that
// NetworkPolicy v1 cannot express, and records every attempt — the recorded,
// attributed denial is N4's raw material.
//
// One proxy runs per sealed namespace. The allowlist arrives via the
// cloudbox-egress-allowlist ConfigMap, mounted as a file and re-read per
// request so a re-seal (allowlist change) needs no restart. Attempts are
// served on the admin port for the control plane to collect; attribution to
// a workload happens control-plane-side by resolving the recorded source IP.
package main

import (
	"bufio"
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

type attempt struct {
	Destination string    `json:"destination"`
	SourceIP    string    `json:"sourceIp"`
	At          time.Time `json:"at"`
	Allowed     bool      `json:"allowed"`
}

type proxy struct {
	mu            sync.Mutex
	attempts      []attempt
	allowlistPath string
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

func (p *proxy) serveAttempts(w http.ResponseWriter, _ *http.Request) {
	p.mu.Lock()
	defer p.mu.Unlock()
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(p.attempts)
}

func main() {
	proxyAddr := flag.String("proxy-addr", ":3128", "egress proxy listen address")
	adminAddr := flag.String("admin-addr", ":3129", "attempt-record listen address")
	allowlist := flag.String("allowlist", "/etc/cloudbox/allowlist", "allowlist file path")
	flag.Parse()

	p := &proxy{allowlistPath: *allowlist}

	admin := http.NewServeMux()
	admin.HandleFunc("GET /attempts", p.serveAttempts)
	admin.HandleFunc("GET /healthz", func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte("ok"))
	})
	go func() { log.Fatal(http.ListenAndServe(*adminAddr, admin)) }()

	log.Printf("cloudbox-proxy listening on %s (admin %s)", *proxyAddr, *adminAddr)
	log.Fatal(http.ListenAndServe(*proxyAddr, p))
}

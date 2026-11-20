// The CLI starts and reuses a local control plane; it never becomes one
// (design D3, ADR 0004). The started process is cloudboxd, the same server an
// operator would run: all enforcement stays inside it.
package main

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"syscall"
	"time"
)

// localState is the whole state file. It records how to reach the daemon and
// which build started it — nothing else. Registered clusters, applications
// and evidence are NOT recorded: that would be durable control-plane state,
// which this release deliberately does not have (design Open Question 3).
type localState struct {
	Addr      string    `json:"addr"`
	PID       int       `json:"pid"`
	Build     string    `json:"build"` // identity of the cloudboxd binary that was started
	Driver    string    `json:"driver"`
	StartedAt time.Time `json:"startedAt"`
}

func stateDir() string {
	if d := os.Getenv("CLOUDBOX_STATE_DIR"); d != "" {
		return d
	}
	if d := os.Getenv("XDG_STATE_HOME"); d != "" {
		return filepath.Join(d, "cloudbox")
	}
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".local", "state", "cloudbox")
}

func statePath() string { return filepath.Join(stateDir(), "control-plane.json") }

// daemonBinary finds cloudboxd: $CLOUDBOXD, else beside this binary, else PATH.
func daemonBinary() (string, error) {
	if p := os.Getenv("CLOUDBOXD"); p != "" {
		return p, nil
	}
	if self, err := os.Executable(); err == nil {
		candidate := filepath.Join(filepath.Dir(self), "cloudboxd")
		if _, err := os.Stat(candidate); err == nil {
			return candidate, nil
		}
	}
	return exec.LookPath("cloudboxd")
}

// buildIdentity names the daemon build: the release version plus a digest of
// the binary, so a daemon started by an older or rebuilt binary is replaced
// rather than reused (design D3).
func buildIdentity(binary string) string {
	f, err := os.Open(binary)
	if err != nil {
		return version
	}
	defer f.Close()
	h := sha256.New()
	_, _ = io.Copy(h, f)
	return version + "+" + hex.EncodeToString(h.Sum(nil))[:12]
}

func healthy(addr string) bool {
	httpc := &http.Client{Timeout: time.Second}
	resp, err := httpc.Get(addr + "/healthz")
	if err != nil {
		return false
	}
	resp.Body.Close()
	return resp.StatusCode == http.StatusOK
}

func readState() (*localState, error) {
	raw, err := os.ReadFile(statePath())
	if err != nil {
		return nil, err
	}
	var st localState
	if err := json.Unmarshal(raw, &st); err != nil {
		return nil, err
	}
	return &st, nil
}

// withLock serialises start-or-reuse across concurrent CLI invocations, so two
// commands racing on a fresh machine still produce one control plane.
func withLock(fn func() (string, error)) (string, error) {
	if err := os.MkdirAll(stateDir(), 0o700); err != nil {
		return "", err
	}
	lock, err := os.OpenFile(filepath.Join(stateDir(), "lock"), os.O_CREATE|os.O_RDWR, 0o600)
	if err != nil {
		return "", err
	}
	defer lock.Close()
	if err := syscall.Flock(int(lock.Fd()), syscall.LOCK_EX); err != nil {
		return "", err
	}
	defer syscall.Flock(int(lock.Fd()), syscall.LOCK_UN)
	return fn()
}

// ensureLocal returns the address of the local control plane, starting one
// when none is running or the running one is from a different build.
func ensureLocal() (string, error) {
	return withLock(func() (string, error) {
		binary, err := daemonBinary()
		if err != nil {
			return "", fmt.Errorf("no control plane addressed and cloudboxd not found to start one (set --server, or install cloudboxd beside cloudbox): %v", err)
		}
		build := buildIdentity(binary)
		if st, err := readState(); err == nil {
			if st.Build == build && healthy(st.Addr) {
				return st.Addr, nil
			}
			// A stale daemon from another build is replaced, never reused.
			stopProcess(st.PID)
		}
		return startLocal(binary, build)
	})
}

func startLocal(binary, build string) (string, error) {
	port, err := freePort()
	if err != nil {
		return "", err
	}
	addr := fmt.Sprintf("127.0.0.1:%d", port)
	driver := os.Getenv("CLOUDBOX_LOCAL_DRIVER")
	if driver == "" {
		driver = "kube" // the released driver; the sim is for the test suite
	}
	logFile, err := os.OpenFile(filepath.Join(stateDir(), "cloudboxd.log"),
		os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o600)
	if err != nil {
		return "", err
	}
	defer logFile.Close()
	cmd := exec.Command(binary, "--addr", addr, "--driver", driver)
	cmd.Stdout, cmd.Stderr = logFile, logFile
	// Detached: the control plane outlives the command that started it.
	cmd.SysProcAttr = &syscall.SysProcAttr{Setsid: true}
	if err := cmd.Start(); err != nil {
		return "", fmt.Errorf("starting %s: %v", binary, err)
	}
	pid := cmd.Process.Pid
	_ = cmd.Process.Release()

	url := "http://" + addr
	deadline := time.Now().Add(2 * time.Minute) // the kube driver installs CRDs at boot
	for !healthy(url) {
		if !alive(pid) {
			return "", fmt.Errorf("the local control plane exited during startup; see %s",
				filepath.Join(stateDir(), "cloudboxd.log"))
		}
		if time.Now().After(deadline) {
			stopProcess(pid)
			return "", errors.New("the local control plane did not become healthy in time")
		}
		time.Sleep(100 * time.Millisecond)
	}
	st := localState{Addr: url, PID: pid, Build: build, Driver: driver, StartedAt: time.Now().UTC()}
	blob, _ := json.MarshalIndent(st, "", "  ")
	if err := os.WriteFile(statePath(), blob, 0o600); err != nil {
		return "", err
	}
	fmt.Fprintf(os.Stderr, "cloudbox: started a local control plane at %s (pid %d, driver %s)\n", url, pid, driver)
	return url, nil
}

func freePort() (int, error) {
	l, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return 0, err
	}
	defer l.Close()
	return l.Addr().(*net.TCPAddr).Port, nil
}

func alive(pid int) bool {
	if pid <= 0 {
		return false
	}
	return syscall.Kill(pid, 0) == nil
}

func stopProcess(pid int) {
	if !alive(pid) {
		return
	}
	_ = syscall.Kill(pid, syscall.SIGTERM)
	for i := 0; i < 50 && alive(pid); i++ {
		time.Sleep(100 * time.Millisecond)
	}
	if alive(pid) {
		_ = syscall.Kill(pid, syscall.SIGKILL)
	}
}

// runServer reports on, or stops, the local control plane.
func runServer(c *client, args []string) int {
	if len(args) == 0 {
		return c.usage("server", "cloudbox server status|stop")
	}
	switch args[0] {
	case "status":
		st, err := readState()
		v := &view{}
		if err != nil || !alive(st.PID) || !healthy(st.Addr) {
			v.add("running", false)
			return c.emit(v)
		}
		v.add("running", true).add("addr", st.Addr).add("pid", st.PID).
			add("build", st.Build).add("driver", st.Driver).add("startedAt", st.StartedAt)
		return c.emit(v)
	case "stop":
		st, err := readState()
		v := &view{}
		if err != nil || !alive(st.PID) {
			v.add("stopped", false).add("reason", "no local control plane is running")
			return c.emit(v)
		}
		stopProcess(st.PID)
		_ = os.Remove(statePath())
		// Evidence lives in the control plane's memory; stopping it ends it.
		v.add("stopped", true).add("pid", st.PID).
			add("note", "evidence held by the local control plane is lost when it stops")
		return c.emit(v)
	}
	return c.usage("server", "cloudbox server status|stop")
}

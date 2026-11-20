// The thin-client transport (CP2): create resources, watch status. No
// enforcement lives here (ADR 0004).
package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/user"
	"time"
)

// Exit statuses distinguish outcomes so a program never parses prose to learn
// what happened (developer-ergonomics).
const (
	exitOK          = 0 // the command did what it was asked
	exitRefused     = 1 // the control plane (or offline check) refused the request
	exitUsage       = 2 // the command line itself was malformed
	exitUnreachable = 3 // no control plane could be reached or started
)

type client struct {
	server string // explicitly addressed control plane; empty means local
	as     string
	output string // "human" | "json"
	// ensure resolves the control plane on first use: the addressed one, or
	// a local one started and reused across commands (control-plane).
	ensure func() (string, error)
}

// failure is a non-success outcome carrying its exit status. A refusal also
// carries the control plane's reason (ADR 0012) and, for not-found, its
// subject; usage and unreachable failures are not refusals and carry neither.
type failure struct {
	exit     int
	status   int
	message  string
	reason   string
	subject  string
	findings any // check's offline report; nil elsewhere
}

func (c *client) base() (string, *failure) {
	if c.server != "" {
		return c.server, nil
	}
	addr, err := c.ensure()
	if err != nil {
		return "", &failure{exit: exitUnreachable, message: err.Error()}
	}
	c.server = addr
	return addr, nil
}

// call performs one request. A response with status >= 400 is a refusal; a
// transport error is an unreachable control plane.
func (c *client) call(method, path string, body any) (map[string]any, *failure) {
	base, f := c.base()
	if f != nil {
		return nil, f
	}
	var reader io.Reader
	if body != nil {
		blob, _ := json.Marshal(body)
		reader = bytes.NewReader(blob)
	}
	req, err := http.NewRequest(method, base+path, reader)
	if err != nil {
		return nil, &failure{exit: exitUsage, message: err.Error()}
	}
	req.Header.Set("X-Cloudbox-User", c.identity())
	httpc := &http.Client{Timeout: 10 * time.Minute} // real-cluster probes are slow
	resp, err := httpc.Do(req)
	if err != nil {
		return nil, &failure{exit: exitUnreachable,
			message: fmt.Sprintf("cannot reach control plane at %s: %v", base, err)}
	}
	defer resp.Body.Close()
	out := map[string]any{}
	raw, _ := io.ReadAll(resp.Body)
	if len(bytes.TrimSpace(raw)) > 0 {
		if err := json.Unmarshal(raw, &out); err != nil {
			out = map[string]any{"body": string(raw)}
		}
	}
	if resp.StatusCode >= 400 {
		msg, _ := out["error"].(string)
		if msg == "" {
			msg = fmt.Sprintf("request refused with status %d", resp.StatusCode)
		}
		reason, _ := out["reason"].(string)
		subject, _ := out["subject"].(string)
		return out, &failure{exit: exitRefused, status: resp.StatusCode, message: msg,
			reason: reason, subject: subject, findings: out["findings"]}
	}
	return out, nil
}

// identity is the acting identity the request asserts. This release
// authenticates no one: the control plane takes it as given.
func (c *client) identity() string {
	if c.as != "" {
		return c.as
	}
	if env := os.Getenv("CLOUDBOX_AS"); env != "" {
		return env
	}
	if u, err := user.Current(); err == nil && u.Username != "" {
		return u.Username
	}
	return "anonymous"
}

// flagValue reads "-x value" style flags from args.
func flagValue(args []string, name string) string {
	for i := 0; i < len(args)-1; i++ {
		if args[i] == name {
			return args[i+1]
		}
	}
	return ""
}

// flagValues reads every occurrence of a repeatable "-x value" flag.
func flagValues(args []string, name string) []string {
	var out []string
	for i := 0; i < len(args)-1; i++ {
		if args[i] == name {
			out = append(out, args[i+1])
		}
	}
	return out
}

// unknownFlag returns the first flag not in known (value flags consume the
// next argument). A flag the command does not define is refused by name, so
// no flag can be mistaken for a capability the command lacks.
func unknownFlag(args []string, valueFlags []string, boolFlags []string) string {
	isValue := map[string]bool{}
	for _, f := range valueFlags {
		isValue[f] = true
	}
	isBool := map[string]bool{}
	for _, f := range boolFlags {
		isBool[f] = true
	}
	for i := 0; i < len(args); i++ {
		a := args[i]
		switch {
		case isValue[a]:
			i++
		case isBool[a]:
		case len(a) > 1 && a[0] == '-':
			return a
		}
	}
	return ""
}

func hasFlag(args []string, name string) bool {
	for _, a := range args {
		if a == name {
			return true
		}
	}
	return false
}

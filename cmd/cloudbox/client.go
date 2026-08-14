// The thin-client transport (CP2): create resources, watch status. No
// enforcement lives here.
package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
)

type client struct {
	server string
	as     string
}

func (c *client) do(method, path string, body any) (map[string]any, int) {
	var reader io.Reader
	if body != nil {
		blob, _ := json.Marshal(body)
		reader = bytes.NewReader(blob)
	}
	req, err := http.NewRequest(method, c.server+path, reader)
	if err != nil {
		fmt.Fprintf(os.Stderr, "cloudbox: %v\n", err)
		os.Exit(2)
	}
	if c.as != "" {
		req.Header.Set("X-Cloudbox-User", c.as)
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		fmt.Fprintf(os.Stderr, "cloudbox: cannot reach control plane at %s: %v\n", c.server, err)
		os.Exit(2)
	}
	defer resp.Body.Close()
	var out map[string]any
	_ = json.NewDecoder(resp.Body).Decode(&out)
	return out, resp.StatusCode
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

func hasFlag(args []string, name string) bool {
	for _, a := range args {
		if a == name {
			return true
		}
	}
	return false
}

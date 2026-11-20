// One surface, two renderings (design D7): every command builds a single view
// of the server's response, and the human and machine-readable renderings are
// both projections of that view. Neither may carry a fact the other lacks.
package main

import (
	"encoding/json"
	"fmt"
	"os"
	"sort"
	"strings"
)

// field is one fact in a view, in display order.
type field struct {
	key   string
	value any
}

// view is what a command reports.
type view struct {
	fields []field
	// human, when set, renders the view for people; otherwise every field is
	// printed as "key: value". Either way the facts come from fields.
	human func(v *view)
}

func (v *view) add(key string, value any) *view {
	v.fields = append(v.fields, field{key, value})
	return v
}

func (v *view) get(key string) any {
	for _, f := range v.fields {
		if f.key == key {
			return f.value
		}
	}
	return nil
}

func (v *view) asMap() map[string]any {
	out := map[string]any{}
	for _, f := range v.fields {
		out[f.key] = f.value
	}
	return out
}

// emit renders a successful view and returns exitOK.
func (c *client) emit(v *view) int {
	if c.output == "json" {
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		_ = enc.Encode(v.asMap())
		return exitOK
	}
	if v.human != nil {
		v.human(v)
		return exitOK
	}
	for _, f := range v.fields {
		fmt.Printf("%s: %s\n", f.key, humanValue(f.value))
	}
	return exitOK
}

// fail renders a non-success outcome and returns its exit status.
func (c *client) fail(command string, f *failure) int {
	if c.output == "json" {
		out := map[string]any{"error": f.message, "outcome": outcomeName(f.exit)}
		if f.status != 0 {
			out["status"] = f.status
		}
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		_ = enc.Encode(out)
		return f.exit
	}
	fmt.Fprintf(os.Stderr, "cloudbox %s: %s\n", command, f.message)
	return f.exit
}

func outcomeName(exit int) string {
	switch exit {
	case exitRefused:
		return "refused"
	case exitUnreachable:
		return "unreachable"
	case exitUsage:
		return "usage"
	}
	return "ok"
}

func (c *client) usage(command, text string) int {
	return c.fail(command, &failure{exit: exitUsage, message: "usage: " + text})
}

// humanValue renders any JSON-shaped value on one line, deterministically.
func humanValue(v any) string {
	switch t := v.(type) {
	case nil:
		return "-"
	case string:
		return t
	case []any:
		parts := make([]string, len(t))
		for i, e := range t {
			parts[i] = humanValue(e)
		}
		return "[" + strings.Join(parts, ", ") + "]"
	case []string:
		return "[" + strings.Join(t, ", ") + "]"
	case map[string]any:
		keys := make([]string, 0, len(t))
		for k := range t {
			keys = append(keys, k)
		}
		sort.Strings(keys)
		parts := make([]string, len(keys))
		for i, k := range keys {
			parts[i] = k + "=" + humanValue(t[k])
		}
		return "{" + strings.Join(parts, " ") + "}"
	default:
		return fmt.Sprint(t)
	}
}

// pick copies the named keys of a server response into a view, in order.
func pick(v *view, from map[string]any, keys ...string) *view {
	for _, k := range keys {
		if val, ok := from[k]; ok {
			v.add(k, val)
		}
	}
	return v
}

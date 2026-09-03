package tools

import (
	"sort"
)

// Spec is the agent-facing description of a registered tool.
type Spec struct {
	Name        string
	Alias       string
	Description string
	Params      []string
	Schema      map[string]any
}

var catalog = []Spec{
	{
		Name:        "http.get",
		Alias:       "GET",
		Description: "HTTP GET; JSON body is parsed when possible",
		Params:      []string{"url"},
		Schema:      objectSchema(map[string]any{"url": strProp("URL")}, []string{"url"}),
	},
	{
		Name:        "http.post",
		Alias:       "POST",
		Description: "HTTP POST JSON body",
		Params:      []string{"url", "body"},
		Schema:      objectSchema(map[string]any{"url": strProp("URL"), "body": map[string]any{"description": "JSON-serializable body"}}, []string{"url", "body"}),
	},
	{
		Name:        "fs.read",
		Alias:       "read",
		Description: "Read a file; .json is parsed",
		Params:      []string{"path"},
		Schema:      objectSchema(map[string]any{"path": strProp("Path")}, []string{"path"}),
	},
	{
		Name:        "fs.write",
		Alias:       "write",
		Description: "Write a file (string or JSON)",
		Params:      []string{"path", "content"},
		Schema:      objectSchema(map[string]any{"path": strProp("Path"), "content": map[string]any{"description": "String or JSON value"}}, []string{"path", "content"}),
	},
	{
		Name:        "fs.list",
		Alias:       "ls",
		Description: "List directory names",
		Params:      []string{"path"},
		Schema:      objectSchema(map[string]any{"path": strProp("Directory path")}, nil),
	},
	{
		Name:        "env.get",
		Alias:       "env",
		Description: "Read an environment variable",
		Params:      []string{"name"},
		Schema:      objectSchema(map[string]any{"name": strProp("Variable name")}, []string{"name"}),
	},
	{
		Name:        "time.now",
		Alias:       "now",
		Description: "Unix timestamp (seconds)",
		Params:      nil,
		Schema:      objectSchema(nil, nil),
	},
	{
		Name:        "time.sleep",
		Alias:       "sleep",
		Description: "Sleep milliseconds",
		Params:      []string{"ms"},
		Schema:      objectSchema(map[string]any{"ms": map[string]any{"type": "integer", "description": "Milliseconds"}}, []string{"ms"}),
	},
}

func strProp(desc string) map[string]any {
	return map[string]any{"type": "string", "description": desc}
}

func objectSchema(props map[string]any, required []string) map[string]any {
	if props == nil {
		props = map[string]any{}
	}
	schema := map[string]any{
		"type":                 "object",
		"properties":           props,
		"additionalProperties": false,
	}
	if len(required) > 0 {
		schema["required"] = required
	}
	return schema
}

func Catalog() []Spec {
	out := make([]Spec, len(catalog))
	copy(out, catalog)
	return out
}

func Names() []string {
	names := make([]string, 0, len(registry))
	for n := range registry {
		names = append(names, n)
	}
	sort.Strings(names)
	return names
}

func Lookup(name string) (Spec, bool) {
	for _, s := range catalog {
		if s.Name == name || s.Alias == name {
			return s, true
		}
	}
	return Spec{}, false
}

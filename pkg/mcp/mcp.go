package mcp

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"neurolang/pkg/object"
	"neurolang/pkg/tools"
)

const protocolVersion = "2024-11-05"

// Version is reported in initialize.serverInfo; set from the CLI.
var Version = "0.11.0"

type rpcRequest struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      json.RawMessage `json:"id"`
	Method  string          `json:"method"`
	Params  json.RawMessage `json:"params"`
}

type rpcError struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}

func Serve(in io.Reader, out io.Writer) error {
	sc := bufio.NewScanner(in)
	sc.Buffer(make([]byte, 0, 64*1024), 8*1024*1024)
	for sc.Scan() {
		line := sc.Bytes()
		if len(line) == 0 {
			continue
		}
		resp, notify := Handle(line)
		if notify || len(resp) == 0 {
			continue
		}
		if _, err := out.Write(append(resp, '\n')); err != nil {
			return err
		}
	}
	return sc.Err()
}

func Handle(line []byte) (resp []byte, notification bool) {
	var req rpcRequest
	if err := json.Unmarshal(line, &req); err != nil {
		return encodeErr(nil, -32700, "parse error"), false
	}
	if req.Method == "" {
		return encodeErr(req.ID, -32600, "invalid request"), false
	}
	if req.ID == nil {
		return nil, true
	}
	switch req.Method {
	case "initialize":
		return encodeOK(req.ID, map[string]any{
			"protocolVersion": protocolVersion,
			"capabilities":    map[string]any{"tools": map[string]any{}},
			"serverInfo":      map[string]any{"name": "neurolang", "version": Version},
			"instructions":    "NeuroLang tools. Prefer !tool.name in generated NL. See neurolang spec / SPEC_AI.md.",
		}), false
	case "ping":
		return encodeOK(req.ID, map[string]any{}), false
	case "tools/list":
		return encodeOK(req.ID, map[string]any{"tools": listTools()}), false
	case "tools/call":
		result, err := callTool(req.Params)
		if err != nil {
			return encodeOK(req.ID, map[string]any{
				"content": []map[string]string{{"type": "text", "text": err.Error()}},
				"isError": true,
			}), false
		}
		return encodeOK(req.ID, result), false
	default:
		return encodeErr(req.ID, -32601, "method not found: "+req.Method), false
	}
}

func listTools() []map[string]any {
	specs := tools.Catalog()
	out := make([]map[string]any, 0, len(specs))
	for _, s := range specs {
		out = append(out, map[string]any{
			"name":        s.Name,
			"description": fmt.Sprintf("%s (!%s) %s", s.Alias, s.Name, s.Description),
			"inputSchema": s.Schema,
		})
	}
	return out
}

func callTool(params json.RawMessage) (map[string]any, error) {
	if len(params) == 0 {
		params = []byte("{}")
	}
	var p struct {
		Name      string `json:"name"`
		Arguments any    `json:"arguments"`
	}
	if err := json.Unmarshal(params, &p); err != nil {
		return nil, fmt.Errorf("invalid params")
	}
	spec, ok := tools.Lookup(p.Name)
	name := p.Name
	if ok {
		name = spec.Name
	}
	args := argsFromJSON(spec, p.Arguments)
	res := tools.Call(name, args, nil)
	if errObj, ok := res.(*object.Error); ok {
		return nil, fmt.Errorf("%s", errObj.Message)
	}
	text := res.Inspect()
	if s, ok := res.(*object.String); ok {
		text = s.Value
	}
	if b, err := json.Marshal(res.ToInterface()); err == nil {
		text = string(b)
	}
	return map[string]any{
		"content": []map[string]string{{"type": "text", "text": text}},
		"isError": false,
	}, nil
}

func jsonToObject(v any) object.Object {
	switch n := v.(type) {
	case float64:
		if n == float64(int64(n)) {
			return &object.Integer{Value: int64(n)}
		}
		return &object.Float{Value: n}
	case json.Number:
		if i, err := n.Int64(); err == nil {
			return &object.Integer{Value: i}
		}
		f, _ := n.Float64()
		return &object.Float{Value: f}
	default:
		return object.FromGoValue(v)
	}
}

func argsFromJSON(spec tools.Spec, raw any) []object.Object {
	if raw == nil {
		return nil
	}
	switch v := raw.(type) {
	case []any:
		out := make([]object.Object, len(v))
		for i, x := range v {
			out[i] = jsonToObject(x)
		}
		return out
	case map[string]any:
		if len(spec.Params) == 0 {
			return nil
		}
		out := make([]object.Object, 0, len(spec.Params))
		for _, k := range spec.Params {
			if val, ok := v[k]; ok {
				out = append(out, jsonToObject(val))
			}
		}
		return out
	default:
		return []object.Object{jsonToObject(v)}
	}
}

func encodeOK(id json.RawMessage, result any) []byte {
	b, _ := json.Marshal(map[string]any{
		"jsonrpc": "2.0",
		"id":      rawID(id),
		"result":  result,
	})
	return b
}

func encodeErr(id json.RawMessage, code int, msg string) []byte {
	b, _ := json.Marshal(map[string]any{
		"jsonrpc": "2.0",
		"id":      rawID(id),
		"error":   rpcError{Code: code, Message: msg},
	})
	return b
}

func rawID(id json.RawMessage) any {
	if len(id) == 0 {
		return nil
	}
	var v any
	if json.Unmarshal(id, &v) != nil {
		return nil
	}
	return v
}

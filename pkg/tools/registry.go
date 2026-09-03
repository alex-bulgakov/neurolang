package tools

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"neurolang/pkg/object"
	"os"
	"path/filepath"
	"time"
)

type ToolHandler func(args []object.Object, env *object.Environment) object.Object

var registry = make(map[string]ToolHandler)

func Register(name string, handler ToolHandler) {
	registry[name] = handler
}

func Call(name string, args []object.Object, env *object.Environment) object.Object {
	if handler, ok := registry[name]; ok {
		return handler(args, env)
	}
	return &object.Error{Message: fmt.Sprintf("tool '%s' not registered or unknown", name)}
}

func init() {
	// Register built-in agent tools

	// !http.get(url)
	Register("http.get", func(args []object.Object, env *object.Environment) object.Object {
		if len(args) < 1 {
			return &object.Error{Message: "!http.get expects at least (url)"}
		}
		urlStr := args[0].Inspect()
		if s, ok := args[0].(*object.String); ok {
			urlStr = s.Value
		}

		resp, err := http.Get(urlStr)
		if err != nil {
			return &object.Error{Message: fmt.Sprintf("HTTP GET error: %s", err.Error())}
		}
		defer resp.Body.Close()

		bodyBytes, err := io.ReadAll(resp.Body)
		if err != nil {
			return &object.Error{Message: fmt.Sprintf("failed to read response: %s", err.Error())}
		}

		// Try auto-parsing JSON
		var parsed any
		if err := json.Unmarshal(bodyBytes, &parsed); err == nil {
			return object.FromGoValue(parsed)
		}

		return &object.String{Value: string(bodyBytes)}
	})

	// !http.post(url, body)
	Register("http.post", func(args []object.Object, env *object.Environment) object.Object {
		if len(args) < 2 {
			return &object.Error{Message: "!http.post expects (url, body)"}
		}
		urlStr := args[0].Inspect()
		if s, ok := args[0].(*object.String); ok {
			urlStr = s.Value
		}

		bodyData, err := json.Marshal(args[1].ToInterface())
		if err != nil {
			return &object.Error{Message: fmt.Sprintf("failed to encode body: %s", err.Error())}
		}

		resp, err := http.Post(urlStr, "application/json", bytes.NewBuffer(bodyData))
		if err != nil {
			return &object.Error{Message: fmt.Sprintf("HTTP POST error: %s", err.Error())}
		}
		defer resp.Body.Close()

		bodyBytes, err := io.ReadAll(resp.Body)
		if err != nil {
			return &object.Error{Message: fmt.Sprintf("failed to read response: %s", err.Error())}
		}

		var parsed any
		if err := json.Unmarshal(bodyBytes, &parsed); err == nil {
			return object.FromGoValue(parsed)
		}
		return &object.String{Value: string(bodyBytes)}
	})

	// !fs.read(path)
	Register("fs.read", func(args []object.Object, env *object.Environment) object.Object {
		if len(args) != 1 {
			return &object.Error{Message: "!fs.read expects (path)"}
		}
		path := args[0].Inspect()
		if s, ok := args[0].(*object.String); ok {
			path = s.Value
		}

		content, err := os.ReadFile(path)
		if err != nil {
			return &object.Error{Message: fmt.Sprintf("fs.read error: %s", err.Error())}
		}

		// If ends with .json, parse automatically
		if filepath.Ext(path) == ".json" {
			var parsed any
			if err := json.Unmarshal(content, &parsed); err == nil {
				return object.FromGoValue(parsed)
			}
		}

		return &object.String{Value: string(content)}
	})

	// !fs.write(path, content)
	Register("fs.write", func(args []object.Object, env *object.Environment) object.Object {
		if len(args) != 2 {
			return &object.Error{Message: "!fs.write expects (path, content)"}
		}
		path := args[0].Inspect()
		if s, ok := args[0].(*object.String); ok {
			path = s.Value
		}

		var data []byte
		if s, ok := args[1].(*object.String); ok {
			data = []byte(s.Value)
		} else {
			marshaled, err := json.MarshalIndent(args[1].ToInterface(), "", "  ")
			if err != nil {
				return &object.Error{Message: fmt.Sprintf("failed to marshal content: %s", err.Error())}
			}
			data = marshaled
		}

		err := os.WriteFile(path, data, 0644)
		if err != nil {
			return &object.Error{Message: fmt.Sprintf("fs.write error: %s", err.Error())}
		}
		return &object.Boolean{Value: true}
	})

	// !fs.list(path)
	Register("fs.list", func(args []object.Object, env *object.Environment) object.Object {
		path := "."
		if len(args) > 0 {
			if s, ok := args[0].(*object.String); ok {
				path = s.Value
			}
		}

		entries, err := os.ReadDir(path)
		if err != nil {
			return &object.Error{Message: fmt.Sprintf("fs.list error: %s", err.Error())}
		}

		elements := make([]object.Object, len(entries))
		for i, entry := range entries {
			elements[i] = &object.String{Value: entry.Name()}
		}
		return &object.List{Elements: elements}
	})

	// !env.get(varName)
	Register("env.get", func(args []object.Object, env *object.Environment) object.Object {
		if len(args) != 1 {
			return &object.Error{Message: "!env.get expects (name)"}
		}
		name := args[0].Inspect()
		if s, ok := args[0].(*object.String); ok {
			name = s.Value
		}
		val := os.Getenv(name)
		return &object.String{Value: val}
	})

	// !time.now()
	Register("time.now", func(args []object.Object, env *object.Environment) object.Object {
		return &object.Integer{Value: time.Now().Unix()}
	})

	// !time.sleep(ms)
	Register("time.sleep", func(args []object.Object, env *object.Environment) object.Object {
		if len(args) != 1 {
			return &object.Error{Message: "!time.sleep expects (milliseconds)"}
		}
		ms, ok := args[0].(*object.Integer)
		if !ok {
			return &object.Error{Message: "!time.sleep expects INTEGER"}
		}
		time.Sleep(time.Duration(ms.Value) * time.Millisecond)
		return &object.Null{}
	})
}

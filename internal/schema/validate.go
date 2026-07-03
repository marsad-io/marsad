package schema

import (
	"fmt"
	"sort"
)

// Validate checks tool call arguments against a spec: no unknown keys, all
// required keys present, values of the declared type. It returns the first
// problem found, with unknown keys reported in stable order.
func Validate(spec ToolSpec, args map[string]any) error {
	var unknown []string
	for key := range args {
		if _, ok := spec.Input.Properties[key]; !ok {
			unknown = append(unknown, key)
		}
	}
	if len(unknown) > 0 {
		sort.Strings(unknown)
		return fmt.Errorf("tool %s: unknown argument %q", spec.Name, unknown[0])
	}

	for _, req := range spec.Input.Required {
		if _, ok := args[req]; !ok {
			return fmt.Errorf("tool %s: missing required argument %q", spec.Name, req)
		}
	}

	for key, value := range args {
		prop := spec.Input.Properties[key]
		if err := checkType(key, prop.Type, value); err != nil {
			return fmt.Errorf("tool %s: %w", spec.Name, err)
		}
	}
	return nil
}

func checkType(key, want string, value any) error {
	ok := true
	switch want {
	case "string":
		_, ok = value.(string)
	case "number":
		switch value.(type) {
		case float64, int, int64:
		default:
			ok = false
		}
	case "boolean":
		_, ok = value.(bool)
	}
	if !ok {
		return fmt.Errorf("argument %q must be a %s, got %T", key, want, value)
	}
	return nil
}

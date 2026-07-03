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
		if err := checkEnum(key, prop.Enum, value); err != nil {
			return fmt.Errorf("tool %s: %w", spec.Name, err)
		}
		if err := checkBounds(key, prop, value); err != nil {
			return fmt.Errorf("tool %s: %w", spec.Name, err)
		}
	}
	return nil
}

// checkEnum rejects values outside a property's declared enum.
func checkEnum(key string, allowed []string, value any) error {
	if len(allowed) == 0 {
		return nil
	}
	s, ok := value.(string)
	if !ok {
		return nil // type mismatch already reported by checkType
	}
	for _, a := range allowed {
		if s == a {
			return nil
		}
	}
	return fmt.Errorf("argument %q must be one of %v, got %q", key, allowed, s)
}

// checkBounds enforces a numeric property's declared minimum and maximum.
func checkBounds(key string, prop Property, value any) error {
	if prop.Minimum == nil && prop.Maximum == nil {
		return nil
	}
	var n float64
	switch v := value.(type) {
	case float64:
		n = v
	case int:
		n = float64(v)
	case int64:
		n = float64(v)
	default:
		return nil // type mismatch already reported by checkType
	}
	if prop.Minimum != nil && n < *prop.Minimum {
		return fmt.Errorf("argument %q must be at least %g, got %g", key, *prop.Minimum, n)
	}
	if prop.Maximum != nil && n > *prop.Maximum {
		return fmt.Errorf("argument %q must be at most %g, got %g", key, *prop.Maximum, n)
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

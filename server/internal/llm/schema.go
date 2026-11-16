package llm

import (
	"fmt"
	"math"
	"sort"
)

// ValidateJSON prueft einen dekodierten JSON-Wert gegen eine Teilmenge von
// JSON Schema: "type" (object, array, string, number, integer, boolean,
// null), "properties", "required", "additionalProperties": false, "items",
// "enum", "minimum", "maximum". Andere Schluesselwoerter werden ignoriert.
// Mehr braucht die Pruefung von Modellantworten nicht; die Anbieter
// bekommen dasselbe Schema als Vorgabe.
func ValidateJSON(v any, schema map[string]any) error {
	return validateAt("$", v, schema)
}

func validateAt(path string, v any, schema map[string]any) error {
	if t, ok := schema["type"].(string); ok {
		if !hasType(v, t) {
			return fmt.Errorf("%s: erwartet %s", path, t)
		}
	}
	if enum, ok := schema["enum"].([]any); ok {
		found := false
		for _, e := range enum {
			if e == v {
				found = true
				break
			}
		}
		if !found {
			return fmt.Errorf("%s: Wert nicht erlaubt", path)
		}
	}
	if n, ok := v.(float64); ok {
		if min, ok := schema["minimum"].(float64); ok && n < min {
			return fmt.Errorf("%s: kleiner als %v", path, min)
		}
		if max, ok := schema["maximum"].(float64); ok && n > max {
			return fmt.Errorf("%s: größer als %v", path, max)
		}
	}
	switch val := v.(type) {
	case map[string]any:
		props, _ := schema["properties"].(map[string]any)
		if req, ok := schema["required"].([]any); ok {
			for _, r := range req {
				key, _ := r.(string)
				if _, present := val[key]; !present {
					return fmt.Errorf("%s: Feld %q fehlt", path, key)
				}
			}
		}
		keys := make([]string, 0, len(val))
		for k := range val {
			keys = append(keys, k)
		}
		sort.Strings(keys)
		for _, k := range keys {
			sub, known := props[k].(map[string]any)
			if !known {
				if extra, ok := schema["additionalProperties"].(bool); ok && !extra {
					return fmt.Errorf("%s: Feld %q nicht erlaubt", path, k)
				}
				continue
			}
			if err := validateAt(path+"."+k, val[k], sub); err != nil {
				return err
			}
		}
	case []any:
		if items, ok := schema["items"].(map[string]any); ok {
			for i, e := range val {
				if err := validateAt(fmt.Sprintf("%s[%d]", path, i), e, items); err != nil {
					return err
				}
			}
		}
	}
	return nil
}

func hasType(v any, t string) bool {
	switch t {
	case "object":
		_, ok := v.(map[string]any)
		return ok
	case "array":
		_, ok := v.([]any)
		return ok
	case "string":
		_, ok := v.(string)
		return ok
	case "number":
		_, ok := v.(float64)
		return ok
	case "integer":
		n, ok := v.(float64)
		return ok && n == math.Trunc(n)
	case "boolean":
		_, ok := v.(bool)
		return ok
	case "null":
		return v == nil
	}
	return true
}
